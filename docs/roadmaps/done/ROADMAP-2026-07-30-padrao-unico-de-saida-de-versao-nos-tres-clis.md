---
status: done
date: 2026-07-30
req: "REQ-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis"
squad: ""
---

# Roadmap: padrao unico de saida de versao nos tres CLIs

> Created: 2026-07-30 | Status: done

## Contexto

REQ: `docs/req/REQ-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis.md`

Duas divergências medidas na `v5.0.0`:

| Superfície | Go | Node.js | Python |
|---|---|---|---|
| `version` | `trackfw v5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `--version` | `trackfw v5.0.0` | **`5.0.0`** | `trackfw 5.0.0` |

E o gate de paridade **assina** a do Node: `check-cli-parity.sh:108` usa regex própria para ele.

**Formato canônico decidido:** `trackfw <semver>`, **sem** prefixo `v`, idêntico nas duas superfícies e
nos três runtimes. A tag Git permanece `v<x.y.z>`.

## Critérios de Aceite

- [x] `version` e `--version` imprimem a **mesma linha**, `trackfw <semver>` sem `v`, nos três runtimes —
      seis saídas byte-idênticas, `hexdump` confirma 14 bytes (`trackfw 5.0.0\n`).
- [x] `internal/version/version.go` armazena a versão sem `v`. Corrigiu as duas superfícies de uma vez,
      porque o cobra usa `SetVersionTemplate("trackfw {{.Version}}\n")` sobre a mesma constante.
- [x] `check-cli-parity.sh` usa a mesma asserção para os três, exigindo o formato exato
      (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`). A regex de exceção do Node.js foi **removida** — sobrevive
      apenas num comentário que registra o que havia ali e por quê.
- [x] Cenário byte-a-byte das seis saídas em `make quality`, com vacuity-guard, single-line guard e
      guard de exit não-zero por captura.
- [x] **Dois** cenários de falsificação, provando os **braços independentes** da asserção:
      Cenário 21 reintroduz o `v` e falha no braço de **formato**; Cenário 22 corrompe `package.json`
      para `9.9.9`, mantendo o formato válido nos seis, e falha só no braço de **bytes**. Sem o segundo,
      metade da asserção ficaria não-provada — o Cenário 21 falha antes de alcançá-la.
- [x] `make quality` exit 0 (23 cenários de falsificação, eram 21) e `validate --json` 0 violações.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier
Wave 3 — ML-3A (gate unificado + falsificação)
```

### Lição acumulada, aplicada ao ML-1A

O ML-1A falhou por **quatro** roadmaps seguidos pelo mesmo padrão: pinar a forma e deixar o
comportamento à interpretação — nomes sem valores, regexes sem o momento da validação, cardinalidades
sem ordem, ordem "de varredura" que não é ordem. Aqui o contrato pina o **texto literal exato** das duas
superfícies e a **asserção literal** que o gate deve usar, não a descrição do formato.

**Risco específico desta feature:** a asserção atual (`^trackfw .+`) é frouxa o bastante para aceitar as
duas formas. Se o ML-3A reaproveitar essa regex, o gate nasce vacuoso e o `v` volta sem ninguém ver. O
gate tem de comparar **bytes entre runtimes**, não casar um padrão permissivo.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar o formato e a asserção do gate
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:** `docs/cli-parity.md` — linha `version` / `--version` da tabela de comandos e
seção própria

**Deve pinar:**
1. **Texto literal** das duas superfícies: `trackfw <semver>`, sem `v`, sem sufixo, uma linha, stdout.
2. **`version` e `--version` produzem saída idêntica** — mesma linha, byte a byte.
3. **Fonte da string** por runtime: Go de `internal/version/version.go` (sem `v`); Node.js de
   `npm/package.json`; Python de `importlib.metadata` com fallback literal.
4. **Asserção do gate**, literal: mesma para os três, exigindo `^trackfw [0-9]+\.[0-9]+\.[0-9]+$`, e
   **comparação byte-a-byte entre runtimes** — a regex sozinha não basta.
5. Registro de que a tag Git permanece `v<x.y.z>` e por quê.
6. Registro de que `-v` está **fora de escopo**, com o motivo.

**Seção escrita:** `## Version output` em `docs/cli-parity.md`, mais a linha `version` / `--version` da
tabela de comandos.

