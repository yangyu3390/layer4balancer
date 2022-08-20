package ratelimit

import (
	"layer4balancer/config"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type client struct {
	numReq      int
	lastSeen    time.Time
	rateLimiter *rate.Limiter
}

type RateLimiter struct {
	clients         map[string]*client
	stop            chan bool
	cleanupInterval time.Duration
	burst           int
	token           int
	mu              sync.Mutex
}

func New(cfg config.RateLimiterCfg) *RateLimiter {

	return &RateLimiter{
		clients:         make(map[string]*client),
		stop:            make(chan bool),
		cleanupInterval: cfg.CleanupInterval,
		burst:           cfg.Burst,
		token:           cfg.Token,
	}
}

func (r *RateLimiter) Start() {

	ticker := time.NewTicker(r.cleanupInterval)

	go func() {
		for {
			select {

			// new check interval has reached
			case <-ticker.C:
				go r.cleanup()

			// request to stop cleanup
			case <-r.stop:
				ticker.Stop()
				log.Info("rate limiter stopped")
				return
			}
		}
	}()
	log.Info("rate limiter started!")
}

func (r *RateLimiter) cleanup() {
	r.mu.Lock()

	for commonName, client := range r.clients {
		if time.Since(client.lastSeen) > r.cleanupInterval {
			delete(r.clients, commonName)
		}
	}
	r.mu.Unlock()
}

func (r *RateLimiter) Stop() {
	r.stop <- true
}

func (r *RateLimiter) Allows(clientId string) bool {

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, found := r.clients[clientId]; !found {
		r.clients[clientId] = &client{
			0,
			time.Now(),
			rate.NewLimiter(2, 4),
		}
	}
	return r.clients[clientId].rateLimiter.Allow()

}
