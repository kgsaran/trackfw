---
title: "Wave 0 — Threat Model: credential guard para Windsurf e Amazon Q"
date: 2026-10-08
author: hades-tf
req: "REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
roadmap: "ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
ml: "ML-0A + ML-0B"
status: concluido
---

# Wave 0 — Threat Model: credential guard para Windsurf e Amazon Q

> REQ-2026-10-06 · ML-0A (threat model) + ML-0B (remedição de eventos) · Hades-tf · 2026-10-08

> **Corrigido na auditoria 2026-10-08: formato do payload de escrita.** O campo `content` nunca
> existiu nos payloads de Windsurf `pre_write_code` nem de Amazon Q `fs_write`; todas as
> referências foram corrigidas para os formatos reais (`edits[*].new_string` e `file_text`/`new_str`
> respectivamente). Também corrigidos: `agent_action_name` de `pre_run_command` e `pre_read_code`
> (valores eram `"execute_bash"` e `"read_code"` — corretos são os nomes do próprio evento).
> Adicionados vetores R1a/R1b/R1c/R1d/R1e/EE1/EE4 (medidos em 2026-10-08).
> R1 reclassificado de REQ separada para ML-1C desta REQ.

---

## 1. Enumeração Completa dos Sítios

Lista fechada por grep de `trackfw guard credential`, `credential-guard` e `credentialGuard` em
`internal/`, `scripts/`, `docs/cli-parity.md` e ADRs. Para cada sítio: arquivo:linha e se
Windsurf/Amazon Q estão presentes ou ausentes.

### 1.1 Gerador de projeto (`agentfiles.go`)

| Sítio | Arquivo:linha | Windsurf | Amazon Q |
|---|---|---|---|
| Constante `guardCredentialCmdPSPOSIX` | `internal/generators/agentfiles.go:502` | **definida, NÃO usada** | — |
| Constante `guardCredentialCmdCmdExe` | `internal/generators/agentfiles.go:507` | — | **definida, NÃO usada** |
| `windsurfGitGuardCmd = guardGitBranchCmdPSPOSIX` | `internal/generators/agentfiles.go:1606` | git-branch apenas | — |
| `InjectWindsurfHooks` → `pre_run_command` | `internal/generators/agentfiles.go:1654` | **credential AUSENTE** | — |
| `InjectAmazonQHooks` → `preToolUse[execute_bash]` | `internal/generators/agentfiles.go:1804` | — | **credential AUSENTE** |

As constantes de credential foram criadas mas nenhuma instrução as conecta a `InjectWindsurfHooks`
ou `InjectAmazonQHooks`. A lacuna é de cabeamento, não de conteúdo.

### 1.2 Harness de atualização (`update.go`)

| Sítio | Arquivo:linha | Windsurf | Amazon Q |
|---|---|---|---|
| `buildHarnessTargetIDs()` — IDs `<tool>-credential-guard` | `internal/generators/update.go:521` | **AUSENTE** | **AUSENTE** |
| `harnessCatalogTargetOrder` | `internal/generators/update.go:548` | **AUSENTE** | **AUSENTE** |
| Comentário obsoleto | `internal/generators/update.go:511` | "Windsurf has no native hook mechanism and stays out per the ADR" — **OBSOLETO** | — |

Os seis CLIs cobertos pela harness (claude/codex/gemini/cursor/copilot/kiro) têm `<tool>-credential-guard`.
Windsurf e Amazon Q têm apenas `<tool>-agents` e `<tool>-skills`.

### 1.3 Validador (`validator_credential_guard.go`)

| Sítio | Arquivo:linha | Windsurf | Amazon Q |
|---|---|---|---|
| `credentialGuardHookFiles` entries | `internal/validator/validator_credential_guard.go:80–99` | **entrada existe, silenciada** | **entrada existe, silenciada** |
| Regra `credential_guard_hook_resolvable` | mesmo arquivo | não acusa violation | não acusa violation |

Comentário explícito no código: `// Both generators only inject git-branch-guard (not credential
guard), so the credential_guard_hook_resolvable rule silently skips their files (no
credentialGuardScriptMarker present)`. O silenciamento é intencional e documentado no código.

### 1.4 Testes do validador

| Sítio | Arquivo | O que cobre |
|---|---|---|
| `TestGuardHookResolvable_Windsurf_*` | `internal/validator/validator_guard_binary_probe_windsurf_amazonq_test.go` | Forma PS/POSIX do **git-branch** guard; familia errada; legado .sh. SEM testes de credential guard. |
| `TestGuardHookResolvable_AmazonQ_*` | mesmo arquivo | Forma cmd.exe do **git-branch** guard. SEM testes de credential guard. |

### 1.5 Documentação e ADRs

