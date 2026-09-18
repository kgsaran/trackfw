# Threat Model — Scaffold Residual Chega a Done (#392)

> Autor: Hades (hades-tf) | Data: 2026-09-18 | ML: ML-0A
> Roadmap: ROADMAP-2026-09-18-conclusao-de-microlote-reimplementada-por-consumidor-e-o-scaffold-placeholder-chega-a-done-sem-gate-que-reprove.md
> REQ: docs/req/REQ-2026-09-18-conclusao-de-microlote-reimplementada-por-consumidor-e-o-scaffold-placeholder-chega-a-done-sem-gate-que-reprove.md

---

## Achados para o Arquiteto — Correções à AC3 (ação necessária antes do ML-1A)

**ACHADO 1 — O braço positivo da AC3 está morto. Substitua antes de ML-1A.**

AC3 pina `ROADMAP-2026-09-16-run-capture` como caso que "**tem** de dar verdadeiro" (ML com `⬜`). Medição direta: esse arquivo tem 0 MLs e 0 pendentes. Causa: a "Nota do arquiteto — 2026-09-17" dentro do próprio arquivo documenta que o scaffold foi removido em 2026-09-17 — um dia antes de a REQ ser criada (2026-09-18). O braço positivo foi escrito já morto.

O implementador do ML-1A tem a menor distância entre "assumir que o predicado está errado" e "ajustar o predicado até o arquivo-fixture disparar" — o que produz um predicado calibrado para um corpus já limpo.

**Substituto disponível:** `ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md` — medido com 2 MLs, ambos `⬜ Pendente`, em `done/`. É um dos 6 arquivos que ML-4A vai corrigir.

**Restrição de ordenação:** o braço positivo substituto deve ser pinado na AC3 **antes de ML-4A rodar** — porque ML-4A limpa exatamente esse arquivo. Se ML-4A rodar primeiro, o braço substituto morre da mesma forma que o original.

---

**ACHADO 2 — AC3's "exatamente 27" contradiz o predicado da própria AC3. Escolha binária necessária.**

AC3 especifica o predicado como `roadmapdoc.ML não concluído` — que é `statusIsComplete`, que é o que este instrumento implementa, e que retorna **32**, não 27.

O 27 é a contagem do arquiteto que usou exclusivamente `⬜`/`🔄`. A diferença (5 roadmaps) são casos com status fora do vocabulário: `ABANDONADO`, `❌ Cancelado`, `❌ Bloqueado`, `🚫 Abandonado`, `pending`. Um deles (`ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-...`, ML-3H com `pending`) é exatamente o bug que AC5 existe para fechar.

**O caminho mais barato para o ML-1A passar na AC3 como escrita (27):** estreitar o predicado para detectar apenas `⬜`/`🔄` — o que silenciosamente exclui `pending` da detecção. Esse é o mesmo padrão do #387 que a ADR cita: o gate fica verde excluindo o caso que existe para pegar.

**Escolha binária para o arquiteto:**
- **Opção A:** AC3's número vira 32. Os 5 casos com status fora do vocabulário entram em escopo. R7 (deve `❌ Bloqueado` bloquear o move para `done`?) precisa de decisão de política explícita antes do ML-1A.
- **Opção B:** O predicado é estreitado para `⬜`/`🔄` (não `statusIsComplete`). AC3's número fica 27. `pending` precisa de braço separado na AC3 ou em AC5. Confirmar que `ROADMAP-2026-08-29` (ML-3H, `pending`) dispara o predicado correto.

Ambas as opções são válidas. A que não é válida é entregar o ML-1A com o predicate implicitamente estreitado sem nomear a escolha.

---

## 1. Completude da Enumeração

### Método

Varredura própria com os seguintes comandos, executados sobre `internal/`, `scripts/`, `.github/`, `.claude/`:

