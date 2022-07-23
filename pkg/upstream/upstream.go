// upstream package provide APIs to create fake upstreams
package upstream

type Upstream struct {
	Host          string
	Port          string
	NumActiveConn int
	IsAlive       bool
	stop          chan bool
}
