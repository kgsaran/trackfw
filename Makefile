BINARY=trackfw
BUILD_DIR=bin
HASH_CMD := $(shell command -v sha256sum >/dev/null 2>&1 && echo sha256sum || echo "shasum -a 256")

.PHONY: build test parity parity-rest parity-falsify self-governance lint quality install clean check-integration-assets package-smoke check-required-full check-gates-remutation gen-manifests

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/trackfw

test:
	TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...

# ML-2G (ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify): o alvo
# `parity` foi dividido em dois -- `parity-rest` (os gates curtos) e
# `parity-falsify` (só o gate que domina o tempo de parede, ~78% do job) --
# para que o job `parity` do CI possa shardar o segundo em jobs de matriz
# enquanto o primeiro roda uma vez só, em paralelo. `parity` continua
# executando os dois, na mesma ordem de antes (build -> resto -> falsify).
# `scripts/check-parity-call-site-pins.sh` não distingue por alvo (lê
# toda linha de recipe do Makefile), então os pins de HASH_CMD_BIN/PYTHON_BIN
# continuam cobertos onde já estavam.
# ML-3A/3B/3C (v8): remoção das reimplementações Node e Python. test-node e
# test-python removidos de quality. Gates de paridade tripla removidos de
# parity-rest. Gates REESCREVER mantidos com braços Go apenas.
parity: build parity-rest parity-falsify

