---
status: Open
date: 2026-09-07
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-08-validate-suprime-usage-em-erro-de-runtime-e-o-gate-que-provaria-isso.md"
---

# REQ: validate imprime usage em erro de runtime e suja o stream json so no Go, e o gate que provaria isso e um gap declarado

> Date: 2026-09-07 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Origem

Issue [#290](https://github.com/kgsaran/trackfw/issues/290), de `lourivalgarciajunior`, medido no
Windows em `e337563`. **Reproduzido pelo arquiteto no macOS** — não é específico de plataforma, e o
limite que o autor declarou fica resolvido.

## Motivation

`trackfw validate` com violação diverge nos 3 CLIs, e a divergência não é simétrica:

| runtime | modo | exit | stderr |
|---|---|---|---|
| **Go** | humano | 1 | **152 bytes** — `Error:` + `Usage:` + `Flags:` |
| **Go** | `--json` | 1 | **21 bytes** — `2 violation(s) found`, linha **não-JSON** |
| Node | humano / `--json` | 1 | **0** / **0** |
| Python | humano / `--json` | 1 | **0** / **0** |

O exit code está correto nos três. **A divergência é só de stderr — e o contrato já a proíbe por
escrito.**

### A causa é uma linha de escopo

`internal/commands/validate.go:19-23` seta `SilenceErrors`/`SilenceUsage` **só dentro do ramo JSON**.
O caminho humano — o que todo desenvolvedor e todo `make quality` percorre — nunca os seta, e o
wrapper de `internal/commands/root.go:96-112` faz exatamente o que está escrito para fazer. **O
wrapper está certo; falta o `validate` opinar fora do ramo JSON.**

### Qual lado é o certo

O comentário do próprio `root.go` descreve o desenho como *"matching Node/Python"*. Pelo critério do
arquivo, **o Go é o outlier** — não os outros dois.

### 🔴 O que mais pesa: o defeito mora dentro de um gap que nós declaramos

`docs/cli-parity.md`, seção **Usage silencing**:

```
<!-- trackfw-contract: gap reason=nenhum gate verifica que a saída de uso (usage/help)
     é suprimida em erros de runtime (padrão de branch, gate de governança, nada
     staged, -m ausente) nos 3 runtimes -->
```

Contrato escrito, gate ausente, comportamento divergindo em silêncio — com o documento já apontando
onde olhar. **Regra sem gate não é regra.** Corrigir só o `validate` faria o defeito renascer no
próximo comando: foi assim que este nasceu.

## Escopo — os três na MESMA REQ, de propósito

Mesma causa ⇒ mesma REQ ⇒ mesmo PR (regra dura do `CLAUDE.md`). Fragmentar é como o defeito
sobrevive com aparência de "registrado".

1. **`--json` não escreve linha não-JSON na stderr** — é o único dos 3 runtimes que suja o stream de
   consumo por máquina. Contrato, não estética. **Maior peso.**
2. **Usage suprimido em erro de runtime**, nos 3 CLIs.
3. **O gate ausente**, que fecha o `gap reason=` e impede a recorrência.

## 🔴 Medição de escopo: o que NÃO é contagem de defeito

O autor do issue estimou "20 de 33 arquivos de comando sem `SilenceUsage`" e foi explícito em não
tratar isso como defeito. Medido pelo arquiteto: são **24 de 37**, dos quais **12** retornam erro de
runtime.

**Esse 12 continua não sendo contagem de defeito.** Testados 4 por comportamento, 2 imprimem usage
**corretamente**:

```
audit-surface   Error: accepts 1 arg(s), received 0      ← erro de ARGUMENTO
sync            Error: required flag(s) "to" not set     ← erro de ARGUMENTO
```

Usage em erro de **argumento** é o comportamento certo; o contrato fala de erro de **runtime**. O
escopo real é menor que 12 e **só sai por medição comportamental, um comando por vez** — nunca por
`grep` da string. Nem a presença da string prova ausência de defeito, nem a ausência prova presença.

## Acceptance Criteria

- [ ] **AC1** — `validate --json` com violação: stderr de **0 bytes** nos 3 runtimes, ou saída
      exclusivamente JSON válido. Verificado por `diff` de três vias de stdout **e** stderr.
- [ ] **AC2** — `validate` humano com violação: **nenhum bloco de usage** nos 3 runtimes; exit 1
      preservado. `diff` de três vias.
- [ ] **AC3** — gate novo cobrindo o `gap reason=`, e o comentário de gap **removido** do
      `cli-parity.md` (deixá-lo é declarar ausente um gate que passou a existir).
- [ ] **AC4** — o gate é falsificado **nas duas direções**: com a supressão desligada, ele reprova;
      com ela ligada, aprova. Guarda de vacuidade contra árvore vazia.
- [ ] **AC5** — o conjunto de comandos com erro de **runtime** que hoje vazam usage é medido **por
      comportamento** e listado. 🔴 "Só o `validate`" é resultado válido se for o que a medição disser.
- [ ] **AC6** — nenhum gate existente do CI dependia da presença dessas linhas na stderr (o autor
      declarou não ter medido isso).

## Negative Scope — o que esta REQ NÃO faz

- **Não** altera exit codes: estão corretos nos 3 runtimes.
- **Não** mexe no wrapper de `root.go` — ele está certo; quem não opina é o comando.
- **Não** altera usage em erro de **argumento**, que é comportamento correto e deve permanecer.
- **Não** varre os 33 comandos por `grep` para "corrigir preventivamente" — só entra o que a medição
  comportamental do AC5 apontar.

## Revisão de escopo — 2026-10-30 (zeus-tf) — este é o escopo vigente

Retomada a pedido do KG. Desde a v8 existe só o binário Go; "3 runtimes" e "diff de três vias" não
se aplicam mais. A triagem de 2026-10-17 (`docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`,
linha 23) já registrava: usage e gate corrigidos no #413; vivo só o AC1.

### Medido em `main` `7b352543` (binário compilado da árvore), fixture strict com 1 violação

| comando | rc | stdout | stderr |
|---|---|---|---|
| `validate` | 1 | 370 B | `Error: 1 violation(s) found` (sem usage) |
| `validate --json` | 1 | 597 B (JSON) | 🔴 `1 violation(s) found` — linha fora do JSON |
| `barrier <roadmap> --wave 1 --json` (blocked) | 1 | 1140 B (JSON) | 0 B |
| `doctor --json`, `update --dry-run --json`, `roadmap show --json` | 0 | JSON | 0 B |

**Mecanismo:** o ramo JSON do `validate` liga `cmd.SilenceErrors` esperando silêncio e devolve um
`error` depois de escrever o documento. No `root.go`, `SilenceErrors` não silencia — só troca
`Error: msg` pela mensagem sem prefixo (comportamento do `branch new`). O `barrier` não tem o
defeito porque sai com `os.Exit(1)` direto.

**Mesma causa, lida no código** (`return fmt.Errorf` depois de escrever o documento `--json`):
- `internal/commands/update.go:85` — `trackfw update: %d target(s) failed`
- `internal/commands/update_harness.go:106` — `trackfw update harness: %d target(s) failed`
Entram nesta REQ (Regra Dura de Causa Raiz).

**Outra categoria, fora do escopo:** erro **antes** de existir documento (`audit-surface` com ref
inválida, `barrier`/`roadmap show` com roadmap inexistente) escreve texto no stderr e nada no
stdout. Não há documento a proteger, e o `cli-parity.md` não tem contrato geral de `--json` que
exija JSON nesse caso. Se for desejado, é decisão de contrato nova, não este defeito.

**Consumidores:** nenhum gate em `scripts/` ou `.github/` lê o stderr do `validate --json` (os que
redirecionam para arquivo não o consultam); nenhum `_test.go` fixa `violation(s) found` nem
`target(s) failed`.

### Decisão de mecanismo (zeus-tf)

Um erro tipado que carrega só o código de saída, reconhecido pelo `root.go`, que sai com esse
código **sem imprimir nada**. Não `os.Exit` no `RunE`: `TestValidateJSONFlag` e afins executam o
comando em processo via `cmd.Execute()`, e um `os.Exit` derrubaria o teste. Isso **revoga** o item
"não mexe no wrapper de `root.go`" do escopo negativo original, limitado ao reconhecimento desse tipo.

### ACs vigentes

- **AC1 — vivo.** `validate --json` com violação: rc=1, stdout só o documento JSON, stderr 0 B.
- **AC1-bis — novo, mesma causa.** `update --json` e `update harness --json` com alvo falho: rc≠0
  como hoje, stdout só o documento, stderr 0 B.
- **AC2 — atendido (#413).** Modo texto sem bloco de usage; medido acima. Continua igual:
  `Error: 1 violation(s) found`, rc=1.
- **AC3/AC4 — atendidos (#413)** para o usage. O gate desta revisão é teste Go do binário em
  processo, falsificado nas duas direções.
- **AC5 — remedido** pela tabela acima: o defeito de stream sujo no caminho de resultado está em
  `validate`, `update` e `update harness`.
- **AC6 — atendido** (consumidores acima).
- **AC7 — contra-braço.** Modo texto do `validate`, do `update` e do `update harness`, e o modo
  lenient (rc=0, stderr vazio), não mudam. Comandos sob `guard` continuam saindo 2 (D7,
  ADR-2026-10-04).
- **AC8 — contrato escrito.** `docs/cli-parity.md` ganha a regra: com `--json`, depois de escrito
  o documento, o stdout carrega só ele, o stderr fica vazio e o resultado vai no código de saída.

### Escopo negativo vigente

- Não altera códigos de saída.
- Não muda erro antes do documento (categoria acima), nem usage de erro de argumento.
- Não muda o modo texto de nenhum comando.
- `root.go` só ganha o reconhecimento do erro tipado; o resto do wrapper fica igual.

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-08-validate-suprime-usage-em-erro-de-runtime-e-o-gate-que-provaria-isso.md`
