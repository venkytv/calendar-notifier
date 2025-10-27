.PHONY: all clean build build-darwin build-linux test

BINARY_NAME=calendar-notifier
BUILD_DIR=build

all: clean build

build: build-darwin build-linux

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/calendar-notifier
	GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/calendar-notifier

build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/calendar-notifier
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/calendar-notifier

test:
	go test ./...

test-verbose:
	go test -v ./...

clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)

install: build-darwin
	@echo "Installing locally..."
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-$(shell uname -m) $(GOPATH)/bin/$(BINARY_NAME) || cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(GOPATH)/bin/$(BINARY_NAME)