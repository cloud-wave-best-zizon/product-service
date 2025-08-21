package tls

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io/ioutil"
    "time"
    
    "go.uber.org/zap"
)

type TLSConfig struct {
    Enabled      bool   `envconfig:"TLS_ENABLED" default:"false"`
    CertPath     string `envconfig:"TLS_CERT_PATH" default:"/run/spire/certs/cert.pem"`
    KeyPath      string `envconfig:"TLS_KEY_PATH" default:"/run/spire/certs/key.pem"`
    CAPath       string `envconfig:"TLS_CA_PATH" default:"/run/spire/certs/ca.pem"`
    ClientAuth   bool   `envconfig:"TLS_CLIENT_AUTH" default:"false"`
}

func LoadTLSConfig(cfg *TLSConfig, logger *zap.Logger) (*tls.Config, error) {
    if !cfg.Enabled {
        return nil, nil
    }
    
    cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load cert/key: %w", err)
    }
    
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    }
    
    // mTLS 설정 (클라이언트 인증)
    if cfg.ClientAuth && cfg.CAPath != "" {
        caCert, err := ioutil.ReadFile(cfg.CAPath)
        if err != nil {
            return nil, fmt.Errorf("failed to read CA cert: %w", err)
        }
        
        caCertPool := x509.NewCertPool()
        caCertPool.AppendCertsFromPEM(caCert)
        
        tlsConfig.ClientCAs = caCertPool
        tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
    }
    
    logger.Info("TLS configuration loaded",
        zap.String("cert_path", cfg.CertPath),
        zap.Bool("client_auth", cfg.ClientAuth))
    
    return tlsConfig, nil
}

// 인증서 자동 리로드를 위한 Watcher
func WatchCertificates(cfg *TLSConfig, reloadFunc func(*tls.Config) error, logger *zap.Logger) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        newTLSConfig, err := LoadTLSConfig(cfg, logger)
        if err != nil {
            logger.Error("Failed to reload TLS config", zap.Error(err))
            continue
        }
        
        if err := reloadFunc(newTLSConfig); err != nil {
            logger.Error("Failed to apply new TLS config", zap.Error(err))
        } else {
            logger.Info("TLS certificates reloaded successfully")
        }
    }
}