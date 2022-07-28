package authz

import (
	"layer4balancer/config"
	"testing"
)

func TestAuthZ(t *testing.T) {

	tests := []struct {
		description  string
		clients      []string
		rules        config.AuthzCfg
		upstreamAddr string
		want         []bool
	}{
		{
			description: "happy path",
			clients:     []string{"client a", "client b"},
			rules: config.AuthzCfg{
				Rules: []string{
					"client a - allow - 127.0.0.1:8000",
					"client b - deny - 127.0.0.1:8000",
				},
			},
			upstreamAddr: "127.0.0.1:8000",
			want:         []bool{true, false},
		},
		{
			description: "empty rules. allow access",
			clients:     []string{"client a", "client b"},
			rules: config.AuthzCfg{
				Rules: []string{},
			},
			upstreamAddr: "127.0.0.1:8000",
			want:         []bool{true, true},
		},
		{
			description: "if no rules matches, allow access by default",
			clients:     []string{"client a", "client b", "client c"},
			rules: config.AuthzCfg{
				Rules: []string{
					"client a - allow - 127.0.0.1:8000",
					"client b - deny - 127.0.0.1:8000",
				},
			},
			upstreamAddr: "127.0.0.1:8000",
			want:         []bool{true, false, true},
		},
		{
			description: "if multiple rules matches, select the first one",
			clients:     []string{"client a", "client a"},
			rules: config.AuthzCfg{
				Rules: []string{
					"client a - allow - 127.0.0.1:8000",
					"client a - deny - 127.0.0.1:8000",
				},
			},
			upstreamAddr: "127.0.0.1:8000",
			want:         []bool{true, true},
		},
	}

	for _, tc := range tests {
		a, err := New(tc.rules)
		if err != nil {
			t.Errorf("%v, %v", tc.clients, tc.rules)
		}
		for i, client := range tc.clients {
			if got := a.Allows(client, tc.upstreamAddr); got != tc.want[i] {
				t.Errorf("%s, (%v) = %v", tc.description, got, tc.want[i])
			}
		}
	}
}
