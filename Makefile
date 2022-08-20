NAME := tcploadbalancer

default: test

test:
	@go test -v --race ./...