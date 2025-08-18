package events

import (
    "time"
)

// Order Service에서 받을 이벤트
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
    ProductID   int     `json:"product_id"`
    ProductName string  `json:"product_name"`
    Quantity    int     `json:"quantity"`
    Price       float64 `json:"price"`
}

// 재고 차감 완료 이벤트
type StockDeductedEvent struct {
    EventID     string    `json:"event_id"`
    OrderID     int       `json:"order_id"`
    ProductID   string    `json:"product_id"`
    Quantity    int       `json:"quantity"`
    NewStock    int       `json:"new_stock"`
    Timestamp   time.Time `json:"timestamp"`
}
