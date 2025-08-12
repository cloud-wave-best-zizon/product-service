#!/bin/bash

# DynamoDB Tables Setup Script
set -e

AWS_REGION=${1:-"ap-northeast-2"}

echo "🗄️ Setting up DynamoDB tables..."
echo "Region: $AWS_REGION"

# Products Table 생성
echo "📦 Creating products-table..."
aws dynamodb create-table \
    --table-name products-table \
    --attribute-definitions \
        AttributeName=product_id,AttributeType=S \
    --key-schema \
        AttributeName=product_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region $AWS_REGION \
    --tags Key=Project,Value=ecommerce Key=Service,Value=product-service

# Orders Table 생성
echo "📋 Creating orders table..."
aws dynamodb create-table \
    --table-name orders \
    --attribute-definitions \
        AttributeName=PK,AttributeType=S \
        AttributeName=SK,AttributeType=S \
        AttributeName=GSI1PK,AttributeType=S \
        AttributeName=GSI1SK,AttributeType=S \
    --key-schema \
        AttributeName=PK,KeyType=HASH \
        AttributeName=SK,KeyType=RANGE \
    --global-secondary-indexes \
        IndexName=GSI1,KeySchema=[{AttributeName=GSI1PK,KeyType=HASH},{AttributeName=GSI1SK,KeyType=RANGE}],Projection={ProjectionType=ALL} \
    --billing-mode PAY_PER_REQUEST \
    --region $AWS_REGION \
    --tags Key=Project,Value=ecommerce Key=Service,Value=order-service

# 테이블 생성 대기
echo "⏳ Waiting for tables to be active..."
aws dynamodb wait table-exists --table-name products-table --region $AWS_REGION
aws dynamodb wait table-exists --table-name orders --region $AWS_REGION

echo "✅ DynamoDB tables created successfully!"

# 테이블 상태 확인
echo "📊 Table status:"
aws dynamodb describe-table --table-name products-table --region $AWS_REGION --query 'Table.{Name:TableName,Status:TableStatus,ItemCount:ItemCount,Size:TableSizeBytes}'
aws dynamodb describe-table --table-name orders --region $AWS_REGION --query 'Table.{Name:TableName,Status:TableStatus,ItemCount:ItemCount,Size:TableSizeBytes}'

# 샘플 상품 데이터 삽입
echo "📝 Inserting sample product data..."
aws dynamodb put-item \
    --table-name products-table \
    --item '{
        "product_id": {"S": "PROD001"},
        "name": {"S": "MacBook Pro 14inch"},
        "stock": {"N": "100"},
        "price": {"N": "2690000"},
        "created_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"},
        "updated_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}
    }' \
    --region $AWS_REGION

aws dynamodb put-item \
    --table-name products-table \
    --item '{
        "product_id": {"S": "PROD002"},
        "name": {"S": "iPad Air"},
        "stock": {"N": "50"},
        "price": {"N": "899000"},
        "created_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"},
        "updated_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}
    }' \
    --region $AWS_REGION

aws dynamodb put-item \
    --table-name products-table \
    --item '{
        "product_id": {"S": "PROD003"},
        "name": {"S": "AirPods Pro"},
        "stock": {"N": "200"},
        "price": {"N": "329000"},
        "created_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"},
        "updated_at": {"S": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}
    }' \
    --region $AWS_REGION

echo "✅ Sample data inserted successfully!"
echo "🎉 DynamoDB setup completed!"