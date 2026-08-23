# Barreira — Config global de modelo

**Data:** 2026-08-23
**Revisor:** Hades (Security Reviewer)
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-08-23-agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global.md`
**ML:** Wave 3 / ML-3A
**Branch:** `fix/agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global`

---

## REPROVADO

Os 13 sites enumerados na Wave 0 estão corretamente migrados. A barreira encontrou **5 sites
adicionais** (3 Python + 2 Node.js) que a Wave 0 não enumerou e que reproduzem o defeito original:
escopo global lendo `agent_models` do cwd em vez de `~/.trackfw/trackfw.yaml`. Dois deles são
caminhos de escrita (`manager.update`). AC11 ("todos os sites") não está satisfeito.

---

## 1 — Enumeração: os 13 sites declarados estão fechados?

**Contados, não afirmados.**

### Go (6/6 migrados)

| Site | Função | Migrado |
|------|--------|---------|
| `internal/commands/integrations_flags.go:225` | `agents install/update` | `config.ResolveAgentModels(opts.scope, ...)` ✅ |
| `internal/commands/integrations_flags.go:339` | `skills install/update` | `config.ResolveAgentModels(opts.scope, ...)` ✅ |
| `internal/generators/update.go:1719` | `update harness` | `config.ResolveAgentModels("global", ...)` ✅ |
| `internal/commands/init.go:417` | `init` | `config.ResolveAgentModels(scope, home, cwd)` ✅ |
| `internal/commands/agents_models.go:87` | `agents models` | `config.LoadGlobalAgentModels(homeDir, cwd)` ✅ |
| `internal/commands/integrations_thirdparty.go:315` | `skills third-party install` | `config.ResolveAgentModels(opts.scope, ...)` ✅ |

**Nota:** `integrations_thirdparty.go:315` não constava na Wave 0 original (listava 5 sites Go +
base = 6). O ML-1A o migrou. Contagem correta: Go tem 6 call sites de produto, todos migrados.

### Node.js (3/5 migrados — os 4 declarados + 1 extra encontrado pelo gate)

| Site | Função | Migrado |
|------|--------|---------|
| `npm/src/commands/integrations.js:224` | `agents install/update` | `resolveAgentModels(options.scope, ...)` ✅ |
| `npm/src/commands/integrations.js:257` | `agents models` | `loadGlobalAgentModels(os.homedir(), ...)` ✅ |
| `npm/src/commands/update-harness.js:761` | `update harness` | `resolveAgentModels('global', homeRoot, ...)` ✅ |
| `npm/src/generators/init.js:1281` (`installIntegrationTarget`) | `init` | sem `agentModels` em options → `{}`  ❌ |
| `npm/src/commands/thirdparty.js:336–337` | `skills third-party --apply-to` (escrita) | sem `agentModels` em options → `{}` ❌ |

### Python (3/6 migrados — os 3 declarados + 3 extras encontrados)

| Site | Função | Migrado |
|------|--------|---------|
| `pypi/trackfw/integrations/command.py:204` | `agents install/update` (1ª chamada) | `load_global_agent_models(home, cwd)` ✅ |
| `pypi/trackfw/integrations/command.py:351` | `agents models` | `resolve_agent_models(resolved_scope, ...)` ✅ |
| `pypi/trackfw/commands/update_harness.py:997` | `update harness` | `resolve_agent_models("global", home, ...)` ✅ |
| `pypi/trackfw/commands/init.py:160` | `init` | `trackfw_config.load(cwd)` ❌ |
| `pypi/trackfw/commands/thirdparty.py:307` | `skills third-party --apply-to` (inspeção) | `trackfw_config.load()` ❌ |
| `pypi/trackfw/commands/thirdparty.py:458` | `skills third-party --apply-to` (escrita `manager.update`) | `trackfw_config.load()` ❌ |

### Residual aceito: `doctor`

Go `internal/commands/doctor.go:86`, Node.js `npm/src/commands/doctor.js:25`,
Python `pypi/trackfw/integrations/doctor.py:164` — todos leem projeto (cwd/project_root).
Consistentes entre os 3 CLIs e declarados como "leitura de diagnóstico, fora do escopo" na auditoria
do ML-1A. Aceitável: `doctor` compara o instalado contra o esperado no projeto, não usa escopo global.

### Há um 15º site?

Sim — e um 16º, 17º e 18º. A Wave 0 anunciou enumeração completa mas estava incompleta para Python
(`init.py`, `thirdparty.py` ×2) e Node.js (`generators/init.js`, `commands/thirdparty.js`). Esses
sites são equivalentes funcionais de `init.go:417` e `integrations_thirdparty.go:315` — que foram
migrados em Go mas não em Python/Node.

---

## 2 — Os cenários do §2 se comportam como previsto?

Tudo medido com `HOME` redirecionado. Nunca contra o ambiente real de KG.

### Cenário A — cwd sem `agent_models`, global tem (o defeito original)

**Medido:**
```
HOME=$tmp/home   cd $tmp/empty-cwd
$ trackfw agents models
source: ~/.trackfw/trackfw.yaml
architect  opus  claude  claude-opus-5
backend    sonnet claude  claude-sonnet-4-6
EXIT: 0
```
Independente do cwd. ✅ Comporta-se como previsto.

### Cenário B — config nos dois lugares, valores DIFERENTES

**Medido:** cwd com `agent_models: opus: claude-opus-PROJECT`, global com `opus: claude-opus-5`.
```
source: ~/.trackfw/trackfw.yaml
architect  opus  claude  claude-opus-5    ← global vence
EXIT: 0
```
A precedência exclusiva (AC13) funciona: global lê do global, ignora projeto. ✅

### Cenário C — config global malformada

**Medido:** `echo ": not: valid: yaml: [" > ~/.trackfw/trackfw.yaml`
```
source: arquivo global malformado
trackfw: aviso: "~/.trackfw/trackfw.yaml" tem YAML malformado — config global de modelo ignorada; usando tier canônico.
EXIT: 0
```
Não-fatal, com caminho absoluto na mensagem, fallback para canonical. ✅ AC12 satisfeito.

### Cenário D1 — HOME indefinido/vazio

**Medido:**
```
unset HOME && trackfw agents models
Error: resolving home directory: $HOME is not defined
EXIT: 1
```
**Divergência da Wave 0:** o modelo de ameaça recomendou "HOME ausente → aviso + canônico, sem
osExit". A implementação retorna `fmt.Errorf("resolving home directory: %w", err)` que cobra torna
exit 1. Não é `osExit` diretamente — é um erro propagado pelo cobra — mas o efeito é o mesmo.

**Avaliação:** Diverge da recomendação mas não de nenhum AC explícito. HOME nunca é indefinido em
ambiente de produção no macOS/Linux — a shell sempre define. O erro é claro e acionável. Não bloqueia
por si só; declarado como residual nomeado.

### Cenário D2 — `~/.trackfw/` sem permissão de leitura (chmod 000)

**Medido:**
```
chmod 000 ~/.trackfw && trackfw agents models
source: não configurado
trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml — usando tier canônico.
EXIT: 0
```
EACCES tratado como ausente + aviso. ✅ Comporta-se como previsto (Wave 0: "EACCES identicamente a
ENOENT"). Terceiro estado permanece indistinguível do "ausente" — residual aceito e documentado.

---

## 3 — Precedência exclusiva tem efeito colateral não previsto?

**Medido e raciocínio.**

### Escopo de projeto perdeu acesso?

Não. `ResolveAgentModels` despacha pelo scope: `!= "global"` → `Load().AgentModels` (cwd), sem
mudança. Confirmado por medição (parity gate Cases 9–10) e auditoria ML-1A: `AC3 escopo de projeto →
claude-sonnet-4-6 / claude-opus-5 sem regressão`.

### Algum comando global passou a ler o projeto por engano?

Para os 13 sites migrados: não. Para os 5 sites NÃO migrados (achados desta barreira): sim — eles
continuam lendo o cwd e ainda não passaram para `resolve_agent_models(scope, ...)` no Python/Node.

### Singleton order-dependence (AC15)

Os testes usam subprocessos (Gate Cases 6–10 cada em subprocesso próprio, confirmado por código do
gate). A sabotagem provada na auditoria ML-2A (Go `integrations_flags.go:225` revertido para `"project"`)
fez o gate sair EXIT 1 imediatamente. ✅

---

## 4 — As mensagens de aviso são acionáveis e não induzem erro novo?

**Medido.**

### AC4 — "configurado no lugar errado"

```
$ (cd projeto-com-agent_models && HOME=diretorio-sem-global trackfw agents models)
source: trackfw.yaml do projeto (não vale para escopo global)
trackfw: agents global: agent_models configurado em trackfw.yaml do projeto mas
         não vale para escopo global. Mova a chave para ~/.trackfw/trackfw.yaml.
