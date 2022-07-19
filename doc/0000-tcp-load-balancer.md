
---

authors: Yang Yu (yangyu3390@gmail.com)

state: implemented

---

  

# Request For Discussion 0 - TCP load balancer



## What

  

Design and implement a basic TCP load balancer that is used to distribute network load across multiple upstreams.

 

## Assumptions and scope

- Assume all the data can fit in memory. No database is required.

- Service discovery is not required. A pre-configured list of upstreams is created during initialisation. In practice, we can use services like `Consul` as a centralized registry that discovers services. 

- For sake of simplicity, TLS is not required between the Load balancer and upstreams.

- High availability is beyond the scope of this design. In practice, to avoid single point of failure problem, we can set up multiple load balancers and use DNS to load balance between them. A solution provided by Nginx Plus can be found [here](https://www.nginx.com/resources/glossary/dns-load-balancing/).

## Details

### Load balancer

A least connections strategy is used tracks the number of connections per upstream.
The alive upstream with least connections will be selected to accept forwarded requests.
If multiple upstreams have the same least connections, select the first one.
To determine whether an upstream is alive or not, please refer to the `Health checker` section.

**Trade off**

The algorithm uses `list` data structure to store upstreams. It iterates the list and selects the one with the least connections. The time complexity is O(N) where N is the number of upsteams.

One improvement can be made is to use `priority queue` data structure. Each element of the priority queue is associated with a priority value - the number of connections. The time complexity of selecting the least connections upstream will be O(logN).

The trade off is simplicity is preferred over efficiency. `priority queue` is more efficient but it is a bit more complicated. Since N is not huge, the improvement from O(N) to O(log(N)) is negligible. Therefore list data structure will be used here.

**API**

`LoadBalancer` interface contains a `Select` method which is used to select an upstream service based on the load balancing strategy. In this implementation, we are going to use least-connection strategy. However, any strategy that implements the interface can be used in the load balancer.
```go
type  LoadBalancer  interface {
	Select(upstreams []*u.Upstream)	 (*u.Upstream, error)
}
```
  

### Rate limiter

Rate limiter is used to track the number of client connections.

Client identity is defined by `common name` which can be retrieved from TLS certificate.
Each client has an associated rate limiter. For the sake of simplicity, a simple fixed window rate limiter will be implemented.

Each time a new client makes a request to our API, a new rate limiter will be initialised and added to the map. For any subsequent requests, the client’s rate limiter will be retrieved from the map and check whether the request is allowed by calling its `RateLimit()` method.

Since multiple goroutines may access the map concurrently, a mutex is required to prevent race conditions.

the clients map will grow indefinitely, taking up more and more resources with every new client and rate limiter.
To prevent this, a background goroutine periodically delete any clients that haven’t been seen for a long time.
To make this work, `lastSeen` field is added in `client` struct. This field is updated whenever a client makes a permitted request.

The `client` and `RateLimiter` structs are defined as following.

```go
type RateLimiterCfg struct {
	CleanupInterval time.Duration
	Limit           int
	Window          time.Duration
}

type client struct {
	numReq   int
	windowIdx int  // window index for the current request. 
				   // This breaks down the time into doscreet windows of  RateLimiterCfg.Window width
	lastSeen time.Time
}

type RateLimiter struct {
	...
	clients map[string]*client
	mu sync.Mutex
	...
}
```


**API**

`New` method will initialise a rate limiter struct.
```go
func  New(cfg config.RateLimiterCfg) *RateLimiter
```

`Start` method will create a ticker and a goroutine that scan the `clients` map regularly. 
```go
func (r *RateLimiter) Start()
```
  
 `cleanup` method deletes the old clients when new check interval has reached.
```go
func (r *RateLimiter) cleanup()
```

`Stop` method stops the goroutine and the ticker.
```go
func (r *RateLimiter) Stop()
```

`ratelimit` method determines whether a request is permitted or not.
```go
func (r *RateLimiter) RateLimit(clientId string) bool
```

### Health checker

Health checker is used to check healthy status for all upstreams on a regular basis.

Unhealthy upstream and healthy upstream are defined as following:
An unhealthy upstream means the attempt to connect to the upstream times out.
A healthy upstream means the attempt to connect to the upstream succeeds.

In the implementation, `list` data structure is used to store upstreams. `HealthChecker` needs to scan the list of upstreams and mark unhealthy upstreams. `Balancer` also needs to read the list to select the upstream with least-connections. 
The main difficulty is how we can avoid data races between the two operations.
The solution I am proposing is to use two channels to pass the list of upstreams between `Balancer` and `HealthChecker`.

When load balancer server starts, it starts the health checking service. The health checking service then launches N doctors for N upstreams. One doctor monitors one upstream. Every doctor has a ticker and a goroutine running inside. A doctor tries to set up a TCP connection with its associated upstream when a new interval has reached. If the connection is not set up successfully within a timeout period, then the upstream is considered unhealthy and will be sent to  `unhealthyUpstreams` channel.

Health Checker will not remove unhealthy upstreams. Doctors still poll the status of those unhealthy upstreams. If they become healthy, doctors will send them to `healthyUpstreams` channel and they will be available for load balancing.

As can be seen below, `Doctor` struct contains and only contains one `upstream`. Both HealthChecker  and doctors share the same `unhealthyUpstreams` channel and `healthyUpstreams` channel.
 
```go
type Upstream struct {
	...
	Host          string
	Port          string
	NumActiveConn int
	IsAlive       bool
	...
}

type HealthCheckCfg struct {
	HealthCheckInterval string
	Timeout             string
}

type  HealthChecker  struct {
	...
	UnhealthyUpstreams chan *u.Upstream
	HealthyUpstreams chan *u.Upstream
	...
}

type  Doctor  struct {
	...
	upstream *u.Upstream
	unhealthyUpstreams chan *u.Upstream
	healthyUpstreams chan *u.Upstream
	...
}
```
Since health checker and all doctors share the same channels, the unhealthy streams sent to doctors' `unhealthyUpstreams` channel are sent to health checker's `UnhealthyUpstreams` as well, same as `healthyUpstreams` channel. In the `server.Start()` method, a goroutine with infinite loop is created to monitor the `UnhealthyUpstreams` channel, `HealthyUpstreams` channel and `loadBalancingReq` channel. The `IsAlive` field of the elements in `UnhealthyUpstreams` channel will be set to false. Similiar to the lements in `HealthyUpstreams` channel, the `IsAlive` field is set to true. In this way, the upstream list is shared by communicating between channels. No locks is required when reading or updating the upstream list.


```go
// server.Start() method
for {
	select {
	...
	case  upstream := <-s.HealthChecker.UnhealthyUpstreams:
	// update the upstream list by marking the upstream as unhealthy in the list
	...

	case  upstream := <-s.HealthChecker.HealthyUpstreams:
	// update the upstream list by marking the upstream as healthy in the list
	...

	case  req := <-s.loadBalancingReq:
	// scan the upstream list to select the alive upstream with least connections.
	...
	}
}
```

**Trade off**

The alternative is we can completely remove the unhealthy upstreams and rely on a service discovery service to detect new upstreams. Once an upstream is detected, LB server will add them into the upstream list. Since service discovery service is not required, this solution is beyond the scope of the design.


**Doctor API**

`Start` method creates a ticker and a goroutine that tries to poll the healthy status of the corresponding upstream  by setting up a TCP connection regularly.
```go
func (d *Doctor) Start()
```

`Stop` method stops the ticker and the goroutine.
```go
func (d *Doctor) Stop()
```
`check` method polls the status of an upstream service. It tries to set up a connection with the upstream service if connection is successful without timeout, close the connection.
If timeout, mark the upstream as unhealthy, push the result to `unhealthyUpstreams` channel.
```go
func (d *Doctor) check()
```
 
**HealthChecker API**

`New` method initialises a `HealthChecker` object.
```go
func  New(cfg config.HealthCheckCfg) *HealthChecker
```

`Start` method creates and launches doctors for upstreams.
```go
func (h *HealthChecker) Start(upstreams []*u.Upstream)
```

`Stop` method stops all the active doctors and the health checker.
```go
func (h *HealthChecker) Stop()
```

### mTLS authN

In mTLS, both the client and server have its own certificate, and both sides need to authenticate using their public/private key pair.

To implement mTLS, the following steps need to be executed.

- generate a CA certificate to sign all of our certificates using the x509 package
the key thing is to set `IsCA` to true.
```go
caCert := &x509.Certificate{
	SerialNumber:          ...,
	Subject:               ...,
	NotBefore:             time.Now(),
	NotAfter:              time.Now().AddDate(365, 0, 0),
	IsCA:                  true, // <- indicating this certificate is a CA certificate.
	KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
}
 ```

- generate RSA public/private key pairs
```go
caKey, err := rsa.GenerateKey(rand.Reader, 2048)
```

- create CA certificate
```go
caBytes, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caKey.PublicKey, caKey)
```
- store the certificate in PEM format 

- now that we have root CA certificate, we can use it to generate server certificate and client certificate

- after generating server certificates and client certificates, we can use them create the TLS Config. The key thing is to set `ClientAuth` to `tls.RequireAndVerifyClientCert`. 

The TLS config is defined as below.
The selected cipher suites are based on Microsoft's recommendation https://docs.microsoft.com/en-us/power-platform/admin/server-cipher-tls-requirements
The cipher suites that are dropped in http2 spec are not included in the supported cipher suites. https://datatracker.ietf.org/doc/html/rfc7540#appendix-A.

```go
tlsConfig := &tls.Config{
	ClientCAs: caCertPool,
	ClientAuth: tls.RequireAndVerifyClientCert,
	MinVersion: tls.VersionTLS12,
	CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
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
```

Clients use `tls.DialWithDialer` to connect to the load balancer and initiate a TLS handshake.


### AuthZ scheme

The authorisation scheme uses `common name` to define client identities. `common name` can be retrieved from TLS certificate.
Authorisation rules are hard-coded in the implementation. 


The structs are defined as below.

```go
type AuthzRule struct {
	isAllowed    bool
	commonName   string
	upstreamAddr string // ip:port
}

type AuthzScheme struct {
	Rules []AuthzRule
}

type  AuthzRule  struct {
	isAllowed bool
	commonName string
	upstream string
}
```

An example rule will look like this:
```
"client a" "allow" "127.0.0.1:8000" // client a is allowed to access upstream 127.0.0.1:8000
"client a" "deny" "127.0.0.1:8001" // client a is denied to access upstream 127.0.0.1:8001
"client b" "deny" "127.0.0.1:8000" // client b is denied to access upstream 127.0.0.1:8000
```


**API**

`New` method creates an authorisation scheme.
```go
func  New(cfg config.AuthzCfg) (*AuthzScheme, error)
```

`Allows` method checks whether `commonName` matches any authorisation rule. If no matches is found, access is allowed.
```go
func (a *AuthzScheme) Allows(commonName string) bool
```


### Accept and forward requests to upstreams using library

After the mTLS handshake is succesfully established, the load balancer server launches a goroutine to handle connection requests, perform load balancing and mark unhealthy upstreams as unhealthy.

```go
// server.Start() method
for {
	select {

	case ctx := <-s.connectReq:
		// handle client connetion requests
		// use authZ and ralelimiter to determine whether requests are permitted.
		// forward the requests to upstreams
		// send a request to disconnectReq channel 
		...
 
	case  upstream := <-s.HealthChecker.UnhealthyUpstreams:
		// update the upstream list by marking the upstream as unhealthy in the list
		...

	case  upstream := <-s.HealthChecker.HealthyUpstreams:
		// update the upstream list by marking the upstream as healthy in the list
		...

	case req := <-s.loadBalancingReq:
		// scan the upstream list to select the upstream with least connections.
		...

	case <-s.stop:
		// stops everything
		...
	}
}
```


### CLI example

To start upstreams, run `go run cmd/upstream/main.go -a <upstream addr, e,g, "localhost:8000">`

To start the load balancer server, run `go run cmd/server/main.go`

To start clients, run `go run cmd/client/main.go -c <client name, e.g. a, b, c. by default a>`. 


