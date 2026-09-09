BINARY=trackfw
BUILD_DIR=bin
# Pinado em toda invocação (ML-2E): mesmo desenho de GO_BIN abaixo -- o
# recipe do make sobrescreve qualquer HASH_CMD_BIN herdado do ambiente do
# processo pai, então um valor forjado exportado pelo usuário não sobrevive
# à chamada de check-roadmap-barrier-contract.sh via `make quality`.
HASH_CMD := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

.PHONY: build test test-node test-python parity parity-rest parity-falsify lint quality install clean sync-integration-assets check-integration-assets package-smoke

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/trackfw

test:
	TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...

test-node:
	cd npm && npm test

test-python:
	python3 -m pytest pypi/tests -q

# ML-2G (ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify): o alvo
# `parity` foi dividido em dois -- `parity-rest` (os ~45 gates curtos) e
# `parity-falsify` (só o gate que domina o tempo de parede, ~78% do job) --
# para que o job `parity` do CI possa shardar o segundo em jobs de matriz
# enquanto o primeiro roda uma vez só, em paralelo. `parity` continua
# executando os dois, na mesma ordem de antes (build -> resto -> falsify),
# então `make parity`/`make quality` local ficam bit-a-bit equivalentes ao
# comportamento anterior a esta divisão -- só a topologia de invocação em CI
# mudou. `scripts/check-parity-call-site-pins.sh` não distingue por alvo (lê
# toda linha de recipe do Makefile), então os pins de HASH_CMD_BIN/PYTHON_BIN
# continuam cobertos onde já estavam.
parity: build parity-rest parity-falsify

parity-rest: build
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
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-artifact-closed-cycle.sh
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
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-git-branch-guard-hook-schema.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-git-branch-guard-hook-schema.sh --self-test
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-hooks-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-harness-hooks-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-serve-address-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-doctor-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-doctor-remote-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-models-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-audit-surface.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-namespace-union.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-thirdparty-parity.sh
	scripts/check-install-version-pin.sh
	scripts/check-ci-workflow-pin-parity.sh
	scripts/check-ci-workflow-job-id-collision.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) HASH_CMD_BIN="$(HASH_CMD)" scripts/check-roadmap-barrier-contract.sh
	scripts/check-ref-separator-portability.sh
	scripts/check-atomic-write-anti-divergence.sh
	scripts/check-shell-posix-portability.sh
	scripts/check-output-encoding-declared.sh
	scripts/check-parity-call-site-pins.sh
	# --self-test: `make parity` roda fora de um pull request, entao nao ha corpo de
	# PR para medir. O autoteste exercita o MESMO matcher que o CI usa (nao ha
	# segunda copia da regex) nas duas direcoes + a guarda de vacuidade. A medicao
	# do corpo real acontece no job `pr-closing-keyword` de .github/workflows/quality.yml.
	scripts/check-pr-closing-keyword.sh --self-test

parity-falsify: build
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/run-gates-falsify-parallel.sh

sync-integration-assets:
	scripts/sync-integration-assets.sh

check-integration-assets:
	scripts/check-integration-assets.sh

package-smoke: check-integration-assets
	# PYTHON_BIN pinado (ML-2E, mesma família de HASH_CMD_BIN acima -- severidade menor
	# porque não há guarda que um binário forjado possa satisfazer vaziamente aqui, só
	# quebra o próprio build/smoke se for forjado).
	PYTHON_BIN=python3 scripts/smoke-integration-packages.sh

lint:
	go vet ./...

quality: test test-node test-python lint parity

install: build
	mv $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)