EXIT: 0
```
Diz exatamente o que fazer. Não induz erro: não há caminho errado a seguir. ✅

### AC4 — "não configurado"

```
trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml —
         usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões.
EXIT: 0
```
Instrução clara: onde criar e o que colocar. É a mensagem que teria evitado o incidente original.  ✅

### Comparação com a mensagem anterior que induzia `$PWD/`

A mensagem anterior não dizia nada (silêncio). Não há inversão de instrução aqui — nenhuma das duas
mensagens aponta para um caminho errado. ✅

---

## 5 — Superfície nova de arquivo em `~/.trackfw/` (a pergunta mais importante)

### Permissões e proprietário

`LoadGlobalAgentModels` usa `os.ReadFile(globalPath)` (Go), `fs.readFileSync` (Node),
`open()` (Python) — sem verificação de `stat` (proprietário, modo de escrita). Se um terceiro tiver
acesso de escrita a `~/.trackfw/` (grupo-write ou world-write), pode injetar `agent_models`.

**Avaliação de risco:** `~/.trackfw/` fica dentro do diretório home do usuário. Permissão de escrita
de terceiro no home directory já implica comprometimento total da conta — não é vetor novo introduzido
por esta REQ. O risco permanece, mas é pré-existente e fora do escopo deste vetor.

### Symlink apontando para fora

**Medido:** `ln -sf /etc/passwd ~/.trackfw/trackfw.yaml`. O parser YAML tenta ler `/etc/passwd`,
não encontra `agent_models` no conteúdo e retorna `{}` com `AgentModelsSourceNone`. O aviso de
"não configurado" é emitido, exit 0. Nenhuma linha do `/etc/passwd` vaza para os agentes. ✅

### Conteúdo hostil — valor com newline / caractere de controle

**Medido:** valor com `\n` (YAML double-quoted `"claude-opus-5\ntools:\n  bash: true\n"`) em
`~/.trackfw/trackfw.yaml`. Escrita via `agents update --force --scope global`:
```
Error: model value contains control character and was rejected: model IDs never require
newlines or other control characters (got "claude-opus-5\ntools:\n  bash: true\n")
EXIT: 1
```
O guard `containsControlChar` (Go `render.go`, extendido para U+2028/U+2029 no ML-5C) rejeita no
write path. ✅

### O escape hatch aceita valor literal — o que impede injeção via `:` ou `"`?

