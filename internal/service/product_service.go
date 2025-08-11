package service
package service

import (
    "context"
    "errors"
    "time"

    "github.com/cloud-wave-best-zizon/product-service/internal/domain"
    "github.com/cloud-wave-best-zizon/product-service/internal/repository"
    "go.uber.org/zap"
)

var (
    ErrProductNotFound   = errors.New("product not found")
    ErrProductExists     = errors.New("product already exists")
    ErrInsufficientStock = errors.New("insufficient stock")
)

type ProductService struct {
    productRepo *repository.ProductRepository
    logger      *zap.Logger
}

func NewProductService(productRepo *repository.ProductRepository, logger *zap.Logger) *ProductService {
    return &ProductService{
        productRepo: productRepo,
        logger:      logger,
    }
}

func (s *ProductService) CreateProduct(ctx context.Context, req domain.CreateProductRequest) (*domain.Product, error) {
    // 중복 체크
    existing, _ := s.productRepo.GetProduct(ctx, req.ProductID)
    if existing != nil {
        return nil, ErrProductExists
    }

    product := &domain.Product{
        ProductID: req.ProductID,
        Name:      req.Name,
        Stock:     req.Stock,
        Price:     req.Price,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    if err := s.productRepo.CreateProduct(ctx, product); err != nil {
        s.logger.Error("Failed to save product", 
            zap.String("product_id", product.ProductID),
            zap.Error(err))
        return nil, err
    }

    s.logger.Info("Product created successfully",
        zap.String("product_id", product.ProductID),
        zap.Int("initial_stock", product.Stock))

    return product, nil
}

func (s *ProductService) GetProduct(ctx context.Context, productID string) (*domain.Product, error) {
    product, err := s.productRepo.GetProduct(ctx, productID)
    if err != nil {
        if err == repository.ErrProductNotFound {
            return nil, ErrProductNotFound
        }
        return nil, err
    }
    return product, nil
}

func (s *ProductService) DeductStock(ctx context.Context, productID string, quantity int) (*domain.StockDeductionResponse, error) {
    // Atomic 재고 차감
    newStock, previousStock, err := s.productRepo.DeductStock(ctx, productID, quantity)
    
    result := &domain.StockDeductionResponse{
        ProductID:     productID,
        PreviousStock: previousStock,
        NewStock:      newStock,
        Deducted:      quantity,
    }

    if err != nil {
        if err == repository.ErrProductNotFound {
            return result, ErrProductNotFound
        }
        if err == repository.ErrInsufficientStock {
            return result, ErrInsufficientStock
        }
        return nil, err
    }

    s.logger.Info("Stock deducted successfully",
        zap.String("product_id", productID),
        zap.Int("previous_stock", previousStock),
        zap.Int("deducted", quantity),
        zap.Int("new_stock", newStock))

    return result, nil
}