parity-rest: build
	# Defeito 3 (PR #352): valida sintaxe YAML de todos os .github/workflows/*.yml
	# antes do push, sem credencial. Guarda de vacuidade: falha se nenhum arquivo
	# encontrado. Fecha a classe: workflow inválido não atravessa mais o ciclo local.
	# ML-3C: também valida `needs:` para detectar referências a jobs removidos.
	python3 scripts/check-workflow-yaml.py
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-validate-rule-pins.sh
	scripts/check-referential-integrity.sh
	scripts/check-req-path-literals.sh
	scripts/check-parity-contract-coverage.sh
	scripts/check-static-assets.sh
	scripts/check-integration-assets.sh
	scripts/check-tty-detection.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-barrier.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-slash-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-rules-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-update-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-release-tag-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-git-branch-guard-hook-schema.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-git-branch-guard-hook-schema.sh --self-test
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-serve-address-parity.sh
	scripts/check-serve-browser-security.sh
	scripts/check-serve-api-file-security.sh
	# ML-1A (REQ-2026-09-17-jira-base-url-*): AC7 anti-reintroduction gate — no authenticated
	# URL built by string concatenation from a config-derived field in internal/**/*.go.
	scripts/check-jira-url-concat.sh --self-test
	scripts/check-jira-url-concat.sh
	scripts/check-raw-read-ban.sh
	# ML-1B (ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf...):
	# todo $(python3 ...) em scripts/*.sh normaliza via strip_cr (lib-crlf-normalize.sh).
	# Impede reintroducao de captura sem normalizacao apos ML-1A corrigir os 19 sitios.
	scripts/check-crlf-normalize-capture.sh
	# ML-1B (ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo...):
	# nenhuma captura $$(cmd ... || echo N) sobre comando que JA emite no caminho de
	# falha. `grep -c` imprime "0" e sai 1: o `|| echo 0` acrescenta uma segunda
	# linha, a captura vira $$'0\n0' e o $$(( )) a jusante quebra — foi o que matou
	# a apuracao do censo de Windows no primeiro shard limpo. Forma correta:
	# VAR=$$( { grep -ac 'PAT' "$$F" || true; } ).
	scripts/check-emitting-capture-fallback.sh
	# ML-2H (ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo...):
	# gate IRMAO do de cima, para a captura SEM FALLBACK NENHUM cujo rc PROPAGA —
	# `v=$$(grep PAT f)`, `v=$$(a | grep PAT)` e `v=$$(a | grep PAT | b)` sob pipefail.
	# Ali o rc de "nao casou" mata o script sob set -e, trocando o diagnostico que a
	# linha seguinte ja escreveu por MORTE MUDA. Forma correta: `v=$$( { grep … || true; } )`
	# — e, quando o valor e COMPARADO com outra captura, o `|| true` SOZINHO vira
	# aprovacao vacua (ML-2I): precisa tambem de guarda de nao-vacuidade.
	# Alargar o gate de cima para cobrir esta forma foi medido e reprovado (ML-2E):
	# sem a exigencia de fallback, a regex dele acusaria todo $$(a | b) legitimo.
	scripts/check-unguarded-capture-rc.sh
	# ML-2A (ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever):
	# todo sítio de escrita em internal/**/*.go (produção) carrega marcador write-containment-allowed:
	# ou reprova. Impede reintrodução de escrita desguardada após a Wave 1. Nasce falsificável.
	unset WRITE_CONTAINMENT_SCAN_DIR && scripts/check-write-containment.sh
	scripts/check-write-containment.sh --self-test
	# ML-1B (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): toda criacao de
	# symlink/fifo em arquivo de teste passa por guarda de capacidade (nao por
	# guarda de plataforma). Gate impede a decima-primeira instancia da issue #315.
	scripts/check-symlink-privilege-guard.sh
	scripts/check-symlink-privilege-guard.sh --self-test
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-models-parity.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-agent-namespace-union.sh
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-thirdparty-parity.sh
	scripts/check-install-version-pin.sh
	scripts/check-install-checksum.sh
	scripts/check-ci-workflow-pin-parity.sh
	scripts/check-ci-workflow-job-id-collision.sh
	scripts/check-ci-workflow-binary-provenance.sh
	# ML-3C: REESCREVER — Partes B e C (pins comportamentais do barrier) preservadas.
	# Apenas as invocações cross-runtime node/py da Parte A foram removidas.
	# ML-1B-bis: a tripwire de disco (TRACKFW_SELF_GOVERNED=1) foi movida para o alvo
	# `self-governance` — invocado pelo CI do upstream, não pela suíte geral. O consumidor
	# continua rodando as Partes B e C (corpus congelado, hashes) sem a tripwire.
	GO_BIN=$(BUILD_DIR)/$(BINARY) HASH_CMD_BIN="$(HASH_CMD)" scripts/check-roadmap-barrier-contract.sh
	# Usage e erro de USO; violacao e erro de RUNTIME. O gate exercita as DUAS
	# direcoes e traz self-test: um gate que so afirmasse a supressao passaria
	# num binario que nunca imprime usage, inclusive quando o usuario erra a linha.
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-usage-silencing.sh
	scripts/check-usage-silencing.sh --self-test
	scripts/check-ref-separator-portability.sh
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
	# ML-4B corretivo: autoteste do gate de concordância entre declared/required/workflow checks.
	# --self-test usa fixtures sinteticas (sem chamada ao gh api nem leitura de workflow).
	# O job CI roda --scope dw (D\W apenas, sem token). A verificacao completa D/R/W
	# usa 'make check-required-full' (requer credencial de mantenedor, ver alvo abaixo).
	python3 scripts/check-required-status-checks.py --self-test
	# ML-1D (ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona): autoteste do gate
	# de anotações de job. A verificação real acontece no workflow check-annotations.yml
	# (workflow_run, só roda da branch default após merge à main).
	python3 scripts/check-job-annotations.py --self-test
	# ML-1A (v8 ROADMAP-2026-09-12-v8-um-binario-muitos-canais): AC3 + partial #338.
	# Generates platform manifests from internal/version/version.go (single source of truth)
	# and verifies each manifest version matches the Go source AND the CHANGELOG top section.
	scripts/check-manifest-version-gate.sh
	# ML-1A (v8 ROADMAP-2026-09-12-v8-um-binario-muitos-canais): D2 — paridade de plataformas.
	# Asserts .goreleaser.yaml build matrix == gen-platform-manifests.sh PLATFORMS.
	# Divergence is permanent: publishing @trackfw-bin/X without a binary is irreversible.
	scripts/check-platform-matrix-parity.sh
	# ML-1A-D6write (v8): normalização semver→PEP440 no nome da wheel.
	scripts/check-wheel-filename.sh
	# ML-1A (v8): D7 — conteúdo dos pacotes publicados.
	scripts/check-channels-content.sh --self-test
	# ML-1D (v8): AC9 — byte-identidade do shim.
	scripts/check-shim-byte-identity.sh
	# ML-1E (v8): AC10 — instalação sob restrição.
	scripts/check-install-restriction.sh
	# ML-1F (v8): gate de NUL literal em fonte. Exceções estruturais removidas (ML-3A apagou os
	# dois arquivos npm/src que tinham NUL; o arquivo de exceções foi limpo junto).
	scripts/check-no-literal-nul-in-source.sh --self-test
	scripts/check-no-literal-nul-in-source.sh
	# ML-3C (v8): pins comportamentais do validate extraídos antes da deleção do check-validate-parity.sh.
	# ML-3E (v8): garante que .goreleaser.yaml declara prerelease: auto no bloco release:.
	# Sem essa chave, o GoReleaser usa false (padrão), e tags rc publicam como latest estável.
	# Medido em 2026-09-16: v8.0.0-rc1 ficou como latest por 3 dias. --self-test inclui
	# as duas direções de falsificação (chave ausente/errada e configuração correta).
	scripts/check-goreleaser-prerelease.sh --self-test
	scripts/check-goreleaser-prerelease.sh

