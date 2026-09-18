---
status: done
date: 2026-08-14
req: "docs/req/REQ-2026-08-14-comando-trackfw-changelog-para-consultar-o-changelog-md-pelo-proprio-cli.md"
squad: ""
---

# Roadmap: comando trackfw changelog para consultar o CHANGELOG.md pelo proprio CLI

> Created: 2026-08-14 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-14-comando-trackfw-changelog-para-consultar-o-changelog-md-pelo-proprio-cli.md -->
REQ: docs/req/REQ-2026-08-14-comando-trackfw-changelog-para-consultar-o-changelog-md-pelo-proprio-cli.md

`CHANGELOG.md` é arquivo único na raiz do projeto (`docs/protocolo de release` em
CLAUDE.md), formato [Keep a Changelog](https://keepachangelog.com/en/1.1.0/):
seções `## [x.y.z] - YYYY-MM-DD` ou `## [Unreleased]`, cada uma com subseções
`### Added`/`### Fixed`/`### Changed`/etc. Nenhum comando hoje o lê — confirmado por
`grep -rn "CHANGELOG" internal/commands/`.

Comandos de referência (mesmo padrão de "ler algo da raiz do projeto e imprimir"):
`internal/commands/status.go` (Go), `npm/src/commands/status.js` (Node),
`pypi/trackfw/commands/status.py` (Python) — todos delegam o trabalho pesado para um
módulo dedicado (`internal/validator`, `npm/src/validator`, `pypi/trackfw/validator.py`)
e o comando em si só chama a função e imprime. Seguir o mesmo padrão de separação:
lógica de parsing em um módulo novo (`internal/changelog/`, `npm/src/changelog/`,
`pypi/trackfw/changelog.py`), comando fino em `internal/commands/changelog.go` (e
equivalentes) só faz parse de flags + chama o módulo + imprime.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `trackfw changelog` (sem flags) imprime a primeira seção `## [...]` do
      `CHANGELOG.md` da raiz do projeto (seja `[Unreleased]` ou a versão mais recente),
      formatada no terminal.
- [x] `trackfw changelog --version <x.y.z>` imprime a seção daquela versão específica;
      erro claro se não existir.
- [x] `trackfw changelog --all` imprime o arquivo inteiro.
- [x] Parsing tolerante ao formato real do `CHANGELOG.md` deste projeto.
- [x] Comportamento idêntico nos 3 CLIs, mensagens de erro byte-idênticas.
- [x] `make quality` passa sem novas divergências de paridade.

## Wave 1 — Go (implementação de referência, 1 ML)
> Dependências: nenhuma

### ML-1A — Módulo `internal/changelog` + comando `trackfw changelog`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/changelog/changelog.go` (novo) — parsing e extração de seções
- `internal/changelog/changelog_test.go` (novo)
- `internal/commands/changelog.go` (novo) — comando cobra fino
- `internal/commands/changelog_test.go` (novo)
- `internal/commands/root.go` (registrar `newChangelogCmd()` em `root.AddCommand(...)`)
**Ações:**
1. Em `internal/changelog/changelog.go`, criar:
   - `type Section struct { Version string; Date string; Body string }` — `Version` é
     `"Unreleased"` (sem colchetes) ou `"x.y.z"`; `Body` é o texto completo da seção
     (incluindo as subseções `### Added` etc.), sem a linha de cabeçalho `## [...]`.
   - `func ParseSections(content string) ([]Section, error)` — separa o arquivo por
     linhas que casam o regex `^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?`; tudo entre
     um cabeçalho de seção e o próximo (ou EOF) vira `Body` da seção anterior; texto
     antes da primeira seção (título do arquivo, preâmbulo "Todas as mudanças...") é
     descartado, não é uma `Section`.
   - `func FirstSection(sections []Section) (Section, error)` — retorna `sections[0]`;
     erro `"CHANGELOG.md has no version sections"` se a lista vier vazia.
   - `func FindVersion(sections []Section, version string) (Section, error)` — busca
     por `Version == version` (comparação exata, sem "v" prefixado — aceitar tanto
     `1.2.3` quanto normalizar removendo um `v`/`V` inicial do argumento do usuário
     antes de comparar); erro
     `fmt.Sprintf("version %q not found in CHANGELOG.md", version)` se não achar.
   - `func FormatSection(s Section) string` — retorna `fmt.Sprintf("## [%s]%s\n\n%s",
     s.Version, dateSuffix, strings.TrimRight(s.Body, "\n")+"\n")` onde `dateSuffix` é
     `" - "+s.Date` se `s.Date != ""`, senão vazio — reproduz o cabeçalho original.
   - `func Read(root string) (string, error)` — lê `filepath.Join(root, "CHANGELOG.md")`;
     erro `"CHANGELOG.md not found — nothing to show"` se `os.IsNotExist`.
2. Em `internal/commands/changelog.go`:
   ```go
   func newChangelogCmd() *cobra.Command {
       var versionFlag string
       var allFlag bool
       cmd := &cobra.Command{
           Use:   "changelog",
           Short: "Show entries from CHANGELOG.md",
           RunE: func(cmd *cobra.Command, args []string) error {
               root, err := os.Getwd()
               if err != nil { return err }
               content, err := changelog.Read(root)
               if err != nil { return err }
               if allFlag {
                   fmt.Print(content)
                   return nil
               }
               sections, err := changelog.ParseSections(content)
               if err != nil { return err }
               var section changelog.Section
               if versionFlag != "" {
                   section, err = changelog.FindVersion(sections, versionFlag)
               } else {
                   section, err = changelog.FirstSection(sections)
               }
               if err != nil { return err }
               fmt.Print(changelog.FormatSection(section))
               return nil
           },
       }
       cmd.Flags().StringVar(&versionFlag, "version", "", "Show a specific version section")
       cmd.Flags().BoolVar(&allFlag, "all", false, "Show the entire CHANGELOG.md")
       return cmd
   }
   ```
   (usar `os.Getwd()` — mesmo padrão de resolução de raiz que `internal/commands/status.go`
   já usa via `validator.GetStatus()`, que também assume cwd == raiz do projeto; não
   inventar um mecanismo de resolução de raiz novo.)
3. Registrar `newChangelogCmd()` em `internal/commands/root.go`, na lista
   `root.AddCommand(...)`, ao lado de `newStatusCmd()`.
4. Testes em `internal/changelog/changelog_test.go`: `ParseSections` com 3 seções
   (`Unreleased` + 2 versões, uma com data, `Unreleased` sem data) — valida `Body` de
   cada uma; `FirstSection` em lista vazia → erro; `FindVersion` com `"6.10.0"` presente
   e ausente; `FindVersion("v6.10.0")` normaliza o "v" e encontra a mesma seção que
   `"6.10.0"`.
5. Testes em `internal/commands/changelog_test.go`: `trackfw changelog` sem flags
   imprime a primeira seção de um `CHANGELOG.md` fixture; `--version` com versão
   existente/inexistente; `--all` imprime o arquivo inteiro; `CHANGELOG.md` ausente →
   mensagem de erro exata.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/changelog/... ./internal/commands/...` verde
- [ ] `go vet ./...` sem warnings
- [ ] Teste manual: `bin/trackfw changelog` na raiz deste repo imprime a seção
      `[Unreleased]` atual; `bin/trackfw changelog --version 6.10.0` imprime a seção da
      release 6.10.0; `bin/trackfw changelog --all` imprime o arquivo inteiro
**Comandos de validação:** `go build ./... && go test ./internal/changelog/... ./internal/commands/... && go vet ./...`

**Execução real (2026-08-15):** implementado por Apolo, auditado pelo orquestrador.
`go build`/`go vet`/`go test ./internal/changelog/... ./internal/commands/...` verdes.
Ajuste de qualidade aplicado após a 1ª entrega: `FormatSection` produzia linha em
branco duplicada quando o `Body` já começava com `\n` (caso real do `CHANGELOG.md`
deste projeto, cujo cabeçalho é seguido de linha em branco) — corrigido com
`strings.TrimLeft(s.Body, "\n")` antes de montar a string final; teste de regressão
`TestFormatSectionDoesNotDuplicateBlankLineWhenBodyStartsWithNewline` adicionado. Teste
manual confirmado: `bin/trackfw changelog`, `--version 6.10.0`, `--version 999.0.0`
(erro claro, exit 1) e `--all` produzem saída correta contra o `CHANGELOG.md` real.

## Wave 2 — Node.js e Python (2 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa

### ML-2A — Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/changelog/index.js` (novo) — port 1:1 de `internal/changelog/changelog.go`:
  `parseSections(content)`, `firstSection(sections)`, `findVersion(sections, version)`,
  `formatSection(section)`, `read(root)` — mesmas mensagens de erro, byte-idênticas ao Go
- `npm/src/commands/changelog.js` (novo) — comando commander fino, mesma estrutura de
  `npm/src/commands/status.js` (usa `process.cwd()` como raiz, mesmo padrão que
  `status.js` já usa)
- `npm/src/commands/index.js` (registrar o novo comando na lista de comandos exportados/
  registrados — seguir o mesmo padrão de registro que `status`/`context` já usam)
- `npm/tests/changelog.test.js` (novo)
**Ações:** replicar 1:1 a lógica do ML-1A em JS puro, lendo o Go real (já implementado
nesta branch) como fonte de verdade — mesmas mensagens de erro, mesmo formato de saída
de `FormatSection`.
**Critérios de aceite:**
- [ ] `cd npm && npm test` verde, mesmos casos do ML-1A
- [ ] mensagens de erro e saída formatada idênticas (byte-a-byte) às do Go
**Comandos de validação:** `cd npm && npm test`

**Execução real (2026-08-15):** `npm test` → 549 passed, 0 failed. Ajuste não previsto
no roadmap: `--version` global do commander (registrado no root) interceptava a flag
`--version <x.y.z>` do subcomando `changelog` — corrigido com `program.enablePositionalOptions()`
em `npm/src/commands/index.js` e `cmd.enablePositionalOptions()` em `changelog.js`;
`trackfw --version`/`trackfw version` continuam funcionando (testes de regressão
verdes). Paridade byte-a-byte confirmada pelo orquestrador contra o binário Go nos 4
cenários (sem flags, `--version` existente, `--all`, `--version` inexistente).

### ML-2B — Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/changelog.py` (novo) — port 1:1 de `internal/changelog/changelog.go`:
  `parse_sections(content)`, `first_section(sections)`, `find_version(sections, version)`,
  `format_section(section)`, `read(root)` — mesmas mensagens de erro, byte-idênticas ao Go
- `pypi/trackfw/commands/changelog.py` (novo) — comando argparse fino, mesma estrutura de
  `pypi/trackfw/commands/status.py` (usa `os.getcwd()` como raiz)
- `pypi/trackfw/cli.py` (registrar o subparser `changelog`, mesmo padrão de `status`/`context`)
- `pypi/tests/test_changelog.py` (novo)
**Ações:** replicar 1:1 a lógica do ML-1A em Python puro, lendo o Go real como fonte de
verdade.
**Critérios de aceite:**
- [ ] `python -m pytest pypi/tests -k changelog` verde, mesmos casos do ML-1A
- [ ] mensagens de erro e saída formatada idênticas ao Go
**Comandos de validação:** `python -m pytest pypi/tests -k changelog`

**Execução real (2026-08-15):** `pytest pypi/tests -k changelog` → 13 passed; suíte
completa `pytest pypi/tests` → 1142 passed, 8 subtests, 0 falhas. Paridade byte-a-byte
confirmada pelo orquestrador contra o binário Go nos 4 cenários. `find_version`/
`format_section` seguem a mesma semântica de `TrimLeft`/`TrimRight` do Go.

## Wave 3 — Validação cruzada (1 ML)
> Dependências: Wave 2 completa

### ML-3A — Paridade e teste manual end-to-end
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/cli-parity.md` (linha `changelog` adicionada ao inventário
de comandos — gap encontrado pelo Apolo durante o ML-2B, corrigido nesta ML)
**Ações:**
1. Rodar `make quality` na raiz.
2. Rodar os 3 binários (Go/Node/Python) com `changelog`, `changelog --version 6.10.0`,
   `changelog --all` e `changelog --version 999.0.0` (versão inexistente) contra o
   `CHANGELOG.md` real deste repo — confirmar saída byte-idêntica nos 3.
**Critérios de aceite:**
- [x] `make quality` verde
- [x] os 4 cenários confirmados idênticos nos 3 CLIs
**Comandos de validação:** `make quality`

**Execução real (2026-08-15):** `make quality` verde (build+vet+test Go, `npm test` 549
passed, `pytest` 1142 passed, 112 cenários de falsificação todos OK). Os 4 cenários
(sem flags, `--version` existente, `--all`, `--version` inexistente) confirmados
byte-idênticos entre os 3 binários via `diff` direto pelo orquestrador. Adicionada
linha `changelog` em `docs/cli-parity.md` (ausente do inventário, achado do Apolo).
