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

# 애플리케이션 실행
make run

# 의존성 설치
make deps

# 빌드
make build

# 테스트
make test

# 코드 포맷팅
make fmt

# 도움말 보기
make help

```
product-service/
├── cmd/
│   └── main.go
├── internal/
│   ├── domain/
│   │   └── product.go
│   ├── handler/
│   │   └── product_handler.go  ← 여기로 이동
│   ├── service/
│   │   └── product_service.go
│   └── repository/
│       └── product_repository.go  ← 올바른 내용으로 수정
├── pkg/
│   ├── config/
│   │   └── config.go
│   └── middleware/
│       └── middleware.go
├── go.mod
├── go.sum
└── README.md
```


# 프로젝트 루트 디렉토리에서 실행
- go mod download
- go mod tidy

# 모든 의존성 다시 다운로드
- go clean -modcache
- go mod download

# 프로젝트 루트에서 실행
# 1. go.mod 재초기화 (이미 있다면 스킵)
- go mod init github.com/cloud-wave-best-zizon/product-service

# 2. 의존성 다운로드
- go get github.com/gin-gonic/gin@v1.9.1
- go get github.com/aws/aws-sdk-go-v2@v1.24.0
- go get github.com/aws/aws-sdk-go-v2/config@v1.26.1
- go get github.com/aws/aws-sdk-go-v2/service/dynamodb@v1.26.0
- go get github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue@v1.12.0
- go get github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression@v1.6.0
- go get github.com/google/uuid@v1.5.0
- go get github.com/kelseyhightower/envconfig@v1.4.0
- go get go.uber.org/zap@v1.26.0

# 3. 정리
- go mod tidy