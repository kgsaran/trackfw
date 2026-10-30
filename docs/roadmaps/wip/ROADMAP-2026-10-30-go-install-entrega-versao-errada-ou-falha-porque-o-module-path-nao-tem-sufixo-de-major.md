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
**Status:** ⬜ Pendente
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
- [ ] As seis seções respondidas com evidência (comando e resultado)
- [ ] Prescrições escritas em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md
grep -q 'Prescrições' docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md
```

---

## Wave 1 — Implementação (2 MLs em paralelo)
> Dependencies: Wave 0 auditada.

### ML-1A — Templates de consumidor instalam pelo install.sh da tag (apolo-tf) — AC1, AC2, AC3, AC5, AC6
**Status:** ⬜ Pendente
**Files affected:** `internal/generators/scaffold_doctor.go`, `internal/generators/scaffold.go`, `internal/generators/consumer_install_test.go`, `docs/cli-parity.md`
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
   nos `_test.go` de `internal/generators`).
**Acceptance criteria:**
- [ ] nenhum template de consumidor emite `go install` nem `releases/latest`
- [ ] teste novo reprova com o template antigo restaurado (contra-braço)
- [ ] template de produtor byte-idêntico ao anterior
- [ ] doctor acusa o `trackfw-validate.yml` antigo e `update` o corrige (medido em fixture)

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/
go test ./internal/generators/ -count=1
```

### ML-1B — Documentação de usuário sem go install (afrodite-tf) — AC4
**Status:** ⬜ Pendente
**Files affected:** `README.md`, `site/index.md`, `site/guide/getting-started.md`, `site/en/index.md`, `site/en/guide/getting-started.md`
**Actions:**
1. Remover o bloco `go install github.com/kgsaran/trackfw/cmd/trackfw@latest` dos cinco arquivos.
   Onde ele era uma das opções de instalação, manter as demais (Homebrew, `install.sh`,
   `install.ps1`, `npm`, `pip`) como já estão; não inventar canal.
2. Se a página tiver frase que cite Go como canal, ajustar para "compilar do código-fonte" só se já
   houver contexto de contribuidor; caso contrário, apenas remover.
**Acceptance criteria:**
- [ ] `grep -rn "cmd/trackfw@" README.md site --include=*.md` sem resultado
- [ ] build do site sem erro (`npm run docs:build` ou o comando do `site/package.json`)

**Gates da wave:**
```bash
python3 scripts/check-workflow-yaml.py
```

---

## Wave 2 — Red team e fechamento
> Dependencies: Wave 1 auditada.

### ML-2A — Red team da instalação pela tag (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-30-redteam-instalacao-de-consumidor-pela-tag.md`
**Actions:**
1. Tentar fazer um template de consumidor voltar a `go install` ou a `releases/latest` sem o teste reprovar.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-instalacao-de-consumidor-pela-tag.md
make quality
```
