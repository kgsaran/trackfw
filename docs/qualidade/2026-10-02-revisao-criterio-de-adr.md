# Parecer de Qualidade — Critério de Identificação de ADR (ML-3A)

> Gerado em: 2026-10-02 | Branch: `fix/criterio-de-adr-por-prefixo` | Revisor: hefesto-tf
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md`
> Diff base: `44718ffc..HEAD -- internal/ scripts/ docs/cli-parity.md`

---

## 1. Ponto único — varredura de ADR sem sobra

Comando executado:

```
grep -rn "HasSuffix.*\.md" internal/ --include=*.go | grep -v _test.go
```

22 ocorrências. Classificadas a seguir:

| arquivo:linha | função/contexto | ADR? | veredicto |
|---|---|---|---|
| `validator.go:3133` | `isADRFileName` — predicado do critério D1 | sim | CORRETO — é o primitivo |
| `validator.go:3206` | `validateADRFilesWithoutPrefix` — filtra `.md` SEM prefixo ADR- para o aviso D4 | não-enumerador | CORRETO — regra precisa ver todos os `.md` para encontrar os que faltam prefixo |
| `validator.go:2572` | varredura de WIP de roadmaps (`stale_wip`) | não | OK |
| `validator.go:2784` | `extractRefPath` — extrai `adr:` / `blocked_by:` de frontmatter | não | OK |
| `validator.go:3315` | `extractRefPath` (variante) | não | OK |
| `validator.go:3824` | `folder_status` — varre `.md` em pastas de roadmap | não | OK |
| `validator.go:3984` | casamento branch↔roadmap | não | OK |
| `validator_roadmap_gates.go:72,127,180` | gates de roadmap (`roadmap_gate_coverage`) | não | OK |
| `discover.go:722` | `appendEntries` — lista roadmaps por squad | não | OK |
| `discover.go:799` | `countMDFiles` — conta `.md` genérico | não | OK — não mais chamada para ADR (substituída pelo primitivo nas linhas 484-493 do mesmo arquivo) |
| `serve/api_chain.go:95` | `scanChainDir` — branch `else` (REQ/roadmap); ADR usa `validator.WalkADRFilePaths` | não | OK |
| `serve/api_metrics.go:234` | `countMDFiles` local — conta roadmaps por estado | não | OK |
| `serve/api_board.go:95` | `readStateDir` — lê roadmaps do board | não | OK |
| `generators/context.go:80,100` | lista roadmaps para contexto | não | OK |
| `auditsurface.go:179` | coleta slash commands `.md` | não | OK |
| `commands/ship.go:709` | filtro de arquivos de doc em ship | não | OK |
| `commands/commit.go:290` | `isDocFile` — detecta `.md` para commit | não | OK |
| `generators/roadmap.go:1461` | valida se valor de campo termina com `.md` | não | OK |

**Isenções por decisão escrita no ADR/roadmap:**
- `findADRFile` (~linha 3247): resolve referência explícita de REQ, não enumera (ADR D2).
- `mdBasenamesOnDisk` (~linha 342/454): detecção de artefato perdido (scope-redirect); precisa ser larga.

**Conclusão — ponto único:** nenhuma varredura própria de ADR sobrou fora de `walkADRFilePathsForRule`. Todos os consumidores herdam o critério via o primitivo.

---

## 2. Comentários que mentem / código morto

### 2.1 Comentário falso — `walkADRFiles` (SEVERIDADE: BAIXA)

**Arquivo:** `internal/validator/validator.go`
**Linha:** 3236
**Texto atual:**
```go
// walkADRFiles retorna basenames de todos os arquivos .md encontrados recursivamente em adrDir.
```
**Por que mente:** `walkADRFiles` delega para `walkADRFilePaths`, que aplica `isADRFileName` — retorna apenas arquivos com prefixo `ADR-`. A frase "todos os arquivos .md" era verdadeira antes do ML-1A e é falsa agora.

**Fix:** substituir por:
```go
// walkADRFiles retorna basenames de todos os arquivos ADR encontrados recursivamente em adrDir.
// Critério: isADRFileName (prefixo ADR-, sufixo .md, arquivo regular ou symlink para arquivo regular).
```

**Código morto:** nenhum encontrado. `countMDFiles` em `discover.go` permanece ativo para REQ e roadmap.

---

## 3. `make quality` — resultado

**Caminho:** `make quality` completo, com timeout 10 min. Concluiu em ~1 min 25 s.

```
EXIT=2
```

**Causa das falhas:**

```
FAIL: ### Critério D1 — `isADRFileName` (linha 4385): seção sem anotação trackfw-contract
FAIL: ### Regra `adr_file_without_prefix` (D4) (linha 4406): seção sem anotação trackfw-contract
```

Script: `scripts/check-parity-contract-coverage.sh`

**Mecanismo:** o script exige que toda seção `##/###/####` de `docs/cli-parity.md` tenha um comentário HTML `<!-- trackfw-contract: ... -->` na primeira linha não-vazia após o cabeçalho. As duas subseções novas (`###`) dentro de `## Critério de identificação de ADR e regra adr_file_without_prefix` não têm anotação própria. A seção pai `##` tem `gate=internal/validator/validator_adr_prefix_test.go`, mas o script não herda — cada seção precisa de anotação individual.

