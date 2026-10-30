---
status: wip
date: 2026-10-30
req: "docs/req/REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md"
squad: "hades-tf, apolo-tf, afrodite-tf"
---

# Roadmap: go install entrega versao errada ou falha porque o module path nao tem sufixo de major

> Created: 2026-10-30 | Status: wip

## Context

REQ: docs/req/REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
ADR: docs/adr/ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md · Fecha o issue #637.

Medido em 2026-10-30: `go install ...@v11.2.0` → rc=1 (`module path must match major version`);
`go list -m github.com/kgsaran/trackfw@latest` → `v1.1.0`. O template de consumidor do
`trackfw-validate.yml` falha (trava o CMDB); a documentação instala a v1.1.0 em silêncio.
O `install.sh` é asset de cada release (`.goreleaser.yaml:44`; v11.2.0 tem `install.sh`,
`install.ps1` e `checksums.txt`), então o script pode vir da tag pinada.

ML-1A (gerador, Go) e ML-1B (documentação de usuário) tocam arquivos disjuntos e rodam em paralelo.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model da troca de mecanismo de instalação nos templates de consumidor (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. Prescrições P1–P8 aceitas e incorporadas ao ML-1A/ML-1B. Achados: o gate `scripts/check-ci-workflow-pin-parity.sh` e `internal/discover/discover_test.go` fixam o template antigo (fora da lista original); `VISION.md:206` também oferece `go install` (fora da REQ); release assets são mutáveis (`.immutable=false`), e a tag pinada limita o raio, não o elimina (residual 1). Hashes pré-fix no Apêndice A do parecer.
**Files affected:** `docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md`
**Actions:**
1. Cadeia de suprimento: comparar `releases/latest/download/install.sh` com
   `releases/download/v<versão>/install.sh` (o script também fica pinado?) e o que o `install.sh`
   verifica (SHA-256 contra `checksums.txt` da mesma tag). O que um atacante precisa controlar em cada caso.
2. Injeção: `version.Version` entra no YAML e na URL. O que garante que ele não carrega metacaractere
   (origem do valor, validação no `install.sh` por `case`).
3. Enumeração fechada dos templates de consumidor que instalam trackfw em CI (GitHub e GitLab) e dos
   sítios de documentação com `go install`; feche com o comando, sem `| head`.
4. Doctor/update: o consumidor com o template antigo recebe achado de divergência e `update` corrige?
   Meça com um consumidor fixture fora da árvore (HOME descartável; não use `git commit` — o guard bloqueia).
5. Alvos de falsificação nas duas direções para AC1, AC2, AC3, AC5 e AC6.
6. Prescrições numeradas para ML-1A e ML-1B, e residual.
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md
grep -q 'Prescrições' docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md
```

---

## Wave 1 — Implementação (2 MLs em paralelo)
> Dependencies: Wave 0 auditada.

### ML-1A — Templates de consumidor instalam pelo install.sh da tag (apolo-tf) — AC1, AC2, AC3, AC5, AC6
**Status:** ✅ Concluído — auditado em 2026-10-30. Gates verdes (generators, discover, paridade com 25 cenários; eram 21). Contra-braço: com os dois geradores revertidos, 4 dos 6 testes novos reprovam; os 2 que passam afirmam o que não mudou (trigger manual, hashes de produtor). Medido pelo arquiteto: o passo `run:` gerado, executado de verdade com `TRACKFW_VERSION=11.2.0`, instala `trackfw 11.2.0`. Desvio aceito: `internal/generators/scaffold_doctor_test.go` fixava `go install` e `setup-go` no consumidor (mesma causa); entrou na lista de arquivos.
**Files affected:** `internal/generators/scaffold_doctor.go`, `internal/generators/scaffold.go`, `internal/generators/consumer_install_test.go`, `internal/generators/testdata/ci-trigger/discover-consumer.golden`, `internal/generators/testdata/ci-trigger/gate-consumer.golden`, `internal/generators/testdata/ci-trigger/gitlab.golden`, `internal/discover/discover_test.go`, `scripts/check-ci-workflow-pin-parity.sh`, `docs/cli-parity.md`, `internal/generators/scaffold_doctor_test.go`
**Actions:**
1. `scaffold_doctor.go`: o template de consumidor do `trackfw-validate.yml` passa a instalar como o
   `trackfw-gate.yml` (`env: TRACKFW_VERSION`, `curl ... install.sh | sh`); job id
   `governance-go-install` mantido; comentários do gerador atualizados. Template de produtor intacto.
2. `scaffold.go`: os templates de consumidor do `trackfw-gate.yml` e do `.gitlab-ci-trackfw.yml`
   baixam o `install.sh` de `releases/download/v<versão>/`, não de `releases/latest/`.
3. `consumer_install_test.go`: teste que percorre os templates de consumidor e reprova `go install`
   de `github.com/kgsaran/trackfw` sem sufixo de major e `releases/latest` em template de CI de
   consumidor; afirma `TRACKFW_VERSION` = `version.Version`.
4. `docs/cli-parity.md`: seção dos templates (≈7540–7560 e a tabela ≈7620) descreve o mecanismo novo.
5. Atualizar testes existentes que fixem o template antigo (grep por `go install` e `releases/latest`
   nos `_test.go` de `internal/generators` e `internal/discover`, e os goldens de `testdata/ci-trigger`).
6. Prescrições da Wave 0 (`docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md`, seção `Prescrições`):
   P1 (três correções no `check-ci-workflow-pin-parity.sh`, guarda de no-op com `if cmp -s`, e o número
   de cenários não cai: medir antes e depois), P2 (URL com `${TRACKFW_VERSION}` entre aspas duplas nos três
   templates), P3 (`timeout-minutes: 10` no job `governance-go-install`), P4/P5 (teste AC2), P6 (AC5 com
   fixture stale de hash `aadf2cb1…cca996`), P7 (hashes de produtor do Apêndice A inalterados).
**Acceptance criteria:**
- [x] nenhum template de consumidor emite `go install` nem `releases/latest`
- [x] teste novo reprova com o template antigo restaurado (contra-braço)
- [x] template de produtor byte-idêntico ao anterior
- [x] doctor acusa o `trackfw-validate.yml` antigo e `update` o corrige (medido em fixture)
- [x] `check-ci-workflow-pin-parity.sh` verde, com cenários ≥ o número medido antes da edição

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/ ./internal/discover/
go test ./internal/generators/ -count=1
go test ./internal/discover/ -count=1
bash scripts/check-ci-workflow-pin-parity.sh
! grep -rn "cmd/trackfw@" README.md site docs/visao-projeto --include=*.md
```

### ML-1B — Documentação de usuário sem go install (afrodite-tf) — AC4
**Status:** ✅ Concluído — auditado em 2026-10-30. Seis arquivos sem `go install`; `vitepress build` rc=0; na VISION saiu também a frase "The Go module builds from source".
**Files affected:** `README.md`, `docs/visao-projeto/VISION.md` (P8), `site/index.md`, `site/guide/getting-started.md`, `site/en/index.md`, `site/en/guide/getting-started.md`
**Actions:**
1. Remover o bloco `go install github.com/kgsaran/trackfw/cmd/trackfw@latest` dos seis arquivos (na `VISION.md:206`, a linha da tabela de canais).
   Onde ele era uma das opções de instalação, manter as demais (Homebrew, `install.sh`,
   `install.ps1`, `npm`, `pip`) como já estão; não inventar canal.
2. Se a página tiver frase que cite Go como canal, ajustar para "compilar do código-fonte" só se já
   houver contexto de contribuidor; caso contrário, apenas remover.
**Acceptance criteria:**
- [x] `grep -rn "cmd/trackfw@" README.md site docs/visao-projeto --include=*.md` sem resultado
- [x] build do site sem erro (`npm run docs:build` ou o comando do `site/package.json`)


---

## Wave 2 — Red team e fechamento
> Dependencies: Wave 1 auditada.

### ML-2A — Red team da instalação pela tag (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. Aprova com ressalvas; P1, P2, P3, P7 e P8 aprovadas. Quatro lacunas de teste viram a Wave 3 (defeito conhecido se corrige no mesmo PR): F1 `version.Version` trocado por literal nos builders de `scaffold.go` passa tudo; F2 as asserções positivas leem o conteúdo bruto, e um template com o env e o passo de instalação comentados passa os checks positivos; F3 `go install -v ...` escapa do `strings.Contains`; F4 o AC5 em processo cobre só o trigger `pull_request`.
**Files affected:** `docs/seguranca/2026-10-30-redteam-instalacao-de-consumidor-pela-tag.md`
**Actions:**
1. Tentar fazer um template de consumidor voltar a `go install` ou a `releases/latest` sem o teste reprovar.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-instalacao-de-consumidor-pela-tag.md
```

---

## Wave 3 — Corretivos do red team (1 ML)
> Dependencies: Wave 2 auditada.

### ML-3A — Testes de instalação de consumidor sem as lacunas F1–F4 (apolo-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. M1 reprova `TestConsumerInstall_VersionStubNotHardcoded` (medido pelo arquiteto: gate e GitLab, nos dois triggers); M2 reprova o teste e o check positivo do script; M3 reprova `TestConsumerInstall_NoGoInstall_NoBuildersConsumers`; AC5 com os dois triggers; script com 27 cenários (eram 25).
**Files affected:** `internal/generators/consumer_install_test.go`, `scripts/check-ci-workflow-pin-parity.sh`
**Actions:**
1. F1: teste que faz stub de `version.Version = "9.9.9-stub"` (restaurado em `t.Cleanup`) e afirma `TRACKFW_VERSION: "9.9.9-stub"` na saída dos três builders de consumidor, nos dois triggers.
2. F2: as asserções positivas (`TRACKFW_VERSION`, URL com `${TRACKFW_VERSION}`) leem só linhas não comentadas, no teste e no `check_consumer_no_latest` do script. Acrescentar ao script um vetor de falsificação com o env e o passo de instalação comentados, com guarda de no-op `if cmp -s`.
3. F3: a detecção de `go install|run|get` do módulo aceita flags entre o verbo e o caminho (regexp `\bgo\s+(install|run|get)\s+(?:-\S+\s+)*github\.com/kgsaran/trackfw`).
4. F4: o teste AC5 também cobre o trigger manual (fixture antiga do trigger manual com sha256 `bf7942d24b84045c2aa9b81f4fc302ecfc0521728a65d0c2c1ff4ac261c1e357`).
**Acceptance criteria:**
- [x] mutação M1 do red team (literal `"11.2.0"` em `scaffold.go`) reprova
- [x] mutação M2 (env e instalação comentados) reprova no teste e num check positivo do script
- [x] mutação M3 (`go install -v`) reprova no teste de `go install`
- [x] AC5 verde para os dois triggers
- [x] cenários do script ≥ 25

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/
go test ./internal/generators/ ./internal/discover/ -count=1
bash scripts/check-ci-workflow-pin-parity.sh
make quality
```
