// client package provides APIs to create fake clients
package client

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

const relpath string = "/certs/"

type Client struct {
	ClientId string
}

func New(clientId string) *Client {
	return &Client{
		ClientId: clientId,
	}
}

func (c *Client) Start() {
	pwd, err := os.Getwd()
	caPath := pwd + relpath + "ca.crt"

	ca, err := ioutil.ReadFile(caPath)
	if err != nil {
		log.Fatalf("could not open certificate file: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(ca)

	clientCert := pwd + relpath + c.ClientId + ".crt"
	clientKey := pwd + relpath + c.ClientId + ".key"
	log.Info("Load key pairs - ", clientCert, clientKey, c.ClientId)
	certificate, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		log.Fatalf("could not load certificate: %v", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      caCertPool,
	}

	ch := make(chan string, 1)

	for {
		go handle(tlsCfg, ch, c.ClientId)

		select {
		case text := <-ch:
			log.Info(text)
		default:
		}

		rand.Seed(time.Now().UnixNano())
		r := rand.Intn(800) + 200
		time.Sleep(time.Duration(r) * time.Millisecond)
	}
}

func handle(tlsCfg *tls.Config, ch chan string, clientId string) {

	c, err := tls.Dial("tcp", "localhost:1234", tlsCfg)
	defer c.Close()
	if err != nil {
		log.Fatalf("client dial error: %s", err)
	}

	msg := []byte("Hello from " + clientId)
	_, err = c.Write(msg)
	if err != nil {
		log.Fatal("error write ", err)
	}
	buffer := make([]byte, 1024)
	n, err := c.Read(buffer)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Fatal("error read ", err)
	}
	ch <- string(buffer[:n])
}
