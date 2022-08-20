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
				Burst:           2,
				Token:           1,
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
				Burst:           0,
				Token:           0,
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
				Burst:           1,
				Token:           1,
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
				Burst:           0,
				Token:           1,
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
