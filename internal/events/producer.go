package events

import (
    "context"
    "encoding/json"
    "time"
    "github.com/segmentio/kafka-go"
    "go.uber.org/zap"
)

type KafkaProducer struct {
    writer *kafka.Writer
    logger *zap.Logger
}

func NewKafkaProducer(brokers string) (*KafkaProducer, error) {
    logger, _ := zap.NewProduction()
    
    writer := &kafka.Writer{
        Addr:     kafka.TCP(brokers),
        Topic:    "order-events",
        Balancer: &kafka.LeastBytes{},
        BatchTimeout: 10 * time.Millisecond,
    }
    
    return &KafkaProducer{
        writer: writer,
        logger: logger,
    }, nil
}

func (p *KafkaProducer) PublishOrderCreated(event OrderCreatedEvent) error {
    eventBytes, err := json.Marshal(event)
    if err != nil {
        p.logger.Error("Failed to marshal event", zap.Error(err))
        return err
    }
    
    msg := kafka.Message{
        Key:   []byte(event.EventID),
        Value: eventBytes,
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    err = p.writer.WriteMessages(ctx, msg)
    if err != nil {
        p.logger.Error("Failed to publish message", 
            zap.String("event_id", event.EventID),
            zap.Error(err))
        return err
    }
    
    p.logger.Info("Event published successfully",
        zap.String("event_id", event.EventID),
        zap.Int("order_id", event.OrderID))
    
    return nil
}

func (p *KafkaProducer) Close() error {
    if p.writer != nil {
        return p.writer.Close()
    }
    return nil
}