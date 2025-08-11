# Product Service

MSA 구조의 상품/재고 관리 서비스

## 목차

- [개요](#개요)
- [기능](#기능)
- [사전 요구사항](#사전-요구사항)
- [프로젝트 구조](#프로젝트-구조)
- [설치 가이드](#설치-가이드)
- [환경 설정](#환경-설정)
- [실행 방법](#실행-방법)
- [API 문서](#api-문서)
- [테스트](#테스트)
- [배포](#배포)
- [문제 해결](#문제-해결)

## 개요

Product Service는 마이크로서비스 아키텍처(MSA) 기반의 상품 및 재고 관리 서비스입니다.

### 주요 특징

- RESTful API 제공
- AWS DynamoDB를 통한 데이터 저장
- 로컬 개발을 위한 인메모리 모드 지원
- 원자적 재고 관리
- Graceful Shutdown 지원
- 구조화된 로깅 (Zap)

## 기능

- **상품 관리**
  - 상품 등록
  - 상품 조회
  - 상품 정보 수정 (추후 구현)
  
- **재고 관리**
  - 재고 차감 (원자적 처리)
  - 재고 추가 (추후 구현)
  
- **헬스 체크**
  - 서비스 상태 확인
  - 연결 모드 확인 (로컬/AWS)

## 사전 요구사항

### 필수 소프트웨어

- **Go** 1.21 이상
- **Git**
- **Make** (선택사항, Makefile 사용 시)
- **AWS CLI** (AWS DynamoDB 사용 시)
- **Docker** (DynamoDB Local 사용 시)

### 설치 확인

```bash
# Go 버전 확인
go version

# Git 확인
git --version

# Make 확인
make --version

# AWS CLI 확인 (선택사항)
aws --version

# Docker 확인 (선택사항)
docker --version
```

## 프로젝트 구조

```
product-service/
├── cmd/
│   └── main.go                 # 애플리케이션 진입점
├── internal/                   # 비공개 애플리케이션 코드
│   ├── domain/
│   │   └── product.go         # 도메인 모델
│   ├── handler/
│   │   └── product_handler.go # HTTP 핸들러
│   ├── service/
│   │   └── product_service.go # 비즈니스 로직
│   └── repository/
│       └── product_repository.go # 데이터 접근 계층
├── pkg/                        # 공개 라이브러리
│   ├── config/
│   │   └── config.go          # 설정 관리
│   └── middleware/
│       └── middleware.go      # HTTP 미들웨어
├── docker-compose.yml          # Docker 구성
├── Makefile                    # 빌드 자동화
├── go.mod                      # Go 모듈 정의
├── go.sum                      # 의존성 체크섬
└── README.md

```

## 설치 가이드

### 1. Go 설치

#### macOS (Homebrew)
```bash
brew install go
```

#### Ubuntu/Debian
```bash
sudo apt update
sudo apt install golang-go
```

#### 공식 다운로드
https://go.dev/dl/

### 2. 프로젝트 클론

```bash
git clone https://github.com/cloud-wave-best-zizon/product-service.git
cd product-service
```

### 3. Go 모듈 설치

```bash
# 의존성 다운로드
go mod download

# 의존성 정리
go mod tidy
```

### 4. AWS CLI 설치 및 설정 (AWS 사용 시)

#### AWS CLI 설치
```bash
# macOS
brew install awscli

# Ubuntu/Debian
sudo apt install awscli
```

#### AWS 자격증명 설정
```bash
aws configure

# 다음 정보 입력:
AWS Access Key ID [None]: YOUR_ACCESS_KEY
AWS Secret Access Key [None]: YOUR_SECRET_KEY
Default region name [None]: ap-northeast-2
Default output format [None]: json
```

### 5. DynamoDB 테이블 생성 (AWS 사용 시)

```bash
# DynamoDB 테이블 생성
aws dynamodb create-table \
    --table-name products-table \
    --attribute-definitions \
        AttributeName=product_id,AttributeType=S \
    --key-schema \
        AttributeName=product_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region ap-northeast-2

# 테이블 생성 확인
aws dynamodb describe-table --table-name products-table --region ap-northeast-2
```

## 환경 설정

### 환경 변수

| 변수명 | 설명 | 기본값 |
|--------|------|--------|
| `PORT` | 서버 포트 | `8080` |
| `AWS_REGION` | AWS 리전 | `ap-northeast-2` |
| `PRODUCT_TABLE_NAME` | DynamoDB 테이블명 | `products-table` |
| `LOG_LEVEL` | 로그 레벨 | `info` |
| `LOCAL_MODE` | 로컬 모드 사용 여부 | `false` |
| `DYNAMODB_ENDPOINT` | DynamoDB Local 엔드포인트 | 없음 |

### .env 파일 예시

```bash
# .env
LOCAL_MODE=false
PORT=8080
AWS_REGION=ap-northeast-2
PRODUCT_TABLE_NAME=products-table
LOG_LEVEL=info
```

## 실행 방법

### 1. 로컬 모드 (인메모리 저장소)

```bash
# Makefile 사용
make run

# 또는 직접 실행
LOCAL_MODE=true go run cmd/main.go
```

### 2. AWS DynamoDB 모드

```bash
# AWS 자격증명이 설정되어 있어야 함
LOCAL_MODE=false go run cmd/main.go

# 또는 (Makefile이 있다면)
make run-aws
```

### 3. DynamoDB Local 모드 (Docker)

```bash
# Docker Compose로 DynamoDB Local 실행
docker-compose up -d

# 애플리케이션 실행
LOCAL_MODE=false DYNAMODB_ENDPOINT=http://localhost:8000 go run cmd/main.go
```

### 4. 개발 모드 (Hot Reload)

```bash
# Air 설치
go install github.com/cosmtrek/air@latest

# 개발 모드 실행
make dev
```

## API 문서

### 기본 URL
```
http://localhost:8080/api/v1
```

### 엔드포인트

#### 1. 헬스 체크
```http
GET /api/v1/health
```

**응답 예시:**
```json
{
  "status": "healthy",
  "mode": "aws",
  "storage": "dynamodb",
  "table": "products-table"
}
```

#### 2. 상품 등록
```http
POST /api/v1/products
Content-Type: application/json

{
  "product_id": "PROD001",
  "name": "맥북 프로 14인치",
  "stock": 100,
  "price": 2690000
}
```

**응답 예시:**
```json
{
  "product_id": "PROD001",
  "name": "맥북 프로 14인치",
  "stock": 100,
  "price": 2690000
}
```

#### 3. 상품 조회
```http
GET /api/v1/products/{id}
```

**응답 예시:**
```json
{
  "product_id": "PROD001",
  "name": "맥북 프로 14인치",
  "stock": 100,
  "price": 2690000
}
```

#### 4. 재고 차감
```http
POST /api/v1/products/{id}/deduct
Content-Type: application/json

{
  "quantity": 10
}
```

**응답 예시:**
```json
{
  "product_id": "PROD001",
  "previous_stock": 100,
  "new_stock": 90,
  "deducted": 10
}
```

### 에러 응답

| 상태 코드 | 설명 |
|-----------|------|
| 400 | 잘못된 요청 (파라미터 오류 등) |
| 404 | 상품을 찾을 수 없음 |
| 409 | 중복된 상품 ID |
| 500 | 서버 내부 오류 |

## 테스트

### 단위 테스트

```bash
# 모든 테스트 실행
go test ./...

# 커버리지 포함
go test -cover ./...

# 상세 출력
go test -v ./...
```

### API 테스트 예시

```bash
# 1. 헬스 체크
curl http://localhost:8080/api/v1/health

# 2. 상품 등록
curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PROD001",
    "name": "맥북 프로 14인치",
    "stock": 100,
    "price": 2690000
  }'

# 3. 상품 조회
curl http://localhost:8080/api/v1/products/PROD001

# 4. 재고 차감
curl -X POST http://localhost:8080/api/v1/products/PROD001/deduct \
  -H "Content-Type: application/json" \
  -d '{"quantity": 10}'

# 5. 재고 부족 테스트
curl -X POST http://localhost:8080/api/v1/products/PROD001/deduct \
  -H "Content-Type: application/json" \
  -d '{"quantity": 1000}'
```

### DynamoDB 데이터 확인

```bash
# 모든 상품 조회
aws dynamodb scan --table-name products-table --region ap-northeast-2

# 특정 상품 조회
aws dynamodb get-item \
    --table-name products-table \
    --key '{"product_id": {"S": "PROD001"}}' \
    --region ap-northeast-2
```

## 배포

### 빌드

```bash
# 로컬 빌드
make build

# 프로덕션 빌드 (Linux)
make build-prod

# Docker 이미지 빌드
make docker-build
```

### Dockerfile 예시

```dockerfile
# 멀티 스테이지 빌드
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
```

## Makefile 명령어

| 명령어 | 설명 |
|--------|------|
| `make help` | 도움말 표시 |
| `make run` | 로컬 모드로 실행 |
| `make run-aws` | AWS DynamoDB로 실행 |
| `make build` | 애플리케이션 빌드 |
| `make test` | 테스트 실행 |
| `make deps` | 의존성 설치 |
| `make fmt` | 코드 포맷팅 |
| `make lint` | 린트 검사 |
| `make clean` | 빌드 파일 정리 |
| `make dev` | 개발 모드 (hot reload) |

## 문제 해결

### 1. Go 모듈 문제

```bash
# 모듈 캐시 정리
go clean -modcache

# 모듈 재초기화
rm go.mod go.sum
go mod init github.com/cloud-wave-best-zizon/product-service
go mod tidy
```

### 2. AWS 자격증명 문제

```bash
# 자격증명 확인
aws sts get-caller-identity

# 환경변수 설정
export AWS_REGION=ap-northeast-2
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
```

### 3. DynamoDB 연결 문제

```bash
# 테이블 존재 확인
aws dynamodb describe-table --table-name products-table

# 리전 확인
aws configure get region
```

### 4. 포트 충돌

```bash
# 8080 포트 사용 프로세스 확인
lsof -i :8080

# 다른 포트로 실행
PORT=8081 go run cmd/main.go
```

### 5. "Missing the key product_id" 에러

Domain 구조체에 DynamoDB 태그가 있는지 확인:
```go
type Product struct {
    ProductID string `json:"product_id" dynamodbav:"product_id"`
    // ...
}
```