**Critérios de aceite:**
- [x] Texto literal das duas superfícies pinado — tabela com nome do programa, formato SemVer sem `v`,
      uma linha, stdout.
- [x] Equivalência `version` ≡ `--version` pinada como **byte-idêntica**, dentro e entre runtimes.
- [x] Asserção do gate pinada literalmente (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) **mais** a exigência de
      comparação byte-a-byte — com o registro de por que a asserção anterior era vacuosa: `^trackfw .+`
      aceitava as duas formas, e o Node.js tinha regex própria que assinava a divergência.
- [x] Escopo negativo registrado: `-v` fora de escopo com o motivo, tag Git permanece `v<x.y.z>`,
      manifestos inalterados.
- [x] Fonte da string por runtime documentada, incluindo o fato de que no Go as duas superfícies
      consomem a mesma constante — logo uma única mudança corrige ambas.

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Arquivos disjuntos — **spawn simultâneo**.

### ML-2A — Go
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `internal/version/version.go`, `internal/commands/version.go`, `internal/commands/version_test.go`

**Ação:** remover o `v` da constante. Isso corrige `version` e `--version` de uma vez, porque ambas
consomem `version.Version` (`internal/commands/version.go:15` e `internal/commands/root.go:22`).

Adicionalmente, `internal/commands/version.go` foi ajustado para usar `fmt.Fprintln(cmd.OutOrStdout(), ...)` em vez de `fmt.Println(...)`, tornando a saída capturável pelo writer do cobra nos testes.

**Evidência empírica (commit f7785ea):**
```
bin/trackfw version   → trackfw 5.0.0
bin/trackfw --version → trackfw 5.0.0
diff → byte-idênticos
```

**Critérios de aceite:**
- [x] `version` e `--version` imprimem `trackfw <semver>` sem `v`, idênticos entre si.
- [x] Teste travando o formato exato das duas superfícies (`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`) e igualdade byte-a-byte (`TestVersionSurfacesByteIdentical`).
- [x] `go build ./...`, `go test ./...`, `go vet ./...` passam.

### ML-2B — Node.js
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `npm/src/commands/index.js`, `npm/tests/version.test.js`

**Ação:** `.version(version)` → `.version(`trackfw ${version}`)` no commander. Saída verificada:
```
node npm/bin/trackfw version   → trackfw 5.0.0
node npm/bin/trackfw --version → trackfw 5.0.0
BYTE-IDENTICAL: ok
```

**Critérios de aceite:**
- [x] `--version` passa a imprimir `trackfw <semver>`, idêntico ao `version`.
- [x] `cd npm && npm test` passa (342 pass, 0 failed).

### ML-2C — Python
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `pypi/tests/test_commands_basic.py`

**Ação:** o Python **já estava no formato canônico** nas duas superfícies — verificado empiricamente via
`xxd`. Este ML adicionou cobertura: três testes que travam o contrato byte-a-byte.

**Saída verificada (xxd):**
```
version:   74 72 61 63 6b 66 77 20 35 2e 30 2e 30 0a  → trackfw 5.0.0\n
--version: 74 72 61 63 6b 66 77 20 35 2e 30 2e 30 0a  → trackfw 5.0.0\n
BYTE-IDENTICAL: ok
```

**Critérios de aceite:**
- [x] Formato verificado empiricamente nas duas superfícies, com comparação byte-a-byte entre elas.
- [x] Teste travando o formato exato de `version` e `--version` (`test_version_flag_format_exact`,
      `test_version_subcommand_format_exact`, `test_version_surfaces_byte_identical`).
- [x] Nenhuma mudança de comportamento — `__init__.py` sem prefixo `v`, fallback `"5.0.0"` correto.
- [x] Suíte Python passa: 727 pass, 0 failed.

---

## Wave 3 — Gate unificado e falsificação (1 ML)
> Dependências: **barrier** — ML-2A, ML-2B e ML-2C concluídos.

