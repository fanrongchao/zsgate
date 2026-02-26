GOCACHE ?= /tmp/go-build

.PHONY: fmt build run-control run-data

fmt:
	GOCACHE=$(GOCACHE) gofmt -w $$(find . -name '*.go')

build:
	GOCACHE=$(GOCACHE) go build ./control-plane/cmd/server
	GOCACHE=$(GOCACHE) go build ./data-plane/cmd/server

run-control:
	GOCACHE=$(GOCACHE) go run ./control-plane/cmd/server

run-data:
	GOCACHE=$(GOCACHE) CONTROL_PLANE_URL=http://localhost:8081 go run ./data-plane/cmd/server
