package healthcheck

import (
	"tcploadbalancer/config"
	u "tcploadbalancer/pkg/upstream"
	"time"

	log "github.com/sirupsen/logrus"
)

type HealthChecker struct {
	UnhealthyUpstreams  chan *u.Upstream
	HealthyUpstreams    chan *u.Upstream
	stop                chan bool
	healthCheckInterval time.Duration
	timeout             time.Duration
	doctors             []*Doctor
}

func New(cfg config.HealthCheckCfg) *HealthChecker {

	h := HealthChecker{
		HealthyUpstreams:    make(chan *u.Upstream),
		UnhealthyUpstreams:  make(chan *u.Upstream),
		stop:                make(chan bool),
		healthCheckInterval: cfg.HealthCheckInterval,
		timeout:             cfg.Timeout,
		doctors:             []*Doctor{},
	}
	return &h
}

func (h *HealthChecker) Start(upstreams []*u.Upstream) {
	for i := range upstreams {
		doctor := &Doctor{
			upstream:            upstreams[i],
			stop:                make(chan bool),
			healthCheckInterval: h.healthCheckInterval,
			timeout:             h.timeout,
			unhealthyUpstreams:  h.UnhealthyUpstreams,
			healthyUpstreams:    h.HealthyUpstreams,
		}
		h.doctors = append(h.doctors, doctor)
		doctor.Start()
		h.doctors = append(h.doctors, doctor)
	}
	log.Info("health checker started!")
	go func() {
		for {
			select {
			case <-h.stop:
				for i := range h.doctors {
					h.doctors[i].Stop()
				}
				h.doctors = []*Doctor{}
				log.Info("health checker stopped")
				return
			default:
			}

		}
	}()
}

func (h *HealthChecker) Stop() {
	h.stop <- true
}
