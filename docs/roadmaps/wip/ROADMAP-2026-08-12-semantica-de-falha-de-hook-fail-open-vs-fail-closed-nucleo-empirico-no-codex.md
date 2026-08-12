---
status: wip
date: 2026-08-12
req: "docs/req/REQ-2026-08-11-semantica-de-falha-de-hook-fail-open-vs-fail-closed-por-cli-verificacao-empirica-do-credential-guard-como-controle-de-negacao.md"
squad: "Ártemis, Prometeu, Hades, Hefesto"
---

# Roadmap: Semantica de falha de hook fail-open vs fail-closed — nucleo empirico no Codex

> Created: 2026-08-12 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-11-semantica-de-falha-de-hook-fail-open-vs-fail-closed-por-cli-verificacao-empirica-do-credential-guard-como-controle-de-negacao.md`

O `scripts/trackfw-credential-guard.sh` é um **controle de negação**: bloqueia a materialização de
credenciais reais por subagentes. Um controle de negação só vale se falhar **fechado**.

A revisão de segurança do `ROADMAP-2026-08-11` (ML-8B, parecer em
`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q3) registrou um achado **explícito, não inferido**:
**nenhuma fonte consultada** — pesquisa em doc primária dos 6 CLIs, ADR, ou documentação dos próprios
fornecedores — estabelece se a **falha na execução de um hook** é tratada como *fail-open* (a
ferramenta prossegue) ou *fail-closed* (a ferramenta bloqueia).

Isso importa porque hoje existem **três caminhos documentados** que terminam em "o guard não roda, em
silêncio" (`docs/cli-parity.md`, "Pré-condições do fix do Codex"):

1. execução fora de repositório git;
2. submódulo/worktree, onde `git rev-parse --show-toplevel` aponta para outra raiz;
3. `GIT_DIR`/`GIT_WORK_TREE` no ambiente, redirecionando a resolução.

Mais o skip silencioso do Codex em projeto não-*trusted* (ADR-2026-08-11, Emenda 1).

**Se a semântica for fail-open, o guard é contornável** por quem consiga influenciar o cwd ou o
ambiente. Se for fail-closed, os três caminhos são degradação de **disponibilidade**, não de
segurança. Resultado muito diferente — e hoje não sabemos qual é.

### Escopo: núcleo empírico no Codex, varredura documental no resto

Decidido por KG em 2026-08-12 (a REQ foi emendada antes deste roadmap). Levantamento:
**5 dos 6 CLIs estão instalados** nesta máquina (Kiro não), mas provar a semântica exige dirigir cada
CLI por um evento de tool-use real — 5 modelos de autenticação, 5 mecânicas de sessão. Só a prova do
Codex no ML-3A do roadmap anterior custou 141 tool calls e esbarrou num gate de trust não
documentado.

O foco vai para onde o risco concreto está: **os três caminhos de "guard não roda" são todos
específicos do Codex**. Claude e Gemini degradam para `/scripts/…` — **fail-to-run**, não
fail-to-wrong-script, e ninguém sem privilégio planta arquivo na raiz do sistema.

### 🔴 A distinção que o roadmap inteiro depende de não perder

São **dois casos diferentes**, que podem ter semânticas diferentes:

| Caso | Descrição | Coberto pelo contrato conhecido? |
|---|---|---|
| **A — script ausente / caminho inválido** | o `command` do hook não resolve; o processo nem chega a rodar | ❌ **Não.** E é **este** que os três caminhos documentados produzem |
| **B — script presente, sai com código != 0** | o hook roda e reprova | ✅ Parcialmente — o contrato de bloqueio conhecido é `exit 2` + stderr |

Medir só o caso B e concluir "fail-closed" seria **responder a pergunta errada**. O caso A é o que
importa para o risco real.

## Acceptance Criteria

- [ ] Caso A e caso B determinados empiricamente para o **Codex**, com evidência (comando executado +
      saída observada), distinguindo também `exit 1` de `exit 2` no caso B.
- [ ] Varredura **documental** (doc primária, URL + citação) para Claude, Gemini, Cursor, Copilot e
      Kiro. Onde a doc não responder: `INDETERMINADO` com o que foi procurado — **nunca inferir por
      analogia**.
