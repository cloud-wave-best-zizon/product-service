# EKS 배포 가이드

Order Service와 Product Service를 Kafka로 연동하여 EKS에 배포하는 전체 가이드입니다.

## 목차

1. [사전 요구사항](#사전-요구사항)
2. [AWS 리소스 설정](#aws-리소스-설정)
3. [EKS 클러스터 생성](#eks-클러스터-생성)
4. [IAM 역할 및 정책 설정](#iam-역할-및-정책-설정)
5. [DynamoDB 테이블 생성](#dynamodb-테이블-생성)
6. [Docker 이미지 빌드 및 푸시](#docker-이미지-빌드-및-푸시)
7. [Kubernetes 리소스 배포](#kubernetes-리소스-배포)
8. [테스트 및 검증](#테스트-및-검증)
9. [모니터링 및 로깅](#모니터링-및-로깅)
10. [문제 해결](#문제-해결)

## 사전 요구사항

### 필수 도구 설치

```bash
# AWS CLI v2
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# eksctl
curl --silent --location "https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp
sudo mv /tmp/eksctl /usr/local/bin

# Docker
sudo apt-get update
sudo apt-get install docker.io
sudo usermod -aG docker $USER

# Helm (선택사항)
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### AWS 계정 설정

```bash
# AWS 자격증명 설정
aws configure

# 계정 ID 확인
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
export AWS_REGION="ap-northeast-2"
echo "Account ID: $AWS_ACCOUNT_ID"
echo "Region: $AWS_REGION"
```

## AWS 리소스 설정

### 환경 변수 설정

```bash
export CLUSTER_NAME="ecommerce-cluster"
export NODE_GROUP_NAME="ecommerce-nodes"
export PRODUCT_TABLE_NAME="products-table"
export ORDER_TABLE_NAME="orders"
```

## EKS 클러스터 생성

### 1. EKS 클러스터 생성

```bash
# EKS 클러스터 생성 (약 15-20분 소요)
eksctl create cluster \
  --name $CLUSTER_NAME \
  --region $AWS_REGION \
  --version 1.28 \
  --nodegroup-name $NODE_GROUP_NAME \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 2 \
  --nodes-max 5 \
  --managed \
  --with-oidc \
  --ssh-access \
  --ssh-public-key ~/.ssh/id_rsa.pub  # SSH 키가 있는 경우
```

### 2. kubectl 설정

```bash
# kubectl 컨텍스트 업데이트
aws eks update-kubeconfig --region $AWS_REGION --name $CLUSTER_NAME

# 연결 확인
kubectl get nodes
```

### 3. AWS Load Balancer Controller 설치

```bash
# 서비스 계정 생성
eksctl create iamserviceaccount \
  --cluster=$CLUSTER_NAME \
  --namespace=kube-system \
  --name=aws-load-balancer-controller \
  --role-name AmazonEKSLoadBalancerControllerRole \
  --attach-policy-arn=arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess \
  --approve

# Helm 차트로 컨트롤러 설치
helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=$CLUSTER_NAME \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller

# 설치 확인
kubectl get deployment -n kube-system aws-load-balancer-controller
```

## IAM 역할 및 정책 설정

### 1. OIDC Provider 정보 확인

```bash
# OIDC Issuer URL 확인
OIDC_URL=$(aws eks describe-cluster --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text)
OIDC_ID=$(echo $OIDC_URL | cut -d '/' -f 5)
echo "OIDC ID: $OIDC_ID"
```

### 2. IAM 정책 생성

```bash
# Product Service 정책 생성
cat > product-service-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:DescribeTable"
      ],
      "Resource": [
        "arn:aws:dynamodb:$AWS_REGION:$AWS_ACCOUNT_ID:table/$PRODUCT_TABLE_NAME",
        "arn:aws:dynamodb:$AWS_REGION:$AWS_ACCOUNT_ID:table/$PRODUCT_TABLE_NAME/index/*"
      ]
    }
  ]
}
EOF

aws iam create-policy \
  --policy-name ProductServicePolicy \
  --policy-document file://product-service-policy.json

# Order Service 정책 생성
cat > order-service-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:PutItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:DescribeTable"
      ],
      "Resource": [
        "arn:aws:dynamodb:$AWS_REGION:$AWS_ACCOUNT_ID:table/$ORDER_TABLE_NAME",
        "arn:aws:dynamodb:$AWS_REGION:$AWS_ACCOUNT_ID:table/$ORDER_TABLE_NAME/index/*"
      ]
    }
  ]
}
EOF

aws iam create-policy \
  --policy-name OrderServicePolicy \
  --policy-document file://order-service-policy.json
```

### 3. IAM 역할 생성

```bash
# Product Service 역할 생성
eksctl create iamserviceaccount \
  --name product-service-sa \
  --namespace ecommerce \
  --cluster $CLUSTER_NAME \
  --attach-policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/ProductServicePolicy \
  --approve

# Order Service 역할 생성
eksctl create iamserviceaccount \
  --name order-service-sa \
  --namespace ecommerce \
  --cluster $CLUSTER_NAME \
  --attach-policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/OrderServicePolicy \
  --approve
```

## DynamoDB 테이블 생성

```bash
# DynamoDB 설정 스크립트 실행
chmod +x scripts/setup-dynamodb.sh
./scripts/setup-dynamodb.sh $AWS_REGION
```

## Docker 이미지 빌드 및 푸시

### 1. ECR 리포지토리 생성

```bash
# ECR 리포지토리 생성
aws ecr create-repository --repository-name product-service --region $AWS_REGION
aws ecr create-repository --repository-name order-service --region $AWS_REGION
```

### 2. Docker 이미지 빌드 및 푸시

```bash
# 이미지 빌드 및 푸시 스크립트 실행
chmod +x scripts/build-and-push.sh

# ECR 주소를 스크립트에 설정
export REGISTRY="$AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com"
sed -i "s/your-account-id/$AWS_ACCOUNT_ID/g" scripts/build-and-push.sh

# 빌드 및 푸시 실행
./scripts/build-and-push.sh v1.0.0
```

## Kubernetes 리소스 배포

### 1. 매니페스트 파일 업데이트

```bash
# Kubernetes 매니페스트에 실제 값들로 업데이트
sed -i "s/YOUR_ACCOUNT_ID/$AWS_ACCOUNT_ID/g" k8s/03-product-service.yaml
sed -i "s/YOUR_ACCOUNT_ID/$AWS_ACCOUNT_ID/g" k8s/04-order-service.yaml
sed -i "s/your-registry/$REGISTRY/g" k8s/03-product-service.yaml
sed -i "s/your-registry/$REGISTRY/g" k8s/04-order-service.yaml
```

### 2. 배포 실행

```bash
# 배포 스크립트 실행
chmod +x scripts/deploy.sh
./scripts/deploy.sh $CLUSTER_NAME $AWS_REGION $AWS_ACCOUNT_ID
```

### 3. 개별 리소스 배포 (선택사항)

```bash
# 순서대로 배포
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/02-kafka.yaml

# Kafka 준비 대기
kubectl wait --for=condition=ready pod -l app=kafka -n ecommerce --timeout=300s

# 서비스 배포
kubectl apply -f k8s/03-product-service.yaml
kubectl apply -f k8s/04-order-service.yaml
kubectl apply -f k8s/05-ingress.yaml
```

## 테스트 및 검증

### 1. 파드 상태 확인

```bash
# 모든 파드 상태 확인
kubectl get pods -n ecommerce

# 로그 확인
kubectl logs -f deployment/product-service -n ecommerce
kubectl logs -f deployment/order-service -n ecommerce
kubectl logs -f deployment/kafka -n ecommerce
```

### 2. 서비스 접근성 테스트

```bash
# LoadBalancer 주소 확인
ALB_URL=$(kubectl get ingress ecommerce-dev-ingress -n ecommerce -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
echo "ALB URL: http://$ALB_URL"

# Health check 테스트
curl -v http://$ALB_URL/api/v1/products/health
curl -v http://$ALB_URL/api/v1/orders/health
```

### 3. API 기능 테스트

```bash
# 상품 조회
curl -X GET http://$ALB_URL/api/v1/products/PROD001

# 주문 생성 (재고 차감 테스트)
curl -X POST http://$ALB_URL/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user123",
    "items": [
      {
        "product_id": 1,
        "product_name": "MacBook Pro 14inch",
        "quantity": 2,
        "price": 2690000
      }
    ],
    "idempotency_key": "order-test-001"
  }'

# 재고 확인
curl -X GET http://$ALB_URL/api/v1/products/PROD001
```

### 4. Kafka 메시지 플로우 확인

```bash
# Product Service 로그에서 Kafka 메시지 처리 확인
kubectl logs -f deployment/product-service -n ecommerce | grep -i kafka

# Kafka 토픽 확인
kubectl exec -it kafka-0 -n ecommerce -- kafka-topics --list --bootstrap-server localhost:9092

# 메시지 확인
kubectl exec -it kafka-0 -n ecommerce -- kafka-console-consumer --bootstrap-server localhost:9092 --topic order-events --from-beginning
```

## 모니터링 및 로깅

### 1. CloudWatch 로그 설정

```bash
# CloudWatch 로그 그룹 생성
aws logs create-log-group --log-group-name /aws/eks/$CLUSTER_NAME/product-service
aws logs create-log-group --log-group-name /aws/eks/$CLUSTER_NAME/order-service
```

### 2. 메트릭 확인

```bash
# HPA 상태 확인
kubectl get hpa -n ecommerce

# 리소스 사용량 확인
kubectl top pods -n ecommerce
kubectl top nodes
```

### 3. 알람 설정 (선택사항)

```bash
# CloudWatch 알람 생성 예시
aws cloudwatch put-metric-alarm \
  --alarm-name "ProductService-HighCPU" \
  --alarm-description "Product Service High CPU Usage" \
  --metric-name CPUUtilization \
  --namespace AWS/EKS \
  --statistic Average \
  --period 300 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --evaluation-periods 2
```

## 문제 해결

### 1. 일반적인 문제

#### Kafka 연결 실패
```bash
# Kafka 파드 상태 확인
kubectl describe pod kafka-0 -n ecommerce

# 네트워크 정책 확인
kubectl get networkpolicy -n ecommerce

# DNS 해결 테스트
kubectl run test-pod --image=busybox -it --rm -- nslookup kafka-service.ecommerce.svc.cluster.local
```

#### DynamoDB 권한 오류
```bash
# ServiceAccount 확인
kubectl describe sa product-service-sa -n ecommerce

# IAM 역할 확인
aws iam get-role --role-name eksctl-ecommerce-cluster-addon-iamserviceaccount-ecommerce-product-service-sa-Role1
```

#### 이미지 Pull 실패
```bash
# ECR 권한 확인
aws ecr get-login-password --region $AWS_REGION | docker login --username AWS --password-stdin $AWS_ACCOUNT_ID.dkr.ecr.$AWS_REGION.amazonaws.com

# 이미지 존재 확인
aws ecr describe-images --repository-name product-service --region $AWS_REGION
```

### 2. 로그 분석

```bash
# 상세 로그 확인
kubectl logs deployment/product-service -n ecommerce --previous
kubectl logs deployment/order-service -n ecommerce --previous

# 이벤트 확인
kubectl get events -n ecommerce --sort-by=.metadata.creationTimestamp
```

### 3. 네트워크 문제

```bash
# 서비스 엔드포인트 확인
kubectl get endpoints -n ecommerce

# 포트 포워딩으로 직접 테스트
kubectl port-forward svc/product-service 8080:80 -n ecommerce
curl http://localhost:8080/api/v1/health
```

## 정리 및 삭제

### 리소스 정리

```bash
# Kubernetes 리소스 삭제
kubectl delete namespace ecommerce

# EKS 클러스터 삭제
eksctl delete cluster --name $CLUSTER_NAME --region $AWS_REGION

# DynamoDB 테이블 삭제
aws dynamodb delete-table --table-name $PRODUCT_TABLE_NAME --region $AWS_REGION
aws dynamodb delete-table --table-name $ORDER_TABLE_NAME --region $AWS_REGION

# ECR 리포지토리 삭제
aws ecr delete-repository --repository-name product-service --force --region $AWS_REGION
aws ecr delete-repository --repository-name order-service --force --region $AWS_REGION

# IAM 정책 삭제
aws iam delete-policy --policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/ProductServicePolicy
aws iam delete-policy --policy-arn arn:aws:iam::$AWS_ACCOUNT_ID:policy/OrderServicePolicy
```

## 추가 개선사항

### 1. 보안 강화
- Network Policy 설정
- Pod Security Standards 적용
- Secrets 관리 개선

### 2. 성능 최적화
- HPA 튜닝
- Resource requests/limits 최적화
- Connection pooling 설정

### 3. 운영 개선
- GitOps (ArgoCD) 도입
- Blue-Green 배포
- Canary 배포

이 가이드를 따라 배포하면 완전히 작동하는 마이크로서비스 환경이 EKS에 구축됩니다.