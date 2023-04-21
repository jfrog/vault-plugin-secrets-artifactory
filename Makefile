GO_ARCH := $(shell go env GOARCH)
TARGET_ARCH := $(shell go env GOOS)_$(GO_ARCH)
GORELEASER_ARCH := $(TARGET_ARCH)

ifeq ($(GO_ARCH), amd64)
GORELEASER_ARCH := $(TARGET_ARCH)_$(shell go env GOAMD64)
endif

GIT_SHORT_HASH := $(shell git rev-parse --short HEAD)
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 | sed  -n 's/v\([0-9]*\).\([0-9]*\).\([0-9]*\)/\1.\2.\3/p')
NEXT_VERSION := $(shell echo $(CURRENT_VERSION) | awk -F '.' '{print $$1 "." $$2 "." $$3 +1}')-dev+$(GIT_SHORT_HASH)

PLUGIN_DIR := dist/artifactory-secrets-plugin_$(GORELEASER_ARCH)
PLUGIN_FILE := artifactory-secrets-plugin
PLUGIN_NAME ?= artifactory
PLUGIN_VAULT_PATH ?= artifactory

ARTIFACTORY_ENV := ./vault/artifactory.env
ARTIFACTORY_SCOPE ?= applied-permissions/groups:readers
export JFROG_URL ?= http://localhost:8082
JFROG_ACCESS_TOKEN ?= $(shell TOKEN_USERNAME=$(TOKEN_USERNAME) JFROG_URL=$(JFROG_URL) ./scripts/getArtifactoryAdminToken.sh)
TOKEN_USERNAME ?= vault-admin
export VAULT_TOKEN ?= root
export VAULT_ADDR ?= http://localhost:8200

.DEFAULT_GOAL := all

all: fmt build test start

build: fmt
	GORELEASER_CURRENT_TAG=$(NEXT_VERSION) goreleaser build --single-target --clean --snapshot

start:
	vault server -dev -dev-root-token-id=root -dev-plugin-dir=$(PLUGIN_DIR) -log-level=DEBUG

disable:
	vault secrets disable $(PLUGIN_VAULT_PATH)

enable:
	vault secrets enable -path=$(PLUGIN_VAULT_PATH) -plugin-version=$(NEXT_VERSION) $(PLUGIN_NAME)

register:
	vault plugin register -sha256=$$(sha256sum $(PLUGIN_DIR)/$(PLUGIN_FILE) | cut -d " " -f 1) -command=$(PLUGIN_FILE) -version=$(NEXT_VERSION) secret $(PLUGIN_NAME)
	vault plugin info -version=$(NEXT_VERSION) secret $(PLUGIN_NAME)

deregister:
	vault plugin deregister -version=$(NEXT_VERSION) secret $(PLUGIN_NAME)
	
upgrade: build register
	vault plugin reload -plugin=$(PLUGIN_NAME)

test:
	go test -v ./...

acceptance:
	export VAULT_ACC=true && \
	export JFROG_ACCESS_TOKEN=$(JFROG_ACCESS_TOKEN) && \
		go test -run TestAcceptance -cover -coverprofile=coverage.txt -v -p 1 -timeout 5m ./...

clean:
	rm -f $(PLUGIN_DIR)/$(PLUGIN_FILE)

fmt:
	go fmt $$(go list ./...)

setup: disable register enable admin testrole

admin:
	vault write $(PLUGIN_VAULT_PATH)/config/admin url=$(JFROG_URL) access_token=$(JFROG_ACCESS_TOKEN)
	vault read $(PLUGIN_VAULT_PATH)/config/admin
	vault write -f $(PLUGIN_VAULT_PATH)/config/rotate
	vault read $(PLUGIN_VAULT_PATH)/config/admin

testrole:
	vault write $(PLUGIN_VAULT_PATH)/roles/test scope="$(ARTIFACTORY_SCOPE)" max_ttl=3h default_ttl=2h
	vault read $(PLUGIN_VAULT_PATH)/roles/test
	vault read $(PLUGIN_VAULT_PATH)/token/test

artifactory: $(ARTIFACTORY_ENV)

$(ARTIFACTORY_ENV):
	@./scripts/run-artifactory-container.sh | tee $(ARTIFACTORY_ENV)

stop_artifactory:
	source $(ARTIFACTORY_ENV) && docker stop $$ARTIFACTORY_CONTAINER_ID
	rm -f $(ARTIFACTORY_ENV)

.PHONY: build clean fmt start disable enable register deregister upgrade test acceptance  setup admin testrole artifactory stop_artifactory
