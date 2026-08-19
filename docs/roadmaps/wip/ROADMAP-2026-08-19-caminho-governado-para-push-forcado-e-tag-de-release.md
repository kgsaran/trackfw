---
status: wip
date: 2026-08-19
req: "docs/req/REQ-2026-08-19-ship-nao-cobre-push-forcado-nem-tag-e-o-guard-bloqueia-o-caminho-bruto.md"
adr: "docs/adr/ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: caminho governado para push forçado e tag de release

> Created: 2026-08-19 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-19-ship-nao-cobre-push-forcado-nem-tag-e-o-guard-bloqueia-o-caminho-bruto.md`
ADR: `docs/adr/ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md`

Medido duas vezes na entrega da `7.1.0`. O guard bloqueia **toda** forma de `git push`; o `ship`
cobre **uma**. O protocolo de release do projeto é inexecutável dentro dos guardrails do projeto.

Forma decidida por KG (ADR): **`ship --force-with-lease`** + **`release tag` separado**, com o
force-push **restrito a branch que já tem PR aberto**.

## 🔴 Riscos que valem para todos os MLs

1. **Nunca `--force` cru.** `--force-with-lease` recusa quando o remoto avançou; `--force` destrói
   trabalho alheio. A diferença não é de estilo.
2. **`release tag` publica.** Defeito nele produz tag errada em repositório público, caro de desfazer.
3. **Fixture com remoto de verdade** (bare local), nunca mock — precedente em
   `check-branch-prune-parity.sh` e `check-doctor-parity.sh`. Mock provaria só que o mock concorda
   com o código.
4. **`make quality` local não fecha AC** — o AC10 exige CI.
5. **Teste por stack não fecha paridade.** Esta série já provou **três vezes** que gate comparando
   saídas reais pega o que teste por runtime não pega.
6. **Não afrouxar o guard.** Ele ser incondicional é o que o torna honesto.

---

## Wave 1 — Push forçado (2 MLs, sequenciais: compartilham `ship`)

### ML-1A — `ship --force-with-lease`, restrito a branch com PR aberto
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos (os 3 stacks, sempre):** `internal/commands/ship.go`, `npm/src/commands/ship.js`,
`pypi/trackfw/commands/ship.py` + testes dos 3.

**Ações:**
- Flag `--force-with-lease`. **Nunca** expor `--force`.
- Antes de forçar, verificar que a branch tem **PR aberto** via CLI de forge já resolvido pelo `ship`.
- **Sem CLI de forge disponível: recusar com orientação**, nunca degradar para push permissivo.
- Sem PR aberto: recusar, dizendo que o caminho é abrir o PR primeiro.

**Critérios de aceite:**
- [ ] `--force-with-lease` funciona em branch rebaseada **com** PR aberto
- [ ] Recusa **sem** PR aberto, com mensagem que nomeia o caminho correto
- [ ] Recusa quando não há CLI de forge, sem degradar
- [ ] `--force` cru **não existe** como flag em nenhum dos 3
- [ ] Não-regressão: push normal do `ship` inalterado
- [ ] `make quality` verde

### ML-1B — Gate de paridade do push forçado + P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-1A
**Arquivos:** `scripts/check-ship-force-parity.sh` (novo), `Makefile` (alvo `parity`),
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md` (seção **nomeando o gate**).

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais** (sucesso, sem-PR, sem-forge), stdout e stderr
- [ ] Fixture com **remoto bare de verdade** e rebase real
- [ ] Cenário P4: sabota o `--force-with-lease` para `--force` e prova que o gate fica vermelho
- [ ] Seção no `cli-parity.md` **nomeando o gate**
- [ ] `make quality` verde

---

## Wave 2 — `release tag` (2 MLs, sequenciais)
> Dependências: independente da Wave 1 em arquivos, **mas** sequencial por prudência: a Wave 2
> publica, e prefiro a Wave 1 auditada antes.

### ML-2A — `trackfw release tag <versão>`
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos (os 3 stacks):** `internal/commands/release.go` (novo) + registro no `root.go`,
`npm/src/commands/release.js` + `index.js`, `pypi/trackfw/commands/release.py` + `cli.py`,
mais testes dos 3.

**Ações:**
- Cria tag **anotada**, com a seção correspondente do `CHANGELOG.md` no corpo.
- Publica pelas **duas** chamadas de API já validadas em produção (ver ADR): cria o objeto de tag,
  depois a ref. Preserva a anotação.
- **Pré-condições, todas recusando com orientação:** árvore limpa; `main` atualizada com o remoto;
  os 4 arquivos de versão batendo com a versão pedida; `CHANGELOG.md` tendo a seção da versão; tag
  ainda não existente local nem remotamente.

**Critérios de aceite:**
- [ ] Tag remota é **anotada**, com a mensagem íntegra — verificado no objeto, não só na ref
- [ ] Cada pré-condição recusa com mensagem que nomeia o que corrigir
- [ ] Recusa se a tag já existe, local **ou** remotamente
- [ ] Versão divergente entre os 4 arquivos → recusa apontando qual diverge
- [ ] `make quality` verde

### ML-2B — Gate de paridade do `release tag` + P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dependência:** ML-2A
**Arquivos:** `scripts/check-release-tag-parity.sh` (novo), `Makefile`,
`scripts/check-gates-falsify.sh`, `docs/cli-parity.md`.

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais** em todos os caminhos de recusa
- [ ] Cenário P4 sabotando a criação do objeto de tag (anotada → leve) e provando gate vermelho
- [ ] Seção no `cli-parity.md` **nomeando o gate**
- [ ] `make quality` verde

---

## Wave 3 — Mensagem do guard

### ML-3A — Guard diz que o comando **inteiro** foi bloqueado
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Arquivos:** `internal/generators/scaffold.go` (literal do script — **fonte canônica**, nunca
editar as 7 cópias à mão) + espelhos Node/Python, `scripts/check-attention-scripts-parity.sh` se
aplicável.

**Ação:** a mensagem de recusa passa a dizer que **nada antes do `git push` executou**, e a do
`push` aponta `trackfw ship` **e** `trackfw release tag`. Custou um ciclo real: um comando composto
com heredoc não gravou o arquivo, não criou a tag, e devolveu só a mensagem do push.

**Critérios de aceite:**
- [ ] Mensagem menciona que o comando inteiro foi bloqueado
- [ ] Mensagem do `push` cita os dois caminhos governados
- [ ] Script **byte-idêntico** entre os 3 CLIs e entre escopos — gate existente cobre
- [ ] Cenário P4 para a mensagem nova
- [ ] `make quality` verde

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`: revisão do escape hatch
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-19-revisao-do-push-forcado-e-do-release-tag.md`

**Ações:** avaliar se a amarração ao PR aberto é contornável (PR fechado? PR de outro repo? branch
renomeada?); se o `release tag` pode ser induzido a publicar tag apontando para commit que não é o
da `main`; e se a dependência do forge abre caminho de degradação silenciosa. **Veredito explícito;
bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** afrouxar o `case push)` do guard; merge de PR; `trackfw release`
  cobrindo bump e CHANGELOG (adiado no ADR, não rejeitado).
- Commits e branch são exclusivos do `trackfw_architect`.
