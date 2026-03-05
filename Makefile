APP_NAME := port_forward
BIN_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.Version=$(VERSION)

MINGW_PATH := /Workspace/Tools/buildchain/x86_64-w64-mingw32/bin
MINGW_CC := $(MINGW_PATH)/x86_64-w64-mingw32-gcc

.PHONY: all linux windows clean test

all: linux windows

linux:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME)_linux_amd64 .

windows:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(MINGW_CC) \
		go build -ldflags "$(LDFLAGS) -H windowsgui" -o $(BIN_DIR)/$(APP_NAME)_windows_amd64.exe .

test:
	go test ./... -timeout 30s

clean:
	rm -rf $(BIN_DIR)
