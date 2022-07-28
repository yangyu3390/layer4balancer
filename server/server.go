package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"sync"
	"tcploadbalancer/config"
	"tcploadbalancer/pkg/authz"
	"tcploadbalancer/pkg/balance"
	"tcploadbalancer/pkg/healthcheck"
	"tcploadbalancer/pkg/ratelimit"
	u "tcploadbalancer/pkg/upstream"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	// Buffer size to handle data from socket
	BUFFER_SIZE = 1024
)

type Server struct {
	listener         net.Listener
	upstreams        []*u.Upstream
	connectReq       chan *tls.Conn
	disconnectReq    chan net.Conn
	loadBalancingReq chan selectUpstreamReq
	tlsConfig        *tls.Config
	rateLimiter      *ratelimit.RateLimiter
	balancer         balance.LoadBalancer
	healthChecker    *healthcheck.HealthChecker
	timeout          time.Duration
	bind             string
	clientsConn      map[string]net.Conn
	stop             chan bool
}

type selectUpstreamReq struct {
	res      chan *u.Upstream
	clientId string
}

func New(cfg config.ServerCfg) (*Server, error) {

	var err error = nil

	caCertFile, err := ioutil.ReadFile(cfg.TlsCfg.CaPath)
	if err != nil {
		log.Error("error reading CA certificate:", err)
	}
	cfg.TlsCfg.CaCertPool = x509.NewCertPool()
	cfg.TlsCfg.CaCertPool.AppendCertsFromPEM(caCertFile)

	authzScheme, err := authz.New(cfg.AuthzCfg)
	if err != nil {
		log.Error("failed to create new Authz scheme", err)
		return nil, err
	}

	tlsConfig, err := makeTlsConfig(&cfg.TlsCfg)
	if err != nil {
		log.Error("failed to create new TLS config", err)
		return nil, err
	}
	// Create server
	server := &Server{
		disconnectReq:    make(chan net.Conn),
		connectReq:       make(chan *tls.Conn),
		loadBalancingReq: make(chan selectUpstreamReq),
		upstreams:        cfg.Upstreams,
		balancer:         balance.New(authzScheme),
		rateLimiter:      ratelimit.New(cfg.RateLimiterCfg),
		healthChecker:    healthcheck.New(cfg.HealthCheckCfg),
		timeout:          cfg.Timeout,
		bind:             cfg.Bind,
		tlsConfig:        tlsConfig,
		stop:             make(chan bool),
	}

	return server, nil
}

func (s *Server) Start() error {

	var err error

	if err != nil {
		return err
	}
	// Start rate limiter
	s.rateLimiter.Start()

	// Start health checker
	s.healthChecker.Start(s.upstreams)

	go func() {

		for {
			select {

			case client := <-s.connectReq:
				s.handleClientConnect(client)

			case upstream := <-s.healthChecker.UnhealthyUpstreams:
				s.markUnhealthyUpstream(upstream)

			case upstream := <-s.healthChecker.HealthyUpstreams:
				s.markHealthyUpstream(upstream)

			case req := <-s.loadBalancingReq:
				s.handleBalancingReq(req)

			case <-s.stop:
				s.rateLimiter.Stop()
				s.healthChecker.Stop()
				if s.listener != nil {
					s.listener.Close()
					//TODO: check active connections and close them
				}
				return
			}
		}
	}()

	// Start listening
	if err := s.Listen(); err != nil {
		log.Error("failed to listen", err)
		s.Stop()
		return err
	}
	log.Info("Load balancer server started!")
	return nil
}

func (s *Server) handleClientConnect(client *tls.Conn) {
	go s.handle(client)
}

func (s *Server) Stop() {
	s.stop <- true
}

func (s *Server) Listen() (err error) {

	s.listener, err = net.Listen("tcp", s.bind)
	if err != nil {
		log.Error("error in net.Listen", err)
		return err
	}

	go func() {
		for {
			if s.listener == nil {
				break
			}
			conn, err := s.listener.Accept()
			if err != nil {
				log.Error("error in listener accept ", err)
				return
			}

			clientConn := tls.Server(conn, s.tlsConfig)
			s.connectReq <- clientConn
		}
	}()

	return nil
}

