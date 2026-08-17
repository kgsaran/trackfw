---
status: wip
date: 2026-08-17
req: "docs/req/REQ-2026-08-17-guard-global-e-instalado-sem-fiacao-e-sua-integridade-nunca-e-verificada.md"
adr: "docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: guard global cabeado com no-op fora de projeto, e integridade independente de fiação

> Created: 2026-08-17 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-17-guard-global-e-instalado-sem-fiacao-e-sua-integridade-nunca-e-verificada.md`
ADR: `docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md` (decisão de KG, 2026-08-17)

Medido: o `git-branch-guard` global é **escrito** (`update.go:493`) e **não é cabeado** em nenhum dos
6 configs globais — o `credential-guard` tem 2 refs em cada um dos 4 existentes, ele tem zero. A
regra de integridade global só avalia configs que **referenciam** o script, então nunca roda para
ele: ficou **3 versões atrasado** (123 linhas contra 369) com `validate` verde.

Medido também, e é o que define a ordem das waves: o script **não tem no-op fora de projeto
trackfw** — num repo sem `trackfw.yaml`, `git push` → `exit 2`. **Cabear antes do no-op quebraria
todos os repositórios da máquina.**

Esta REQ cobre **escopo global**. O escopo de projeto (`validate` não detectar hook na forma relativa
antiga) é a `REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md`,
com roadmap próprio **depois** deste — as duas tocam os mesmos arquivos de validador.

## Acceptance Criteria

- [ ] AC1 — Script é **no-op** (exit 0) fora de projeto trackfw, e mantém o comportamento atual dentro.
- [ ] AC2 — `git-branch-guard` cabeado no escopo global nos mesmos CLIs do `credential-guard`.
- [ ] AC3 — Integridade de script global escrito pelo trackfw é verificada **independentemente** de
      haver config referenciando-o.
- [ ] AC4 — Script defasado/adulterado em `~/.trackfw/scripts/` é **acusado**; hoje passa limpo.
- [ ] AC5 — Não-regressão: verificação do `credential-guard` global, que hoje funciona, segue
      funcionando e não acusa em dobro.
- [ ] AC6 — Paridade nos 3 CLIs, com gate; conteúdo do script byte-idêntico entre escopos.
- [ ] AC7 — Cenários de falsificação (P4) com baseline **e** detecção para cada ML.
- [ ] AC8 — `make quality` verde; `trackfw validate` sem novas violações.

## 🔴 Riscos que valem para TODOS os MLs deste roadmap

1. **O template do guard é a fonte do `corrupt_literal` dos Cenários 60/61/62/63.** Mexer no
   template faz o literal deixar de casar e os cenários viram **inertes** — foi exatamente o que
   derrubou o Cenário 58 no rebase de 2026-08-16, e o gate acusou. Depois de tocar o template, rode
   `make quality` e **cole as linhas `OK [falsify/...]` dos braços de detecção**, provando que ainda
   reprovam. Exit code verde não basta.
2. **São 7 cópias do script.** Tudo sai **do gerador**, byte-idêntico. Nunca editar cópia a cópia.
3. **`check-gates-falsify.sh` é arquivo compartilhado por todos os MLs** — por isso as waves são
   **sequenciais**, não paralelas. Nenhum ML aqui roda em paralelo com outro.
4. **Falso-positivo em escopo global afeta todos os repositórios da máquina de uma vez.** É o risco
   dominante do roadmap inteiro.
5. **Não usar o binário do `PATH`** — pode estar velho, e `--version` não distingue o build. Compilar
   e usar `./bin/trackfw`.

---

## Wave 1 — No-op (bloqueia tudo o mais)

### ML-1A — Script vira no-op fora de projeto trackfw
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** template do guard no gerador Go + espelhos Node/Python + referência em `scripts/`
(7 cópias, todas pelo gerador), testes dos 3, `scripts/check-gates-falsify.sh`.

**Ação:** o script passa a sair com **0**, sem bloquear nada, quando não houver `trackfw.yaml` na
raiz do repositório corrente. Dentro de projeto trackfw, comportamento **inalterado**.

**Decisões que são suas, registre no relatório:** como localizar a raiz (subir diretórios até achar
`trackfw.yaml`? usar `git rev-parse --show-toplevel`?) e o custo disso, que roda em **toda** chamada
de ferramenta. Meça, não presuma — se subir diretórios custar caro, diga.

**Critérios de aceite:**
- [ ] Repo **sem** `trackfw.yaml`: `git push`, `git commit`, `git branch nova` → **exit 0**.
- [ ] Repo **com** `trackfw.yaml`: bateria completa inalterada — `git push`, `git commit`,
      `switch -c/-C/--create`, `checkout -b/-q -b/--no-track -b/--orphan`, `git branch nova`,
      `worktree add -b`, `env FOO=bar git push`, `env git`, `command git` → **exit 2**;
      leitura (`git branch`, `-a`, `-r`, `--list`, `-v`, `--show-current`, `-d`, `-D`) → **exit 0**;
      prosa (`trackfw commit -m "veja: git status; git push é bloqueado"`) → **exit 0**.
- [ ] Subdiretório de projeto trackfw **continua** protegido (a raiz é encontrada subindo).
- [ ] 7 cópias byte-idênticas; `trackfw validate` sem divergência de integridade.
- [ ] Cenário de falsificação novo (baseline + detecção) para o no-op.
- [ ] **Cenários 60/61/62/63 continuam reprovando** nos braços de detecção — cole as linhas.
- [ ] `make quality` verde.

---

## Wave 2 — Fiação global (depende da Wave 1)

### ML-2A — Cabear o `git-branch-guard` no escopo global
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A concluído e auditado. **Cabear antes do no-op quebra todos os repositórios.**
**Arquivos:** alvos/geradores de harness nos 3 CLIs, `scripts/check-harness-hooks-parity.sh`,
`scripts/check-gates-falsify.sh`.

**Ação:** acrescentar a fiação global do `git-branch-guard` nos mesmos CLIs em que o
`credential-guard` já é cabeado, seguindo **exatamente** o padrão dele — é o modelo de referência,
já validado por gate de paridade estrutural.

**Estado atual medido (refs por config):**
```
~/.claude/settings.json    credential-guard=2   git-branch-guard=0
~/.codex/hooks.json        credential-guard=2   git-branch-guard=0
~/.gemini/settings.json    credential-guard=2   git-branch-guard=0
~/.copilot/settings.json   credential-guard=2   git-branch-guard=0
~/.cursor/hooks.json       ausente
~/.kiro/hooks/...json      ausente
```

**Critérios de aceite:**
- [ ] Fiação global presente nos mesmos CLIs do `credential-guard`, com paridade estrutural nos 3 runtimes.
- [ ] `check-harness-hooks-parity.sh` cobre a fiação nova e passa.
- [ ] `update harness` é **idempotente**: rodar duas vezes não duplica entradas.
- [ ] Não-regressão: fiação do `credential-guard` inalterada.
- [ ] Cenário de falsificação novo (baseline + detecção).
- [ ] `make quality` verde.

---

## Wave 3 — Integridade independente de fiação (depende da Wave 2)

### ML-3A — Verificar integridade de script global mesmo sem config apontando para ele
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/validator/validator_git_branch_guard.go` e/ou
`validator_credential_guard.go` + espelhos Node/Python, `scripts/check-gates-falsify.sh`.

