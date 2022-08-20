package server

import (
	"fmt"
	"layer4balancer/config"
	u "layer4balancer/pkg/upstream"
	"os"
	"testing"
	"time"
)

func createTestConfig() config.ServerCfg {
	healthCheckCfg := config.HealthCheckCfg{
		HealthCheckInterval: 1 * time.Second,
		Timeout:             1 * time.Second,
	}

	rateLimiterCfg := config.RateLimiterCfg{
		CleanupInterval: 20 * time.Second,
		Burst:           1,
		Token:           1,
	}

	authzCfg := config.AuthzCfg{
		Rules: []string{
			"client.a-allow-127.0.0.1:8000",
		},
	}
	pwd, _ := os.Getwd()
	certPath := fmt.Sprintf(pwd + "/../certs/server.crt")

	keyPath := fmt.Sprintf(pwd + "/../certs/server.key")

	caPath := fmt.Sprintf(pwd + "/../certs/ca.crt")

	tlsCfg := config.TlsCfg{
		CertPath: certPath,
		KeyPath:  keyPath,
		CaPath:   caPath,
	}

	serverCfg := config.ServerCfg{
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
		},
	}
	return serverCfg
}

func TestStart(t *testing.T) {
	cfg := createTestConfig()
	server, err := New(cfg)
	if err != nil {
		t.Error("failed to create a new server")
	}
	err = server.Start()
	if err != nil {
		t.Error("failed to start the new server")
	}
	server.Stop()
}
