# Hooks de agente falham ABERTOS quando o script não resolve — o credential-guard é contornável

> 2026-08-12 · descoberto no `ROADMAP-2026-08-12-semantica-de-falha-de-hook-...` (ML-1A empírico no
> Codex + ML-1B documental)

## O fato

**Quando o `command` de um hook não resolve (script ausente, caminho inválido), a ferramenta
prossegue.** O agente detecta a falha, imprime um aviso, e executa a chamada mesmo assim.

Confirmado:

| CLI | Caso A (não resolve) | Caso B (`exit != 0`) | Como se soube |
|---|---|---|---|
| Claude Code | **FAIL-OPEN** | `exit 1` fail-open · `exit 2` fail-closed | doc primária |
| Codex CLI | **FAIL-OPEN** | `exit 1` fail-open · `exit 2` fail-closed | **empírico**, `codex-cli 0.147.0` |
| Cursor | **FAIL-OPEN** (padrão) | fail-open salvo `exit 2`; há opt-in `failClosed: true` | doc primária |
| Gemini | INDETERMINADO | fora de `{0,2}` é fail-open | doc primária |
| Copilot | fail-closed em `preToolUse` | fail-closed | doc primária |
| Kiro | INDETERMINADO | **depende da superfície** — abas IDE × CLI da doc divergem | doc primária |

## Por que isso importa

O `scripts/trackfw-credential-guard.sh` é um **controle de negação**. Se ele não roda, **nada
bloqueia** — e o agente não trata isso como erro fatal. Logo, qualquer condição que impeça o script
de ser encontrado **desliga o controle em silêncio**.

Condições conhecidas que produzem exatamente isso (`docs/cli-parity.md`, "Pré-condições do fix do
Codex"): fora de repositório git · submódulo/worktree · `GIT_DIR`/`GIT_WORK_TREE` no ambiente · e,
historicamente, caminho relativo puro resolvido contra cwd derivado (corrigido no
`ROADMAP-2026-08-11`).

## A pista estava no relato original, e passou despercebida

O bug que abriu o ciclo (screenshot do projeto CMDB, 2026-08-11) dizia:

```
Failed with non-blocking status code: /bin/sh: scripts/trackfw-credential-guard.sh:
No such file or directory
```

**`non-blocking`.** Durante toda a janela do bug, o guard não estava apenas quebrado — estava
**falhando aberto**. A doc do Claude é explícita: *"a mistyped path in `settings.json` leaves the
gate silently disabled."*

Isso reclassifica retroativamente aquele incidente: não era degradação de disponibilidade, era
**bypass silencioso de um controle de segurança**.

## Discriminadores observáveis (economiza tempo de investigação)

- **Codex:** a saída distingue `hook: PreToolUse Failed` (prosseguiu) de
  `hook: PreToolUse Blocked` (bloqueou). Se você vê `Failed`, o hook **não** bloqueou.
- **Só `exit 2` exato bloqueia.** `exit 1` com o **mesmo** stderr de um `exit 2` continua fail-open —
  testado com confundidor fechado. O discriminador é o código, não a mensagem.
- Vale também sob sandbox restrito real (`-s workspace-write`), não é artefato de bypass.

## Armadilhas de teste (macOS)

- `codex exec` estoura o timeout padrão de 2 min de ferramentas de shell. Use background + polling;
  **`timeout`/`gtimeout` não existem** neste macOS por padrão.
- Isole `CODEX_HOME`, não `$HOME` — e **nunca** escreva no `~/.codex/config.toml` real do usuário.
  Hooks de projeto só carregam em projeto *trusted*; use `--dangerously-bypass-hook-trust`.
  Ver `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`.

## Armadilha de leitura de doc

A página de hooks do **Kiro** tem **duas abas (IDE e CLI)** com semânticas de exit code
**diferentes** para a mesma seção. Ler só a aba visível produz um veredito errado com aparência de
citação sólida. Qual superfície o trackfw mira no Kiro é **pergunta em aberto**.

## O cwd do hook do Codex NÃO acompanha o agente (medido, 2026-08-12)

Verificado no ML-1C, com dois experimentos:

1. **`cd` de shell** (`mkdir sub && cd sub && git init`, depois chamada separada): o hook viu sempre
   a raiz do fixture. Sozinho isso **não prova nada** — cada `command_execution` do Codex é um
   `/bin/zsh -lc` novo, então não haveria cwd a herdar de qualquer forma.
2. **Parâmetro de working directory da própria chamada de ferramenta**: a chamada executou
   comprovadamente dentro de `sub` (o `pwd` real imprimiu `.../fixture/sub`), e **mesmo assim** o
   hook que a precedeu viu a raiz. Prova de ponta a ponta: o próprio `command` do hook é
   `$(git rev-parse --show-toplevel)/...`, expandido pelo Codex antes de invocar — se tivesse
   resolvido a partir de `sub`, o caminho não existiria e o disparo teria sido `Failed`. O append no
   log aconteceu.

**Conclusão:** o cwd do hook está desacoplado **tanto** do `cd` de shell **quanto** do parâmetro de
working directory da chamada. Um agente induzido **não** contorna o guard só mudando de diretório.

⚠️ **Isto não fecha a classe de ameaça.** Continuam **não medidas** outras vias alcançáveis sem `cd`
dentro da própria raiz da sessão — por exemplo apagar `scripts/trackfw-credential-guard.sh`, ou
redirecionar `.git` via gitfile (o caminho de submódulo/worktree já documentado). Não confunda
"aquele vetor específico foi refutado" com "o guard é inalcançável".

## Hipótese de mitigação (não implementada, não validada)

Converter "não consegui rodar" em "bloqueio" no próprio comando emitido — algo como
`sh -c 'test -x <script> && exec <script> || exit 2'`, de modo que a ausência do script produza
`exit 2` em vez de 127. **Não foi avaliada** quanto a portabilidade entre os 6 CLIs, custo por
chamada, nem efeito colateral em projeto legitimamente sem o script. Assunto de REQ própria.

## Relacionado

- `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md` — método e evidência empírica
- `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md` — 5 CLIs, doc primária
- `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` — Q3 levantou a pergunta que este ciclo respondeu
- `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`
