# Modelo de ameaça — Config global de modelo

**Data:** 2026-08-23
**Branch:** `fix/agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global`
**Revisor:** Hades (Security Reviewer)
**Wave:** 0 — bloqueia toda a implementação
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-08-23-agents-update-de-escopo-global-resolve-o-pin-de-modelo-da-config-global.md`
**REQ:** `docs/req/REQ-2026-08-23-agents-update-de-escopo-global-perde-o-pin-de-modelo-porque-le-agent-models-do-cwd.md`
**ADR:** `docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md`

---

## 1. Completude de enumeração

A REQ lista: `config.Load()`, os chamadores que montam `PlanRequest`, `agents models` e os
equivalentes Node/Python. A lista está **incompleta**. O mapeamento completo de superfícies que
lêem `agent_models` do cwd e escrevem ou exibem resultado em escopo global é o seguinte.

### Superfícies confirmadas por leitura de código

#### Go

| Arquivo:linha | Comando | Escopo do artefato | Observação |
|---|---|---|---|
| `internal/config/config.go:125` | — (base) | — | `Load()` singleton: lê `trackfw.yaml` do `os.ReadFile("trackfw.yaml")`, relativo ao cwd, sem fallback. `sync.Once` — resolvido uma vez por processo. |
| `internal/commands/integrations_flags.go:228` | `agents install`, `agents update` | global ou project | `AgentModels: config.Load().AgentModels` na montagem do `PlanRequest`. |
| `internal/commands/integrations_flags.go:339` | `skills install`, `skills update` | global ou project | Idem. |
| `internal/generators/update.go:1723` | `update harness` (via `harnessXxxTarget`) | **global** | `AgentModels: config.Load().AgentModels` dentro de `UpdateHarness`. Função documentada como "nunca requer trackfw.yaml ou cwd de projeto", mas lê `AgentModels` do cwd mesmo assim. |
| `internal/commands/init.go:421` | `init` (via `installAITools`) | global (padrão) | `AgentModels: config.Load().AgentModels`; o scope resolve para `"global"` em `init.go:376` quando não há TTY. |
| `internal/commands/agents_models.go:68` | `agents models` | leitura apenas | `cfg := config.Load()` — lê do cwd e exibe. É a superfície do AC5; também é afetada pelo defeito. |

#### Node.js

| Arquivo:linha | Função | Observação |
|---|---|---|
| `npm/src/config/index.js:load()` | — (base) | `path.join(cwd \|\| process.cwd(), 'trackfw.yaml')` — mesmo padrão do Go. |
| `npm/src/commands/integrations.js:224` | `agents update/install` | `options.agentModels = configModule.load().agentModels \|\| {}` |
| `npm/src/commands/integrations.js:253` | `agents models` | `const agentModels = configModule.load().agentModels \|\| {}` |
| `npm/src/commands/update-harness.js` (`catalogBundleTarget`) | `update harness` | `agentModels: projectConfig.load().agentModels \|\| {}` — mesmo defeito do Go. |

#### Python

| Arquivo:linha | Função | Observação |
|---|---|---|
| `pypi/trackfw/config.py:143–153` | `load(cwd=None)` | `os.path.join(cwd or os.getcwd(), "trackfw.yaml")` — singleton. |
| `pypi/trackfw/integrations/command.py:200` | `agents update/install` | `trackfw_config.load().get("agent_models", {})` |
| `pypi/trackfw/integrations/command.py:331` | `agents models` | `trackfw_config.load().get("agent_models", {})` |
| `pypi/trackfw/commands/update_harness.py:996` | `update harness` | `trackfw_config.load().get("agent_models", {})` — mesmo defeito do Go. |
| `pypi/trackfw/commands/update.py:346` | `update` | `project_config.load(cwd).get("agent_models", {})` — passa `cwd` explícito; é escopo de projeto, não escopo global. Não é afetado pelo defeito desta REQ. |

### O defeito de `update harness` já tem REQ própria

`REQ-2026-08-21-update-harness-le-trackfw-yaml-do-cwd-e-escreve-em-escopo-global.md` está **aberta**
e ataca a mesma classe de defeito. A correção da presente REQ (ler de `~/.trackfw/trackfw.yaml` para
escopo global) resolve também os sites de `update harness` se a implementação estender o resolvedor
global a todas as chamadas com `Scope: "global"`. Recomendação: **absorver os AC1/AC2/AC3 da
REQ-2026-08-21** nesta REQ, redirecionando a REQ mais antiga para o residual de sanitização de
valor (uma linha arbitrária, `"` ou `:` no frontmatter — dano que persiste independente da fonte).
Decisão final pertence ao arquiteto; o resolvedor global deve cobrir todos os sites listados acima,
não só os dois citados na REQ.

