---
status: done
date: 2026-07-30
req: "REQ-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go"
squad: ""
---

# Roadmap: reservar -v para verbose e remover atalho de versao no Go

> Created: 2026-07-30 | Status: done

## Contexto

REQ: `docs/req/REQ-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md`

`-v` é aceito apenas pelo Go, como atalho de `--version`, exposto por default do cobra
(`InitDefaultVersionFlag` registra o shorthand `v` quando o campo `Version` está preenchido). Decisão:
**remover** e **reservar** `-v` / `--verbose` para futuro modo verboso nos três.

**Escopo negativo:** não implementar verbose; não aceitar `-v` como no-op; não unificar mensagem nem
exit code de flag desconhecida (divergência pré-existente de framework, vale para toda flag).

## Critérios de Aceite

- [x] `trackfw -v` não imprime versão em runtime nenhum e sai com código não-zero nos três — Go passa a
      `Error: unknown shorthand flag: 'v' in -v` (exit 1); Node.js exit 1; Python exit 2.
- [x] `--version` e `version` permanecem inalterados, byte-idênticos nos três. A regressão do
      `SetVersionTemplate` que este roadmap antecipou **não** ocorreu.
- [x] `cli-parity.md` registra `-v` / `--verbose` como reservado, com proibição explícita, motivo, e a
      fronteira do que **não** é unificado, com baseline medido.
- [x] Gate cobre `-v` nos três com **duas** asserções (exit não-zero **e** saída que não casa o formato
      de versão), com vacuity-guard. Falsificação no Cenário 23.
- [x] `make quality` exit 0 (**24** cenários de falsificação, eram 23) e `validate --json` 0 violações.
      Barrier das três waves: `passed`.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier
Wave 2 — ML-2A (Go, único com mudança de código)
   ↓ barrier
Wave 3 — ML-3A (gate + falsificação)
```

**Não há waves paralelas nesta entrega.** Só o Go muda comportamento; Node.js e Python já rejeitam `-v`
e não têm o que alterar. Criar MLs vazios para eles seria cerimônia sem conteúdo — a paridade é
verificada pelo gate no ML-3A, que é onde ela pertence.

### Risco específico desta entrega

O `-v` do Go **não é declarado no código** — vem do default do cobra. Um implementador que procure por
`"v"` em `internal/commands/` não encontra nada e pode concluir que já está removido. O ML-2A precisa
verificar por **execução**, não por leitura.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar a reserva do `-v`
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:** `docs/cli-parity.md` — seção `## Version output`

**Deve pinar:**
1. `-v` **não é** atalho de `--version` em runtime nenhum; sai com código não-zero nos três.
2. `-v` / `--verbose` é **reservado** para modo verboso. Proibido vinculá-lo a outra semântica.
3. A razão da reserva: convenção do ecossistema (`docker`, `kubectl`, `ansible`, `ssh`, `curl`), e o
   fato de que nenhum dos três CLIs tem `--verbose` hoje.
4. **Fronteira explícita:** mensagem e exit code de flag desconhecida **não** são unificados —
   divergência pré-existente de framework (cobra 1, commander 1, argparse 2), válida para toda flag.
   Registrar o baseline medido, para ninguém tentar alcançar identidade byte-a-byte e recorrer a hack.
5. Que a reserva é **de contrato**, não de superfície: nenhum runtime aceita `-v` como no-op, porque
   flag aceita sem efeito é indistinguível de flag quebrada.

**Seção escrita:** `### \`-v\` is reserved for verbose — never bound to \`--version\`` em
`docs/cli-parity.md`, substituindo a antiga `### Out of scope: the -v shorthand`.

**Critérios de aceite:**
- [x] Reserva e proibição registradas com o motivo — convenção do ecossistema e ausência de `--verbose`
      nos três hoje. Registrado também que a flag foi exposta por **default de framework**, não por
      decisão de design.
- [x] Fronteira de não-unificação registrada com o baseline medido (`--zzz`: cobra 1, commander 1,
      argparse 2, três textos distintos), com a justificativa de por que forçar identidade seria escopo
      muito maior — e a nota explícita de que a fronteira existe para ninguém tentar e recorrer a hack.
- [x] Razão de não aceitar no-op registrada: flag aceita sem efeito é indistinguível de flag quebrada.
- [x] Registrado que implementar verbose **não** faz parte da reserva, e por quê.

---

## Wave 2 — Remover o atalho no Go (1 ML)
> Dependências: ML-1A completo.

### ML-2A — Desvincular `-v` de `--version` no Go
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `internal/commands/root.go`, `internal/commands/version_test.go`

**Diagnóstico:** o shorthand vem do cobra, não do código do projeto. Com `Version` preenchido
(`root.go:22`), o `InitDefaultVersionFlag` registra `--version` com shorthand `v` se o atalho estiver
livre. **Não existe nenhuma declaração de `-v` para procurar e apagar.**