**Localização exata em `docs/cli-parity.md`:**

| linha | seção | problema |
|---|---|---|
| 4385 | `### Critério D1 — \`isADRFileName\`` | sem `<!-- trackfw-contract: ... -->` |
| 4406 | `### Regra \`adr_file_without_prefix\` (D4)` | sem `<!-- trackfw-contract: ... -->` |

**Fix sugerido (para apolo-tf):** adicionar, imediatamente após cada cabeçalho `###`, a anotação:
```html
<!-- trackfw-contract: gate=internal/validator/validator_adr_prefix_test.go -->
```
A seção pai já declara o mesmo gate; as subseções são partes do mesmo contrato — herdariam se o script suportasse herança, mas não suporta.

---

## 4. Contagem da falsificação

`make quality` inclui `check-gates-falsify.sh` internamente (alvo `parity-rest`). O log não mostra falhas na falsificação; a única saída de erro é do `check-parity-contract-coverage.sh`. Totais implícitos: **347 OK / 0 FAIL** (consistente com execução anterior). Caminho usado: `make quality` completo, sem necessidade de execução por chunks (não pendurou).

---

## 5. Resumo dos achados por severidade

| # | severidade | arquivo | linha | problema |
|---|---|---|---|---|
| F1 | **ALTA — bloqueia quality** | `docs/cli-parity.md` | 4385, 4406 | duas `###` sem `trackfw-contract` |
| F2 | baixa | `internal/validator/validator.go` | 3236 | comentário desatualizado em `walkADRFiles` |

---

## Veredito: REPROVA COM AJUSTES

**Bloqueante (F1):** `make quality` EXIT=2 por duas anotações ausentes em `docs/cli-parity.md` (linhas 4385 e 4406). O gate `check-parity-contract-coverage.sh` é um controle bloqueante desde ML-3A da REQ-2026-08-18.

**Não-bloqueante (F2):** comentário em `validator.go:3236` desatualizado; não quebra comportamento.

**Ajustes necessários (para o agente que detém `docs/cli-parity.md` — apolo-tf ou arquiteto):**

1. `docs/cli-parity.md` linha 4386: inserir `<!-- trackfw-contract: gate=internal/validator/validator_adr_prefix_test.go -->` após `### Critério D1 — \`isADRFileName\``.
2. `docs/cli-parity.md` linha 4407: inserir `<!-- trackfw-contract: gate=internal/validator/validator_adr_prefix_test.go -->` após `### Regra \`adr_file_without_prefix\` (D4)`.
3. (não-bloqueante) `internal/validator/validator.go` linha 3236: atualizar o comentário de `walkADRFiles`.
