package config

import (
    "github.com/kelseyhightower/envconfig"
)

type Config struct {
    Port             string `envconfig:"PORT" default:"8080"`
    AWSRegion        string `envconfig:"AWS_REGION" default:"ap-northeast-2"`
    ProductTableName string `envconfig:"PRODUCT_TABLE_NAME" default:"products-table"`
    LogLevel         string `envconfig:"LOG_LEVEL" default:"info"`
    LocalMode        bool   `envconfig:"LOCAL_MODE" default:"true"` // AWS 없이 로컬 실행 모드
}

func Load() (*Config, error) {
    var cfg Config
    if err := envconfig.Process("", &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}