**Medido:** `claude-opus-5: danger` em `~/.trackfw/trackfw.yaml` → nenhum WARN de
`looksLikeSuspectModelValue` (o valor começa com `claude-`, passa o filtro de prefixo). O write
path aceita e escreve `model: claude-opus-5: danger` no frontmatter:

```yaml
model: claude-opus-5: danger
```

Este é YAML potencialmente inválido — `claude-opus-5: danger` com `: ` pode ser interpretado como
um mapeamento aninhado por parsers YAML 1.2. **Isto é o residual documentado da
`REQ-2026-08-21-update-harness`** (vault: `rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21`)
— "Um valor com `"` ou `:` pode produzir frontmatter YAML inválido (DoS, não injeção de instrução)".

**O que mudou com esta REQ:** a fonte do valor passou de qualquer-cwd para `~/.trackfw/trackfw.yaml`
— que só pode ser escrito pelo próprio usuário ou por `trackfw init --scope global`. O ataque de CWD
hostil (qualquer diretório contendo `trackfw.yaml` malicioso) foi eliminado para o escopo global. O
residual de colon/quote persiste mas é agora auto-infligido (o usuário controla o arquivo). Isso é
uma melhoria de superfície real.

**O `looksLikeSuspectModelValue` tem um gap para valores iniciando com `claude-`:**
qualquer valor que comece com `claude-` passa sem aviso mesmo que contenha `:`, `"` ou espaços. A
falha silenciosa persiste. Documentado como residual pré-existente, não agravado por esta REQ.

