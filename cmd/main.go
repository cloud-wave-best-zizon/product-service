package main

import (
	"context"
	"crypto/tls"  // 추가
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"  // 추가
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/cloud-wave-best-zizon/product-service/internal/events"
	"github.com/cloud-wave-best-zizon/product-service/internal/handler"
	"github.com/cloud-wave-best-zizon/product-service/internal/repository"
	"github.com/cloud-wave-best-zizon/product-service/internal/service"
	"github.com/cloud-wave-best-zizon/product-service/pkg/config"
	"github.com/cloud-wave-best-zizon/product-service/pkg/middleware"
	pkgtls "github.com/cloud-wave-best-zizon/product-service/pkg/tls"  // 수정: product-service로!
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"  // 추가
	"go.uber.org/zap"
)

func main() {
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

	// TLS 설정 로드 (여기로 이동!)
	tlsConfig := &pkgtls.TLSConfig{}
	if err := envconfig.Process("", tlsConfig); err != nil {
		logger.Fatal("Failed to load TLS config", zap.Error(err))
	}

	logger.Info("Service configuration", 
		zap.String("port", cfg.Port),
		zap.String("kafka_brokers", cfg.KafkaBrokers),
		zap.Bool("kafka_enabled", cfg.KafkaEnabled),
		zap.String("dynamodb_endpoint", cfg.DynamoDBEndpoint),
		zap.Bool("tls_enabled", tlsConfig.Enabled))

	// 모드 확인
	if cfg.LocalMode && cfg.DynamoDBEndpoint == "" {
		logger.Info("Running in LOCAL MODE - using in-memory storage")
	} else if cfg.DynamoDBEndpoint != "" {
		logger.Info("Running with DynamoDB Local", zap.String("endpoint", cfg.DynamoDBEndpoint))
	} else {
		logger.Info("Running in AWS MODE")
	}

	// DynamoDB 클라이언트 초기화
	dynamoClient, err := repository.NewDynamoDBClient(cfg)
	if err != nil {
		log.Fatal("Failed to create DynamoDB client:", err)
	}

	// Repository, Service, Handler 초기화
	productRepo := repository.NewProductRepository(dynamoClient, cfg.ProductTableName)

	// DynamoDB Local 사용 시 테이블 생성
	if dynamoClient != nil {
		if err := productRepo.CreateTableIfNotExists(context.Background()); err != nil {
			logger.Error("Failed to create table", zap.Error(err))
			// 테이블이 이미 존재하는 경우는 무시
		} else {
			logger.Info("Table checked/created successfully")
		}
	}

	productService := service.NewProductService(productRepo, logger)
	productHandler := handler.NewProductHandler(productService, logger)

	// Kafka Consumer 초기화 및 시작
	var kafkaConsumer *events.KafkaConsumer
	if cfg.KafkaEnabled {
		kafkaConsumer = events.NewKafkaConsumer(
			cfg.KafkaBrokers,
			productService,
			logger,
		)
		defer kafkaConsumer.Close()
		
		// Consumer 시작 (고루틴)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		
		go kafkaConsumer.StartConsuming(ctx)
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
			status := gin.H{
				"status": "healthy",
				"mode":   "production",
				"tls":    tlsConfig.Enabled,
			}
			if cfg.LocalMode && cfg.DynamoDBEndpoint == "" {
				status["mode"] = "local"
				status["storage"] = "in-memory"
			} else if cfg.DynamoDBEndpoint != "" {
				status["mode"] = "local"
				status["storage"] = "dynamodb-local"
				status["endpoint"] = cfg.DynamoDBEndpoint
			} else {
				status["mode"] = "aws"
				status["storage"] = "dynamodb"
				status["table"] = cfg.ProductTableName
			}

			// Kafka 상태 확인
			if cfg.KafkaEnabled && kafkaConsumer != nil {
				if err := kafkaConsumer.HealthCheck(); err != nil {
					status["kafka"] = "unhealthy"
					status["kafka_error"] = err.Error()
				} else {
					status["kafka"] = "healthy"
				}
			} else {
				status["kafka"] = "disabled"
			}

			c.JSON(200, status)
		})
		
		// 메트릭스 엔드포인트 (선택적)
		v1.GET("/metrics", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"kafka_brokers": cfg.KafkaBrokers,
				"kafka_group_id": cfg.KafkaGroupID,
				"kafka_enabled": cfg.KafkaEnabled,
			})
		})
	}

	// Server 설정 (한 번만!)
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	// TLS 설정 적용
	var tlsConfigMutex sync.RWMutex
	if tlsConfig.Enabled {
		tlsCfg, err := pkgtls.LoadTLSConfig(tlsConfig, logger)
		if err != nil {
			logger.Fatal("Failed to load TLS configuration", zap.Error(err))
		}
		srv.TLSConfig = tlsCfg

		// 인증서 자동 리로드
		go pkgtls.WatchCertificates(tlsConfig, func(newCfg *tls.Config) error {
			tlsConfigMutex.Lock()
			defer tlsConfigMutex.Unlock()
			srv.TLSConfig = newCfg
			return nil
		}, logger)
	}

	// Server 시작 (한 번만!)
	go func() {
		logger.Info("Starting server",
			zap.String("port", cfg.Port),
			zap.Bool("local_mode", cfg.LocalMode),
			zap.Bool("kafka_enabled", cfg.KafkaEnabled),
			zap.Bool("tls_enabled", tlsConfig.Enabled))

		var err error
		if tlsConfig.Enabled {
			err = srv.ListenAndServeTLS("", "") // 인증서는 TLSConfig에서 로드
		} else {
			err = srv.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}
	logger.Info("Server exited")
}