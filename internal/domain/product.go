package domain

import (
	"time"
)

type Product struct {
    ProductID string    `dynamodbav:"product_id" json:"product_id"`
    Name      string    `dynamodbav:"name"       json:"name"`
    Price     float64   `dynamodbav:"price"      json:"price"`
    Stock     int       `dynamodbav:"stock"      json:"stock"`
    CreatedAt time.Time `dynamodbav:"created_at" json:"created_at"`
    UpdatedAt time.Time `dynamodbav:"updated_at" json:"updated_at"`
}

type CreateProductRequest struct {
    ProductID string  `json:"product_id" binding:"required"`
    Name      string  `json:"name"       binding:"required"`
    Price     float64 `json:"price"      binding:"required"`
    Stock     int     `json:"stock"      binding:"required"`
}

type DeductStockRequest struct {
	Quantity int `json:"quantity" binding:"required,min=1"`
}

type ProductResponse struct {
    ProductID string  `json:"product_id"`
    Name      string  `json:"name"`
    Price     float64 `json:"price"`
    Stock     int     `json:"stock"`
}

type StockDeductionResponse struct {
	ProductID     string `json:"product_id"`
	PreviousStock int    `json:"previous_stock"`
	NewStock      int    `json:"new_stock"`
	Deducted      int    `json:"deducted"`
}