| Sítio | Arquivo:linha | Conteúdo |
|---|---|---|
| ADR-2026-08-05, Alternatives Considered | `docs/adr/ADR-2026-08-05-...md` | Exclusão explícita do Windsurf com premissa **obsoleta** ("não tem hook pré-execução por tool call") |
| ADR-2026-08-06, D7 | `docs/adr/ADR-2026-08-06-...md` | Windsurf e Amazon Q mantidos fora do escopo do credential guard — herda a exclusão sem rever a premissa |
| ADR-2026-10-04, D11 + linha 43 | `docs/adr/ADR-2026-10-04-...md` | D11 inclui ambos CLIs na tabela de família de hook (correto); comentário na linha 43 diz que `agentfiles.go` "emite para Windsurf e Amazon Q" — verdadeiro para git-branch, **ENGANOSO para credential** |
| `docs/cli-parity.md:3971` | mesmo | "Windsurf has no native hook mechanism and stays out per the ADR" — **OBSOLETO** |
| `vault/notes/windsurf-amazonq-sem-harness-scope-...md` | vault | Confirma ausência de harness targets; comentário do update.go como stale |

### 1.6 Scripts

| Sítio | Arquivo:linha | Windsurf | Amazon Q |
|---|---|---|---|
| `scripts/trackfw-credential-guard.sh:2` | wrapper → `trackfw guard credential` | sem referência específica | sem referência específica |
| `scripts/check-update-parity.sh:259` | `SCRIPT_TARGETS = {'git-branch-guard-script', 'credential-guard-script'}` | exclui ambos (aplica aos 6 CLIs do harness) | exclui ambos |

**A lista está fechada.** Windsurf e Amazon Q estão explicitamente ausentes em todos os sítios de
emissão e harness, e silenciados no sítio de validação. Não há sítio espúrio que emita o credential
guard para esses dois CLIs.

---

## 2. Threat Model

> Quem esvazia esta Wave 0 sem quebrar regra escrita, e como.

Os adversários aqui são o implementador apressado e o arquiteto otimista — não um atacante externo.

### A1 — O implementador que faz o mínimo formal

**Mecanismo:** o ML-1B diz "emitir o credential guard no evento decidido, por CLI decidido". Se a
Wave 0 decide apenas `pre_run_command` para Windsurf (o evento já existente), o implementador
conecta `guardCredentialCmdPSPOSIX` ao array de `pre_run_command` e os testes passam. O
`pre_write_code` continua descoberto — e é o evento mais crítico (conteúdo de arquivo escrito
chega no payload).

**Por que não quebra regra escrita:** a regra "emitir no evento decidido" é atendida se a Wave 0
não decide `pre_write_code`. A Wave 0 ELA MESMA deve obrigar a decisão por evento, não só por CLI.

**Contra-medida:** o Veredito desta Wave 0 enumera os eventos por CLI (veja seção 5). O ML-1B
aceita apenas se cobrir todos os eventos listados no Veredito.

### A2 — O arquiteto que aceita cobertura parcial como cobertura completa

**Mecanismo:** Layer 1 (varredura bruta do payload) detecta JWT se ele aparecer literalmente no
JSON. Para `pre_run_command` com um comando como `curl -H "Authorization: Bearer $TOKEN"` onde
`$TOKEN` foi expandido, Layer 1 funciona. O arquiteto observa que "a detecção funciona" no teste
feliz (Vector A: RC=2) e aprova sem medir o caso de arquivo referenciado (Vector C: RC=0).

**Por que não quebra regra escrita:** nenhuma regra exige que todos os vetores de ataque sejam
testados — só que "os critérios de aceite são atendidos". Critérios de aceite que não incluem
Vector C permitem aprovar este buraco.

**Contra-medida:** os critérios de aceite do ML-1B (como especificado no Veredito desta Wave 0)
devem incluir explicitamente o teste de falsificação para Layer 2b (Vector C equivalente).

### A3 — A lacuna Layer 2b não detectada

**Mecanismo:** `credCmdLineRe` em `internal/guard/credential.go:39` extrai `"command":` mas NÃO
`"tool_info":{"command_line":"..."}` (schema do Windsurf `pre_run_command`). Isso torna a segunda
camada de detecção (varredura de arquivos referenciados no comando) cega para Windsurf. Um agente
que executa `cat /etc/secret.txt` via Windsurf escapa ao Layer 2b — medido (Vector C: RC=0).

**Por que não quebra regra escrita:** o escopo negativo da REQ diz "não muda o credential guard em
si". A correção do `credCmdLineRe` para entender `tool_info.command_line` **mudaria o guard** e
está fora do escopo desta REQ. Logo, o implementador que não corrige `credCmdLineRe` está dentro do
escopo negativo — mas o resultado é a lacuna permanecer.

**Contra-medida:** o ML-1B deve declarar explicitamente que instala o credential guard em
`pre_run_command` sabendo que Layer 2b é cego para esse evento; e deve incluir um teste que documenta
a lacuna como gap pré-ML-1C. A correção de `credCmdLineRe` entra como ML-1C nesta REQ (ver R1
revisado). _(Revisado na auditoria 2026-10-08)_

