BINARY=trackfw
BUILD_DIR=bin
# Pinado em toda invocação (ML-2E): mesmo desenho de GO_BIN abaixo -- o
# recipe do make sobrescreve qualquer HASH_CMD_BIN herdado do ambiente do
# processo pai, então um valor forjado exportado pelo usuário não sobrevive
# à chamada de check-roadmap-barrier-contract.sh via `make quality`.
HASH_CMD := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

.PHONY: build test test-node test-python parity parity-rest parity-falsify lint quality install clean sync-integration-assets check-integration-assets package-smoke check-required-full check-gates-remutation

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
	scripts/check-serve-browser-security.sh
	scripts/check-raw-read-ban.sh
	# ML-1B (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): toda criacao de
	# symlink/fifo em arquivo de teste passa por guarda de capacidade (nao por
	# guarda de plataforma). Gate impede a decima-primeira instancia da issue #315.
	scripts/check-symlink-privilege-guard.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-doctor-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-doctor-remote-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-models-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-audit-surface.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-namespace-union.sh
	# ML-2D (ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-agents-install-nao-registra-e-escreve-sempre-no-primeiro.md):
	# 4 cenários × 3 runtimes, compara trackfw.yaml DEPOIS de `agents install --scope project`.
	# O check-artifact-parity.sh compara artefatos GERADOS; este gate cobre o trackfw.yaml MODIFICADO.
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agents-install-yaml-parity.sh --self-test
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agents-install-yaml-parity.sh
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
	# ML-NOVO (ROADMAP-2026-09-11-serve-interpola-host-...): gate da classe — todo
	# check-*.sh deve ter consumidor. Inclui --self-test para falsificar os 4 braços.
	scripts/check-orphan-gates.sh --self-test
	scripts/check-orphan-gates.sh
	# --self-test: `make parity` roda fora de um pull request, entao nao ha corpo de
	# PR para medir. O autoteste exercita o MESMO matcher que o CI usa (nao ha
	# segunda copia da regex) nas duas direcoes + a guarda de vacuidade. A medicao
	# do corpo real acontece no job `pr-closing-keyword` de .github/workflows/quality.yml.
	scripts/check-pr-closing-keyword.sh --self-test
	# ML-2A (ROADMAP-2026-09-06-ratchet-por-nome-e-classe-propria-para-suite-que-nao-carrega):
	# autoteste do ratchet de nomes do Windows. Roda localmente sem artefatos de CI (--self-test
	# usa artefatos sinteticos). A verificacao real acontece no step "ML-2A — ratchet de nomes"
	# do job windows-full-suites em .github/workflows/quality.yml.
	python3 scripts/check-windows-known-failures.py --self-test
	# ML-4B corretivo (ROADMAP-2026-09-06-ratchet-por-nome-e-classe-propria-para-suite-que-nao-carrega):
	# autoteste do gate de concordância entre declared/required/workflow checks.
	# --self-test usa fixtures sinteticas (sem chamada ao gh api nem leitura de workflow).
	# O job CI roda --scope dw (D\W apenas, sem token). A verificacao completa D/R/W
	# usa 'make check-required-full' (requer credencial de mantenedor, ver alvo abaixo).
	python3 scripts/check-required-status-checks.py --self-test
	# ML-1C (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): autoteste do smoke
	# de consumidor by_agent (2 agentes). O smoke completo roda no job
	# consumer-smoke-by-agent em quality.yml (requer binários dos 3 CLIs).
	# O --self-test aqui verifica apenas a estrutura do script e a lógica de
	# detecção de #320 (sem invocar os CLIs reais).
	scripts/check-consumer-smoke-by-agent.sh --self-test
	# ML-1D (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): autoteste do gate
	# de anotações de job. A verificação real acontece no workflow check-annotations.yml
	# (workflow_run, só roda da branch default após merge à main).
	python3 scripts/check-job-annotations.py --self-test

parity-falsify: build
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/run-gates-falsify-parallel.sh

check-required-full:
	# D/R/W — verificação completa: declared vs required_status_checks (API) vs workflow checks.
	# Requer credencial de mantenedor: gh auth login com scope 'repo'.
	# NUNCA em CI: GITHUB_TOKEN não tem permissão de administrador para ler
	# /branches/main/protection (medido em run CI 34605163678, PR #317 — retornou 404).
	# Adicionar este alvo a parity/quality/parity-rest reconectaria a dependência de token
	# ao CI, que foi a causa raiz do job permanentemente vermelho (ML-4B corretivo).
	# Pré-condição de release: execute ANTES de 'git tag -a' (ver CLAUDE.md §Protocolo de Release, passo 3.5).
	python3 scripts/check-required-status-checks.py

check-gates-remutation:
	# AC5 (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): re-mutação de gates existentes.
	# Roda a suíte completa de falsificação (sem sharding) para garantir que gates que eram
	# corretos no dia 1 não ficaram vacuosos por mudanças adjacentes.
	#
	# POR QUE EXISTE: o #309 mostrou que check-python-writes-lf.sh verificava PRESENÇA de
	# newline= mas não o VALOR. Um gate de valor-errado (`newline="\r\n"`) passaria. A suíte
	# de falsificação (check-gates-falsify.sh) não tinha esse cenário — foi adicionado por
	# este roadmap. Este alvo garante que a suíte completa (incluindo cenários de valor-errado)
	# rode antes de todo release.
	#
	# NUNCA em CI de PR: a suíte completa leva ~78% do tempo de CI. Rodar aqui, em CI de PR,
	# duplicaria o job parity-falsify (shardado) e tornaria o PR inteiro mais lento. O mesmo
	# padrão do check-required-full: pré-condição de release, não bloqueio de PR diário.
	#
	# Pré-condição de release: execute ANTES de 'git tag -a'.
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
