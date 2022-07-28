package main

import (
	"flag"
	"net"

	log "github.com/sirupsen/logrus"
)

func main() {
	addr := flag.String("a", "localhost:8000", "upstream addr")
	flag.Parse()

	go handle(*addr)

	block := make(chan bool)
	<-block
}

func handle(addr string) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("upstream started ", addr)
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// handle incoming data
		buffer := make([]byte, 1024)
		numBytes, err := c.Read(buffer)
		if err != nil {
			log.Error(err)
			c.Close()
			return
		}
		log.Println(string(buffer[:numBytes]))

		// handle reply
		msg := "Reply from upstream " + addr

		_, err = c.Write([]byte(msg))
		if err != nil {
			log.Error(err)
			c.Close()
			return
		}
		c.Close()
	}
}