**Solução aplicada:** pré-registrar `root.Flags().Bool("version", false, "version for trackfw")` em
`newRootCmd()` antes de `AddCommand`. O cobra só executa `InitDefaultVersionFlag` se
`Flags().Lookup("version") == nil` — portanto a pré-declaração impede o registro do shorthand `v`.
O `SetVersionTemplate` continua ativo porque o cobra detecta `version=true` na execução e aplica o
template normalmente.

**Evidência de execução:**
```
$ bin/trackfw version
trackfw 5.0.0
EXIT_VERSION: 0

$ bin/trackfw --version
trackfw 5.0.0
EXIT_FLAG: 0

$ diff <(bin/trackfw version) <(bin/trackfw --version)
[vazio — byte-idênticos]

$ bin/trackfw -v
Error: unknown shorthand flag: 'v' in -v
EXIT_V: 1
```

**Critérios de aceite:**
- [x] `trackfw -v` não imprime versão e sai com código não-zero.
- [x] `trackfw --version` e `trackfw version` inalterados: `trackfw 5.0.0`, byte-idênticos entre si.
- [x] Teste travando a rejeição de `-v` (`TestShortVFlagRejected`, `TestShorthandVNotRegistered`) e a preservação das duas superfícies (testes do PR #91 inalterados).
- [x] `go build ./...`, `go test ./...`, `go vet ./...` passam.

---

## Wave 3 — Gate e falsificação (1 ML)
> Dependências: **barrier** — ML-2A concluído.

### ML-3A — Cobrir `-v` no gate e provar não-vacuidade
**Status:** ✅ Concluído
**Agente:** Artemis
**Commits:** `6b8011c` (gate positivo), `f110c02` (Cenário 23)

**Ações realizadas:**
1. Bloco `-v flag` inserido em `scripts/check-cli-parity.sh` antes de `check-integration-cli-parity.sh`.
   Três estágios por runtime: vacuity-guard (saída não-vazia), Assertion-1 (exit -ne 0), Assertion-2
   (grep -Eq negativo contra `_VERSION_RE`). Nenhum runtime produz linha matching a regex na rejeição:
   Go (`Error: unknown shorthand flag: 'v' in -v` + usage), Node (`error: unknown option '-v'`),
   Python (usage + `error: unrecognized arguments: -v`).
2. Cenário 23 em `scripts/check-gates-falsify.sh`: copia cmd/ + internal/ + go.mod/go.sum; remove a
   pré-declaração `root.Flags().Bool("version", ...)` via sed (guarda de padrão: `cmp -s`); guarda de
   vivacidade (`build_go_or_fail` + confirmação que `-v` exits 0 com versão no formato esperado);
   `assert_fails_with "cli-parity/v-flag-accepted"` rodando o gate com `cd T23` para que
   `go build ./cmd/trackfw` use o internal/ corrompido. Seam Go-only (Node/Python já rejeitavam -v).
3. Cenários 21 e 22 do PR #91 permanecem verdes (`cli-parity/version-v-prefix`,
   `cli-parity/version-byte-mismatch`). Total: 23 → 24 cenários; 14 gates.

**Critérios de aceite:**
- [x] Cenário cobre `-v` nos três, com as duas asserções.
- [x] Seam verificado por execução: com o atalho reintroduzido, o gate **falha**.
- [x] Cenários de `version` / `--version` inalterados e verdes.
- [x] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.

---

## Matriz de verificação empírica do orquestrador (Wave 2)

Executando os CLIs, não lendo relatórios.

| Invocação | Go | Node.js | Python |
|---|---|---|---|
| `version` | `trackfw 5.0.0` (exit 0) | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `--version` | `trackfw 5.0.0` (exit 0) | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `-v` | `Error: unknown shorthand flag: 'v' in -v` (**exit 1**) | rejeitado (exit 1) | rejeitado (exit 2) |

`diff` entre `version` e `--version` no Go: **sem diferenças**. Os três rejeitam `-v` com código
não-zero, e nenhum imprime a versão.

**A regressão que o roadmap antecipou não ocorreu.** O `SetVersionTemplate("trackfw {{.Version}}\n")`
continua sendo aplicado: o caminho escolhido pré-registra `Flags().Bool("version", ...)` **sem**
shorthand, e o cobra só adiciona a flag dele quando `Flags().Lookup("version") == nil` — então o
template permanece ativo porque o cobra ainda detecta `version=true` em execução e o aplica. O
`--version` do PR #91 não regrediu.

Os exit codes divergentes (1 · 1 · 2) são exatamente o baseline pré-existente de framework registrado
no contrato, e **não** são objeto desta entrega.

**Suítes:** Go limpo · `npm test` 342 passed · `pytest` 727 passed.
