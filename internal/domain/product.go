package domain

import (
    "time"
)

type Product struct {
    ProductID   string    `json:"product_id"`
    Name        string    `json:"name"`
    Stock       int       `json:"stock"`
    Price       float64   `json:"price"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
    ProductID string  `json:"product_id" binding:"required"`
    Name      string  `json:"name" binding:"required"`
    Stock     int     `json:"stock" binding:"required,min=0"`
    Price     float64 `json:"price" binding:"required,min=0"`
}

type DeductStockRequest struct {
    Quantity int `json:"quantity" binding:"required,min=1"`
}

type ProductResponse struct {
    ProductID string  `json:"product_id"`
    Name      string  `json:"name"`
    Stock     int     `json:"stock"`
    Price     float64 `json:"price"`
}

type StockDeductionResponse struct {
    ProductID     string `json:"product_id"`
    PreviousStock int    `json:"previous_stock"`
    NewStock      int    `json:"new_stock"`
    Deducted      int    `json:"deducted"`
}