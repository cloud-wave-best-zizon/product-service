package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port             string `envconfig:"PORT" default:"8081"`
	AWSRegion        string `envconfig:"AWS_REGION" default:"ap-northeast-2"`
	ProductTableName string `envconfig:"PRODUCT_TABLE_NAME" default:"products-table"`
	LogLevel         string `envconfig:"LOG_LEVEL" default:"info"`
	LocalMode        bool   `envconfig:"LOCAL_MODE" default:"false"`
	DynamoDBEndpoint string `envconfig:"DYNAMODB_ENDPOINT" default:""` // DynamoDB Local 엔드포인트
	
	// Kafka 설정
	KafkaBrokers   string `envconfig:"KAFKA_BROKERS" default:"localhost:9092"`
	KafkaGroupID   string `envconfig:"KAFKA_GROUP_ID" default:"product-service"`
	KafkaEnabled   bool   `envconfig:"KAFKA_ENABLED" default:"true"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}