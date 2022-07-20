package balance

import (
	"errors"
	a "tcploadbalancer/pkg/authz"
	u "tcploadbalancer/pkg/upstream"

	log "github.com/sirupsen/logrus"
)

type LoadBalancer interface {
	Select(clientId string, upstreams []*u.Upstream) (*u.Upstream, error)
}

type LeastConnectionBalancer struct {
	authz a.AuthzScheme
}

func New(authz a.AuthzScheme) LoadBalancer {
	return &LeastConnectionBalancer{
		authz: authz,
	}
}

// Select upstream server using Least connection strategy
func (s *LeastConnectionBalancer) Select(clientId string, upstreams []*u.Upstream) (*u.Upstream, error) {

	if len(upstreams) == 0 {
		log.Error("zero upstreams")
		return nil, errors.New("zero upstreams")
	}

	var leastConnectionUpstream *u.Upstream

	for idx := range upstreams {
		upstreamAddr := upstreams[idx].Host + ":" + upstreams[idx].Port
		// check authz rules
		if upstreams[idx].IsAlive == false || s.authz.Allows(clientId, upstreamAddr) == false {
			continue
		}

		if leastConnectionUpstream == nil || upstreams[idx].NumActiveConn < leastConnectionUpstream.NumActiveConn {
			leastConnectionUpstream = upstreams[idx]
		}
	}

	if leastConnectionUpstream == nil {
		log.Error("No upstreams available for ", clientId)
		return nil, errors.New("No upstreams available")
	}

	return leastConnectionUpstream, nil
}
