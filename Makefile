GOARCH = amd64
ACCESS_TOKEN ?= $(shell printenv ACCESS_TOKEN || echo 'access_token')
ARTIFACTORY_URL ?= $(shell printenv ARTIFACTORY_URL || echo 'http://localhost:8080')
ARTIFACTORY_SCOPE ?= $(shell printenv ARTIFACTORY_SCOPE || echo 'applied-permissions/groups:readers')

UNAME = $(shell uname -s)

ifndef OS
	ifeq ($(UNAME), Linux)
		OS = linux
	else ifeq ($(UNAME), Darwin)
		OS = darwin
	endif
endif

.DEFAULT_GOAL := all

all: fmt build test start

build:
	GOOS=$(OS) GOARCH="$(GOARCH)" go build -o vault/plugins/artifactory cmd/artifactory/main.go

start:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=./vault/plugins -log-level=DEBUG

test:
	go test -v ./...

enable:
	vault secrets enable artifactory

clean:
	rm -f ./vault/plugins/artifactory

fmt:
	go fmt $$(go list ./...)

setup:	enable
	vault write artifactory/config/admin url=$(ARTIFACTORY_URL) access_token=$(ACCESS_TOKEN)
	vault read artifactory/config/admin
	vault write artifactory/roles/test scope="$(ARTIFACTORY_SCOPE)" username="test-user" max_ttl=3h default_ttl=2h
	vault read artifactory/roles/test

artifactory:
	cat test/http-create-response.txt | nc -l 8080

.PHONY: build clean fmt start enable test setup artifactory
