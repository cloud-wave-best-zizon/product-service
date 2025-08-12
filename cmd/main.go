package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloud-wave-best-zizon/product-service/internal/events"
	"github.com/cloud-wave-best-zizon/product-service/internal/handler"
	"github.com/cloud-wave-best-zizon/product-service/internal/repository"
	"github.com/cloud-wave-best-zizon/product-service/internal/service"
	"github.com/cloud-wave-best-zizon/product-service/pkg/config"
	"github.com/cloud-wave-best-zizon/product-service/pkg/middleware"
	"github.com/gin-gonic/gin"
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
		kafkaConsumer, err = events.NewKafkaConsumer(
			cfg.KafkaBrokers,
			cfg.KafkaGroupID,
			productService,
			logger,
		)
		if err != nil {
			logger.Error("Failed to create Kafka consumer", zap.Error(err))
			// Kafka 오류는 치명적이지 않게 처리 (선택적)
		} else {
			if err := kafkaConsumer.Start(); err != nil {
				logger.Error("Failed to start Kafka consumer", zap.Error(err))
			} else {
				logger.Info("Kafka consumer started successfully")
			}
		}
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

	// Server 시작
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("Starting server",
			zap.String("port", cfg.Port),
			zap.Bool("local_mode", cfg.LocalMode),
			zap.Bool("kafka_enabled", cfg.KafkaEnabled))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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