func (s *Server) handle(clientConn *tls.Conn) {

	clientConn.Handshake()
	defer clientConn.Close()
	if len(clientConn.ConnectionState().PeerCertificates) == 0 {
		log.Info("no peer certificate. client connection may be closed")
		return
	}
	clientId := clientConn.ConnectionState().PeerCertificates[0].Subject.CommonName

	req := selectUpstreamReq{
		res:      make(chan *u.Upstream, 1),
		clientId: clientId,
	}
	s.loadBalancingReq <- req
	upstream := <-req.res
	if upstream == nil {
		return
	}

	if s.rateLimiter.Allows(clientId) == false {
		return
	}
	upstreamAddr := upstream.Host + ":" + upstream.Port
	log.Info("Balancer: ", "select upstream ", upstreamAddr)

	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, 1*time.Second)

	// if attemp to connect to the upstream fails, put it into the UnhealthyUpstreams channel
	if err != nil {
		log.Info("find an unhealthy upstream during regular LB operation", upstreamAddr)
		s.healthChecker.UnhealthyUpstreams <- upstream
		return
	}
	defer upstreamConn.Close()
	defer func() {
		// update number of connection
		upstream.NumActiveConn--
	}()
	wg := new(sync.WaitGroup)
	wg.Add(2)
	go proxy(upstreamConn, clientConn, clientId, upstreamAddr, "-> lb ->", wg)
	go proxy(clientConn, upstreamConn, clientId, upstreamAddr, "<- lb <-", wg)
	wg.Wait()
}

func proxy(to net.Conn, from net.Conn, clientId, upstreamAddr, direction string, wg *sync.WaitGroup) {

	buf := make([]byte, BUFFER_SIZE)
	// TODO: set read write deadline
	for {
		nRead, errRead := from.Read(buf)
		if nRead > 0 {
			nWrite, errWrite := to.Write(buf[0:nRead])
			if errWrite != nil {
				log.Error("error write to upstream ", errWrite)
				break
			}

			if nRead != nWrite {
				log.Error("error short write to upstream ", io.ErrShortWrite)
				break
			}
		}

		if errRead == io.EOF {
			break
		}

		if errRead != nil {
			log.Error("error reading from client ", errRead)
			break
		}
	}
	l := fmt.Sprintf("%s %s upstream %s ", clientId, direction, upstreamAddr)
	log.Printf(l)
	wg.Done()
}

func (s *Server) markUnhealthyUpstream(upstream *u.Upstream) {
	if upstream == nil {
		log.Error("unhealthy upstream is nil")
		return
	}
	if s.upstreams == nil {
		log.Error("empty upstream list")
		return
	}
	idx := -1
	for i := range s.upstreams {
		if s.upstreams[i] == upstream {
			idx = i
			break
		}
	}
	// upstream is not found
	if idx == -1 {
		log.Info("unhealthy upstream not found in upstream list")
		return
	}

	s.upstreams[idx].IsAlive = false
	log.Info("find an unhealthy upstream", upstream.Host+":"+upstream.Port)
}

func (s *Server) markHealthyUpstream(upstream *u.Upstream) {
	if upstream == nil {
		log.Error("healthy upstream is nil")
		return
	}
	if s.upstreams == nil {
		log.Error("empty upstream list")
		return
	}
	idx := -1
	for i := range s.upstreams {
		if s.upstreams[i] == upstream {
			idx = i
			break
		}
	}
	// upstream is not found
	if idx == -1 {
		log.Info("upstream not found in upstream list")
		return
	}
	if s.upstreams[idx].IsAlive == false {
		s.upstreams[idx].IsAlive = true
		log.Info("unhealthy upstream becomes healthy", upstream.Host+":"+upstream.Port)
	}
}

func (s *Server) handleBalancingReq(req selectUpstreamReq) {
	upstream, err := s.balancer.Select(req.clientId, s.upstreams)
	if err != nil {
		req.res <- nil
	} else {
		upstream.NumActiveConn++
		req.res <- upstream
	}

}

func makeTlsConfig(tlsCfg *config.TlsCfg) (*tls.Config, error) {

	tlsConfig := &tls.Config{
		ClientCAs:                tlsCfg.CaCertPool,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		},
	}

	var crt tls.Certificate
	var err error
	if crt, err = tls.LoadX509KeyPair(tlsCfg.CertPath, tlsCfg.KeyPath); err != nil {
		return nil, err
	}

	tlsConfig.Certificates = []tls.Certificate{crt}

	return tlsConfig, nil
}
