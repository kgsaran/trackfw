# Drift de conteúdo do bloco de regras trackfw entre os 3 CLIs — 2026-07-29

## Contexto

ML-5E (corretivo, `ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`) pediu para
ligar `generators.InjectRulesForTool` ao caminho de instalação por catálogo (`trackfw agents|skills
install --targets <tool>`, e `trackfw init --ai-tools`), restaurando a criação de `GEMINI.md`,
`.github/copilot-instructions.md`, `.windsurfrules` e `.amazonq/developer/guidelines.md` em projeto
novo — comportamento perdido quando o ML-5A removeu os aliases deprecated (`trackfw gemini|copilot|
windsurf|amazonq`).

## Achado não óbvio

Ao ligar a injeção ao fluxo canônico `agents install --targets <tool>` (arquivo
`internal/commands/integrations_flags.go`, função `executeIntegrationMutation`), o gate
`scripts/check-identity-parity.sh` (parte de `make quality`) passa a falhar para os 7 targets com
superfície de regras (`claude, codex, gemini, copilot, windsurf, amazonq, cursor`):

```
Identity parity [with-identity] target 'amazonq': artifact count mismatch (go=13 node=12 python=12)
... (mesma divergência +1 para os 7 targets)
```

Esse gate roda `agents install --targets <tool> --scope project` nos 3 CLIs (Go, Node, Python) em
projetos-tmp separados e exige snapshot **byte-a-byte idêntico** de toda a árvore do projeto. Antes
desta correção, o gate passava de forma vácua nesse aspecto: nenhum dos 3 CLIs criava o arquivo de
regras a partir do comando `agents install` (só os aliases deprecated do Go — que nunca existiram em
Node/Python — o faziam, fora do que este gate testa). Ao consertar isso só no Go, Go passa a emitir
um artefato a mais que Node e Python — quebra de paridade real, não pré-existente.

**Wire Node.js/Python também não resolve.** Prova concreta: gerei `.windsurfrules` via
`generators.InjectRulesForTool` (Go), `npm/src/generators/init.js:injectRulesForTool` (Node) e
`pypi/trackfw/generators/init_gen.py:inject_rules_for_tool` (Python) para o mesmo tool a partir de
diretórios vazios e comparei com `diff`. **O conteúdo já diverge entre os 3 runtimes hoje**,
independente deste ML:

- Chain de estados: Go emite `backlog / wip / blocked / done / abandoned`
  (`trackfwRulesBlock()` em `internal/generators/agentfiles.go` linha ~39); Node
  (`trackfwRulesBlock` em `npm/src/generators/init.js` linha ~461) e Python
  (`_trackfw_rules_block()` em `pypi/trackfw/generators/init_gen.py` linha ~231) emitem
  `backlog / analyzing / wip / blocked / done / abandoned` — Node/Python incluem `analyzing`,
  Go não.
- Item de protocolo "ML lifecycle" presente em Node/Python mas ausente em Go.
- Bloco `### Architecture Directives (mandatory)` presente em Go mas ausente em Node/Python — e,
  aliás, **duplicado duas vezes seguidas dentro do próprio Go** (`internal/generators/agentfiles.go`
  linhas 52–61 e 63–72, idêntico ao caractere).
- Seção `### Key Commands` presente em Go, ausente em Node/Python.

Ou seja: mesmo se os 3 runtimes chamassem `inject_rules_for_tool`/`injectRulesForTool` no mesmo call
site, o gate trocaria "count mismatch" por "hash mismatch" — continuaria vermelho.

## Decisão necessária (não tomada por mim — está fora do escopo do ML-5E)

Reconciliar o texto do bloco de regras entre os 3 CLIs é uma mudança de conteúdo que a ADR do ML-5E
não sancionou, e que precisa de REQ própria (qual é o texto canônico? quem decide?). Três
alternativas ficaram para o orquestrador escolher:

(a) Aceitar `check-identity-parity.sh` vermelho e landar a correção só no Go.
(b) Reverter o hook em `internal/commands/integrations_flags.go` (fluxo `agents install`) e manter
    só o hook em `internal/commands/init.go` (`trackfw init --ai-tools`), que não é exercitado por
    nenhum gate de `make quality` hoje — mas isso descumpre a instrução explícita do handoff de
    ligar exatamente ao fluxo canônico `agents install --targets <tool>`.
(c) Bloquear ML-5E até uma REQ nova reconciliar o bloco de regras nos 3 runtimes; só então religar
    `agents install`.

Ambos os hooks de código (init.go e integrations_flags.go) foram implementados e têm testes
Go verdes cobrindo criação a partir de projeto vazio e idempotência; o bloqueio é só o gate de
paridade cross-CLI.

## Referências

- `internal/commands/init.go` (`installAITools`)
- `internal/commands/integrations_flags.go` (`executeIntegrationMutation`)
- `internal/generators/agentfiles.go` (`trackfwRulesBlock`, `agentFiles`, `agentHeaders`)
- `scripts/check-identity-parity.sh`
- `npm/src/generators/init.js` (`trackfwRulesBlock`, ~linha 461)
- `pypi/trackfw/generators/init_gen.py` (`_trackfw_rules_block`, ~linha 231)
- `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` (ML-5E)

## Resolução (ML-5G, mesma sessão/branch — 2026-07-29)

**Decisão do usuário: reconciliar agora, nesta branch**, em vez das três alternativas (a/b/c)
levantadas acima. Texto canônico definido por união dos três runtimes (não por escolher um só):
lista de estados agora inclui `analyzing` nos três; item de ciclo de vida de ML e a diretiva de
ADRs globais (como item numerado do Agent Protocol) presentes nos três; seção `Key Commands`
propagada para Node/Python; duplicação do `Architecture Directives` removida do Go. Ordem final das
seções: Protocol → Attention Signal → Architecture Directives → Key Commands (maioria 2-1 de
Node/Python sobre a ordem antiga do Go).

Confirmado que **reconciliar só o texto não bastava**: o gate `check-identity-parity.sh` reprovava
por *contagem* de artefatos (`go=13 node=12 python=12`), porque só o Go tinha a injeção ligada ao
caminho de instalação por catálogo. Por isso o ML-5G também ligou
`injectRulesForTool`/`inject_rules_for_tool` em `npm/src/integrations/index.js:execute` (escopo
`install`) e em `pypi/trackfw/integrations/command.py:run` + `pypi/trackfw/commands/init.py`
(bloco `--ai-tools`), espelhando exatamente o que já existia só no Go
(`integrations_flags.go`/`init.go:installAITools`). Isso conclui a superfície descrita no ML-5E.

Novo gate `scripts/check-rules-parity.sh` (adicionado ao target `parity` do `Makefile`, com cenário
de falsificação em `scripts/check-gates-falsify.sh`) prova que os 4 arquivos auxiliares de regras
saem byte-idênticos dos três runtimes a partir de `trackfw init --ai-tools <tools>` — cobre a
divergência de texto isoladamente, já que `check-identity-parity.sh` só reporta "hash mismatch" sem
apontar o bloco de regras especificamente.

Resultado: `make quality` verde, `bin/trackfw validate --json` com 0 violações,
`check-identity-parity.sh` verde para as 7 targets/11 combinações com superfície de regras.
