package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/cloud-wave-best-zizon/product-service/internal/events"
    "github.com/cloud-wave-best-zizon/product-service/internal/handler"
    "github.com/cloud-wave-best-zizon/product-service/internal/repository"
    "github.com/cloud-wave-best-zizon/product-service/internal/service"
    "github.com/cloud-wave-best-zizon/product-service/pkg/config"
    "github.com/cloud-wave-best-zizon/product-service/pkg/middleware"
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "go.uber.org/zap"
)

func main() {
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using environment variables")
    }
    
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }

    // DynamoDB 클라이언트 초기화
    dynamoClient, err := repository.NewDynamoDBClient(cfg)
    if err != nil {
        log.Fatal("Failed to create DynamoDB client:", err)
    }

    productRepo := repository.NewProductRepository(dynamoClient, cfg.ProductTableName)
    productService := service.NewProductService(productRepo, logger)
    productHandler := handler.NewProductHandler(productService, logger)

    // Kafka Consumer
    var kafkaConsumer *events.KafkaConsumer
    if cfg.KafkaEnabled {
        kafkaConsumer = events.NewKafkaConsumer(
            cfg.KafkaBrokers,
            productService,
            logger,
        )
        defer kafkaConsumer.Close()
        
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()
        
        go kafkaConsumer.StartConsuming(ctx)
        logger.Info("Kafka consumer started")
    }

    // Gin Router
    router := gin.New()
    router.Use(gin.Recovery())
    router.Use(middleware.Logger(logger))
    router.Use(middleware.RequestID())

    v1 := router.Group("/api/v1")
    {
        v1.POST("/products", productHandler.CreateProduct)
        v1.GET("/products/:id", productHandler.GetProduct)
        v1.POST("/products/:id/deduct", productHandler.DeductStock)
        v1.GET("/health", func(c *gin.Context) {
            c.JSON(200, gin.H{
                "status": "healthy",
                "service": "product-service",
                "tls": os.Getenv("TLS_ENABLED") == "true",
            })
        })
    }

    srv := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: router,
    }

    // SPIFFE/SPIRE TLS 설정 (Order Service와 동일한 방식)
    if os.Getenv("TLS_ENABLED") == "true" {
        ctx := context.Background()
        source, err := workloadapi.NewX509Source(
            ctx,
            workloadapi.WithClientOptions(
                workloadapi.WithAddr("unix:///run/spire/sockets/agent.sock"),
            ),
        )
        if err != nil {
            logger.Warn("Failed to create X509Source, falling back to HTTP", zap.Error(err))
        } else {
            defer source.Close()
            
            tlsCfg := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
            srv.TLSConfig = tlsCfg
            logger.Info("SPIFFE/SPIRE TLS configured")
        }
    }

    // Server 시작
    go func() {
        logger.Info("Starting server",
            zap.String("port", cfg.Port),
            zap.Bool("tls_enabled", os.Getenv("TLS_ENABLED") == "true"))

        var err error
        if os.Getenv("TLS_ENABLED") == "true" && srv.TLSConfig != nil {
            err = srv.ListenAndServeTLS("", "")
        } else {
            err = srv.ListenAndServe()
        }

        if err != nil && err != http.ErrServerClosed {
            logger.Fatal("Failed to start server", zap.Error(err))
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")
    if kafkaConsumer != nil {
        kafkaConsumer.Stop()
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatal("Server forced to shutdown", zap.Error(err))
    }
    logger.Info("Server exited")
}