- [ ] Registrado, para Claude e Gemini, **por que** a verificação empírica não foi considerada
      necessária (degradação fail-to-run) — para a decisão ser reavaliável e não parecer omissão.
- [ ] Se o Codex for **fail-open**: mitigação proposta ou risco aceito com justificativa explícita.
      Se **fail-closed**: registrar que os três caminhos passam a ser degradação de disponibilidade.
- [ ] Resultado consolidado em `docs/cli-parity.md`, tabela por CLI com a evidência de cada célula.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** altera `scripts/trackfw-credential-guard.sh`.
- **Não** altera o wiring de caminho dos hooks (encerrado no `ROADMAP-2026-08-11`).
- **Não** corrige os três caminhos de "guard não roda" — são consequência, não causa; corrigi-los só
  faz sentido depois de saber se são problema de segurança ou de disponibilidade. Se a correção for
  necessária, vira **REQ nova**.
- **Não** faz verificação empírica em Claude, Gemini, Cursor, Copilot ou Kiro.
- **Não** altera código de produto (`internal/`, `npm/src/`, `pypi/trackfw/`).

---

## Wave 1 — Determinação da semântica (2 MLs em **paralelo**)
> Dependências: nenhuma. **Paralelizáveis**: entregam arquivos disjuntos e não compartilham nenhum
> arquivo além do working context (append-only).

### ML-1A — Prova empírica: Codex, casos A e B
**Status:** 🔄 Em andamento
**Agente:** Ártemis (`artemis-tf`)
**Entregável:** `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md` (novo)

**Método sugerido** (validado no ML-3A do roadmap anterior; ajuste se necessário e explique):

1. Repositório git de fixture, **`$HOME` isolado**, `--dangerously-bypass-hook-trust` (o Codex ignora
   hooks de projeto não-*trusted* em silêncio — ver ADR-2026-08-11 Emenda 1 e
   `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`).
   **Nunca** escrever no `~/.codex/config.toml` real do usuário.
2. `.codex/hooks.json` com um hook `PreToolUse` no matcher `Bash`.
3. **O discriminante:** o comando da ferramenta (o `Bash` que o Codex vai executar) escreve uma
   **marca** em arquivo. Se a marca existir depois de o hook falhar, **a ferramenta prosseguiu →
   fail-open**. Se não existir, **fail-closed**.
4. Variar o hook:
   - **Caso A** — `command` apontando para caminho inexistente;
   - **Caso B1** — script existente que sai com `exit 1`;
   - **Caso B2** — script existente que sai com `exit 2` (contrato de bloqueio conhecido);
   - **Controle positivo** — hook que sai com `exit 0`: a marca **tem** que existir, senão o
     experimento não está medindo nada.
5. Registrar comando exato e saída observada de cada braço.

**Critérios de aceite:**
- [ ] Os 4 braços executados (A, B1, B2, controle positivo) com evidência colada.
- [ ] O **controle positivo** passa — sem ele, um "fail-closed" pode ser apenas o experimento não
      estar disparando a ferramenta.
- [ ] Veredito explícito por braço: `FAIL-OPEN` / `FAIL-CLOSED` / `INDETERMINADO`.
- [ ] `$HOME` isolado; nenhum arquivo de configuração pessoal do usuário modificado
      (`git status --porcelain` + confirmação explícita de que `~/.codex/` não foi tocado).
- [ ] Se impraticável após tentativa real (ex.: auth interativa), `INDETERMINADO` com o que foi
      tentado — **resultado legítimo, não falha**.
- [ ] Nenhum arquivo fora de `docs/pesquisa/` e `docs/agents-working-context.md`.

### ML-1B — Varredura documental: Claude, Gemini, Cursor, Copilot, Kiro
**Status:** 🔄 Em andamento
**Agente:** Prometeu (`prometeu-tf`)
**Entregável:** `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md` (novo)

**Ações:**
1. Para cada um dos 5 CLIs, procurar em **doc primária do fornecedor** o que acontece quando um hook
   falha: a ferramenta prossegue ou é bloqueada? Distinguir **caso A** (comando não resolve) de
   **caso B** (sai != 0).