### Superfície não afetada

`internal/generators/update.go:1963` (`projectAgentModels = config.Load().AgentModels`, `Scope: "project"`) é
escopo de projeto por definição; ler do cwd é o comportamento correto. Idem `pypi/.../update.py:346`.

### O singleton como restrição de design

`Load()` é `sync.Once` em Go, `if (_instance) return _instance` em Node, `if _instance is not None`
em Python. Em um único processo, `update.go:150` usa `Scope: "project"` e `update.go:1723` usa
`Scope: "global"`, **ambos chamando `config.Load()`**. A resolução por escopo não pode ser uma
mudança em `Load()` — tem que ser um resolvedor separado, chamado antes de montar o `PlanRequest`,
que seleciona o arquivo certo conforme o scope que o chamador já conhece. Uma mudança em `Load()`
produziria order-dependence: o teste que exercita só escopo global num processo isolado passa; o bug
aparece quando escopo de projeto resolve primeiro e o singleton fica preso no cwd do projeto.
`Reset()` é documentado "somente para testes" — usá-lo mid-command para re-ler é anti-padrão que
o ML-1A deve rejeitar explicitamente.

---

## 2. Modelo de ameaça

O adversário aqui é o **ambiente**, não um atacante externo. Para cada cenário: o que está medido
versus o que está raciocínio.

### Cenário A — cwd sem `trackfw.yaml` (o caso que aconteceu)

**Medido:** `cd /tmp && trackfw agents models` → exit 0, tier canônico (alias, não pin). Silêncio.
`config.Load()` (`config.go:126-128`) recebe `os.ReadFile` retornando `ENOENT` → `return` sem erro.
O usuário vê alias onde esperava pin; pode não perceber por dias (como aconteceu).

**Para `agents update --scope global`:** mesmo comportamento — `PlanRequest.AgentModels` é mapa
vazio; `Render()` não compõe modelo, arquivos de agente ficam com alias. A execução bem-sucedida
não é evidência de pin.

### Cenário B — `agent_models` em dois lugares com valores diferentes

**Raciocínio:** hoje isso ocorre: este repositório tem `agent_models` no `trackfw.yaml` de projeto
(`opus: "5"`, `sonnet: "4.6"`); `~/.trackfw/trackfw.yaml` não existe. Rodando de dentro deste repo
o pin aplica; de qualquer outro diretório não aplica. Sem a implementação desta REQ, o usuário que
roda `update harness` fora do repo **reverte o pin** sem aviso — exatamente o que ocorreu em
2026-08-23.

**Após a implementação (precedência a decidir — ver seção 4):** o comportamento depende da regra
escolhida. Ver seção 4.

### Cenário C — `~/.trackfw/trackfw.yaml` malformado

**Medido (proxy):** `trackfw.yaml` malformado no cwd → `trackfw agents models` exit 1,
`MalformedConfigMessage` na stderr. O processo aborta antes de qualquer leitura de catálogo.

**Raciocínio para config global:** se o resolvedor global passar o conteúdo de
`~/.trackfw/trackfw.yaml` pelo mesmo caminho fatal (`MalformedConfigMessage` + `osExit(1)`), um
arquivo global corrompido brickará **todo** comando do trackfw, de todo diretório, incluindo
`update harness` — que é documentado como "nunca requer trackfw.yaml". A mensagem de erro usa o
literal `"trackfw.yaml"` (sem caminho), então o usuário não saberá qual arquivo corrigir.

Os três CLIs têm a mesma constante (`MalformedConfigMessage` / `MALFORMED_CONFIG_MESSAGE`) — é uma
armadilha de paridade: mudar a política de fatal para não-fatal no resolvedor global exige mudar nos
três.

**Recomendação:** para a config global, o resolvedor deve ser **não-fatal**: YAML malformado → aviso
com o caminho absoluto (`~/.trackfw/trackfw.yaml`) na stderr + fallback para canonical tier. Nunca
`osExit(1)` a partir de arquivo global.

### Cenário D — `HOME` não definido ou `~/.trackfw/` sem permissão de leitura