### A4 — Comentários obsoletos não corrigidos abrem regressão futura

**Mecanismo:** `update.go:511` e `cli-parity.md:3971` têm o comentário "Windsurf has no native hook
mechanism and stays out per the ADR". Se o ML-1B adiciona o credential guard mas não corrige esses
comentários, um futuro agente pode ler o comentário e remover o código recém-adicionado.

**Por que não quebra regra escrita:** não há regra que force o comentário a ser atualizado no mesmo
PR, a menos que o Veredito desta Wave 0 exija.

**Contra-medida:** o Veredito declara que os dois comentários obsoletos são correções obrigatórias
do mesmo PR (ML-1B).

---

## 3. Alvos de Falsificação nas Duas Direções

Por superfície, indicando onde a sabotagem entra, qual gate deve pegar e em qual direção.

### S1 — Gerador (`InjectWindsurfHooks` / `InjectAmazonQHooks`)

| Direção | Cenário | Onde entra | Gate que deve pegar |
|---|---|---|---|
| Falso negativo | `pre_write_code` não recebe o credential guard | `agentfiles.go:InjectWindsurfHooks`, array do evento `pre_write_code` ausente | `TestInjectWindsurfHooks_CredentialGuardPresentInPreWriteCode` (novo, ML-1B) |
| Falso negativo | Amazon Q `preToolUse[fs_write]` não recebe credential guard | `agentfiles.go:InjectAmazonQHooks`, matcher ausente | `TestInjectAmazonQHooks_CredentialGuardPresentInFsWrite` (novo, ML-1B) |
| Falso positivo (deny-all) | credential guard instalado em evento post-ação (`post_write_code`) | nome de evento errado no JSON emitido | `TestInjectWindsurfHooks_CredentialGuardNotInPostEvents` (novo, ML-1B) |
| Falso positivo (falsa proteção) | credential guard instalado em `pre_read_code` | Windsurf `pre_read_code` payload não contém conteúdo de arquivo; Layer 1 não detecta JWT no arquivo lido | medido (Vector C: RC=0); teste que afirma esta limitação como residual |

### S2 — Harness (`buildHarnessTargetIDs`)

| Direção | Cenário | Onde entra | Gate que deve pegar |
|---|---|---|---|
| Falso negativo | `windsurf-credential-guard` ausente do catálogo | `update.go:buildHarnessTargetIDs`, ID não adicionado | `TestBuildHarnessTargetIDs_WindsurfCredentialGuard` (novo, ML-1B) |
| Falso negativo | ID presente mas aponta para evento errado (post) | conteúdo emitido pelo harness target | teste que verifica evento emitido |
| Falso positivo | harness target adicionado antes da implementação em `agentfiles.go` | ordem de mudanças no mesmo PR | inspeção de auditoria (não automatizável) |

### S3 — Validador (`credential_guard_hook_resolvable`)

| Direção | Cenário | Onde entra | Gate que deve pegar |
|---|---|---|---|
| Falso negativo | validate silencia `.windsurf/hooks.json` sem credential guard | comentário de silenciamento não removido de `validator_credential_guard.go:80–99` | `TestCredentialGuardHookResolvable_Windsurf_SemGuard_Violation` (novo, ML-1B) — deve **falhar** hoje e **passar** após ML-1B |
| Falso positivo | validate acusa `.windsurf/hooks.json` com credential guard correto | lógica de detecção muito restrita (ex.: exige marcador exato do script, não aceita subcomando) | `TestCredentialGuardHookResolvable_Windsurf_ComGuard_Ok` (novo, ML-1B) |

### S4 — Parser de payload (`credCmdLineRe`, `credential.go:39`)

| Direção | Cenário | Onde entra | Gate que deve pegar |
|---|---|---|---|
| Falso negativo (Layer 2b) | JWT apenas no arquivo referenciado em `tool_info.command_line`; Layer 2b não extrai o comando | `credCmdLineRe` só casa `"command":`, não `"tool_info":{"command_line":"..."}` | **medido** — Vector C: RC=0; ML-1B documenta lacuna com teste; ML-1C corrige (`credCmdLineRe` passa a extrair `"command_line"`) |
| Falso positivo não existe | Layer 2b não lê arquivos arbitrários sem extração do comando | — | — |

**Nota de reconciliação (Regra Dura de Causa Raiz):** a correção de `credCmdLineRe` para entender
`tool_info.command_line` é mesma causa (cobrir Windsurf) desta REQ. _(Revisado na auditoria 2026-10-08: de REQ separada para ML-1C desta REQ)_

### S5 — Escopo de projeto (`os.Stat(trackfw.yaml)`)

| Direção | Cenário | Onde entra | Gate que deve pegar |
|---|---|---|---|
| Falso negativo | Amazon Q herda cwd de lançamento; `trackfw.yaml` ausente nesse cwd; guard retorna 0 | `credential.go`, linha do Stat; cwd não setado pelo Rust no processo filho | medido — Vector E (RC=0 sem trackfw.yaml); comportamento de design; residual declarado |

