# product-service/Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

# 의존성 파일 복사
COPY go.mod go.sum ./
RUN go mod download

# 소스 코드 복사
COPY . .

# 빌드
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/main.go

# 최종 이미지
FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 빌드된 바이너리 복사
COPY --from=builder /app/main .

# 실행
CMD ["./main"]