**O ponto cego a fechar, com o mecanismo já provado:** `validateGuardGlobalScriptIntegrity` só avalia
os configs que **referenciam** o `scriptMarker`. Sem fiação, o laço nunca entra e a regra nunca roda
— foi assim que 3 versões passaram. **A verificação de integridade está condicionada à fiação, e
deveria estar condicionada à existência do artefato:** se o trackfw escreveu o script, o trackfw
verifica o script.

> A Wave 2 faz a regra passar a rodar **de graça** para o `git-branch-guard`. Este ML existe porque
> isso **não deve depender** da fiação — qualquer artefato global futuro cai no mesmo buraco.

**Critérios de aceite:**
- [ ] Script global presente e **divergente** do template é acusado, **mesmo sem** config referenciando.
- [ ] Script global **ausente** não é acusado (não é erro não ter instalado).
- [ ] Não-regressão do `credential-guard` global: continua sendo verificado, **sem duplicar** o aviso
      agora que há dois caminhos possíveis de disparo.
- [ ] `$HOME` do teste é **controlado pelo fixture**, nunca o real — há precedente de vazamento de
      ambiente neste tipo de cenário (Cenário 46, ML-1B de 2026-08-12).
- [ ] Cenário de falsificação (baseline + detecção), com prova de não-vacuidade.
- [ ] `make quality` verde.

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`: revisão do guard em escopo global
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md`

**Ações:** o no-op é **superfície de ataque nova** — avaliar se dá para induzir o no-op dentro de um
projeto trackfw (cwd manipulado, `trackfw.yaml` removido/renomeado, symlink, subdiretório com
`trackfw.yaml` falso). Avaliar se a fiação global introduziu caminho de desarme. Confirmar que a
integridade nova não é vacuosa. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora deste roadmap:** o escopo de projeto (`validate` não detectar hook na forma relativa antiga)
  tem REQ e roadmap próprios, **depois** deste — mesmos arquivos de validador, não pode ser paralelo.
- **Fora de escopo, já declarado no ADR:** repositório sem `trackfw.yaml` não é protegido. Deliberado.
- Commits e branch são exclusivos do `trackfw_architect`.
