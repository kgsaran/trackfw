BINARY=trackfw
BUILD_DIR=bin

.PHONY: build test test-node test-python parity lint quality install clean sync-integration-assets check-integration-assets package-smoke

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/trackfw

test:
	TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...

test-node:
	cd npm && npm test

test-python:
	python3 -m pytest pypi/tests -q

parity: build
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-cli-parity.sh
	scripts/check-validate-parity.sh
	scripts/check-static-assets.sh
	scripts/check-integration-assets.sh

sync-integration-assets:
	scripts/sync-integration-assets.sh

check-integration-assets:
	scripts/check-integration-assets.sh

package-smoke: check-integration-assets
	scripts/smoke-integration-packages.sh

lint:
	go vet ./...

quality: test test-node test-python lint parity

install: build
	mv $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)
