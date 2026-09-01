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
	scripts/check-referential-integrity.sh
	scripts/check-parity-contract-coverage.sh
	scripts/check-static-assets.sh
	scripts/check-integration-assets.sh
	scripts/check-python-writes-lf.sh
	scripts/check-homedir-parity.sh
	scripts/check-tty-detection.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-identity-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-artifact-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-barrier.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-slash-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-rules-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-update-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-roadmap-move-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-branch-new-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-branch-prune-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-commit-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-ship-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-ship-force-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-push-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-push-force-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-release-tag-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-unknown-command-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-attention-scripts-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-hooks-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-harness-hooks-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-serve-address-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-doctor-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-models-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-audit-surface.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-namespace-union.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-gates-falsify.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-thirdparty-parity.sh
	scripts/check-install-version-pin.sh
	scripts/check-ci-workflow-pin-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-roadmap-barrier-contract.sh
	scripts/check-ref-separator-portability.sh

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
