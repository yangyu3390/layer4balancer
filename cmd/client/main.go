package main

import (
	"flag"
	c "layer4balancer/pkg/client"
)

func main() {
	name := flag.String("c", "a", "client name")
	flag.Parse()

	client := c.New("client." + *name)
	go client.Start()

	block := make(chan bool)
	<-block
}