```bash
grep -rn --include="*.go" \
  "statusIsComplete\|parseMLProgress\|contentHasMarker\|\*\*Status:\*\*\|statusVocabulary" \
  internal/ | grep -v "_test.go"

grep -rn --include="*.go" \
  "statusIsComplete\|parseMLs\|fenceMask\|parseWaves\|parseGates\|mlStatusMarker" \
  internal/ | grep -v "_test.go" | grep -v "barrier\.go"

grep -rn \
  "statusIsComplete\|parseML\|parseWave\|parseGate\|fenceMask\|mlStatusMarker" \
  --include="*.js" --include="*.py" --include="*.ts" . 2>/dev/null
```

### Resultado — lista fechada com evidência

**Sites que RESPONDEM "este ML está concluído?"**

| # | Sítio | Localização | Implementação | Qualidade |
|---|---|---|---|---|
| 1 | `statusIsComplete` | `internal/commands/barrier.go:300` | primeiro token, vocabulário fechado (`✅`, `done`, `concluido`), fence-aware, stripVS16, diacriticsFolder | correto |
| 2 | `parseMLProgress` | `internal/serve/api_board.go:137-168` | `strings.Contains(trimmed, "✅")` — sem máscara de cerca, sem validação de primeiro token | bug: conta `✅` em qualquer posição, inclusive dentro de cerca de código |
| 3 | `contentHasMarker` | `internal/validator/validator.go:2312` | verifica presença do heading de aceite (`wip_acceptance`) — não lê status de ML individual | não responde a pergunta |

Resultado dos greps: **nenhum outro consumidor encontrado** em `internal/`, `scripts/`, `.github/`, `.claude/`, `.js`, `.py`, `.ts`.

**Sites que ESCREVEM o campo `**Status:**`**

| # | Sítio | Localização | Valor escrito | Correto? |
|---|---|---|---|---|
| A | `wave0Block` (constante Go) | `internal/generators/roadmap.go:73` | `**Status:** ⬜ Pendente` | correto |
| B | ML generation loop | `internal/generators/roadmap.go:266` | `**Status:** ⬜ Pendente` | correto |
| C | ML from-req loop | `internal/generators/roadmap.go:336` | `**Status:** ⬜ Pendente` | correto |
| D | Slash command `/trackfw:roadmap` (ML-0A) | `internal/generators/scaffold.go:354` | `**Status:** pending` | BUG — `pending` não está no `statusVocabulary` |
| E | Slash command `/trackfw:roadmap` (ML-1A+) | `internal/generators/scaffold.go:376` | `**Status:** ⬜ Pendente` | correto |

**Sub-especificação descoberta:** o roadmap enumera `scaffold.go:350-384` como um único sítio, mas ele tem dois comportamentos distintos. ML-0A (linha 354) escreve `pending`; ML-1A+ (linha 376) escreve `⬜ Pendente`. O `roadmap new` (via `wave0Block` em roadmap.go:73) já escreve `⬜ Pendente` para ML-0A — a divergência existe apenas no caminho do slash command, não no `roadmap new`.

**Nota sobre `scripts/windows-repro/run.ps1:396`:** Contém `**Status:** ✅` dentro de um heredoc PowerShell que gera *fixtures de teste* para o barrier — não é um consumidor de decisão de status. Verificado: é dado de entrada para o barrier, não lógica de decisão.

**Nota sobre v8:** A v8 tem implementação única em Go. Grep por `statusIsComplete`, `parseMLs`, etc. em `.js` e `.py` retornou vazio. Node.js e Python foram removidos.

### Veredicto

A lista de **quatro sítios** do roadmap está **fechada**. Nenhum quinto sítio foi encontrado.

---

## Medições Próprias do Corpus

### Metodologia

Script Python fence-aware rodado em `docs/roadmaps/done/` (192 arquivos). O script implementa `fenceMask` equivalente ao `barrier.go:350-388` e `statusIsComplete` equivalente ao `barrier.go:300-309`. Predicate: ML cujo primeiro token do valor de `**Status:**` não está em `{"✅", "done", "concluido"}` (após fold de diacríticos).

Comando:

```bash
python3 -c '
import os, re, unicodedata
# [script completo no scratchpad de sessão]
# Conta roadmaps em done/ com pelo menos um ML não-completo (fence-aware, first-token)
'
```

### Resultados

