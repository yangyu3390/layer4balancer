package main

import (
	"layer4balancer/config"
	s "layer4balancer/server"

	log "github.com/sirupsen/logrus"
)

func main() {
	serverCfg := config.InitConfig()
	server, err := s.New(serverCfg)
	if err != nil {
		log.Error("failed to create a new server", err)
		return
	}
	err = server.Start()
	if err != nil {
		log.Error("failed to start the new server", err)
	}
	block := make(chan bool)
	<-block
}