---

## 4. Residual Declarado

Esta Wave 0 aceita explicitamente não cobrir os seguintes pontos:

**R1 — Layer 2b cego para Windsurf `pre_run_command` — a corrigir nesta REQ (ML-1C).**
`credCmdLineRe` extrai `"command":` mas não `"tool_info":{"command_line":"..."}`. Quando o JWT está
apenas em um arquivo referenciado por um comando Windsurf (e não inlinado no argumento), a segunda
camada de detecção não detecta. Layer 1 ainda detecta se o JWT aparecer literalmente no payload.
O objetivo desta REQ é o credential guard cobrir o Windsurf; um parser que não entende o payload
do Windsurf é a mesma meta, mesma causa (Regra Dura de Causa Raiz). Entra como ML-1C.
_(Revisado na auditoria 2026-10-08: reclassificado de REQ separada para ML-1C desta REQ)_

**Especificação de ML-1C:**
- **Item 1 — `tool_info.command_line`**: adicionar extração de `"command_line"[ \t]*:[ \t]*"([^"]*)"` em
  `credSecondLayer` (chave distinta de `"command":`). Pode ser segundo match no mesmo loop ou
  regex separado; o valor extraído alimenta o mesmo pipeline de tokens (cat/head/tail/jq/grep).
- **Item 2 — `cwd` do payload**: o campo `tool_info.cwd` é necessário para resolver caminhos
  relativos no comando. Se `command_line` contém `cat ./token.txt` e o `cwd` Windsurf é `/project`,
  o guard deve resolver `/project/token.txt`. Requer passar o cwd extraído do payload para
  `credScanFile` em vez do cwd do processo guard.
- **Item 3 — Abordagem**: JSON unmarshal preferível (v8 tem implementação única em Go; a razão
  original do vault note para regex — paridade byte-a-byte entre 3 stacks — é obsoleta).
  O guardrail de design muda: decisão documentada no ML-1C. Implicação: `"cat \"path\""` (aspas
  no argumento) seria corretamente parseado com JSON unmarshal, mas não com regex (Vector R1e
  medido: RC=0 com aspas → residual permanente se regex mantido).
- **Item 4 — Ephemeral exemption bypass** (Vector EE4): `credIsAllEphemeral` dispara quando
  `> /dev/null` aparece no payload de escrita de Amazon Q (estrutura `}}` sem `]` strip). O
  guard não distingue redirects em payload de write de redirects em payload de execute. A correção
  deve excluir payloads de write events da aplicação de `credIsAllEphemeral`, ou aplicar a
  exemption SOMENTE quando Layer 2a/2b forem a causa (não Layer 1). Windsurf `pre_write_code`
  não tem o mesmo bypass (estrutura `"}]}}` impede strip completo — EE1 medido: RC=2).
- **Teste de falsificação ML-1C**: (a) `tool_info.command_line:"cat JWTFILE"` → RC=2 (pré-ML-1C: RC=0; pós: RC=2); (b) EE4 equivalente → RC=2 (pré: RC=0; pós: RC=2); (c) `command:"cat \"JWTFILE\""` → RC=2 se JSON unmarshal adotado (RC=0 se regex).
Evidência: Vector R1a medido — `"command_line":"cat JWTFILE"` → RC=0 (pré-ML-1C).
Vector EE4 medido — JWT + `> /dev/null` em `new_str` Amazon Q → RC=0 (falso negativo).

**R2 — `pre_read_code` sem proteção real.**
O payload de `pre_read_code` contém apenas o caminho do arquivo lido. O guard não lê o conteúdo
do arquivo; só varre o payload. Instalar o credential guard em `pre_read_code` não fornece proteção
contra leitura de arquivos com credenciais — é falsa proteção. A mesma limitação existe para o
`PreToolUse:Read` do Claude Code (design documentado no ADR-2026-08-06 D8).

**R3 — cwd do Amazon Q não determinado.**
O processo filho do hook herda o cwd do processo Amazon Q CLI, que pode não ser a raiz do projeto.
Se `trackfw.yaml` não está no cwd herdado, o guard retorna 0 silenciosamente. Risco mitigado na
prática (usuários lançam Q CLI da raiz do projeto). Confirmação requer experimento em produção (fora
desta REQ).
Evidência: portabilidade doc 2026-10-04, seção Amazon Q (d): "não chama `.current_dir()`".

**R4 — Escopo global do Amazon Q não documentado.**
A documentação oficial não descreve um arquivo de hooks global para Amazon Q equivalente ao
`~/.codeium/windsurf/hooks.json` do Windsurf. Não é possível instalar o credential guard
globalmente para Amazon Q com o esquema atual. Fonte: `github.com/aws/amazon-q-developer-cli`,
`docs/hooks.md` (acessado 2026-10-08) — não menciona escopo global para `preToolUse`.