| Medida | Valor (este instrumento) | Valor (arquiteto, 2026-09-18) | Diferença |
|---|---|---|---|
| Roadmaps em `done/` | 192 | 192 | zero |
| Com ML `⬜`/`🔄` (fence-aware, arquiteto) | — | **27** | — |
| Com ML não-completo (fence-aware, primeiro token, este script) | **32** | — | +5 |
| Com `exit 1  # placeholder gate` (grep literal) | **4** | 4 | zero |

**Discrepância 32 vs 27: explicada e reconciliada.**

O arquiteto mediu especificamente `⬜` e `🔄` (estados explicitamente "não-done"). Meu predicado é mais amplo: qualquer status cujo primeiro token não está no vocabulário de conclusão. Os 5 extras são roadmaps com status fora do vocabulário mas também fora dos emojis de "pendente":

| Roadmap | Status encontrado |
|---|---|
| `ROADMAP-2026-07-25-identidade-humanizada-agentes.md` | `ABANDONADO — abordagem revertida...` |
| `ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-...` | `❌ Cancelado` (dois MLs) |
| `ROADMAP-2026-08-21-release-tag-ancora-versao-...` | `❌ Bloqueado — veredito BLOQUEAR...` |
| `ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-...` | `pending` (ML-3H) — este é sítio do bug AC5 |
| `ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-...` | `🚫 **Abandonado**` |

A diferença é de predicado, não de instrumento quebrado. O predicado do arquiteto (explicitamente `⬜`/`🔄`) é mais conservador — ele não cobra roadmaps com MLs explicitamente cancelados/abandonados/bloqueados. Meu predicado (`statusIsComplete` como vai ser implementado) reprovaria AC6 para todos os 32 — uma questão de design que precisa ser decidida antes da implementação do AC6.

**Implicação de design para AC6:** Se o gate recusar qualquer ML cujo status não está no vocabulário de conclusão (que é o que `statusIsComplete` faz), os 5 roadmaps extras seriam retroativamente inválidos se tentassem mover para `done/` via CLI. Como eles já estão em `done/`, não são afetados. Mas para roadmaps futuros: um ML marcado `❌ Bloqueado` ou `🚫 Abandonado` bloquearia o move para `done/`. Pode ser intencional ou um falso positivo dependente de política.

### Counter-arm

