package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloud-wave-best-zizon/product-service/internal/service"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.uber.org/zap"
)

// 보상(컴펜세이션) 이벤트 발행용 인터페이스
type CompensationProducer interface {
		// 재고 차감 실패 보상 이벤트 발행
		PublishStockDeductionFailed(orderID int, productID string, qty int, reason string) error
}
	
// OrderCreatedEvent는 order-service에서 발행하는 이벤트 구조체
type OrderCreatedEvent struct {
	EventID     string      `json:"event_id"`
	OrderID     int         `json:"order_id"`
	UserID      string      `json:"user_id"`
	TotalAmount float64     `json:"total_amount"`
	Items       []OrderItem `json:"items"`
	Status      string      `json:"status"`
	Timestamp   time.Time   `json:"timestamp"`
	RequestID   string      `json:"request_id"`
}

type OrderItem struct {
	ProductID   int     `json:"product_id"`   // order-service에서는 int
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
}

type KafkaConsumer struct {
	consumer       *kafka.Consumer
	productService *service.ProductService
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	compensationProducer CompensationProducer

}

func NewKafkaConsumer(brokers string, groupID string, productService *service.ProductService, logger *zap.Logger) (*KafkaConsumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":        brokers,
		"group.id":                 groupID,
		"auto.offset.reset":        "earliest",
		"enable.auto.commit":       false,
		"session.timeout.ms":       6000,
		"max.poll.interval.ms":     300000,
		"heartbeat.interval.ms":    3000,
		"fetch.min.bytes":          1,
		"fetch.wait.max.ms":        500,
		"enable.partition.eof":     false,
		"api.version.request":      true,
		"go.delivery.reports":      false,
		"go.events.channel.enable": true,
	}

	consumer, err := kafka.NewConsumer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &KafkaConsumer{
		consumer:       consumer,
		productService: productService,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

func (kc *KafkaConsumer) Start() error {
	topics := []string{"order-events"}
	
	err := kc.consumer.SubscribeTopics(topics, nil)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	kc.logger.Info("Kafka consumer started", zap.Strings("topics", topics))

	go kc.consume()
	return nil
}

func (kc *KafkaConsumer) consume() {
	defer kc.consumer.Close()

	for {
		select {
		case <-kc.ctx.Done():
			kc.logger.Info("Kafka consumer stopped")
			return
		default:
			msg, err := kc.consumer.ReadMessage(1000 * time.Millisecond)
			if err != nil {
				if err.(kafka.Error).Code() == kafka.ErrTimedOut {
					continue
				}
				kc.logger.Error("Error reading message", zap.Error(err))
				continue
			}

			if err := kc.processMessage(msg); err != nil {
				kc.logger.Error("Error processing message",
					zap.Error(err),
					zap.String("topic", *msg.TopicPartition.Topic),
					zap.Int32("partition", msg.TopicPartition.Partition),
					zap.Int64("offset", int64(msg.TopicPartition.Offset)))
				continue
			}

			// 메시지 처리 성공 시 커밋
			_, err = kc.consumer.CommitMessage(msg)
			if err != nil {
				kc.logger.Error("Error committing message", zap.Error(err))
			}
		}
	}
}

func (kc *KafkaConsumer) processMessage(msg *kafka.Message) error {
	kc.logger.Info("Processing message",
		zap.String("topic", *msg.TopicPartition.Topic),
		zap.String("key", string(msg.Key)),
		zap.Int64("offset", int64(msg.TopicPartition.Offset)))

	var event OrderCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// 주문 생성 이벤트 처리
	return kc.handleOrderCreatedEvent(event)
}

func (kc *KafkaConsumer) handleOrderCreatedEvent(event OrderCreatedEvent) error {
	ctx := context.Background()
	
	kc.logger.Info("Processing order created event",
		zap.Int("order_id", event.OrderID),
		zap.String("user_id", event.UserID),
		zap.Int("items_count", len(event.Items)))

	// 각 주문 아이템에 대해 재고 차감
	for _, item := range event.Items {
		productID := fmt.Sprintf("PROD%03d", item.ProductID)
		
		kc.logger.Info("Deducting stock",
			zap.String("product_id", productID),
			zap.Int("quantity", item.Quantity))

		result, err := kc.productService.DeductStock(ctx, productID, item.Quantity)
		if err != nil {
			kc.logger.Error("Failed to deduct stock",
				zap.String("product_id", productID),
				zap.Int("quantity", item.Quantity),
				zap.Int("order_id", event.OrderID),
				zap.Error(err))

			// 보상 트랜잭션 이벤트 발행
			if kc.compensationProducer != nil {
				reason := "stock_insufficient"
				if err.Error() == "product not found" {
					reason = "product_not_found"
				}
				
				compensationErr := kc.compensationProducer.PublishStockDeductionFailed(
					event.OrderID, productID, item.Quantity, reason)
				if compensationErr != nil {
					kc.logger.Error("Failed to publish compensation event", 
						zap.Error(compensationErr))
				}
			}

			return fmt.Errorf("stock deduction failed for product %s: %w", productID, err)
		}

		kc.logger.Info("Stock deducted successfully",
			zap.String("product_id", productID),
			zap.Int("previous_stock", result.PreviousStock),
			zap.Int("new_stock", result.NewStock),
			zap.Int("deducted", result.Deducted),
			zap.Int("order_id", event.OrderID))
	}

	kc.logger.Info("Order processing completed",
		zap.Int("order_id", event.OrderID),
		zap.String("request_id", event.RequestID))

	return nil
}

func (kc *KafkaConsumer) Stop() {
	kc.logger.Info("Stopping Kafka consumer")
	kc.cancel()
}

// 런타임에 보상 프로듀서 주입
func (kc *KafkaConsumer) SetCompensationProducer(p CompensationProducer) {
		kc.compensationProducer = p
}


// HealthCheck는 Kafka consumer의 상태를 확인합니다
func (kc *KafkaConsumer) HealthCheck() error {
	metadata, err := kc.consumer.GetMetadata(nil, false, 5000)
	if err != nil {
		return fmt.Errorf("kafka consumer health check failed: %w", err)
	}

	if len(metadata.Brokers) == 0 {
		return fmt.Errorf("no kafka brokers available")
	}

	return nil
}