**Medido:** `HOME=/nonexistent trackfw agents models` (cwd sem `trackfw.yaml`) → exit 0, canônico,
silêncio. `os.ReadFile` recebe ENOENT (o home inexistente impede achar o cwd relativo também, mas o
comando não tenta achar `~/.trackfw/`).

**Raciocínio para o resolvedor global:** se o resolvedor usar `os.UserHomeDir()` (Go) /
`os.homedir()` (Node) / `os.path.expanduser("~")` (Python) e `HOME` não estiver definido, Go retorna
erro, Node/Python podem retornar `/` ou `""`. O resolvedor precisa de tratamento explícito:
`HOME` ausente → aviso + canônico, sem `osExit`.

**Medido (terceiro estado de EACCES):** `chmod 000 trackfw.yaml` no cwd → `trackfw agents models`
exit 0, canônico silencioso. A lógica atual (`if err != nil { return }` em `config.go:127`) trata
`EACCES` identicamente a `ENOENT` — silêncio. AC4 declara dois estados ("não configurado" vs
"configurado no lugar errado"); existe um terceiro: **configurado mas ilegível**. Os três são
atualmente indistinguíveis na saída.

### Cenário E — pin apontando para modelo sem acesso

**Raciocínio:** o trackfw não valida acesso ao modelo — apenas escreve o ID no frontmatter. O usuário
descobre na primeira invocação do agente (erro da API do Claude Code, não do trackfw). Esse cenário
não é alterado pela mudança de fonte; a mitigação correta é `agents models` mostrando o ID resolvido,
que é o AC5.

---

## 3. Alvos de falsificação nas duas direções

Para cada alvo: onde a sabotagem entra, o que o gate deve acusar.

### (a) Escopo global voltando a ler do cwd

**Onde entra:**
- Go: `internal/generators/update.go:1723` (e os dois sites de `integrations_flags.go`) usam
  `config.Load().AgentModels` em vez do novo resolvedor global. O `PlanRequest.AgentModels` fica
  vazio quando não há `trackfw.yaml` no cwd.
- Node: `catalogBundleTarget` mantém `projectConfig.load().agentModels`.
- Python: `pypi/trackfw/commands/update_harness.py:996` mantém `trackfw_config.load()`.

**Como detectar:** rodar `agents update --force --scope global --targets claude` de dois cwd distintos
— um sem `trackfw.yaml`, um com `~/.trackfw/trackfw.yaml` configurado — e comparar os 12 arquivos
em `~/.claude/agents/`. Se o resultado diferir entre os dois cwd, o defeito persiste. O gate do
AC8 deve automatizar esse cenário com `HOME` redirecionado.

**Gate esperado (AC8a):** falsa negativa — o teste passa mesmo que o cwd sem `trackfw.yaml` produza
alias em vez de pin. Gate forte: comparar a linha `model:` do agente com o valor em
`~/.trackfw/trackfw.yaml`, não com a ausência de erro.

### (b) Escopo de projeto passando a ler do global

**Onde entra:**
- `internal/generators/update.go:1963` (`Scope: "project"`, `projectAgentModels = config.Load().AgentModels`).
  Se o resolvedor global for chamado aqui por engano, projetos sem `agent_models` no `trackfw.yaml`
  passariam a usar o global silenciosamente.
- `internal/commands/integrations_flags.go:339` quando `--scope project`.

**Como detectar:** criar projeto de teste com `trackfw.yaml` **sem** `agent_models`, criar
`~/.trackfw/trackfw.yaml` com `agent_models`, rodar `agents install --scope project --targets claude`,
verificar que o pin global **não** aparece nos artefatos de projeto.

**Gate esperado (AC8b):** falsa positiva — a linha `model:` no artefato de projeto mostra alias (não
pin), enquanto a do global mostra pin. Se ambos mostram pin, houve vazamento de config global para
projeto.

### Alvo adicional — singleton order-dependence

**Onde entra:** um processo que resolve escopo de projeto primeiro (via `config.Load()`) e depois
tenta resolver escopo global recebe o singleton preso no arquivo do cwd. Qualquer teste de integração
que chama `update` (projeto) e `update harness` (global) no mesmo processo mascarará o bug.

**Gate:** os testes do ML-1A devem usar subprocessos (`exec.Command`) ou `config.Reset()` explícito
entre os dois scopes, e documentar qual é qual.

---