### ML-3A — Unificar a asserção e provar não-vacuidade
**Status:** ✅ Concluído
**Agente:** Artemis
**Commit:** 459edd6

**Ações:**
1. **Remover a regex específica do Node.js** de `check-cli-parity.sh:108`. Os três passam a usar a mesma
   asserção literal pinada no contrato.
2. Cenário comparando **bytes** das duas superfícies entre os três runtimes.
3. Cenário de falsificação: seam que reintroduz o `v` num runtime e prova que o gate reprova. Corromper a
   **implementação**, nunca a asserção, com guarda de padrão contra `sed` obsoleto.

**Detalhes da implementação:**

`check-cli-parity.sh` — linhas 103-109 substituídas por:
- Captura das 6 saídas com guard de exit não-zero por superfície.
- Vacuity guard: falha explícita se qualquer saída for vazia.
- Single-line guard: contrato exige exatamente uma linha (detecta warning preamble).
- Format assertion única `^trackfw [0-9]+\.[0-9]+\.[0-9]+$` para os 3 runtimes × 2 superfícies.
- Byte-comparison: as 6 saídas devem ser idênticas byte a byte.

`check-gates-falsify.sh` — dois novos cenários:
- **Cenário 21 (seam A — regex arm):** corrompe `npm/src/commands/version.js` para `trackfw v${version}`;
  gate reprova com `"node version format invalid"`.
- **Cenário 22 (seam B — byte-comparison arm):** corrompe `npm/package.json` para versão `9.9.9`;
  Node imprime `trackfw 9.9.9`, Go/Python imprimem `5.0.0`; formato passa a regex, byte-comparison reprova.
- Total de cenários: 21 → 23 (14 gates provados não-vacuosos, contagem estável).

**Evidências empíricas:**
```
$ bash scripts/check-cli-parity.sh
Integration CLI parity lifecycle checks passed
CLI parity smoke checks passed
EXIT=0

$ bash scripts/check-gates-falsify.sh
...
OK   [falsify/cli-parity/version-v-prefix]
OK   [falsify/cli-parity/version-byte-mismatch]
Falsification checks passed (all 23 scenarios, 14 gates proved non-vacuous)
EXIT=0

$ make quality → EXIT=0

$ bin/trackfw validate --json
{"summary":{"violations":0,"warnings":0,"mode":"strict","exit_code":0},"violations":[],"warnings":[]}

$ git status --short → (vazio após commit)
```

**Critérios de aceite:**
- [x] Regex específica do Node.js removida; asserção única para os três.
- [x] Comparação byte-a-byte das duas superfícies.
- [x] Seam verificado por execução: com o `v` reintroduzido, o gate **falha** (Cenário 21 OK); com versão divergente, byte-comparison **falha** (Cenário 22 OK).
- [x] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.

---

## Matriz de verificação empírica do orquestrador (Wave 2)

Executando os três CLIs, não lendo relatórios.

| Superfície | Go | Node.js | Python |
|---|---|---|---|
| `version` | `trackfw 5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |
| `--version` | `trackfw 5.0.0` | `trackfw 5.0.0` | `trackfw 5.0.0` |

`diff` entre os três runtimes e entre as duas superfícies: **sem diferenças**.
`hexdump`: `74 72 61 63 6b 66 77 20 35 2e 30 2e 30 0a` — 14 bytes, `trackfw 5.0.0\n`.

**Suítes:** Go limpo · `npm test` 342 passed · `pytest` 727 passed.

### O gate agora reprova — e isso é o resultado esperado

```
$ bash scripts/check-cli-parity.sh
EXIT=1
```

A regex de exceção do Node.js (`^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$`, linha 108) deixou de casar assim
que o `--version` do Node passou a imprimir `trackfw 5.0.0`. A linha que **assinava** a divergência
agora **bloqueia** a convergência — prova direta de que a exceção por runtime era o que mantinha o
problema invisível.

`make quality` permanece vermelho até o ML-3A remover a exceção. Estado intermediário legítimo: os
critérios de aceite da Wave 2 exigem as suítes por runtime, não o gate agregado, justamente porque o
gate é o objeto do ML-3A.
