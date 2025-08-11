package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/cloud-wave-best-zizon/product-service/internal/domain"
	pkgconfig "github.com/cloud-wave-best-zizon/product-service/pkg/config"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type ProductRepository struct {
	client    *dynamodb.Client
	tableName string
	localMode bool
	// 로컬 모드용 인메모리 저장소
	localStore map[string]*domain.Product
	mu         sync.RWMutex
}

func NewDynamoDBClient(cfg *pkgconfig.Config) (*dynamodb.Client, error) {
	if cfg.LocalMode {
		// 로컬 모드에서는 nil 반환
		return nil, nil
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.AWSRegion),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return dynamodb.NewFromConfig(awsCfg), nil
}

func NewProductRepository(client *dynamodb.Client, tableName string) *ProductRepository {
	return &ProductRepository{
		client:     client,
		tableName:  tableName,
		localMode:  client == nil,
		localStore: make(map[string]*domain.Product),
	}
}

func (r *ProductRepository) CreateProduct(ctx context.Context, product *domain.Product) error {
	if r.localMode {
		r.mu.Lock()
		defer r.mu.Unlock()

		if _, exists := r.localStore[product.ProductID]; exists {
			return errors.New("product already exists")
		}

		r.localStore[product.ProductID] = product
		return nil
	}

	av, err := attributevalue.MarshalMap(product)
	if err != nil {
		return fmt.Errorf("failed to marshal product: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      av,
	})

	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

func (r *ProductRepository) GetProduct(ctx context.Context, productID string) (*domain.Product, error) {
	if r.localMode {
		r.mu.RLock()
		defer r.mu.RUnlock()

		product, exists := r.localStore[productID]
		if !exists {
			return nil, ErrProductNotFound
		}

		// 깊은 복사를 위해 새 객체 생성
		productCopy := *product
		return &productCopy, nil
	}

	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"product_id": &types.AttributeValueMemberS{Value: productID},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	if result.Item == nil {
		return nil, ErrProductNotFound
	}

	var product domain.Product
	if err := attributevalue.UnmarshalMap(result.Item, &product); err != nil {
		return nil, fmt.Errorf("failed to unmarshal product: %w", err)
	}

	return &product, nil
}

func (r *ProductRepository) DeductStock(ctx context.Context, productID string, quantity int) (newStock int, previousStock int, err error) {
	if r.localMode {
		r.mu.Lock()
		defer r.mu.Unlock()

		product, exists := r.localStore[productID]
		if !exists {
			return 0, 0, ErrProductNotFound
		}

		previousStock = product.Stock

		if product.Stock < quantity {
			return 0, previousStock, ErrInsufficientStock
		}

		product.Stock -= quantity
		product.UpdatedAt = time.Now()

		return product.Stock, previousStock, nil
	}

	// Get current stock first
	product, err := r.GetProduct(ctx, productID)
	if err != nil {
		return 0, 0, err
	}
	previousStock = product.Stock

	// Atomic update with condition
	update := expression.Set(
		expression.Name("stock"),
		expression.Minus(
			expression.Name("stock"),
			expression.Value(quantity),
		),
	).Set(
		expression.Name("updated_at"),
		expression.Value(time.Now()),
	)

	// 재고가 충분한 경우에만 업데이트
	condition := expression.GreaterThanEqual(
		expression.Name("stock"),
		expression.Value(quantity),
	)

	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(condition).
		Build()
	if err != nil {
		return 0, previousStock, err
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"product_id": &types.AttributeValueMemberS{Value: productID},
		},
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ReturnValues:              types.ReturnValueAllNew,
	}

	result, err := r.client.UpdateItem(ctx, input)
	if err != nil {
		var ccf *types.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return 0, previousStock, ErrInsufficientStock
		}
		return 0, previousStock, err
	}

	// 업데이트된 재고 반환
	var updatedProduct domain.Product
	if err := attributevalue.UnmarshalMap(result.Attributes, &updatedProduct); err != nil {
		return 0, previousStock, err
	}

	return updatedProduct.Stock, previousStock, nil
}
