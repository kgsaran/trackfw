---
status: Proposed
date: 2026-08-05
author: "kg.saran@gmail.com"
---

# ADR: hook de guarda contra materializacao de credenciais reais por subagentes

> Date: 2026-08-05 | Status: Proposed

## Context

`REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
documenta 2 incidentes reais num projeto consumidor do trackfw (ea-cmdb): subagentes especialistas
(QA e backend) materializaram JWTs reais em texto plano (arquivos soltos + stdout) ao validar
endpoints autenticados "com evidência real". Nenhum gate de pre-commit pega esse padrão, porque
audita apenas o que está staged — não stdout nem arquivos fora do índice do git.

O trackfw já resolve exatamente essa classe de problema — comportamento indesejado e recorrente de
subagentes que nenhum gate de commit alcança — via o mecanismo de "attention hooks"
(`internal/generators/hooks.go:InjectHooksDetected` + `agentfiles.go:InjectXHooks` por CLI, com
paridade em `npm/src/generators/hooks.js` e `pypi/trackfw/generators/hooks.py`). Hoje esse mecanismo
cobre só o evento de pedir permissão/pergunta ao usuário (`AskUserQuestion`/`ToolPermission`/
`PermissionRequest`, conforme o CLI).

Levantamento de código + documentação oficial de cada CLI (2026-08-05) mostrou que o suporte a um
evento "pré-execução de comando Bash" **não é uniforme**:

| CLI | Suporte pré-execução p/ Bash | Observação |
|---|---|---|
| Claude Code | Confirmado — `PreToolUse`/`PostToolUse`, matcher regex por `tool_name` | Já usado hoje com `matcher:"AskUserQuestion"` |
| Codex | Existe (`PreToolUse` intercepta só shell por design, ou `PermissionRequest` já usado hoje) | trackfw hoje usa `PermissionRequest` com matcher `.*`; não há matcher dedicado a "Bash" — filtro precisa inspecionar `tool_input.command` |
| Gemini CLI | `BeforeTool` existe na doc pública; trackfw hoje usa `Notification[ToolPermission]` como proxy | Precisa confirmar se `BeforeTool` intercepta antes da execução real |
| GitHub Copilot | `preToolUse` existe; formato trackfw atual não tem campo de matcher | Filtro por Bash precisa ser feito no próprio script via stdin |
| Cursor | `beforeShellExecution` é nativamente Bash-specific | trackfw hoje usa `preToolUse` genérico sem matcher — precisa migrar de evento |
| Kiro | `PreToolUse` com matcher por `tool_name` (regex) já usado hoje (`.*`) | Doc pública também descreve hooks orientados a `PostFileSave`; não há confirmação de que `PreToolUse` intercepta antes da execução de Bash especificamente. **Resolvido (ML-2F, 2026-08-05)**: confirmado via `kiro.dev/docs/hooks/`, `.../hooks/types` e `.../hooks/actions/` que `PreToolUse` ("Before a tool is about to execute", Can block: Yes) é um trigger real e distinto de `PostFileSave` — Kiro permanece na wave nativa. O wiring pré-existente também usava um schema inválido (`event`/matcher-objeto/sem `version`); corrigido junto (ver `docs/cli-parity.md`, seção "Kiro wiring (ML-2F)"). |
| Windsurf | **Não existe** — confirmado por `REQ-2026-06-20-attention-hooks-agent-clis.md` e por comentário já existente no código (`agentfiles.go`, bloco de regras: "there is no automatic hook for this") | Fora do escopo nativo de hooks; só instrução textual em `.windsurfrules` |

Também existe uma lição já registrada no projeto: `REQ-2026-08-04-scripts-de-attention-hooks-...`
corrigiu divergência **entre Go/Node/Python** nos scripts shell dos attention hooks e criou o gate
`check-attention-scripts-parity.sh` (`make quality` → alvo `parity`). Esse gate cobre **só os dois
scripts shell**, não o conteúdo dos `hooks.json`/`settings.json` gerados por CLI — e já existem hoje
divergências reais de formato entre os 3 stacks para Codex, Gemini e Copilot (campos extras só no
Python, estrutura de JSON diferente para Copilot) que nenhum gate atual detecta.

## Decision

1. **Modo padrão: avisador, não bloqueante.** O hook de guarda (`trackfw-credential-guard.sh`)
   detecta padrão de JWT (`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`) e de secrets comuns
   (ex.: `AKIA[0-9A-Z]{16}`) no comando (`PreToolUse`) e no output (`PostToolUse`), e por padrão
   **avisa** (loga/sinaliza) sem interromper a execução — evita que falsos positivos (hash de commit,
   base64 de outra natureza) travem workflows legítimos sem stakeholder pedir isso explicitamente.
   Um modo estrito (bloqueante, exit 2 no `PreToolUse`) fica disponível via `trackfw.yaml`
   (`credential_guard.mode: warn|block`, default `warn`).
2. **Wave 1 cobre todos os 6 CLIs com algum hook nativo pré/pós-execução já suportado hoje**: Claude
   Code, Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro. Cada um desses precisa de investigação de
   payload própria (documentada na tabela de Context) antes da implementação — não é copy-paste do
   formato Claude. Windsurf fica **fora de todas as waves nativas** por não ter hook pré-execução
   disponível (ver Alternatives Considered).
3. **Paridade Go/Node/Python desde o primeiro commit** — reaproveitando o padrão já usado pelos
   `InjectXHooks` existentes, mas **o gate de paridade precisa ser estendido**: o gate atual
   (`check-attention-scripts-parity.sh`) só compara os scripts shell; o roadmap precisa adicionar
   verificação de paridade estrutural para os `hooks.json`/`settings.json` gerados por CLI (campo a
   campo, não byte-a-byte, já que os formatos nativamente diferem por CLI) — para não repetir o erro
   já documentado em `REQ-2026-08-04-scripts-de-attention-hooks-divergem-...`.
4. **Teste de sabotagem obrigatório**: um teste que materializa um JWT sintético de fato e confirma
   que o hook detecta — não um self-test que reimplementa a checagem em paralelo (lição de
   `qualidade-selftest-paralelo-falso-verde`, citada na REQ).

## Consequences

**Positivas:**
- Fecha uma lacuna real de segurança que nenhum gate de pre-commit alcança (stdout e arquivos fora
  do índice do git).
- Modo avisador por padrão reduz risco de regressão em workflows existentes ao adotar a feature.
- Extensão do gate de paridade previne uma segunda ocorrência do problema já resolvido em
  `REQ-2026-08-04-scripts-de-attention-hooks-...` (formatos por CLI divergindo silenciosamente entre
  os 3 stacks).

**Negativas / riscos aceitos:**
- Modo avisador por padrão não impede a materialização em si — só sinaliza depois (para
  `PostToolUse`) ou no momento do comando (para `PreToolUse`, mas sem bloquear). Times que precisam
  de bloqueio real precisam ativar o modo estrito explicitamente — risco de ficar em modo permissivo
  por desconhecimento.
- 3 dos 6 CLIs da wave 1 (Codex, Gemini, Copilot) não têm matcher nativo dedicado a "Bash" no formato
  hoje suportado pelo trackfw — a filtragem terá que acontecer dentro do próprio script via inspeção
  de payload (stdin), aumentando a superfície de cada script de guarda em vez de delegar ao matcher
  do CLI. Aumenta custo de manutenção por CLI.
- Regex de JWT/AWS-key é heurística — falso-negativo é possível para outros formatos de credencial
  (API keys custom, tokens opacos sem prefixo reconhecível). Este ADR não cobre um detector
  exaustivo de segredos, só os padrões citados na REQ.

## Alternatives Considered

- **Bloqueante por padrão em todos os CLIs.** Rejeitado: risco de falso-positivo (hash de commit,
  base64 diverso) travando comandos legítimos sem uma opção simples de desligar — decisão explícita
  do usuário do trackfw ao refinar esta REQ, favorecendo avisador com bloqueio opt-in.
- **Wave 1 restrita só a Claude Code** (único CLI com suporte 100% confirmado sem investigação
  adicional). Rejeitado nesta ADR: decisão explícita do usuário foi priorizar cobertura ampla
  (6 CLIs) na wave 1, aceitando o custo de investigação de payload por CLI dentro da própria wave.
- **Incluir Windsurf na wave 1 via wrapper de shell fora do padrão de hooks.** Rejeitado: não existe
  hoje um evento de hook nativo pré-execução no Windsurf (confirmado por REQ anterior e pela
  documentação oficial da Cascade, que só expõe `pre_user_prompt`/`post_write_code`/
  `post_cascade_response` — nenhum por-tool-call). Uma abordagem via wrapper de shell fugiria do
  padrão de "hook gerado pelo trackfw" usado por todos os outros CLIs e foi deixada fora de escopo
  desta REQ; pode ser objeto de uma REQ own se houver demanda.
- **Reusar o gate de paridade existente sem extensão.** Rejeitado: o gate atual só verifica os
  scripts shell byte-a-byte; os hooks.json por CLI têm formatos nativamente diferentes entre CLIs
  (não é possível comparação byte-a-byte entre eles) e já divergem hoje entre Go/Node/Python sem
  detecção — reusar sem estender deixaria a mesma lacuna que motivou a REQ anterior.
