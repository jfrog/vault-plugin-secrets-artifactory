GOARCH = amd64

UNAME = $(shell uname -s)

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

.DEFAULT_GOAL := all

all: fmt build start

build:
	GOOS=$(OS) GOARCH="$(GOARCH)" go build -o vault/plugins/artifactory cmd/artifactory/main.go

start:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=./vault/plugins -log-level=DEBUG

enable:
	vault secrets enable artifactory

clean:
	rm -f ./vault/plugins/artifactory

fmt:
	go fmt $$(go list ./...)

setup:	enable
	vault write artifactory/config/admin  url=http://localhost:8080 access_token=access_token max_ttl=5m default_ttl=2m
	vault read artifactory/config/admin
	vault write artifactory/roles/test scope="scope goes here" username="unsure" max_ttl=5m default_ttl=2m refreshable=true
	vault read artifactory/roles/test

.PHONY: build clean fmt start enable setup
