package events

import (
    "context"
    "encoding/json"
    "fmt"
    
    "github.com/segmentio/kafka-go"
    "github.com/cloud-wave-best-zizon/product-service/internal/service"
    "go.uber.org/zap"
)

type KafkaConsumer struct {
    reader         *kafka.Reader
    productService *service.ProductService
    logger         *zap.Logger
    cancel         context.CancelFunc
    ctx            context.Context
}

func NewKafkaConsumer(brokers string, productService *service.ProductService, logger *zap.Logger) *KafkaConsumer {
    reader := kafka.NewReader(kafka.ReaderConfig{
        Brokers:     []string{brokers},
        Topic:       "order-events",
        GroupID:     "product-service-consumer",
        MinBytes:    10e3,
        MaxBytes:    10e6,
        StartOffset: kafka.FirstOffset,
    })
    
    ctx, cancel := context.WithCancel(context.Background())
    
    return &KafkaConsumer{
        reader:         reader,
        productService: productService,
        logger:         logger,
        ctx:            ctx,
        cancel:         cancel,
    }
}

// HealthCheck 메서드 추가
func (c *KafkaConsumer) HealthCheck() error {
    if c.reader == nil {
        return fmt.Errorf("kafka reader not initialized")
    }
    return nil
}

// Stop 메서드 추가
func (c *KafkaConsumer) Stop() {
    c.logger.Info("Stopping Kafka consumer")
    if c.cancel != nil {
        c.cancel()
    }
}

func (c *KafkaConsumer) StartConsuming(ctx context.Context) {
    c.logger.Info("Starting Kafka consumer")
    
    for {
        select {
        case <-ctx.Done():
            c.logger.Info("Context cancelled, stopping consumer")
            return
        case <-c.ctx.Done():
            c.logger.Info("Consumer stopped")
            return
        default:
            msg, err := c.reader.ReadMessage(ctx)
            if err != nil {
                if err == context.Canceled {
                    return
                }
                c.logger.Error("Failed to read message", zap.Error(err))
                continue
            }
            
            var event OrderCreatedEvent
            if err := json.Unmarshal(msg.Value, &event); err != nil {
                c.logger.Error("Failed to unmarshal event", zap.Error(err))
                continue
            }
            
            c.logger.Info("Processing order event",
                zap.String("event_id", event.EventID),
                zap.Int("order_id", event.OrderID),
                zap.Int("items_count", len(event.Items)))
            
            // 각 상품의 재고 차감
            for _, item := range event.Items {
                productID := item.ProductID  // string으로 수정됨
                
                result, err := c.productService.DeductStock(ctx, productID, item.Quantity)
                if err != nil {
                    c.logger.Error("Failed to deduct stock",
                        zap.String("product_id", productID),
                        zap.Int("quantity", item.Quantity),
                        zap.Error(err))
                    continue
                }
                
                c.logger.Info("Stock deducted successfully",
                    zap.String("product_id", productID),
                    zap.Int("previous_stock", result.PreviousStock),
                    zap.Int("new_stock", result.NewStock),
                    zap.Int("deducted", result.Deducted))
            }
        }
    }
}

func (c *KafkaConsumer) Close() error {
    c.Stop()
    if c.reader != nil {
        return c.reader.Close()
    }
    return nil
}