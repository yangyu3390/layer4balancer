package ratelimit

import (
	"layer4balancer/config"
	"testing"
	"time"
)

func TestRateLimit(t *testing.T) {

	tests := []struct {
		description string
		config      config.RateLimiterCfg
		clientReqs  []string
		want        []bool
	}{
		{
			description: "happy path",
			config: config.RateLimiterCfg{
				CleanupInterval: 60 * time.Second,
				Limit:           1,               // max number of requests within the time window
				Window:          1 * time.Second, // time window
			},
			clientReqs: []string{
				"client a",
				"client b",
				"client a",
				"client a",
			},
			want: []bool{true, true, false, false},
		},
		{
			description: "reject all requests",
			config: config.RateLimiterCfg{
				CleanupInterval: 60 * time.Second,
				Limit:           0,
				Window:          1 * time.Second,
			},
			clientReqs: []string{
				"client a",
				"client b",
				"client a",
				"client a",
			},
			want: []bool{false, false, false, false},
		},
	}

	for _, tc := range tests {
		r := New(tc.config)

		for i, req := range tc.clientReqs {
			got := r.Allows(req)
			if got != tc.want[i] {
				t.Errorf("%s, %v != %v", tc.description, got, tc.want[i])
			}
		}

	}
}

func TestCleanup(t *testing.T) {
	tests := []struct {
		description string
		config      config.RateLimiterCfg
		clientReqs  []string
		sleep       time.Duration
		want        []int
	}{
		{
			description: "client a got deleted twice and client b got deleted once",
			config: config.RateLimiterCfg{
				CleanupInterval: 100 * time.Millisecond,
				Limit:           1,
				Window:          1 * time.Second,
			},
			clientReqs: []string{
				"client a",
				"client b",
				"client a",
				"client a",
			},
			sleep: 1 * time.Second,
			// count the number of elements in the client map
			want: []int{1, 1, 1, 1},
		},
		{
			description: "no clients got deleted",
			config: config.RateLimiterCfg{
				CleanupInterval: 10 * time.Second,
				Limit:           0,
				Window:          1 * time.Second,
			},
			clientReqs: []string{
				"client a",
				"client b",
				"client a",
				"client a",
			},
			sleep: 1 * time.Second,
			want:  []int{1, 2, 2, 2},
		},
	}

	for _, tc := range tests {
		r := New(tc.config)
		r.Start()
		for i, req := range tc.clientReqs {
			r.Allows(req)
			// prevent data race in test
			r.mu.Lock()
			got := len(r.clients)
			if got != tc.want[i] {
				r.mu.Unlock()
				t.Errorf("%s, %v != %v", tc.description, got, tc.want[i])
			}
			r.mu.Unlock()
			time.Sleep(tc.sleep)
		}
		r.Stop()
	}
}
