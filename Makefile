GOARCH = amd64
ARTIFACTORY_ENV := ./vault/artifactory.env
ARTIFACTORY_SCOPE ?= applied-permissions/groups:readers
ARTIFACTORY_URL ?= http://localhost:8082
JFROG_ACCESS_TOKEN ?= $(shell TOKEN_USERNAME=$(TOKEN_USERNAME) ARTIFACTORY_URL=$(ARTIFACTORY_URL) ./scripts/getArtifactoryAdminToken.sh)
TOKEN_USERNAME ?= vault-admin
UNAME = $(shell uname -s)
VAULT_TOKEN ?= $(shell printenv VAULT_TOKEN || echo 'root')
VAULT_ADDR ?= $(shell printenv VAULT_ADDR || echo 'http://localhost:8200')

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

disable:
	vault secrets disable artifactory

enable:
	vault secrets enable artifactory

upgrade: build
	vault plugin register -sha256=$$(sha256sum ./vault/plugins/artifactory | cut -d " " -f 1) secret artifactory
	vault plugin reload -plugin=artifactory

clean:
	rm -f ./vault/plugins/artifactory

fmt:
	go fmt $$(go list ./...)

setup: disable enable admin testrole

admin:
	vault write artifactory/config/admin url=$(ARTIFACTORY_URL) access_token=$(JFROG_ACCESS_TOKEN)
	vault read artifactory/config/admin
	vault write -f artifactory/config/rotate
	vault read artifactory/config/admin

testrole:
	vault write artifactory/roles/test scope="$(ARTIFACTORY_SCOPE)" max_ttl=3h default_ttl=2h
	vault read artifactory/roles/test
	vault read artifactory/token/test

artifactory: $(ARTIFACTORY_ENV)

$(ARTIFACTORY_ENV):
	@./scripts/run-artifactory-container.sh | tee $(ARTIFACTORY_ENV)

stop_artifactory:
	source $(ARTIFACTORY_ENV) && docker stop $$ARTIFACTORY_CONTAINER_ID
	rm -f $(ARTIFACTORY_ENV)

.PHONY: build clean fmt start disable enable test setup admin testrole artifactory stop_artifactory
