GO_ARCH=$(shell go env GOARCH)
TARGET_ARCH=$(shell go env GOOS)_${GO_ARCH}
GORELEASER_ARCH=${TARGET_ARCH}

ifeq ($(GO_ARCH), amd64)
GORELEASER_ARCH=${TARGET_ARCH}_$(shell go env GOAMD64)
endif

CURRENT_VERSION?=$(shell git describe --tags --abbrev=0 | sed  -n 's/v\([0-9]*\).\([0-9]*\).\([0-9]*\)/\1.\2.\3/p')
NEXT_VERSION := $(shell echo ${CURRENT_VERSION}| awk -F '.' '{print $$1 "." $$2 "." $$3 +1 }' )

PLUGIN_DIR=dist/artifactory-secrets-plugin_${GORELEASER_ARCH}
PLUGIN_FILE=artifactory-secrets-plugin

ARTIFACTORY_ENV := ./vault/artifactory.env
ARTIFACTORY_SCOPE ?= applied-permissions/groups:readers
ARTIFACTORY_URL ?= http://localhost:8082
JFROG_ACCESS_TOKEN ?= $(shell TOKEN_USERNAME=$(TOKEN_USERNAME) ARTIFACTORY_URL=$(ARTIFACTORY_URL) ./scripts/getArtifactoryAdminToken.sh)
TOKEN_USERNAME ?= vault-admin
UNAME = $(shell uname -s)
VAULT_TOKEN ?= $(shell printenv VAULT_TOKEN || echo 'root')
VAULT_ADDR ?= $(shell printenv VAULT_ADDR || echo 'http://localhost:8200')

.DEFAULT_GOAL := all

all: fmt build test start

build: fmt
	GORELEASER_CURRENT_TAG=${NEXT_VERSION} goreleaser build --single-target --clean --snapshot

start:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=${PLUGIN_DIR} -log-level=DEBUG

test:
	go test -v ./...

disable:
	vault secrets disable artifactory

enable:
	vault secrets enable artifactory

register: build
	vault plugin register -sha256=$$(sha256sum ${PLUGIN_DIR}/${PLUGIN_FILE} | cut -d " " -f 1) -command=${PLUGIN_FILE} secret artifactory

deregister:
	value plugin deregister -version=${NEXT_VERSION} secret artifactory
	
upgrade: register
	vault plugin reload -plugin=artifactory

clean:
	rm -f ${PLUGIN_DIR}/${PLUGIN_FILE}

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
