---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md"
squad: ""
---

# Roadmap: `trackfw context` reporta `ADRs (0)` onde `status` reporta 145

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-trackfw-context-reporta-adrs-zero-onde-status-reporta-145-e-o-agente-que-roda-context-primeiro-conclui-que-nao-ha-decisoes-arquiteturais.md
ADR: docs/adr/ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md
Origem: **#450**. Causa: `context.go:39` faz `os.ReadDir(adrDir)` sem descer nas subpastas de estado.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [ ] Todos os sítios que enumeram ADR levantados e classificados
- [ ] `context` e `status` consomem o **mesmo** resolvedor (ADR D3)
- [ ] Layout plano continua funcionando **e** subpastas de estado passam a ser enumeradas
- [ ] A saída do `context` deixa de poder dizer `ADRs (0)` e nomear um ADR num warning
- [ ] `Governance score` medido antes/depois: diferença de **exatamente 20 pontos**
- [ ] Gate falsificável impedindo enumerador novo fora do ponto único
- [ ] `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: quantos enumeradores de ADR existem, e quais estão errados
> Dependências: nenhuma. 🔴 **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(grep -rn 'os.ReadDir\|filepath.WalkDir\|filepath.Glob' --include='*.go' internal/ | grep -v _test | wc -l | tr -d ' '); test "$n" -gt 0 && echo "Gate W0: $n sitios de enumeracao de diretorio no produto — a triagem parte deste universo" || { echo "GATE FALHOU: zero sitios de enumeracao encontrados — a regua esta quebrada, nao o produto" >&2; exit 1; }
```

### ML-0A — enumerar os sítios que leem ADR e medir a divergência
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos:** leitura de `internal/`; escrita em `docs/seguranca/2026-09-29-wave0-context-adr-zero.md`

**Tarefa:**
1. Levantar **todos** os sítios que enumeram ADR — não só `context` e `status`. Inclua `serve`,
   `validate`, `discover`, `metrics` e o que mais existir.
2. Classificar cada um: **(i)** usa o ponto único · **(ii)** implementação própria **correta** ·
   **(iii)** implementação própria **errada**.
3. 🔴 **Medir a divergência entre comandos numa mesma fixture** — é o sintoma que denuncia
   implementação duplicada. Rode todos os que reportam contagem de ADR e cole os números lado a lado.
4. 🔴 **Refutar ou confirmar a premissa da ADR** de que existe (ou é viável construir) **um** ponto
   único que sirva aos dois. Se `status` e `context` precisarem de dados diferentes (ex.: um precisa
   do status do frontmatter e o outro não), **diga agora** — isso muda o desenho do ML-1A.

**Critérios de aceite:**
- [ ] Todos os sítios enumerados com arquivo, linha e classe, **nenhum sem razão escrita**
- [ ] Tabela de divergência medida numa fixture única, com os números lado a lado
- [ ] Veredito explícito sobre a viabilidade do ponto único **antes** de alguém escrevê-lo
- [ ] Verificação de que o layout **plano** deste repositório não regride sob a mudança proposta

## Wave 1 — o ponto único, e o gate que o sustenta
> Dependências: **Wave 0 auditada.** Os dois MLs tocam arquivos distintos → paralelos, **se** a
> Wave 0 confirmar que o resolvedor vive em arquivo próprio.

### ML-1A — resolvedor único de ADR, consumido por `context` e `status`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente

**Critérios de aceite:**
- [ ] Layout plano (`adr_dir/*.md`) **continua** funcionando — é o layout deste repositório
- [ ] Layout com subpastas de estado passa a ser enumerado
- [ ] `context` e `status` chamam o **mesmo** resolvedor
- [ ] 🔴 **O teste que mede o efeito:** numa fixture com ADRs em subpastas, a saída do `context`
      **não** contém `## ADRs (0)` junto de um warning que nomeia um ADR
- [ ] 🔴 **Score medido antes/depois:** diferença de **exatamente 20 pontos**. Nem mais — se subir
      40, a mudança tocou outra categoria e isso precisa ser explicado
- [ ] Reconciliação: uma frase por teste, dizendo o que **mediu**

### ML-1B — gate que impede enumerador de ADR fora do ponto único
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente

**Critérios de aceite:**
- [ ] 🔴 Falsificável nas **duas** direções, com as execuções coladas
- [ ] Anti-vacuidade: declara quantos sítios examinou e **reprova se examinar zero**
- [ ] 🔴 **O discriminante ignora comentários** — este projeto já pagou por gate que aceitava token
      em comentário **duas vezes** (`check-crlf-normalize-capture.sh` e, em 2026-09-28,
      `check-init-preserves-user-config.sh`, nas duas direções). Falsifique esse caso explicitamente.

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o binário como
caixa-preta. A Regra Dura de Reconciliação pega contradição interna; **não** pega premissa errada
compartilhada entre implementação e teste.

**Critérios de aceite:**
- [ ] Cenário do #450 reconstruído do zero (`adr_dirs` com subpastas), `context` e `status` concordando
- [ ] Ataque a layouts adversariais: `adr_dir` inexistente · vazio · com subpasta sem `.md` ·
      `.md` que não começa com `ADR-` · symlink · subpasta aninhada em dois níveis
- [ ] Veredito explícito: sobrou enumerador de ADR fora do ponto único? A ADR **não** está satisfeita
      enquanto sobrar sítio
