.PHONY: build run clean test

GO = go
BINARY_SERVER_NAME = pubsub-server
BUILD_DIR = build

build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY_SERVER_NAME) ./cmd/server

run: build
	$(BUILD_DIR)/$(BINARY_SERVER_NAME)


clean:
	rm -rf $(BUILD_DIR)

test:
	$(GO) test -v ./...


