BINARY=trackfw
BUILD_DIR=bin

.PHONY: build test lint install clean

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/trackfw

test:
	go test ./...

lint:
	go vet ./...

install: build
	mv $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)