parity-falsify: build
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/run-gates-falsify-parallel.sh

# ML-1B-bis (ROADMAP-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-
# repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md):
# Alvo de governança do upstream. Invoca a tripwire de disco que exige os 144
# roadmaps do mantenedor no disco — não faz parte de `parity-rest` porque o
# consumidor não possui esses artefatos. Chamado pelo CI do upstream
# (parity-other-gates, .github/workflows/quality.yml).
# TRACKFW_SELF_GOVERNED=1: pin verificado por check-parity-call-site-pins.sh
# (ML-2E, mesma família de HASH_CMD_BIN). Remover este pin reprova o gate.
self-governance: build
	TRACKFW_SELF_GOVERNED=1 GO_BIN=$(BUILD_DIR)/$(BINARY) HASH_CMD_BIN="$(HASH_CMD)" scripts/check-roadmap-barrier-contract.sh

check-required-full:
	# D/R/W — verificação completa: declared vs required_status_checks (API) vs workflow checks.
	# Requer credencial de mantenedor: gh auth login com scope 'repo'.
	# NUNCA em CI: GITHUB_TOKEN não tem permissão de administrador para ler
	# /branches/main/protection (medido em run CI 34605163678, PR #317 — retornou 404).
	# Pré-condição de release: execute ANTES de 'git tag -a'.
	python3 scripts/check-required-status-checks.py

check-gates-remutation:
	# AC5: re-mutação de gates existentes. Roda a suíte completa de falsificação.
	# NUNCA em CI de PR. Pré-condição de release: execute ANTES de 'git tag -a'.
	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/run-gates-falsify-parallel.sh

check-integration-assets:
	scripts/check-integration-assets.sh

package-smoke: build check-integration-assets
	# PYTHON_BIN pinado (ML-2E).
	PYTHON_BIN=python3 scripts/smoke-integration-packages.sh

lint:
	go vet ./...

quality: test lint parity

install: build
	mv $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

# gen-manifests — generates per-platform @trackfw-bin/<platform>/package.json artefacts
# from the single version source in internal/version/version.go. Output is gitignored
# (build/npm-platform/). Run this before publishing platform npm packages.
gen-manifests:
	scripts/gen-platform-manifests.sh

clean:
	rm -rf $(BUILD_DIR)
