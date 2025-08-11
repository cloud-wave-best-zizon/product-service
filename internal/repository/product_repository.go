package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/cloud-wave-best-zizon/product-service/internal/handler"
    "github.com/cloud-wave-best-zizon/product-service/internal/repository"
    "github.com/cloud-wave-best-zizon/product-service/internal/service"
    "github.com/cloud-wave-best-zizon/product-service/pkg/config"
    "github.com/cloud-wave-best-zizon/product-service/pkg/middleware"
    "go.uber.org/zap"
)

func main() {
    // Logger 초기화
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Config 로드
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // 로컬 모드 확인
    if cfg.LocalMode {
        logger.Info("Running in LOCAL MODE - no AWS connection required")
    }

    // DynamoDB 클라이언트 초기화 (로컬 모드에서는 nil 반환)
    dynamoClient, err := repository.NewDynamoDBClient(cfg)
    if err != nil {
        logger.Error("Failed to create DynamoDB client", zap.Error(err))
        if !cfg.LocalMode {
            log.Fatal("Cannot run without AWS in production mode")
        }
    }

    // Repository, Service, Handler 초기화
    productRepo := repository.NewProductRepository(dynamoClient, cfg.ProductTableName)
    productService := service.NewProductService(productRepo, logger)
    productHandler := handler.NewProductHandler(productService, logger)

    // Gin Router 설정
    router := gin.New()
    router.Use(gin.Recovery())
    router.Use(middleware.Logger(logger))
    router.Use(middleware.RequestID())

    // Routes
    v1 := router.Group("/api/v1")
    {
        v1.POST("/products", productHandler.CreateProduct)
        v1.GET("/products/:id", productHandler.GetProduct)
        v1.POST("/products/:id/deduct", productHandler.DeductStock)
        v1.GET("/health", func(c *gin.Context) {
            status := gin.H{
                "status": "healthy",
                "mode":   "production",
            }
            if cfg.LocalMode {
                status["mode"] = "local"
                status["storage"] = "in-memory"
            }
            c.JSON(200, status)
        })
    }

    // Server 시작
    srv := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: router,
    }

    go func() {
        logger.Info("Starting server", 
            zap.String("port", cfg.Port),
            zap.Bool("local_mode", cfg.LocalMode))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("Failed to start server", zap.Error(err))
        }
    }()

    // Graceful Shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatal("Server forced to shutdown", zap.Error(err))
    }
    logger.Info("Server exited")
}