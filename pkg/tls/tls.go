package tls

import (
    "context"
    "crypto/tls"
    "fmt"
    "time"
    
    "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "go.uber.org/zap"
)

type TLSConfig struct {
    Enabled      bool   `envconfig:"TLS_ENABLED" default:"false"`
    SocketPath   string `envconfig:"SPIRE_SOCKET_PATH" default:"unix:///run/spire/sockets/agent.sock"`
}

var x509Source *workloadapi.X509Source

func LoadTLSConfig(cfg *TLSConfig, logger *zap.Logger) (*tls.Config, error) {
    if !cfg.Enabled {
        logger.Info("TLS is disabled")
        return nil, nil
    }
    
    ctx := context.Background()
    
    // SPIRE Workload API를 통해 X509 소스 생성
    source, err := workloadapi.NewX509Source(
        ctx,
        workloadapi.WithClientOptions(
            workloadapi.WithAddr(cfg.SocketPath),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("unable to create X509Source: %w", err)
    }
    
    x509Source = source
    
    // mTLS 서버 설정 생성
    tlsConfig := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
    tlsConfig.MinVersion = tls.VersionTLS12
    
    logger.Info("SPIRE TLS configuration loaded",
        zap.String("socket_path", cfg.SocketPath),
        zap.Bool("mtls_enabled", true))
    
    return tlsConfig, nil
}

func WatchCertificates(cfg *TLSConfig, reloadFunc func(*tls.Config) error, logger *zap.Logger) {
    if x509Source == nil {
        logger.Error("X509Source is not initialized")
        return
    }
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // SPIRE가 자동으로 인증서를 갱신하므로 별도 처리 불필요
        // 하지만 로깅을 위해 상태 체크
        svid, err := x509Source.GetX509SVID()
        if err != nil {
            logger.Error("Failed to get X509 SVID", zap.Error(err))
            continue
        }
        
        logger.Info("Certificate status",
            zap.String("spiffe_id", svid.ID.String()),
            zap.Time("expiry", svid.Certificates[0].NotAfter),
            zap.Duration("ttl", time.Until(svid.Certificates[0].NotAfter)))
    }
}

// Cleanup 함수 추가
func Cleanup() {
    if x509Source != nil {
        x509Source.Close()
    }
}
