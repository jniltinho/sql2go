# Variables for UPX
UPX_VERSION      := 5.1.1
UPX_ARCHIVE      := upx-$(UPX_VERSION)-amd64_linux.tar.xz
UPX_DIR          := upx-$(UPX_VERSION)-amd64_linux
UPX_BIN          := /usr/local/bin/upx
UPX_URL          := https://github.com/upx/upx/releases/download/v$(UPX_VERSION)/$(UPX_ARCHIVE)

# Binary name
APP        := sql2go
BIN        := bin/$(APP)
PREFIX     := sql2go/cmd

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTIDY=$(GOCMD) mod tidy

VERSION    := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS    := -ldflags "-s -w -X $(PREFIX).Version=$(VERSION) -X $(PREFIX).BuildDate=$(BUILD_DATE)"

# Default run parameters
DB_NAME=test
HOST=127.0.0.1
PORT=3306
USER=root
PASS=
OUT_DIR=./models
MERGE=false

.PHONY: all build clean run tidy help install-upx

all: build

## build: Build the project and generate the binary
build:
	rm -f $(BIN)
	mkdir -p bin
	$(GOBUILD) -o $(BIN) $(LDFLAGS) main.go 
	upx --best --lzma $(BIN)

## run: Run the project with custom parameters (example: make run DB_NAME=my_database)
run: build
	./$(BIN) generate -db $(DB_NAME) -host $(HOST) -port $(PORT) -user $(USER) -pass "$(PASS)" -out $(OUT_DIR) $(if $(filter true,$(MERGE)),-merge,)

## clean: Remove the binary and clean the Go cache
clean:
	$(GOCLEAN)
	rm -f $(BIN)

## tidy: Organize Go dependencies
tidy:
	$(GOTIDY)

## help: Show this help message
help:
	@echo "Available commands:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST)

## install-upx: Installs the UPX binary locally (requires root permissions for mv)
install-upx:
	@echo "Installing UPX binary..."
	curl -ksSL "$(UPX_URL)" -o "$(UPX_ARCHIVE)"
	tar -xf "$(UPX_ARCHIVE)"
	chmod +x "$(UPX_DIR)/upx"
	sudo mv "$(UPX_DIR)/upx" "$(UPX_BIN)"
	rm -rf "$(UPX_DIR)" "$(UPX_ARCHIVE)"