2. Toda célula preenchida traz **URL + citação literal**. Sem citação → `INDETERMINADO`, com registro
   do que foi procurado e onde. **Nunca inferir por analogia com outro CLI.**
3. Fontes de partida: as mesmas de `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`
   (seção "Fontes consultadas"). Kiro é `INDETERMINADO` esperado — a doc já se mostrou omissa quanto
   a semântica de execução.
4. Registrar explicitamente, para **Claude e Gemini**, o raciocínio de por que a verificação empírica
   não foi considerada necessária: a degradação com variável indefinida é **fail-to-run**
   (`/scripts/…`, caminho absoluto na raiz do sistema onde nenhuma parte sem privilégio planta
   arquivo), não fail-to-wrong-script. Isso está no parecer
   `docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q2.

**Critérios de aceite:**
- [ ] Os 5 CLIs cobertos, com os casos A e B distinguidos.
- [ ] Toda afirmação tem URL + citação; toda lacuna é `INDETERMINADO` com evidência da busca.
- [ ] O raciocínio de Claude/Gemini está registrado (item 4).
- [ ] Nenhum arquivo fora de `docs/pesquisa/` e `docs/agents-working-context.md`.

---

## Barreira B1 — Avaliação do resultado (Zeus)
> Dependências: ML-1A e ML-1B concluídos e auditados.

Zeus avalia o veredito do Codex e decide:

- **fail-closed** → os três caminhos documentados viram degradação de **disponibilidade**; segue para
  a Wave 2 apenas para registro.
- **fail-open** → o guard é **contornável**; Zeus decide se abre **REQ nova** de mitigação (fora
  deste roadmap, cujo escopo negativo proíbe corrigir os caminhos) e a Wave 2 registra o risco.
- **INDETERMINADO** → registra como não verificável, com o que foi tentado.

---

## Wave 2 — Parecer de segurança (1 ML)
> Dependências: Barreira B1.

### ML-2A — Implicação de segurança do resultado
**Status:** ⬜ Pendente
**Agente:** Hades (`hades-tf`)
**Entregável:** `docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md` (novo). **Não modifica
código.**

**Ações:** dado o veredito do ML-1A, avaliar: (i) o credential-guard continua sendo um controle de
negação efetivo? (ii) os três caminhos documentados mudam de severidade? (iii) há mitigação de baixo
custo (ex.: o próprio guard verificar sua própria execução, ou o gate detectar ausência)? Achados
vão para Zeus, **não são implementados aqui**.

**Critérios de aceite:**
- [ ] Parecer respondendo (i)–(iii), ancorado no resultado empírico, não em hipótese.
- [ ] Recomendação explícita: risco aceito × REQ nova de mitigação.
- [ ] Nenhum arquivo de código modificado.

---

## Wave 3 — Consolidação documental (1 ML)
> Dependências: Wave 2.

### ML-3A — `docs/cli-parity.md`
**Status:** ⬜ Pendente
**Agente:** Hefesto (`hefesto-tf`)
**Arquivos afetados:** `docs/cli-parity.md` **somente**. Não modifica código de produto.

**Ações:** consolidar numa seção nova a tabela por CLI (caso A × caso B × veredito × evidência),
a conclusão do parecer de segurança, e — para Claude e Gemini — a justificativa de por que não houve
verificação empírica. Rodar `make quality`.

**Critérios de aceite:**
- [ ] Seção coerente com os dois documentos de pesquisa **e** com o parecer.
- [ ] `INDETERMINADO` registrado como tal, sem eufemismo.
- [ ] `make quality` exit 0; `internal/`, `npm/src/`, `pypi/trackfw/` intocados.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Paralelismo:** apenas na Wave 1 (ML-1A × ML-1B — entregáveis disjuntos). As Waves 2 e 3 são
  sequenciais e dependem do resultado da Wave 1.
- **Regra de paridade dos 3 CLIs não se aplica:** este roadmap não altera código de produto.
- **Um escritor por vez em `docs/cli-parity.md`:** só o ML-3A escreve nele, e só no fim.