### Resumo da superfície nova

| Vetor | Resultado medido | Veredito |
|-------|------------------|---------|
| Arquivo ausente | Aviso + canonical, exit 0 | ✅ seguro |
| YAML malformado | Aviso + canonical, exit 0, não-fatal | ✅ AC12 |
| Symlink → /etc/passwd | Nenhum dado vaza, exit 0 | ✅ seguro |
| Valor com `\n` (U+000A) | Rejeitado no write path, exit 1 | ✅ blocked |
| Valor com U+2028/2029 | Idem (ML-5C) | ✅ blocked |
| Valor com `:` sem `\n` (ex: `claude-opus-5: danger`) | Escrito; YAML potencialmente inválido | ⚠️ residual documentado |
| Arquivo world-writable | Sem verificação; risco pré-existente | ⚠️ out-of-scope |
| HOME indefinido | exit 1, mensagem clara | ⚠️ diverge da recomendação da Wave 0, sem AC |

---

## Achados que bloqueiam (ordenados por severidade)

### B1 — BLOQUEANTE — Python `init.py:160`: escopo global lê cwd

**Arquivo:** `pypi/trackfw/commands/init.py:160`
**Comportamento observado:** `_am = trackfw_config.load(cwd).get("agent_models", {})` — quando
`resolve_scope(None)` retorna `"global"` (sem TTY), o valor passado para `plan_deployments` vem do
cwd, não de `~/.trackfw/trackfw.yaml`. Equivalente Python de `init.go:421`, que foi migrado.
**Efeito:** `trackfw init` em máquina nova sem `trackfw.yaml` no cwd instala agentes sem pin, mesmo
com `~/.trackfw/trackfw.yaml` configurado. Reproduz o defeito original pela rota de onboarding.
**Correção esperada:** `resolve_agent_models(scope, home, cwd)` — análoga ao Go `init.go:417`.

### B2 — BLOQUEANTE — Python `thirdparty.py:458`: write path sem resolução de escopo

**Arquivo:** `pypi/trackfw/commands/thirdparty.py:458`
**Comportamento observado:** `agent_models=trackfw_config.load().get("agent_models", {})` passado
para `manager.update(agent_plans, force=False)`. Esta é uma **escrita real** de arquivo de agente.
Quando `scope == "global"` e o cwd não tem `trackfw.yaml`, o agente é escrito sem pin.
**Efeito:** `trackfw skills third-party install --scope global --apply-to architect` atualiza o
arquivo do agente global sem o pin configurado em `~/.trackfw/trackfw.yaml`. Reproduz o defeito
original pelo caminho de referência de terceiros.
**Correção esperada:** `resolve_agent_models(scope, home, os.getcwd())` — análoga ao Go
`integrations_thirdparty.go:315`.

### B3 — BLOQUEANTE — Node.js `thirdparty.js:336`: write path sem agentModels

**Arquivo:** `npm/src/commands/thirdparty.js:336–337`
**Comportamento observado:** `buildPlans('agents', { targets: [...], scope, ... })` sem `agentModels`
→ `options.agentModels || {}` → `{}`. `manager.update(agentPlans, {})` escreve o agente sem pin.
**Efeito:** idêntico ao B2 mas no runtime Node.js. Viola paridade com Go.
**Correção esperada:** `resolveAgentModels(scope, os.homedir(), process.cwd())` e passar
`agentModels` em options — análogo ao Go `integrations_thirdparty.go:315`.

