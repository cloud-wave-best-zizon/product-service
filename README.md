# product-service

## 개요
MSA 구조의 상품/재고 관리 서비스

## 기능
- 상품 등록
- 상품 조회
- 재고 차감

## API
- POST /api/v1/products - 상품 등록
- GET /api/v1/products/{id} - 상품 조회
- POST /api/v1/products/{id}/deduct - 재고 차감
- GET /api/v1/health - 헬스 체크

## 실행
```bash
make run

```
product-service/
├── cmd/
│   └── main.go
├── internal/
│   ├── domain/
│   │   └── product.go
│   ├── handler/
│   │   └── product_handler.go
│   ├── service/
│   │   └── product_service.go
│   └── repository/
│       └── product_repository.go
├── pkg/
│   ├── config/
│   │   └── config.go
│   └── middleware/
│       └── middleware.go
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
├── README.md
└── k8s/
    ├── deployment.yaml
    ├── service.yaml
    └── configmap.yaml
```
