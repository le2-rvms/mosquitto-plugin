ifneq (,$(wildcard .env))
include .env
export
endif

#SHELL := /bin/bash
BINARY_DIR := build
DOCKER_IMAGE := ghcr.io/le2-tech/mosquitto

GOFLAGS :=
CGO_ENABLED ?= 1

.PHONY: all build-auth build-queue bcryptgen clean docker-build docker-run mod

mod:
	go mod tidy

test:
	go test ./...
	go vet ./...

mkdir:
	mkdir -p $(BINARY_DIR)

build-auth-dev:
	CGO_ENABLED=$(CGO_ENABLED) go build -buildmode=c-shared -gcflags "all=-N -l" -ldflags "" -o $(BINARY_DIR)/auth-plugin ./plugin/authplugin

build-auth:
	CGO_ENABLED=$(CGO_ENABLED) go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o $(BINARY_DIR)/auth-plugin ./plugin/authplugin

build-queue:
	CGO_ENABLED=$(CGO_ENABLED) go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o $(BINARY_DIR)/queue-plugin ./plugin/queueplugin

build-conn:
	CGO_ENABLED=$(CGO_ENABLED) go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o $(BINARY_DIR)/conn-plugin ./plugin/connplugin

run-bcryptgen:
	go run ./cmd/bcryptgen --salt slat_foo123 --password public

clean:
	rm -rf $(BINARY_DIR)

local-run: mod clean mkdir build-auth build-queue build-conn
	mosquitto --version
	PG_DSN=${PG_DSN} QUEUE_DSN=${QUEUE_DSN} mosquitto -c ./config/mosquitto.conf

# Build a runnable Mosquitto image with the plugin baked in
# --progress=plain
docker-build-dev:
	docker build . -f docker/Dockerfile --build-arg APP_ENV=dev -t $(DOCKER_IMAGE) 

docker-build-dev-ubuntu:
	docker build . -f docker/Dockerfile.ubuntu --build-arg APP_ENV=dev -t $(DOCKER_IMAGE) 

# Quick run; assumes a postgres reachable per mosquitto.conf DSN
docker-run-origin:
	docker run --rm -it \
	  -p 1883:1883 -p 9001:9001 -p 9002:9002 \
	  -w /mosquitto \
	  -v $(PWD)/mosquitto.http.conf:/mosquitto/config/mosquitto.conf \
	  eclipse-mosquitto:latest mosquitto -c ./config/mosquitto.conf

docker-bash:
	docker run --rm -it $(DOCKER_IMAGE) bash

api-clients:
	curl http://localhost:9002/api/clients

# password_file:
# 	touch ./config/password_file
# 	chmod 0700 ./config/password_file
# 	mosquitto_passwd -b ./config/password_file ${MOSQUITTO_USER_OPS} ${MOSQUITTO_PASSWORD_OPS}
# 	mosquitto_passwd -b ./config/password_file ${MOSQUITTO_USER_APP_WEB} ${MOSQUITTO_PASSWORD_APP_WEB}

# acl_file:
# 	envsubst < ./config/acl_file.tmpl > ./config/acl_file
# 	chmod 0700 ./config/acl_file

setup:
	config/setup.sh