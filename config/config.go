package config

import (
	"crypto/x509"
	"fmt"
	u "layer4balancer/pkg/upstream"
	"os"
	"time"
)

type HealthCheckCfg struct {
	HealthCheckInterval time.Duration
	Timeout             time.Duration
}

type RateLimiterCfg struct {
	CleanupInterval time.Duration
	Limit           int
	Window          time.Duration
}

type AuthzCfg struct {
	Rules []string
}

type ServerCfg struct {
	HealthCheckCfg
	RateLimiterCfg
	AuthzCfg
	TlsCfg
	Bind      string
	Upstreams []*u.Upstream
	Timeout   time.Duration
}

type TlsCfg struct {
	CaCertPool *x509.CertPool
	CertPath   string
	KeyPath    string
	CaPath     string
}

func InitConfig() ServerCfg {
	healthCheckCfg := HealthCheckCfg{
		HealthCheckInterval: 3 * time.Second,
		Timeout:             1 * time.Second,
	}

	rateLimiterCfg := RateLimiterCfg{
		CleanupInterval: 20 * time.Second,
		Limit:           1,
		Window:          1 * time.Second,
	}

	authzCfg := AuthzCfg{
		Rules: []string{
			"client.a-deny-127.0.0.1:8000",
			"client.b-allow-127.0.0.1:8000",
			"client.c-deny-127.0.0.1:8000",
			"client.d-allow-127.0.0.1:8000",
			"client.e-allow-127.0.0.1:8000",

			"client.a-allow-127.0.0.1:8001",
			"client.b-allow-127.0.0.1:8001",
			"client.c-deny-127.0.0.1:8001",
			"client.d-allow-127.0.0.1:8001",
			"client.e-allow-127.0.0.1:8001",

			"client.a-allow-127.0.0.1:8002",
			"client.b-allow-127.0.0.1:8002",
			"client.c-allow-127.0.0.1:8002",
			"client.d-allow-127.0.0.1:8002",
			"client.e-allow-127.0.0.1:8002",
		},
	}
	pwd, _ := os.Getwd()
	certPath := fmt.Sprintf(pwd + "/certs/server.crt")

	keyPath := fmt.Sprintf(pwd + "/certs/server.key")

	caPath := fmt.Sprintf(pwd + "/certs/ca.crt")
	tlsCfg := TlsCfg{
		CertPath: certPath,
		KeyPath:  keyPath,
		CaPath:   caPath,
	}

	serverCfg := ServerCfg{
		HealthCheckCfg: healthCheckCfg,
		RateLimiterCfg: rateLimiterCfg,
		AuthzCfg:       authzCfg,
		TlsCfg:         tlsCfg,
		Bind:           ":1234",
		Timeout:        1 * time.Second,
		Upstreams: []*u.Upstream{
			{
				Host:          "127.0.0.1",
				Port:          "8000",
				NumActiveConn: 0,
				IsAlive:       true,
			},
			{
				Host:          "127.0.0.1",
				Port:          "8001",
				NumActiveConn: 0,
				IsAlive:       true,
			},
			{
				Host:          "127.0.0.1",
				Port:          "8002",
				NumActiveConn: 0,
				IsAlive:       true,
			},
		},
	}
	return serverCfg
}
