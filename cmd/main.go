package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "sync"
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

var (
    globalX509Source *workloadapi.X509Source
    sourceMutex sync.RWMutex
)

func main() {
    // .env 파일 로드
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using environment variables")
    }
    
    // Logger 초기화
    logger, _ := zap.NewProduction()
    defer logger.Sync()

    // Config 로드
    cfg, err := config.Load()
    if err != nil {
        log.Fatal("Failed to load config:", err)
    }

    logger.Info("Service configuration",
        zap.String("port", cfg.Port),
        zap.Bool("kafka_enabled", cfg.KafkaEnabled),
        zap.Bool("tls_enabled", cfg.TLSEnabled))

    // DynamoDB 클라이언트 초기화
    dynamoClient, err := repository.NewDynamoDBClient(cfg)
    if err != nil {
        log.Fatal("Failed to create DynamoDB client:", err)
    }

    // Repository, Service, Handler 초기화
    productRepo := repository.NewProductRepository(dynamoClient, cfg.ProductTableName)
    productService := service.NewProductService(productRepo, logger)
    productHandler := handler.NewProductHandler(productService, logger)

    // Kafka Consumer 초기화
    var kafkaConsumer *events.KafkaConsumer
    var consumerCtx context.Context
    var consumerCancel context.CancelFunc
    
    if cfg.KafkaEnabled {
        kafkaConsumer = events.NewKafkaConsumer(
            cfg.KafkaBrokers,
            productService,
            logger,
        )
        defer kafkaConsumer.Close()
        
        consumerCtx, consumerCancel = context.WithCancel(context.Background())
        defer consumerCancel()
        
        go kafkaConsumer.StartConsuming(consumerCtx)
        logger.Info("Kafka consumer started")
    }

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
            sourceMutex.RLock()
            source := globalX509Source
            sourceMutex.RUnlock()
            
            status := gin.H{
                "status": "healthy",
                "service": "product-service",
                "tls": cfg.TLSEnabled,
                "kafka": cfg.KafkaEnabled,
            }
            
            if source != nil {
                status["spiffe"] = "connected"
            } else {
                status["spiffe"] = "disconnected"
            }
            
            c.JSON(200, status)
        })
    }

    // HTTP Server 생성
    srv := &http.Server{
        Addr:    ":" + cfg.Port,
        Handler: router,
    }

    // SPIFFE 비동기 초기화
    if cfg.TLSEnabled {
        go initializeAndMonitorSPIFFE(logger, srv)
    }

    // 서버 시작
    go func() {
        for {
            time.Sleep(2 * time.Second)
            
            sourceMutex.RLock()
            source := globalX509Source
            sourceMutex.RUnlock()
            
            if cfg.TLSEnabled && source != nil {
                // SPIFFE 준비됨 - HTTPS로 시작
                tlsCfg := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
                srv.TLSConfig = tlsCfg
                logger.Info("Starting HTTPS with mTLS on port " + cfg.Port)
                
                if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
                    logger.Error("HTTPS server error", zap.Error(err))
                }
                break
            } else if !cfg.TLSEnabled {
                // TLS 비활성화 - HTTP로 시작
                logger.Info("Starting HTTP on port " + cfg.Port)
                
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                    logger.Error("HTTP server error", zap.Error(err))
                }
                break
            }
            
            logger.Info("Waiting for SPIFFE to be ready...")
        }
    }()

    // Graceful Shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Shutting down server...")
    
    // Kafka Consumer 종료
    if kafkaConsumer != nil {
        kafkaConsumer.Stop()
    }
    if consumerCancel != nil {
        consumerCancel()
    }

    // HTTP Server 종료
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatal("Server forced to shutdown", zap.Error(err))
    }
    
    logger.Info("Server exited")
}

func initializeAndMonitorSPIFFE(logger *zap.Logger, srv *http.Server) {
    for {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        source, err := workloadapi.NewX509Source(
            ctx,
            workloadapi.WithClientOptions(
                workloadapi.WithAddr("unix:///run/spire/sockets/agent.sock"),
            ),
        )
        cancel()

        if err != nil {
            logger.Warn("Waiting for SPIFFE", zap.Error(err))
            time.Sleep(5 * time.Second)
            continue
        }

        sourceMutex.Lock()
        globalX509Source = source
        sourceMutex.Unlock()

        logger.Info("SPIFFE initialized with 5-minute certificates")
        
        // 인증서 만료 모니터링
        go monitorCertificateExpiry(source, logger)
        break
    }
}

func monitorCertificateExpiry(source *workloadapi.X509Source, logger *zap.Logger) {
    for {
        svid, err := source.GetX509SVID()
        if err != nil {
            logger.Error("Failed to get SVID", zap.Error(err))
            time.Sleep(30 * time.Second)
            continue
        }
        
        if svid != nil && len(svid.Certificates) > 0 {
            expiry := svid.Certificates[0].NotAfter
            remaining := time.Until(expiry)
            
            logger.Info("Certificate status",
                zap.Duration("remaining", remaining),
                zap.Time("expires_at", expiry))
            
            if remaining < 30*time.Second {
                logger.Error("Certificate about to expire! All communications will fail!")
            }
        }
        
        time.Sleep(30 * time.Second)
    }
}