### B4 — BLOQUEANTE — Node.js `generators/init.js:1281`: `installIntegrationTarget` sem agentModels

**Arquivo:** `npm/src/generators/init.js:1281–1299`
**Comportamento observado:** `options = { targets: [target], scope, onSkip }` — sem `agentModels`.
`buildPlans` usa `options.agentModels || {}` → `{}`. `execute('agents', 'install', options, roots)`
instala sem pin.
**Efeito:** `trackfw init --scope global` (ou sem TTY) instala agentes sem pin mesmo com config
global. Reproduz o defeito original pela rota de onboarding Node.js.
**Correção esperada:** `resolveAgentModels(scope, os.homedir(), cwd)` antes da chamada, passando
o resultado como `agentModels` em options.

### B5 — BLOQUEANTE — Python `thirdparty.py:307`: inspeção sem resolução de escopo

**Arquivo:** `pypi/trackfw/commands/thirdparty.py:307`
**Comportamento observado:** `agent_models=trackfw_config.load().get("agent_models", {})` passado
para `plan_deployments` na fase de inspeção (linha 313: `manager.inspect(agent_plans[0])`). Se o
`plan_deployments` renderiza o agente esperado com `{}` em vez do pin global, a comparação com o
arquivo instalado pode divergir, causando falso positivo ou falso negativo na validação de `--apply-to`.
**Severidade inferior** a B1–B4 (inspeção, não escrita direta), mas a inconsistência entre o
agente renderizado para inspeção e o agente real (que tem pin) pode recusar o `--apply-to` mesmo
quando correto.
**Correção esperada:** idem B2.

---

## Residuais nomeados (não bloqueantes)

| # | Descrição | Classe |
|---|-----------|--------|
| R1 | HOME indefinido → exit 1 (recomendação da Wave 0: non-fatal) | Divergência de recomendação sem AC; HOME nunca indefinido em produção |
| R2 | EACCES em `~/.trackfw/` indistinguível de ENOENT | Pré-existente; "terceiro estado" do §2 da Wave 0 |
| R3 | Valor com `:` passa `looksLikeSuspectModelValue` se começa com `claude-` | Pré-existente; documentado em `rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21.md` |
| R4 | Doctor (Go/Node/Python) lê projeto — não global | Aceito explicitamente; comportamento correto para diagnóstico de projeto |

---

## Conclusão

Os **13 sites enumerados pela Wave 0 estão corretamente migrados**. A arquitetura escolhida
(exclusividade de escopo, LoadGlobalAgentModels não-fatal, AC14 lendo projeto para diagnóstico sem
usar o valor) é sólida. Os cenários do §2 se comportam como previsto, exceto pelo HOME indefinido
que é divergência de recomendação sem impacto prático.

O bloqueio vem da **enumeração incompleta da Wave 0**. A Wave 0 anunciou "mapeamento completo" com
6+4+3 sites, mas perdeu os equivalentes Python de `init` e `third-party` e os equivalentes Node.js
dos mesmos. Esses sites produzem a mesma classe de defeito que esta REQ se propõe a fechar.

**O que o ML-xA de correção precisa fazer:**
1. `pypi/trackfw/commands/init.py:160` → `resolve_agent_models(scope, home, cwd)`
2. `pypi/trackfw/commands/thirdparty.py:307,458` → `resolve_agent_models(scope, home, os.getcwd())`
3. `npm/src/generators/init.js:installIntegrationTarget` → `resolveAgentModels(scope, os.homedir(), cwd)` + passar em options
4. `npm/src/commands/thirdparty.js:336` → idem
5. Parity gate: Cases 11–12 para init e thirdparty --apply-to, análogos aos Cases 6–10

Estes são de escopo cirúrgico. Não tocam na arquitetura. O padrão de migração já está estabelecido
pelos 13 sites anteriores — é questão de aplicar o mesmo resolvedor nas rotas faltantes.