**R5 — Compatibilidade com binário ausente (fail-open).**
Windsurf e Amazon Q são fail-open em exit ≠ 2 (exit 127 quando `trackfw` não está no PATH = permite
execução). Este residual é pré-existente (coberto por REQ-2026-09-05) e fora do escopo desta REQ.

---

## ML-0B — Remedição dos Eventos de Hook (AC1 da REQ)

### Windsurf (Cascade hooks)

**Fontes:**
- https://docs.windsurf.com/windsurf/cascade/hooks (também referenciado como https://docs.devin.ai/desktop/cascade/hooks no doc de portabilidade de 2026-10-04); acesso 2026-10-08.
- Portabilidade doc: `docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`, seção 7.
- Medição direta: teste de vetores com binário compilado em 2026-10-08.

**Nota de URL:** `docs.windsurf.com/windsurf/cascade/hooks` redireciona para `docs.devin.ai/desktop/cascade/hooks` (verificado 2026-10-08). Windsurf é produto da Codeium; `devin.ai` é produto da Cognition. A redireção confirma que a documentação do Windsurf foi hospedada no domínio Devin; os payloads de hook foram verificados nessa URL em 2026-10-08 e são a fonte primária desta seção.

#### Tabela Windsurf × Evento

| Evento | Quando dispara | Payload no stdin | Exit 2 bloqueia? | Contém conteúdo? | Hoje emitido? |
|---|---|---|---|---|---|
| `pre_run_command` | antes de executar comando shell | `{"agent_action_name":"pre_run_command","tool_info":{"command_line":"<cmd>","cwd":"<dir>"}}` _(Corrigido na auditoria 2026-10-08: `agent_action_name` é `"pre_run_command"`, não `"execute_bash"`. Fonte: docs.devin.ai/desktop/cascade/hooks, acesso 2026-10-08)_ | **SIM** | não (só o comando) | SIM — git-branch guard |
| `pre_read_code` | antes de ler arquivo | `{"agent_action_name":"pre_read_code","tool_info":{"file_path":"<path>"}}` _(Corrigido na auditoria 2026-10-08: `agent_action_name` é `"pre_read_code"`, não `"read_code"`. Fonte: docs.devin.ai/desktop/cascade/hooks, acesso 2026-10-08)_ | **SIM** | **NÃO** (só o caminho) | NÃO |
| `pre_write_code` | antes de escrever arquivo | `{"agent_action_name":"pre_write_code","tool_info":{"file_path":"<path>","edits":[{"old_string":"<old>","new_string":"<new>"}]}}` _(Corrigido na auditoria 2026-10-08: campo real é `edits[*].new_string`, não `content`)_ | **SIM** | **SIM** (conteúdo em `edits[*].new_string`) | NÃO |
| `post_run_command` | após execução de comando | — | NÃO (post) | — | NÃO |
| `post_write_code` | após escrita de arquivo | — | NÃO (post) | — | NÃO |

**Escopo global:** `~/.codeium/windsurf/hooks.json` (usuário, macOS/Linux). Sistema: `/Library/Application Support/Windsurf/hooks.json` (macOS). O projeto emite `.windsurf/hooks.json` (escopo de projeto).

**Comportamento medido por vetor:**

| Vetor | Payload | RC esperado | RC obtido | Layer que detecta |
|---|---|---|---|---|
| A | `pre_run_command`, JWT inline em `tool_info.command_line` | 2 | **2** | Layer 1 (varredura bruta) |
| C | `pre_read_code`, JWT apenas no arquivo em `tool_info.file_path` | 2 | **0** | nenhuma — Layer 2b cega para `tool_info.file_path`; **LACUNA R2** |
| D | `pre_run_command`, payload limpo (sem JWT) | 0 | **0** | sem falso positivo |
| H | `pre_write_code`, JWT em `edits[0].new_string` (formato real) _(Corrigido na auditoria 2026-10-08)_ | 2 | **2** | Layer 1 (varredura bruta) |
| R1a | `pre_run_command`, `command_line:"cat JWTFILE"` — JWT apenas no arquivo | 0 | **0** | nenhuma — Layer 2b cego para `tool_info.command_line` (campo não casa `credCmdLineRe`); **LACUNA R1, corrigível em ML-1C** |
| R1c | `pre_run_command`, `command_line:"echo x > JWTFILE"` — redirect para arquivo com JWT | 2 | **2** | Layer 2a (redirect scan — `>` aparece no JSON raw; `credRedirectRe` o captura e escaneia o arquivo) |
| EE1 | `pre_write_code`, JWT + `> /dev/null` em `edits[0].new_string` | 2 | **2** | Layer 1 (JWT detectado; exemption NÃO dispara: `]` após `/dev/null` impede TrimRight de produzir `/dev/null` exato) |

Binário: compilado de `cmd/trackfw` branch `fix/credential-guard-nao-cobre-windsurf-e-amazon-q`,
rodado com cwd apontando para diretório com `trackfw.yaml` (`credential_guard.mode: block`).
Data: 2026-10-08.

