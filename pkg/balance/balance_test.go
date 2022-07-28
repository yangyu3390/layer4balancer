package balance

import (
	a "layer4balancer/pkg/authz"
	u "layer4balancer/pkg/upstream"
	"testing"
)

func TestBalancer(t *testing.T) {

	tests := []struct {
		description string
		upstreams   []*u.Upstream
		clientId    string
		authz       a.AuthzScheme
		want        int
	}{
		{
			description: "happy path",
			upstreams: []*u.Upstream{
				{
					Host:          "0.0.0.0",
					Port:          "8000",
					NumActiveConn: 10,
					IsAlive:       true,
				},
				{
					Host:          "0.0.0.1",
					Port:          "8000",
					NumActiveConn: 5,
					IsAlive:       true,
				},
				{
					Host:          "0.0.0.2",
					Port:          "8000",
					NumActiveConn: 11,
					IsAlive:       true,
				},
			},
			clientId: "ClientA",
			authz: a.AuthzScheme{
				Rules: []a.AuthzRule{
					{
						IsAllowed:    true,
						CommonName:   "ClientA",
						UpstreamAddr: "0.0.0.0:8000",
					},
					{
						IsAllowed:    false,
						CommonName:   "ClientA",
						UpstreamAddr: "0.0.0.1:8000",
					},
				},
			},
			want: 0, // even though ":8001" has least connections, but ClientA is denied to access it.
		},
		{
			description: "empty upstream list",
			upstreams:   []*u.Upstream{},
			clientId:    "ClientA",
			authz: a.AuthzScheme{
				Rules: []a.AuthzRule{},
			},
			want: -1,
		},
		{
			description: "multiple upstreams have the same least conn, pick the first one",
			upstreams: []*u.Upstream{
				{
					Host:          "127.0.0.0",
					Port:          "8000",
					NumActiveConn: 10,
					IsAlive:       true,
				},
				{
					Host:          "127.0.0.1",
					Port:          "8000",
					NumActiveConn: 5,
					IsAlive:       true,
				},
				{
					Host:          "127.0.0.1",
					Port:          "8000",
					NumActiveConn: 5,
					IsAlive:       true,
				},
			},
			clientId: "ClientA",
			authz: a.AuthzScheme{
				Rules: []a.AuthzRule{
					{
						IsAllowed:    true,
						CommonName:   "ClientA",
						UpstreamAddr: "127.0.0.0:8000",
					},
					{
						IsAllowed:    true,
						CommonName:   "ClientA",
						UpstreamAddr: "127.0.0.1:8000",
					},
				},
			},
			want: 1,
		},
	}

	for _, tc := range tests {
		lb := LeastConnectionBalancer{
			authz: tc.authz,
		}
		got, _ := lb.Select(tc.clientId, tc.upstreams)
		if tc.want == -1 {
			if got != nil {
				t.Errorf("%s, %v", tc.description, got)
			}
		} else {
			if got != tc.upstreams[tc.want] {
				t.Errorf("%s, %v != %v", tc.description, got, tc.upstreams[tc.want])
			}
		}
	}
}
