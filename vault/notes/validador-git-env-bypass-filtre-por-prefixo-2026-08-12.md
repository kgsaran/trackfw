# Controle que depende de `git`: filtre `GIT_*` por prefixo, não enumere variáveis

> 2026-08-12 · ML-3B (achado) + ML-1B (correção) do
> `ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard`

## O erro de enquadramento

Ao descobrir que `GIT_DIR`/`GIT_WORK_TREE` herdadas do ambiente derrotavam uma regra de segurança, o
reflexo — inclusive o de Zeus, escrito no prompt do ML — foi:

> *"enumere as variáveis que **redirecionam** onde o repositório é lido"*

**Frame errado.** Produziu uma denylist de 8 nomes que **não fecha o problema**.

## O vetor real

O que derrota o controle **não é redirecionar** o repositório. É **fazer o `git` falhar por qualquer
motivo** — porque todo call site trata falha do subprocesso como *"sem âncora, fica em silêncio"* ou
*"cai no disco"*. **Falha aberta.**

Contraexemplo que quebrou a denylist:

```bash
GIT_CONFIG_COUNT=abc trackfw validate
```

`GIT_CONFIG_COUNT` **não redireciona nada** — injeta configuração via
`GIT_CONFIG_COUNT`/`GIT_CONFIG_KEY_n`/`GIT_CONFIG_VALUE_n`. Um valor malformado faz o `git` sair
**128** (`fatal: unable to parse command-line config`), e o controle silencia **inteiro**.

## A regra

**Filtre por prefixo, não por enumeração.** Ao invocar `git` de dentro de um controle de segurança:

1. **remova do ambiente do processo filho tudo que começa com `GIT_`**;
2. **ancore explicitamente** o repositório com `git -C <root>`, em vez de confiar no cwd;
3. prefira **argumentos em array** (`execFileSync`/`exec.Command`) a interpolação de string.

Qualquer lista fechada de nomes vai envelhecer: o git ganha variáveis novas a cada versão, e basta
**uma** que faça o processo falhar para derrotar um controle que trata falha como silêncio.

## Como reconhecer que você caiu nisso

Sintoma: o controle **passa** (`✓ No violations found.`, exit 0) num ambiente onde deveria acusar, e
**nada** no output menciona git. Falha silenciosa é a assinatura.

Teste mínimo, e vale rodar em qualquer controle que dependa de `git`:

```bash
GIT_CONFIG_COUNT=abc <comando>     # o controle ainda acusa?
GIT_DIR=/outro/.git <comando>      # e aqui?
```

## Ainda em aberto neste repositório

O mesmo padrão de `exec.Command("git", ...)` **sem ambiente limpo** existe **fora** do validador:
`internal/forge/resolve.go:192`, `internal/commands/branch.go:105`, `internal/commands/ship.go:111`.
**Não foram corrigidos** — o ML-1B era escopado ao validador. Não são controles de segurança, mas o
mesmo raciocínio se aplica se um dia forem.

## Relacionado

- `docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-...md` — **Emenda 3**
- `docs/seguranca/2026-08-12-estado-final-ancoragem-no-head.md` — o PoC original
- `docs/cli-parity.md`, "Pré-condições do fix do Codex" — **`GIT_DIR`/`GIT_WORK_TREE` já haviam
  mordido esta linha de trabalho antes**, e reapareceram em código escrito depois
