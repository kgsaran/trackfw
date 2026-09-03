---
status: wip
date: 2026-09-02
squad: ares-tf
req: "docs/req/REQ-2026-09-02-prs-usam-palavra-chave-de-fechamento-em-portugues-e-nenhuma-issue-fecha-automaticamente.md"
---

# Roadmap: Gate e template de PR exigem palavra-chave de fechamento em inglês

> Criado em: 2026-09-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-02-prs-usam-palavra-chave-de-fechamento-em-portugues-e-nenhuma-issue-fecha-automaticamente.md

## Diagnóstico

O PR #247 abre com **"Fecha #246."** na primeira linha. Foi mergeado, e a issue #246 **continuou
aberta** — fechada à mão horas depois, quando alguém reparou.

O GitHub só reconhece `close(s|d)`, `fix(es|ed)`, `resolve(s|d)` — **em inglês**. Como escrevemos
todos os PRs em português, **nenhum PR deste repositório jamais fechou uma issue automaticamente**.

**Não existe `.github/PULL_REQUEST_TEMPLATE.md`** — verificado. Cada corpo é escrito do zero, o que
explica por que a forma correta nunca se firmou por hábito.

**O modo de falha é silencioso e invertido:** você escreve a intenção, o merge tem sucesso, nada
acontece, e o texto do PR **afirma** que fechou. É o mesmo padrão da auditoria desta sprint —
artefato que se reporta saudável estando inerte.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Acceptance Criteria

- [ ] Template de PR existe, com linha de fechamento em inglês **e o motivo escrito**
- [ ] Gate reprova corpo com palavra-chave em português e aprova com a inglesa
- [ ] 🔴 Controle: menção a issue em **prosa** não reprova
- [ ] 🔴 Guarda de vacuidade: corpo ilegível → falha ou `not_evaluated`, nunca passa calado
- [ ] Ligação no workflow verificada por execução
- [ ] `make quality` verde

## Wave 1 — O template e o gate
> Dependências: nenhuma.

### ML-1A — `.github/PULL_REQUEST_TEMPLATE.md` e gate de palavra-chave
**Status:** ⬜ Pendente
**Agente:** `ares-tf`
**Files affected:** `.github/PULL_REQUEST_TEMPLATE.md` (novo),
`scripts/check-pr-closing-keyword.sh` (novo), `.github/workflows/quality.yml`, `Makefile`,
`docs/cli-parity.md`

**Ações:**
1. Template com uma linha `Closes #` pronta e uma nota curta explicando **por que inglês**.
   🔴 A explicação importa mais que a linha: sem ela, o próximo tradutor "conserta" para português
   e o defeito volta idêntico.
2. Gate que lê o corpo do PR e reprova quando encontra palavra-chave em português referenciando
   issue (`Fecha #`, `Fechado #`, `Corrige #`, `Corrigido #`, `Resolve #`, `Encerra #`) **e não há**
   forma inglesa válida para a mesma issue.

🔴 **O controle de falso positivo é mais difícil que a detecção, e decide se o gate sobrevive.**
Este repositório referencia issue e PR em prosa o tempo todo — *"o mesmo sítio do #238"*, *"portado
do #223"*, *"fecha o item 7"*. Um gate ruidoso é desligado, e aí não guarda nada. **Meça o falso
positivo contra corpos de PR reais deste repositório** (`gh pr list --state merged --json body`),
não contra exemplos inventados. **Zero falso positivo nos PRs já mergeados é critério, não meta.**

🔴 **Regex literal é evadível** — leia
`vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md` antes de escrever a
asserção, e declare quais formas você aceita e recusa.

🔴 **O gate novo entra em `check-gates-falsify.sh`.** A auditoria de hoje mediu **4 gates vácuos**, e
os 4 estão fora desse harness — é a causa raiz de eles sobreviverem ao `make quality` verde. Um gate
novo que nasça fora dele repete a classe.

**Critérios de aceite:**
- [ ] `Fecha #123` no corpo → **reprova nomeando a linha**; `Closes #123` → **passa**
- [ ] 🔴 **Falso positivo medido contra os corpos reais dos PRs já mergeados deste repo:** zero
      reprovações indevidas, com o número de corpos testados no relatório
- [ ] 🔴 **Vacuidade:** corpo vazio / evento sem payload / execução fora de `pull_request` →
      **falha ou `not_evaluated`**, verificado por execução
- [ ] O gate tem cenário em `scripts/check-gates-falsify.sh`
- [ ] Ligado no workflow e **verificado por execução**, não presumido
- [ ] `make quality` verde e `trackfw validate` exit 0

**Comandos de validação:** `bash scripts/check-pr-closing-keyword.sh`, `make quality`

## Verificação

O próximo PR que referencie uma issue fechando-a de verdade no merge. Não há gate que feche isso
sozinho — o gate previne a forma errada, não prova a certa.

## Barreira final

Arquiteto. **Sem `hades-tf`** — não há superfície de ataque. `hefesto-tf` só se o gate crescer além
de um script.
