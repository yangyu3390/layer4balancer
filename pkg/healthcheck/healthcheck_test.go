package healthcheck

import (
	"layer4balancer/config"
	u "layer4balancer/pkg/upstream"
	"sync"
	"testing"
	"time"
)

func TestHealthChecker(t *testing.T) {
	// TODO: add another test.
	// create an upstream server, check the number of elements in HealthyUpstream channel
	tests := []struct {
		description string
		upstreams   []*u.Upstream
		want        int
	}{
		{
			description: "three unreachable upstreams",
			upstreams: []*u.Upstream{
				{
					Host:          "127.0.0.3",
					Port:          "8000",
					NumActiveConn: 0,
				},
				{
					Host:          "127.0.0.4",
					Port:          "8001",
					NumActiveConn: 10,
				},
				{
					Host:          "127.0.0.5",
					Port:          "8002",
					NumActiveConn: 20,
				},
			},
			want: 3, // detect
		},
	}
	// avoid race conditions in tests
	var mu sync.Mutex
	for _, tc := range tests {
		hc := New(config.HealthCheckCfg{
			HealthCheckInterval: 1 * time.Second,
			Timeout:             1 * time.Second,
		})
		hc.Start(tc.upstreams)

		set := make(map[*u.Upstream]bool)
		stop := make(chan bool)
		go func() {
			for {
				select {
				case unhealthy := <-hc.UnhealthyUpstreams:
					mu.Lock()
					set[unhealthy] = true
					mu.Unlock()
				case <-stop:
					return
				}
			}
		}()

		time.Sleep(3 * time.Second)
		mu.Lock()
		got := len(set)
		mu.Unlock()
		if got != tc.want {
			t.Errorf("%s, %v != %v", tc.description, got, tc.want)
		}
		hc.Stop()
		stop <- true
	}

}
