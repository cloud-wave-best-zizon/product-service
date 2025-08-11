# Makefile for product-service

# 변수 설정
APP_NAME=product-service
MAIN_PATH=cmd/main.go
DOCKER_IMAGE=product-service:latest
GO=go
GOFLAGS=-v

# 색상 정의
GREEN=\033[0;32m
NC=\033[0m # No Color

.PHONY: help run build test clean docker-build docker-run deps lint fmt

# 기본 타겟
help:
	@echo "사용 가능한 명령어:"
	@echo "  make run         - 애플리케이션 실행 (로컬 모드)"
	@echo "  make build       - 애플리케이션 빌드"
	@echo "  make test        - 테스트 실행"
	@echo "  make clean       - 빌드 파일 정리"
	@echo "  make deps        - 의존성 다운로드"
	@echo "  make docker-build - Docker 이미지 빌드"
	@echo "  make docker-run   - Docker 컨테이너 실행"
	@echo "  make lint        - 코드 린트 검사"
	@echo "  make fmt         - 코드 포맷팅"

# 애플리케이션 실행 (로컬 모드)
run:
	@echo "$(GREEN)Starting $(APP_NAME) in local mode...$(NC)"
	LOCAL_MODE=true $(GO) run $(MAIN_PATH)

# 애플리케이션 빌드
build:
	@echo "$(GREEN)Building $(APP_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -o bin/$(APP_NAME) $(MAIN_PATH)

# 테스트 실행
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test -v ./...

# 빌드 파일 정리
clean:
	@echo "$(GREEN)Cleaning build files...$(NC)"
	rm -rf bin/
	$(GO) clean

# 의존성 다운로드
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy

# Docker 이미지 빌드
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE) .

# Docker 컨테이너 실행
docker-run:
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run -p 8080:8080 -e LOCAL_MODE=true $(DOCKER_IMAGE)

# 코드 린트 검사
lint:
	@echo "$(GREEN)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
		golangci-lint run ./...; \
	fi

# 코드 포맷팅
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...

# 모든 빌드 및 테스트 실행
all: deps fmt lint test build

# 개발 모드 실행 (파일 변경 감지)
dev:
	@echo "$(GREEN)Starting in development mode with hot reload...$(NC)"
	@if command -v air >/dev/null 2>&1; then \
		LOCAL_MODE=true air; \
	else \
		echo "Air not installed. Installing..."; \
		go install github.com/cosmtrek/air@latest; \
		LOCAL_MODE=true air; \
	fi

# DB 마이그레이션 (나중에 사용)
migrate-up:
	@echo "$(GREEN)Running migrations...$(NC)"
	@echo "Not implemented yet"

migrate-down:
	@echo "$(GREEN)Rolling back migrations...$(NC)"
	@echo "Not implemented yet"

# 프로덕션 빌드
build-prod:
	@echo "$(GREEN)Building for production...$(NC)"
	CGO_ENABLED=0 GOOS=linux $(GO) build -a -installsuffix cgo -o bin/$(APP_NAME) $(MAIN_PATH)

# 버전 정보 출력
version:
	@echo "$(GREEN)Version information:$(NC)"
	@$(GO) version
	@echo "App: $(APP_NAME)"