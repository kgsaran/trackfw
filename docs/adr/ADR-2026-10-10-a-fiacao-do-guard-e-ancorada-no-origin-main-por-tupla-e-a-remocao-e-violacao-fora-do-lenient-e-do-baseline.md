---
status: Proposed
date: 2026-10-10
author: "zeus-tf"
---

# ADR: a fiação do guard é ancorada no origin/main por tupla e a remoção é violação fora do lenient e do baseline

> Date: 2026-10-10 | Status: Proposed

REQ: `docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`
Estende: `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
e `ADR-2026-09-17` (discriminante de 4 estados do `origin/main`, `loadOriginMainAnchor`).

## Context

O ADR-2026-08-12 decidiu que contra agente induzido com escrita irrestrita **não há prevenção**: a
resposta é **detecção ancorada no git**. As regras de integridade de guard cobrem o script, o modo e a
resolubilidade do comando — mas não a **fiação** (a entrada de hook que chama o guard).

**Medido em 2026-10-10**, em worktree de `origin/main` (`39ad84de`), `trackfw` instalado, nos dois
cenários — com o harness global desta máquina e com `HOME` apontando para diretório vazio (o aviso
sobre `~/.claude/settings.json` some, prova de que o `HOME` foi respeitado):

| `.claude/settings.json` | `trackfw validate` |
|---|---|
| intacto (base) | 169 warnings |
| `hooks.PreToolUse` apagado inteiro | 169 warnings — **saída idêntica byte a byte** |
| `trackfw guard git-branch` e `trackfw guard credential` trocados por `true` | 169 warnings — **idêntica** |

O caminho de menor esforço para desligar o controle é o único que nenhuma regra vê.

Fatos do código que restringem o desenho:

- A única regra de **presença** (`validateCredentialGuardPresenceRequired`,
  `internal/validator/validator_credential_guard.go:879`) cobre só Windsurf e Amazon Q, e usa o
  **próprio arquivo** como referência — ela some junto com o que deveria vigiar no caso "chave
  apagada" e não distingue "nunca instalado".
- O gerador (`internal/generators/agentfiles.go:368,395` e equivalentes por CLI) **deixa de emitir**
  a fiação de projeto quando o guard global está instalado, mas **não remove** a que já existe no
  disco (`mergeClaudeHookArray` só acrescenta). Logo, `trackfw update` não produz remoção.
- `credentialGuardAnchoredRules` (`validator_credential_guard_integrity.go:249`) ancora 3 regras de
  `credential_guard`; as 2 de `git_branch_guard` ficam de fora (AC5 da REQ).
- O repositório roda em LENIENT MODE até 2027-12-31: regra fora de `lenientCarveoutRules`
  (`validator.go:597`) vira warning, e regra fora da isenção de baseline pode ser tolerada.

## Decision

**D1 — Referência: a cópia do arquivo de hook em `origin/main`, não o HEAD.** Ancorar no HEAD é
derrotado por um `git commit` da remoção. O `origin/main` só muda por merge, que passa por revisão.
Reaproveita o discriminante de 4 estados do `loadOriginMainAnchor` (ADR-2026-09-17), estendido do
`trackfw.yaml` para os arquivos de hook — sem detector novo: sem git / sem `origin` → silencioso;
`origin/main` ilegível → **falha fechada**; arquivo ausente em `origin/main` → "nunca instalado";
presente → compara.

**D2 — Unidade de comparação: a tupla (arquivo, evento, matcher, guard).** Não a chave
`hooks.PreToolUse`. Cada tupla de guard presente em `origin/main` e ausente no disco é uma violação
nomeando arquivo, evento, matcher e guard. Isso cobre, com uma regra: chave apagada, matcher
apagado, matcher estreitado (`Bash|PowerShell` → `Bash`) e comando neutralizado (`true`). O guard
é reconhecido pelo comando canônico emitido pelo gerador, não por substring — `echo trackfw guard
credential` **não** conta como fiação.

**D3 — População: todos os arquivos de hook de projeto que o gerador escreve com um guard** —
`credentialGuardHookFiles` (Claude Code, Codex, Gemini, Cursor, Copilot, Kiro), Windsurf e Amazon Q,
para os dois guards (`credential`, `git-branch`). A lista é fechada e o Wave 0 confirma que está
completa.

**D4 — "Nunca instalado" ≠ "instalado e removido".** Tupla ausente em `origin/main` e no disco →
silêncio (remédio: `trackfw update`). Tupla presente em `origin/main` e ausente no disco → violação
(remédio: investigação). A mensagem diz qual dos dois é.

**D5 — Severidade e carve-outs.** A regra nova (nome proposto: `guard_wiring_removed`) entra em
`credentialGuardAnchoredRules`, na isenção de baseline e em `lenientCarveoutRules`. Critério do
carve-out atendido: a violação é contradição entre artefatos vivos (`origin/main` × disco), nunca
dívida histórica.

**D6 — AC5: as 2 regras de `git_branch_guard` entram em `credentialGuardAnchoredRules`.** A
assimetria não está documentada como deliberada em lugar nenhum; o mesmo mecanismo a fecha.

## Consequences

- Remoção da fiação na working tree **ou commitada na branch do PR** passa a ser violação, mesmo em
  lenient. Só um merge em `main` muda a referência — e esse é o limite declarado do ADR-2026-08-12.
- Primeira execução em projeto cuja `main` já perdeu a fiação: silêncio (é "nunca instalado" do ponto
  de vista da âncora). Declarado.
- Custo: uma leitura `git show origin/main:<arquivo>` por arquivo de hook existente, no mesmo
  carregamento do anchor.

### Resíduos declarados (a confirmar no Wave 0)

- **Escopo global** (`~/.claude/settings.json` etc.): fora do git, sem âncora. Não coberto.
- **Contornos que não tocam o arquivo rastreado**, como `.claude/settings.local.json` (não
  rastreado) ou chaves de desligamento de hooks do próprio CLI. O Wave 0 verifica na documentação
  de cada CLI o que existe; o que não puder ser ancorado vira resíduo nomeado.
- **Adversário com permissão de merge em `main`.** Fora, pelo ADR-2026-08-12.
- **O hook `AskUserQuestion` (attention-signal)** não é guard e fica fora.

## Alternatives Considered

- **Âncora no HEAD**, como sugeria a REQ: derrotada por `git commit`. Rejeitada.
- **Manifesto de instalação** gravado pelo `update`: é mais um arquivo na mesma superfície de escrita
  do agente; apagar os dois juntos é tão fácil quanto apagar um. Rejeitada.
- **Comparar só a existência de `hooks.PreToolUse`**: não pega matcher estreitado nem comando
  neutralizado, que foi medido. Rejeitada.
- **Estender `validateCredentialGuardPresenceRequired` a todos os CLIs**: usa o próprio arquivo como
  referência e acusaria "nunca instalado" como adulteração em todo repositório novo (AC3b). Rejeitada.