## 4. Residual declarado — e a decisão que falta

### A decisão pendente: precedência quando `agent_models` existe nos dois lugares

O ADR (Decision 2) diz que "`agent_models` é chave de escopo global por natureza" e que manter a
chave no `trackfw.yaml` de projeto e aplicá-la a artefato global é "a inversão que produziu o
defeito". O ADR rejeita busca ascendente com o argumento "o mesmo defeito, mais difícil de prever".

**O argumento do ADR, lido com cuidado, rejeita também o merge (global preenche lacunas do projeto)**
pelos mesmos motivos: o resultado continua dependendo de dois arquivos, e uma chave no projeto
substituindo o global é a inversão exata que o ADR condena.

**Recomendação de Hades: escopo escolhe o arquivo, exclusivamente.**

- Escopo global → lê de `~/.trackfw/trackfw.yaml`. Não lê o `trackfw.yaml` de projeto. Se a chave
  não existir no arquivo global, trata como "não configurado" (AC6) — tier canônico com aviso (AC4).
- Escopo de projeto → lê do `trackfw.yaml` do cwd, como hoje.

**Por que:** é a única regra que elimina order-dependence e produz comportamento previsível sem
conhecer o cwd. Qualquer merge (global-over-project ou project-over-global) reintroduz dependência
do cwd para o resultado global.

**O que esta regra custa — e por que é relevante para KG:**

1. **A correção manual de hoje quebra.** Rodar `agents update --force --scope global --targets claude`
   de dentro deste repositório é **o que produziu o pin correto ontem**. Sob escopo exclusivo, o
   mesmo comando executado do mesmo cwd, após a implementação, ignorará o `trackfw.yaml` do projeto
   e lerá `~/.trackfw/trackfw.yaml` — que não existe. Resultado: os 12 agentes voltam a alias.
   KG terá que criar `~/.trackfw/trackfw.yaml` manualmente antes de qualquer teste da implementação.

2. **Projetos com `agent_models` no `trackfw.yaml` de projeto hoje (como este repositório) deixam de
   influenciar escopo global.** Esse é o comportamento correto segundo o ADR, mas a migração precisa
   ser comunicada: o valor deve ser movido (ou copiado) para `~/.trackfw/trackfw.yaml`.

3. **AC4 fica parcialmente insatisfazível sem ler o arquivo de projeto para diagnóstico.**
   "Configurado no lugar errado" (chave existe no projeto, não existe no global) só pode ser
   detectada se o resolvedor global **leu o projeto** para verificar, mesmo sem usar o valor para
   escrever. O implementador que ler "global lê do global" e parar aí não implementa o aviso de AC4.
   Recomendação explícita: o resolvedor global deve verificar se o `trackfw.yaml` do cwd tem
   `agent_models`, e se sim emitir o aviso do AC4 "chave está no projeto, mas não vale para escopo
   global — mova para `~/.trackfw/trackfw.yaml`". Não usar o valor, apenas advertir.

### Residuais que esta REQ não fecha

- **Sanitização de valor** (`REQ-2026-08-21-update-harness-le-trackfw-yaml-do-cwd-e-escreve-em-escopo-global.md`,
  seção "Residual documentado"): valor com `\n`, `"` ou `:` no frontmatter ainda produz bytes
  arbitrários no arquivo de agente global. A mudança de fonte não elimina esse dano — apenas muda
  quem controla o valor. A REQ-2026-08-21 deve ser rescoped para atacar exclusivamente esse residual
  depois que esta REQ fecha o problema de fonte.

- **Terceiro estado de EACCES:** `trackfw.yaml` ilegível é tratado como ausente (silêncio).
  AC4 declara dois estados; existe um terceiro. Esta REQ não precisa fechar os três se o aviso de
  "ilegível" for considerado escopo futuro — mas deve ser declarado como residual explícito.

- **`~/.trackfw/trackfw.yaml` não existe hoje na máquina de KG.** O caminho feliz após a
  implementação exige criação manual (ou por `trackfw init --scope global`). AC6 ("ausência não
  quebra nada") cobre o comportamento, mas a UX de criação não está no escopo desta REQ.

- **Paridade de mensagem de erro para config global malformada.** Se a política for não-fatal com
  aviso, o texto do aviso precisa ser idêntico nos três CLIs (parity trap do `MalformedConfigMessage`).
  O ML-1A deve definir e testar a constante nos três antes de considerar AC7 satisfeito.
