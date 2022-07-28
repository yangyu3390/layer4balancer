package healthcheck

import (
	u "layer4balancer/pkg/upstream"
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

type Doctor struct {
	upstream            *u.Upstream
	stop                chan bool
	healthCheckInterval time.Duration
	timeout             time.Duration
	unhealthyUpstreams  chan *u.Upstream
	healthyUpstreams    chan *u.Upstream
}

func (d *Doctor) Start() {
	ticker := time.NewTicker(d.healthCheckInterval)

	go func() {
		for {
			select {

			case <-ticker.C:
				go d.check()

			case <-d.stop:
				ticker.Stop()
				return
			default:
			}
		}
	}()
}

func (d *Doctor) Stop() {
	d.stop <- true
}

// check polls the status of an upstream service
// it tries to set up a connection with the upstream service
// if connection is successful without timeout, close the connection
// if timeout, mark the upstream as unhealthy, push the result to unhealthyUpstreams channel
func (d *Doctor) check() {

	address := d.upstream.Host + ":" + d.upstream.Port
	conn, err := net.DialTimeout("tcp", address, d.timeout)
	if err != nil {
		d.unhealthyUpstreams <- d.upstream
	} else {
		message := "Health checker: Hello from Doctor\n"
		_, err = conn.Write([]byte(message))
		if err != nil {
			log.Error("doctor write error ", err)
		}
		d.healthyUpstreams <- d.upstream
		conn.Close()
	}
}
