package handler

import (
	"net/http"

	"github.com/cloud-wave-best-zizon/product-service/internal/domain"
	"github.com/cloud-wave-best-zizon/product-service/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ProductHandler struct {
	productService *service.ProductService
	logger         *zap.Logger
}

func NewProductHandler(productService *service.ProductService, logger *zap.Logger) *ProductHandler {
	return &ProductHandler{
		productService: productService,
		logger:         logger,
	}
}

func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req domain.CreateProductRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	product, err := h.productService.CreateProduct(c.Request.Context(), req)
	if err != nil {
		if err == service.ErrProductExists {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Product already exists",
			})
			return
		}

		h.logger.Error("Failed to create product",
			zap.String("product_id", req.ProductID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create product",
		})
		return
	}

	response := domain.ProductResponse{
		ProductID: product.ProductID,
		Name:      product.Name,
		Stock:     product.Stock,
		Price:     product.Price,
	}

	c.JSON(http.StatusCreated, response)
}

func (h *ProductHandler) GetProduct(c *gin.Context) {
	productID := c.Param("id")

	product, err := h.productService.GetProduct(c.Request.Context(), productID)
	if err != nil {
		if err == service.ErrProductNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Product not found",
			})
			return
		}

		h.logger.Error("Failed to get product",
			zap.String("product_id", productID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get product",
		})
		return
	}

	response := domain.ProductResponse{
		ProductID: product.ProductID,
		Name:      product.Name,
		Stock:     product.Stock,
		Price:     product.Price,
	}

	c.JSON(http.StatusOK, response)
}

func (h *ProductHandler) DeductStock(c *gin.Context) {
	productID := c.Param("id")

	var req domain.DeductStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	result, err := h.productService.DeductStock(c.Request.Context(), productID, req.Quantity)
	if err != nil {
		if err == service.ErrProductNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Product not found",
			})
			return
		}

		if err == service.ErrInsufficientStock {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":     "Insufficient stock",
				"available": result.PreviousStock,
				"requested": req.Quantity,
			})
			return
		}

		h.logger.Error("Failed to deduct stock",
			zap.String("product_id", productID),
			zap.Error(err))

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to deduct stock",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