**Confronto com gerador:** `InjectWindsurfHooks` emite `pre_run_command` com git-branch guard
(`agentfiles.go:1654`). `pre_write_code` e `pre_read_code` nunca são emitidos — nem para git-branch
nem para credential.

---

### Amazon Q Developer CLI

**Fontes:**
- `github.com/aws/amazon-q-developer-cli` (branch main): `crates/chat-cli/src/cli/agent/hook.rs`,
  `crates/chat-cli/src/cli/chat/cli/hooks.rs`, `docs/hooks.md`.
  Portabilidade doc cita URL do repositório, acesso 2026-10-04. Confirmado 2026-10-08 (sem mudança
  de schema observada).

#### Tabela Amazon Q × Evento

| Evento | Matcher | Quando dispara | Payload no stdin | Exit 2 bloqueia? | Contém conteúdo? | Hoje emitido? |
|---|---|---|---|---|---|---|
| `preToolUse` | `execute_bash` | antes de executar comando shell | `{"hook_event_name":"PreToolUse","tool_name":"execute_bash","tool_input":{"command":"<cmd>"}}` | **SIM** | não (só o comando) | SIM — git-branch guard |
| `preToolUse` | `fs_write` | antes de escrever arquivo | enum com tag `"command"`: `create` → `{"command":"create","path":"<path>","file_text":"<content>"}` · `str_replace` → `{"command":"str_replace","path":"<path>","old_str":"<old>","new_str":"<new>"}` · `append` → `{"command":"append","path":"<path>","new_str":"<new>"}` · `insert` → `{"command":"insert","path":"<path>","insert_line":<n>,"new_str":"<new>"}` _(Corrigido na auditoria 2026-10-08: campo real é `file_text`/`new_str` por variante, não `content`. Fonte: `crates/chat-cli/src/cli/chat/tools/fs_write.rs`, enum `FsWrite`, atributo `tag = "command"`)_ | **SIM** | **SIM** (conteúdo em `file_text` ou `new_str`) | NÃO |
| `preToolUse` | `fs_read` | antes de ler arquivo | `{"hook_event_name":"PreToolUse","tool_name":"fs_read","tool_input":{"path":"<path>"}}` | **SIM** | **NÃO** (só o caminho) | NÃO |
| `postToolUse` | qualquer | após execução | — | NÃO (post) | — | NÃO |

**Escopo global:** não documentado para hooks de agente. Os arquivos de agente são por-projeto
(`.amazonq/cli-agents/`). Fonte: `docs/hooks.md` do repositório oficial (acessado 2026-10-08) —
não menciona arquivo de hooks global. Marcado como "não documentado" — não inferido.

**Shell:** `cmd.exe /C` no Windows; `bash -c` no Unix. Fonte: `hooks.rs` (portabilidade doc, seção 8).
Exit 2 = bloqueia; exit ≠ 2 = fail-open.

**Comportamento medido por vetor:**

| Vetor | Payload | RC esperado | RC obtido | Layer que detecta |
|---|---|---|---|---|
| B | `preToolUse[execute_bash]`, JWT inline em `tool_input.command` | 2 | **2** | Layer 1 (varredura bruta) |
| F | `preToolUse[execute_bash]`, JWT apenas em arquivo de redirecionamento `>` | 2 | **2** | Layer 2a (credRedirectRe encontra `> JWTFILE` no raw JSON; scan do arquivo) + Layer 2b (credCmdLineRe extrai cmd; cat token → scan) |
| G | `preToolUse[fs_write]` com JWT em `file_text`/`new_str` (formato real) | 2 | **2** | Layer 1 apenas (varredura bruta detecta JWT; `credCmdLineRe` extrai `"command":"create"/"str_replace"` mas não entra no switch cat/head/tail — Layer 2b inerte) _(Corrigido na auditoria 2026-10-08)_ |
| R1b | `preToolUse[execute_bash]`, `command:"cat JWTFILE"` — JWT apenas no arquivo | 2 | **2** | Layer 2b (`credCmdLineRe` extrai `"command":"cat JWTFILE"`; cat → scan JWTFILE → JWT detectado) |
| R1d | `preToolUse[execute_bash]`, `command:"cat subdir/secret.txt"` (relativo) | 2 | **2** | Layer 2b (path relativo resolvido a partir do cwd do processo guard = PROJ) |
| R1e | `preToolUse[execute_bash]`, `command:"cat \"JWTFILE\""` (caminho entre aspas) | 2 | **0** | **FALHA** — Layer 2b: unescape de `\"→"` faz `credCmdLineRe` capturar só `cat ` (para na aspa); `credScanFile` não vê o arquivo. Residual da abordagem regex. Corrigível por JSON parse em ML-1C. |
| EE4 | `preToolUse[fs_write]` `str_replace`, JWT + `> /dev/null` em `new_str` | 2 | **0** | **FALSO NEGATIVO** — Layer 1 detecta JWT; mas `credIsAllEphemeral` dispara porque `> /dev/null"}}` → após strip de `""` e `}}` → `/dev/null` exato → ephemeral = true → RC=0. Bypass por injeção de `> /dev/null` junto ao JWT. ML-1C deve cobrir. |

