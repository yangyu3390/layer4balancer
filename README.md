# layer4proxy
Layer 4 TCP proxy


## Prerequiste

Install go1.18


## Features

- Load balancing

- Health checking

- Rate limiting

- mTLS authentication

- Authorization


## Design doc

Design doc can be found [here](https://github.com/yangyu3390/layer4balancer/blob/main/doc/0000-tcp-load-balancer.md) 


## Certificates

To generate CA and certificates, run `go run certs/makecerts.go`

By default it will generate one CA, one certificate for load balancing server, and five certificates for clients.

To generate certificates for more clients, modify `certs/client_certs.json` file, add more client id in the file and re-run the command.


## Test


### Unit test

run `make test`


## Simulation

Below is how the simulation works:

A client sends `Hello from [client name]` to load balancing server, lb server picks an upstream and 
forward the msg to the upstream. 
The upstream receives the msg, prints it out and replies with `Reply from upstream [upstream address]` to lb server.
lb server forwards the msg to the client. The client receives the msg and prints it out.

**upstream**

run `go run cmd/upstream/main.go -a [upstream addr]` to start an upstream.

For example, `go run cmd/upstream/main.go -a "localhost:8001"` will start an upstream with address localhost:8001.

You can start multiple upstreams by running the command multiple times with different address.

**server**

run `go run cmd/server/main.go` to start the load balancing server

**client**

run `go run cmd/client/main.go -c [client id]` to start a client

For example, `go run cmd/client/main.go -c b` will start an client with client id `client.b`

A client will send requests every x ms, wherer x is a random value between 200ms ~ 1000ms. 

You can start multiple clients by running the command multiple times with different client id.

Do rememeber to generate certificates for those clients.



## Configuration


Configuration is initialised in `config/config.go`

Some key parameters are listed as following. 

If you want to try other parameters, simply modify the file and restart the server.

Health check interval is 3 seconds, meaning a docker checks an upstream every 3 seconds.
A docker tries to connect to the upstream within 1 second. If timeout, the upstream is marked as unhealthy.


```go
healthCheckCfg := HealthCheckCfg{
    HealthCheckInterval: 3 * time.Second,
    Timeout:             1 * time.Second,
}
```

Rate limiter allows each client an average of 2 requests per second, with a maximum of 4 requests in a single burst. 
The background gorouine will do clean up every 20 seconds.

```go
rateLimiterCfg := RateLimiterCfg{
    CleanupInterval: 20 * time.Second,
    Burst:           2,
    Token:           4,
}
```

Simple authorization rules are defined as below. 
If no rules matches, by default, a client is allowed to access any upstreams.

```go
authzCfg := AuthzCfg{
    Rules: []string{
        "client.a-deny-127.0.0.1:8000",
        "client.b-allow-127.0.0.1:8000",
        "client.c-deny-127.0.0.1:8000",
        "client.d-allow-127.0.0.1:8000",
        "client.e-allow-127.0.0.1:8000",

        "client.a-allow-127.0.0.1:8001",
        "client.b-allow-127.0.0.1:8001",
        "client.c-deny-127.0.0.1:8001",
        "client.d-allow-127.0.0.1:8001",
        "client.e-allow-127.0.0.1:8001",

        "client.a-allow-127.0.0.1:8002",
        "client.b-allow-127.0.0.1:8002",
        "client.c-deny-127.0.0.1:8002",
        "client.d-allow-127.0.0.1:8002",
        "client.e-allow-127.0.0.1:8002",
    },
}
```