| Roadmap | Esperado | Resultado | Veredicto |
|---|---|---|---|
| `ROADMAP-2026-09-17-leniencia-sem-prazo-...` (#387, legitimamente concluído) | FALSE (todos completos) | 7 MLs, 7 completos, 0 pendentes | PASS — instrumento correto na direção de não-falso-positivo |
| `ROADMAP-2026-09-16-run-capture-...` (ocorrência #2 original) | TRUE (tinha pendentes) | 0 MLs, 0 pendentes | Arquivo JÁ FOI LIMPO pelo arquiteto em 2026-09-17 (nota no arquivo). O counter-arm de AC3 foi escrito antes da limpeza. |

**Conclusão do counter-arm:** o instrumento está correto. O arquivo run-capture foi limpo antes de 2026-09-18 (ver "Nota do arquiteto — 2026-09-17" dentro do arquivo). A discrepância 32 vs 27 é de definição de predicate, não de instrumento quebrado.

---

## 2. Modelo de Ameaça dos Gates Propostos

Adversário: o implementador apressado e o arquiteto otimista.

### Cenários de contorno do AC6 (`roadmap move done` recusa ML pendente)

**C1 — `mv` / `git mv` direto (custo: zero)**
`MoveRoadmap` é invocado pelo CLI. `mv docs/roadmaps/wip/X.md docs/roadmaps/done/X.md` bypassa AC6. Mitigação: AC7 (regra do validator) vale em qualquer estado — fecha este desvio se a implementação estiver correta.

**C2 — Editar o arquivo depois de mover (custo: zero, pós-transição)**
Mover legitimamente (todos os MLs ✅) e depois editar um ML de volta para ⬜. AC6 já passou. A REQ exclui explicitamente regra de `validate` sobre `done/` (Negative scope). Residual declarado.

**C3 — `roadmap move X abandoned` → `roadmap move X done` (custo: dois comandos)**
`MoveRoadmap` não tem máquina de estado — valida apenas que o estado de destino é nome válido. AC6 proposto verifica o conteúdo quando o destino é `done`, então este desvio é fechado se AC6 é baseado em conteúdo (independente do estado de origem).

**C4 — Status com marcador de conclusão + texto que o nega (custo: zero)**
`**Status:** ✅ Ainda não concluído` — primeiro token `✅` → `statusIsComplete` retorna true → AC6 não recusa. Design intencional (primeiro token é autoritativo). Residual aceito, documentado em barrier.go:290.

**C5 — Status indentado (custo: zero, acidental)**
`   **Status:** ⬜ Pendente` — `statusLineRe` é ancorado em `^`. A linha indentada não casa. ML sem status reconhecido → AC6 trata como não-concluído → recusa o move. Direção: **falso positivo** conservador.

**C6 — Manipulação de cercas de código**
`**Status:** ✅` dentro de cerca de código: mascarado pelo `fenceMask`. Não afeta AC6 (que usa `roadmapdoc` com fence-awareness). Afeta `serve` (bug AC4, fechado separadamente).

**C7 — Rótulo duplicado Wave/ML**
Dois `## Wave 0`. `barrier.go:877-882` faz `break` no primeiro. Se AC6 usa `parseMLs` sobre o documento inteiro (não wave-scoped), os MLs da segunda cópia são capturados. AC8 fecha este caso independentemente — o rótulo duplicado é uma violação nomeada antes da transição.

**C8 — Scaffold ML-0A com `pending` bloqueando AC6 (falso positivo estrutural)**
Roadmap gerado via `/trackfw:roadmap`: ML-0A tem `**Status:** pending`. Quando concluído e atualizado para `✅ Concluído`, AC6 passa. Se não atualizado (ou valor fora do vocabulário), AC6 recusa — verdadeiro para um ML genuinamente pendente, mas bloqueia totalmente roadmaps slash-command antes de AC5 ser implementado. Esta é a motivação direta do sequenciamento AC5 antes de AC6.

### Resumo de ameaças por AC

| AC | Ameaça principal | Contorno mais barato | Fechado por |
|---|---|---|---|
| AC6 | `mv` direto | 0 custo | AC7 (any-state) |
| AC6 | Edição pós-transição | 0 custo | Residual declarado |
| AC6 | Scaffold `pending` bloqueia move legítimo | N/A — falso positivo | AC5 (scaffold converge antes) |
| AC7 | Apagar bloco de gates inteiro | 1 linha | Discriminante de cobertura |
| AC7 | Renomear ou apagar o heading Wave 0 | 1 linha | **Ponto cego — ver seção abaixo** |
| AC7 | Substituir por `true`/`exit 0` | 1 linha | Residual declarado (Tier 2) |
| AC8 | Label duplicado silencioso | 0 custo | AC8 (nova regra) |

---

## 3. Falsificação nas Duas Direções, por Superfície

### AC6 — `roadmap move ... done` recusa ML pendente

**Direção 1 (falso negativo):**
- `**Status:** ✅ Mas não fiz` → primeiro token `✅` → AC6 permite move. Design intencional.
- Rótulo duplicado Wave/ML — se AC6 usa lógica de primeiro-casamento (como barrier), segunda cópia invisível.

**Direção 2 (falso positivo — o risco nomeado pela ADR-2026-08-17):**
- ML-0A gerado via slash command com `**Status:** pending` — AC6 recusa mesmo quando o ML está genuinamente concluído, pois `pending` não está no vocabulário. Se o guard gera falsos positivos estruturalmente, o usuário o desliga. Mitiga com AC5.
- ML com status `❌ Bloqueado` ou `🚫 Abandonado` — AC6 também recusa (primeiro token não no vocabulário). Pode ser falso positivo dependendo de política: um ML cancelado é "concluído" ou é "pendente"? Esta questão não está resolvida na ADR atual. Medição: 5 roadmaps existentes em `done/` têm este padrão.
- ML com status indentado por erro → não casado pelo regex ancorado em `^` → AC6 recusa. Ruidoso mas conservador.

**Contra-braço obrigatório (exigido pela AC6):** roadmap com todos os MLs `✅ Concluído` → `roadmap move X done` deve passar sem recusa. Testado no AC3 contra o roadmap do #387: 7 MLs, todos completos.

### AC7 — Gate por perda de cobertura (não por presença de placeholder)

**Ponto cego crítico encontrado: renomear ou apagar o heading Wave 0**

`parseWaves` (barrier.go:408-441) usa `waveHeadingRe = ^## Wave (\S+) ` seguido de validação por `waveLabelRe = ^\d+(?:-[a-z0-9]+)?$`. Se o heading `## Wave 0 — Threat Model` for:
- Renomeado para `## Threat Model` → `waveHeadingRe` não casa → Wave 0 não existe para o parser.
- Renomeado para `## Wave zero — Threat Model` → `zero` falha `waveLabelRe` → `parseWaves` retorna `barrierUsageError` (aborta com `usageExit`) em vez de lista válida. Comportamento diferente de "não encontrado".
- Apagado completamente → Wave 0 não existe.

**Consequência:** um discriminante "Wave 0 deve ter gate" falha vacuosamente se não há Wave 0 no documento. O checador precisa de uma segunda asserção: "o documento deve conter Wave 0 quando em `wip/`." Custo do bypass: 1 linha (renomear o heading). É o mesmo custo de apagar o bloco de gates — Tier 1.

**Direção 1 (falso negativo):**
- Discriminante ingênuo "placeholder presente": apagar o bloco de gates (1 linha) → parseGates retorna vazio → check passa. Corrigido pelo discriminante de cobertura.
- Renomear ou apagar heading Wave 0 (1 linha) → Wave 0 desaparece → discriminante não tem objeto → falha vacuosa. **Não corrigido pelo discriminante de cobertura** sem asserção adicional.
- Tier 2 (substituir por `true`/`exit 0`) → gate "tem conteúdo real" → não reprova. Residual declarado.

**Direção 2 (falso positivo — ADR-2026-08-17):**
- Roadmap em `backlog/` com placeholder `exit 1` → AC8-bis exige que NÃO reprove. Seis arquivos de `backlog/` confirmados com placeholder legítimo.
- Waves de implementação (Wave 1, 2, etc.) sem gate → legítimo. Discriminante deve ser scoped a Wave 0 exclusivamente.
- Roadmap com Wave 0 legítima cujo gate real usa `exit 1` como nome de variável ou parte de lógica → seria falso positivo. Improvável mas possível. A detecção do placeholder deve usar o literal exato: `exit 1  # placeholder gate fails closed`.

### AC8 — Rótulo duplicado falha fechada

**Direção 1 (falso negativo):**
- `barrier.go:877-882` faz `break` no primeiro rótulo → segunda `## Wave 0` invisível.
- Fixture de falsificação: `ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal` com scaffold duplicado (linhas 13-60 e 78-103), confirmado pelo arquiteto. Este roadmap tem ML-0A e ML-1A com `⬜ Pendente` na segunda cópia.

**Direção 2 (falso positivo):**
- Prosa que menciona "Wave 0" não casa `waveHeadingRe` — o regex exige `^## Wave <token> ` com espaço final.
- Roadmap em `backlog/` com rótulo duplicado → AC8-bis: não cobra em `backlog`. Seis arquivos de `backlog/` verificados: zero rótulos duplicados (medição do arquiteto: zero em `backlog/`, `analyzing/`, `abandoned/`).

---

## 4. Curva de Custo do Atacante para AC7

Ponto de partida: `parseGates` (barrier.go:592) trata wave **sem** bloco `**Gates da wave:**` como zero gates — legal, check passa. Logo o discriminante "placeholder presente" é derrotado apagando o bloco.

| Tier | Custo | Ação | Discriminante "placeholder presente" | Discriminante "perda de cobertura (Wave 0)" |
|---|---|---|---|---|
| 0 | Zero | Deixar `exit 1` intacto | Reprova | Reprova |
| 1a | 1 linha removida | Apagar o bloco `**Gates da wave:**` inteiro | **Passa** — `parseGates` retorna vazio | Reprova — Wave 0 sem gate em `wip` |
| 1b | 1 linha renomeada | Renomear/apagar o heading `## Wave 0 — Threat Model` | **Passa** — Wave 0 não existe para o parser | **Passa** — Wave 0 não existe para verificar |
| 2 | 1 linha substituída | Substituir `exit 1` por `true`, `exit 0`, `: `, `echo ok` | Reprova (placeholder textual) | **Passa** — gate tem "conteúdo real" |
| 3 | Elaboração | Gate com teste irrelevante: `test -f /etc/hosts`, `ls /tmp` | Reprova (placeholder) | **Passa** |

**Achado novo em relação ao roadmap:** o Tier 1b (renomear/apagar o heading Wave 0) tem o mesmo custo que o Tier 1a mas NÃO é fechado pelo discriminante de cobertura proposto. É uma lacuna na especificação do AC7. A mitigação é adicionar uma segunda asserção: "documento em `wip/` deve ter Wave 0" — mas essa asserção é potencialmente destrutiva para roadmaps legítimos que não usam a convenção Wave 0. Esta questão precisa de decisão explícita antes da implementação do ML-3B.

**Implicação:** O discriminante de cobertura fecha os Tiers 0 e 1a. Os Tiers 1b e 2+ são residuais. O Tier 1b é um achado novo que NÃO estava mapeado nos AC do roadmap.

---

## 5. Residual Declarado

O desenho desta REQ aceita explicitamente não cobrir os seguintes casos:

**R1 — Roadmaps já em `done/` com ML pendente (27-32 de 192)**
AC6 é prospectivo: só cobre transições futuras. Os roadmaps históricos não são tocados. Justificado pela decisão 7 da ADR: sanar 27 (ou 32) arquivos retroativamente é risco desproporcional, e o mecanismo de carve-out necessário é a superfície de ataque que o #387 acabou de fechar.

**R2 — Gate semanticamente vazio (Tier 2+ da curva de custo)**
`true`, `exit 0`, `echo ok` — qualquer comando que sempre passa sem verificar nada. O gate estrutural não distingue comandos triviais de verificações reais. Detectar isso exigiria análise semântica de shell.

**R3 — Edição pós-transição (C2 do modelo de ameaça)**
ML editado de volta para ⬜ depois de chegar a `done/`. Não detectado por nenhuma regra desta REQ.

**R4 — Status com marcador de conclusão + texto contraditório**
`**Status:** ✅ Mas na verdade não está feito` passa todos os gates. Design intencional.

**R5 — Waves de implementação sem gate (Wave 1, 2, etc.)**
O discriminante AC7 é scoped a Wave 0 — única wave que recebe gate do template. Waves de implementação sem gate são legítimas.

**R6 — Tier 1b: renomear/apagar o heading Wave 0**
Renomear `## Wave 0 — Threat Model` para qualquer outro heading faz a Wave 0 desaparecer do parser. O discriminante de cobertura falha vacuosamente. Custo: 1 linha. Este residual é NOVO — não estava mapeado no roadmap original. A decisão de fechar ou aceitar este caso precisa ser tomada antes do ML-3B.

**R7 — MLs com status `❌ Bloqueado` ou `🚫 Abandonado`**
`statusIsComplete` retorna false para esses tokens. AC6 recusaria um roadmap com ML bloqueado/abandonado. Não está claro se isso é correto (um ML bloqueado bloqueia o done?) ou falso positivo (ML bloqueado é resolvido de outra forma). Cinco roadmaps existentes em `done/` têm este padrão — impacto retroativo zero, mas impacto prospectivo não definido.

**R8 — Zero-width characters no token de status**
U+200B antes de ✅ fica grudado ao token e causa rejeição (barrier.go:290). Falso negativo de usabilidade, não vetor de segurança.

---

*Documento produzido por Hades (hades-tf) como entregável do ML-0A. Nenhuma linha de implementação foi escrita. As medições são resultado de grep e script Python fence-aware executados no corpus real de 192 roadmaps em `done/`.*