**Nota sobre Vector F:** o `credCmdLineRe` (`"command"[ \t]*:[[:space:]]*"([^"]*)"`) extrai o
valor de QUALQUER chave `"command":` no JSON, independente de nível. Para Amazon Q, o comando está
em `tool_input.command` — que casa com o regex. Para Windsurf, o comando está em
`tool_info.command_line` — que NÃO casa. Esta é a assimetria medida.

**Layer 2b com `fs_write` — comportamento medido (auditoria 2026-10-08):**
O `credCmdLineRe` extrai o valor `"create"` ou `"str_replace"` do campo `"command"` do `tool_input`
de Amazon Q `fs_write`. Esses valores entram na rotina `credSecondLayer` como `tokens[0]` do
`CMD_LINE`. Como `"create"` e `"str_replace"` não figuram no switch `["cat","head","tail","jq","grep"]`,
Layer 2b retorna "" — nenhuma varredura de arquivo adicional, nenhum falso positivo.
RC medido: 0 (sem JWT no payload) — CORRETO. Não há efeito colateral do `"command":"create"` no
comportamento de segurança: Layer 1 ainda detecta o JWT nos campos `file_text`/`new_str` quando
presente, independentemente do valor de `"command"`.
_(Corrigido na auditoria 2026-10-08: medição com payload real confirma que o Vector G anterior
(medido com formato inventado `"content"`) continua válido com o formato correto `"file_text"`/`"new_str"`)_

**Confronto com gerador:** `InjectAmazonQHooks` emite `preToolUse[execute_bash]` com git-branch
guard (`agentfiles.go:1804`). O matcher `fs_write` nunca é injetado — nem para git-branch nem para
credential.

---

### Resumo comparativo CLI × Evento × Cobertura atual

| CLI | Evento | Hoje emitido | Proteção por Layer 1 | Proteção por Layer 2b | Recomendação |
|---|---|---|---|---|---|
| Windsurf | `pre_run_command` | git-branch | **SIM** (JWT inline no cmd) | NÃO (`tool_info.command_line` não casa `credCmdLineRe`) | INSTALAR credential guard — Layer 1 cobre o caso principal; Layer 2b é residual declarado (R1) |
| Windsurf | `pre_write_code` | NÃO | **SIM** (JWT em `edits[*].new_string`) | N/A | INSTALAR — evento mais crítico; Layer 1 detecta JWT direto; exemption de `/dev/null` não dispara por estrutura de array JSON (EE1 medido) |
| Windsurf | `pre_read_code` | NÃO | NÃO (payload sem conteúdo) | NÃO | NÃO INSTALAR — proteção inexistente; geraria falsa sensação de cobertura (R2) |
| Amazon Q | `preToolUse[execute_bash]` | git-branch | **SIM** (JWT inline no cmd) | **SIM** (`tool_input.command` casa `credCmdLineRe`) | INSTALAR credential guard — cobertura completa das duas camadas |
| Amazon Q | `preToolUse[fs_write]` | NÃO | **SIM** (JWT em `file_text`/`new_str` por variante) — **CAVEAT: bypass EE4** (JWT + `> /dev/null` em `new_str` → exemption dispara → RC=0; ML-1C deve corrigir) | N/A | INSTALAR — Layer 1 detecta JWT direto; bypass de exemption documentado como gap ML-1C |
| Amazon Q | `preToolUse[fs_read]` | NÃO | NÃO (payload sem conteúdo) | NÃO | NÃO INSTALAR — mesma razão de `pre_read_code` do Windsurf (R2) |

---

## Veredito

**Wave 1 LIBERADA**, com os requisitos de implementação e falsificação abaixo.

### Recomendação por CLI

**Windsurf:**
- INSTALAR `trackfw guard credential` em **`pre_run_command`** (já tem git-branch; adicionar credential no mesmo array). Layer 1 detecta JWTs que aparecem literalmente no payload. Layer 2b é cego para `tool_info.command_line` — lacuna R1; correção programada para ML-1C desta REQ.
- INSTALAR `trackfw guard credential` em **`pre_write_code`** (novo evento para Windsurf; conteúdo escrito aparece no payload). Esse é o evento de maior risco para materialização de credenciais em arquivo.
- NÃO INSTALAR em `pre_read_code` (payload sem conteúdo; instalar seria falsa proteção — residual R2).

