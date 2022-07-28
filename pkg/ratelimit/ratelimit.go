package ratelimit

import (
	"layer4balancer/config"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type client struct {
	numReq int
	// window index for the current request.
	// This breaks down the time into discrete windows of RateLimiterCfg.Window width
	windowIdx int
	lastSeen  time.Time
}

type RateLimiter struct {
	clients         map[string]*client
	stop            chan bool
	cleanupInterval time.Duration
	limit           int
	window          time.Duration
	mu              sync.Mutex
}

func New(cfg config.RateLimiterCfg) *RateLimiter {

	return &RateLimiter{
		clients:         make(map[string]*client),
		stop:            make(chan bool),
		cleanupInterval: cfg.CleanupInterval,
		limit:           cfg.Limit,
		window:          cfg.Window,
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
	currWindowIdx := int(time.Now().UnixMilli() / int64(r.window/time.Millisecond))

	// new client
	if _, found := r.clients[clientId]; !found {
		r.clients[clientId] = &client{
			numReq:    0,
			windowIdx: currWindowIdx,
			lastSeen:  time.Now(),
		}
	}

	// if windowidx is smaller than the current window index, then we know it is not rate limited.
	if r.clients[clientId].windowIdx < currWindowIdx {
		r.clients[clientId].windowIdx = currWindowIdx
		r.clients[clientId].numReq = 0
		r.clients[clientId].lastSeen = time.Now()
	} else {
		r.clients[clientId].numReq++
		r.clients[clientId].lastSeen = time.Now()
	}

	if r.clients[clientId].numReq > r.limit {
		log.Info("Rate limiter: ", clientId, " is rate limited")
		return false
	}

	return true
}
