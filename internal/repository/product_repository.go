package repository

import (
    "context"
    "errors"
    "fmt"
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
}

func NewDynamoDBClient(cfg *pkgconfig.Config) (*dynamodb.Client, error) {
    awsCfg, err := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion(cfg.AWSRegion),
    )
    if err != nil {
        return nil, err
    }

    return dynamodb.NewFromConfig(awsCfg), nil
}

func NewProductRepository(client *dynamodb.Client, tableName string) *ProductRepository {
    return &ProductRepository{
        client:    client,
        tableName: tableName,
    }
}

func (r *ProductRepository) CreateProduct(ctx context.Context, product *domain.Product) error {
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