**Amazon Q:**
- INSTALAR `trackfw guard credential` em **`preToolUse[execute_bash]`** (já tem git-branch; adicionar credential como segundo hook no mesmo matcher ou como matcher separado). Cobertura de Layer 1 + Layer 2b completa para este evento.
- INSTALAR `trackfw guard credential` em **`preToolUse[fs_write]`** (novo matcher; conteúdo escrito aparece em `tool_input.file_text` para variante `create` e `tool_input.new_str` para variantes `str_replace`/`insert`/`append`). Mesmo mecanismo de Layer 1 confirmado para `pre_write_code` do Windsurf. **Bypass de exemption medido (EE4)**: JWT + `> /dev/null` em `new_str` → RC=0; correção programada em ML-1C. _(Corrigido na auditoria 2026-10-08: campo real é `file_text`/`new_str`, não `content`)_
- NÃO INSTALAR em `preToolUse[fs_read]` (payload sem conteúdo — residual R2).

### Parser de payload (`internal/guard/credential.go`)

O guard atual entende os payloads dos dois CLIs com as limitações declaradas:
- **Layer 1** (`internal/guard/credential.go`, varredura bruta de stdin): funciona para qualquer evento onde o JWT aparece literalmente no JSON — `pre_run_command` (cmd inline em `tool_info.command_line`), `pre_write_code` (conteúdo em `edits[*].new_string`), `preToolUse[execute_bash]` (cmd inline em `tool_input.command`), `preToolUse[fs_write]` (conteúdo em `file_text`/`new_str` por variante). Sem mudança no guard. _(Campo `content` nunca existiu nesses payloads; corrigido na auditoria 2026-10-08)_
- **Layer 2b** (`internal/guard/credential.go:39`, `credCmdLineRe`): funciona para Amazon Q `preToolUse[execute_bash]` (chave `"command"` casa o regex); NÃO funciona para Windsurf `pre_run_command` (chave `"tool_info":{"command_line":"..."}` não casa). Correção via ML-1C desta REQ (ver R1 revisado).

### Requisitos obrigatórios para o ML-1B (Wave 1)

1. Emitir credential guard em: Windsurf `pre_run_command` + `pre_write_code`; Amazon Q `preToolUse[execute_bash]` + `preToolUse[fs_write]`.
2. Adicionar harness targets `windsurf-credential-guard` e `amazonq-credential-guard` em `buildHarnessTargetIDs()`.
3. Remover silenciamento em `validator_credential_guard.go:80–99`; a regra `credential_guard_hook_resolvable` deve acusar violação para `.windsurf/hooks.json` e `.amazonq/cli-agents/q_cli_default.json` sem credential guard.
4. Incluir testes nas duas direções: (a) arquivo SEM credential guard → violation; (b) arquivo COM credential guard correto → sem violation.
5. Incluir teste que DOCUMENTA a lacuna Layer 2b como gap pré-ML-1C (Vector C equivalente: payload Windsurf `pre_run_command` com `tool_info.command_line` apontando para arquivo com JWT → RC=0 — o teste passa quando RC=0 e a frase de reconciliação diz "Layer 2b não detecta JWT em arquivo referenciado via `tool_info.command_line` — a corrigir em ML-1C"). ML-1C adicionará o teste de inversão (RC=2 após correção do `credCmdLineRe`).
6. NÃO instalar credential guard em `pre_read_code` (Windsurf) nem `preToolUse[fs_read]` (Amazon Q).
7. Corrigir comentários obsoletos: `update.go:511` e `cli-parity.md:3971` ("Windsurf has no native hook mechanism and stays out per the ADR" → remover ou atualizar para refletir a realidade).
8. A linha emitida segue D11 revista da ADR-2026-10-04: família PS/POSIX para Windsurf; família cmd.exe para Amazon Q.
9. Incluir teste que DOCUMENTA o bypass de ephemeral exemption (EE4) como gap pré-ML-1C: payload Amazon Q `fs_write` `str_replace` com JWT + `> /dev/null` em `new_str` → RC=0 (teste passa quando RC=0 e frase de reconciliação diz "credIsAllEphemeral não deve aplicar-se a payloads de write — a corrigir em ML-1C").

### Achado de mesma causa em outro sítio

O comentário obsoleto em `cli-parity.md:3971` é o mesmo defeito do comentário em `update.go:511`.
Mesma causa (premissa ADR-2026-08-05 obsoleta não propagada para todos os sítios), mesmo PR.
Não abre REQ separada (Regra Dura de Causa Raiz).

A lacuna Layer 2b (`credCmdLineRe` não lê `tool_info.command_line`) é o mesmo objetivo desta REQ
(credential guard cobrir Windsurf) — mesma meta, mesma REQ. Absorvida como ML-1C. _(Revisado na
auditoria 2026-10-08: de REQ separada para ML-1C desta REQ)_

O bypass de ephemeral exemption (Vector EE4: JWT + `> /dev/null` em `new_str` Amazon Q → RC=0) é
mecanismo do guard que afeta diretamente eventos de escrita cobertos por esta REQ — mesma meta
(credential guard eficaz em fs_write), mesma REQ. Absorvido em ML-1C (Item 4 da especificação).

---

*Hades-tf — 2026-10-08*
