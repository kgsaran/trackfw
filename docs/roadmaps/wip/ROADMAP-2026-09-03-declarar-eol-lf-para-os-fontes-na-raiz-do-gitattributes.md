---
status: wip
date: 2026-09-03
squad: hefesto-tf
req: "docs/req/REQ-2026-09-03-a-raiz-do-gitattributes-nao-declara-eol-para-os-fontes-e-217-arquivos-go-chegam-em-crlf-no-windows.md"
---

# Roadmap: Declarar `eol=lf` para os fontes na raiz do `.gitattributes`

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-03-a-raiz-do-gitattributes-nao-declara-eol-para-os-fontes-e-217-arquivos-go-chegam-em-crlf-no-windows.md

## Diagnóstico

Medido em clone local com `core.autocrlf=true` — o default do runner Windows:

```
arquivos .go com CRLF no checkout:  217
total de arquivos .go:              217
```

O PR #254 do reporter (mergeado) removeu o impedimento — os 3 testes passaram de **igualdade** para
**contenção** — mas **não acrescentou a linha**. Verificado na `main`: a raiz tem só
`.trackfw-log merge=union`.

**Maior redução de contagem de Windows ainda disponível, e custa uma linha.**

## Acceptance Criteria

- [ ] `eol=lf` declarado para os fontes dos 3 stacks, enumerados **por medição**
- [ ] Falsificação: com a regra 0 arquivos em CRLF; sem ela, 217
- [ ] 🔴 Os 3 testes de contenção do #254 continuam passando
- [ ] 🔴 A metade de **produto** do CRLF permanece **visível**
- [ ] `merge=union` intacto; sem ruído de renormalização
- [ ] `make quality` verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — A regra
> Dependências: nenhuma. PR #254 já mergeado.

### ML-1A — `eol=lf` na raiz, enumerado por medição
**Status:** ⬜ Pendente
**Agente:** `hefesto-tf`
**Files affected:** `.gitattributes` (raiz)

**Ações:**
1. Clonar com `-c core.autocrlf=true` em `$(mktemp -d)` e **listar** o que chega em CRLF. A lista é
   a fonte do escopo — não presumir por extensão.
2. Declarar `eol=lf` para os fontes e para artefatos que testes leem **como texto**.

🔴 **A linha que separa este ML de esconder um defeito:** o parser de frontmatter é **cego a CRLF** e
emitiu frontmatter duplicado em `TestRenderOpenCodeAgent`. Declarar `eol` sobre os **assets que o
produto processa** esconderia isso. O ML-1C mediu e **removeu** o pin que já tinha feito, porque
fixar só o lado esperado **cura zero** e apaga a evidência. **Enumere o que fica de fora, e por quê.**

🔴 **Os 3 testes de contenção do #254 são o controle mais importante.** Eles comparam a raiz com uma
constante de **produto** (`GITATTRIBUTES_BLOCK`, nos 3 geradores). A regra nova entra **fora** do
bloco, e a contenção tem de continuar satisfeita — nos **3** runtimes, porque o Python passa por
universal newlines e medir num só engana.

**Critérios de aceite:**
- [ ] Clone `autocrlf=true`: **0** arquivos de fonte em CRLF (antes: 217 `.go`)
- [ ] 🔴 Falsificação: removendo a regra, volta a 217
- [ ] 🔴 `go test ./internal/generators/ -run GitAttributes`, `npm test`, `pytest` — os 3 testes de
      contenção **passam**
- [ ] 🔴 Enumeração escrita do que ficou **de fora** por ser entrada de produto
- [ ] `git check-attr merge -- .trackfw-log` → `union`
- [ ] `git ls-files --eol` sem ruído de renormalização
- [ ] `make quality` verde e `trackfw validate` exit 0

**Comandos de validação:** `git clone --local -c core.autocrlf=true`, `make quality`

## Verificação que só o CI fecha

A contagem de Windows na `main`. **É a mesma medição da Wave 2 da REQ de desmascaramento** — as duas
fecham no mesmo run, e por isso este ML entra **antes** de disparar a medição.

## Barreira final

Arquiteto. **Sem `hades-tf`** — configuração de repositório, sem superfície de ataque.
