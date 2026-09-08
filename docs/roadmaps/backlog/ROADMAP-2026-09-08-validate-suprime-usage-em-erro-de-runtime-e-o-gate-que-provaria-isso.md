---
status: backlog
date: 2026-09-08
squad: apolo-tf
req: "docs/req/REQ-2026-09-07-validate-imprime-usage-em-erro-de-runtime-e-suja-o-stream-json-so-no-go-e-o-gate-que-provaria-isso-e-um-gap-declarado.md"
---

# Roadmap: `validate` suprime usage em erro de runtime, e o gate que provaria isso

> Criado em: 2026-09-08 | Status: backlog

## Context

REQ: `docs/req/REQ-2026-09-07-validate-imprime-usage-em-erro-de-runtime-e-suja-o-stream-json-so-no-go-e-o-gate-que-provaria-isso-e-um-gap-declarado.md` · Issue [#290](https://github.com/kgsaran/trackfw/issues/290)

## Diagnóstico — reproduzido pelo arquiteto no macOS (o autor mediu no Windows)

```
Go     humano   exit=1   152 bytes   Error: + Usage: + Flags:
Go     --json   exit=1    21 bytes   linha NAO-JSON na stderr
Node   humano   exit=1     0 bytes
Node   --json   exit=1     0 bytes
Python humano   exit=1     0 bytes
Python --json   exit=1     0 bytes
```

Não é específico de plataforma. `internal/commands/validate.go:19-23` seta
`SilenceErrors`/`SilenceUsage` **só dentro do ramo JSON**; o caminho humano nunca os seta.

🔴 O defeito mora **dentro de um gap que o próprio `docs/cli-parity.md` declara** por comentário
`trackfw-contract: gap reason=...`. Regra escrita, gate ausente, comportamento divergindo em silêncio.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Contrato e gate

### ML-1A — `--json` para de sujar o stream de máquina
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **maior peso**

É o único dos 3 runtimes que escreve linha **não-JSON** na stderr. Contrato de consumo, não estética.

### ML-1B — Usage suprimido em erro de runtime, nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`

O comentário do `root.go` descreve o desenho como *"matching Node/Python"* — pelo critério do próprio
arquivo, **o Go é o outlier**.

### ML-1C — O gate que fecha o `gap reason=`
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **é o que impede a recorrência**

Falsificação nas duas direções + guarda de vacuidade. E **remover o comentário de gap** do
`cli-parity.md` — deixá-lo é declarar ausente um gate que passou a existir.

## Escopo negativo

- **Não** altera exit codes: corretos nos 3.
- **Não** mexe no wrapper de `root.go` — ele está certo; quem não opina é o comando.
- 🔴 **Não** altera usage em erro de **argumento** (`accepts 1 arg(s)`, `required flag not set`) —
  é comportamento **correto** e o contrato só fala de erro de **runtime**.
- **Não** varre os 33 comandos por `grep`: o escopo real sai por **medição comportamental** (AC5 da
  REQ). "Só o `validate`" é resultado válido.

## Critérios de Aceite

- [ ] `validate --json` com violação: stderr de 0 bytes ou só JSON válido, nos 3 runtimes
- [ ] `validate` humano: nenhum bloco de usage, exit 1 preservado, `diff` de três vias
- [ ] gate novo cobrindo o gap, falsificado nas duas direções; comentário de gap removido
- [ ] conjunto de comandos que vazam usage em erro de **runtime** medido por comportamento
