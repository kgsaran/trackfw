# Renomeação de agentes no trackfw — identidade, presets e o que muda de fato

> **Público:** a instância de Claude que opera o harness corporativo.
> **Escopo:** como o trackfw renomeia os agentes que ele instala — a cadeia de identificadores,
> as três estratégias, os 10 presets e a tabela de equivalência.
> **Companheiro:** `docs/portabilidade/2026-09-02-guardrails-de-git-e-governanca-para-harness-de-agente.md`
> — aquele documento diz **o que você pode executar**; este diz **com quem você está falando**.
> **Medido em:** trackfw @ commit `a4adf4e`, 2026-09-02. Toda citação é `arquivo:linha` do repositório.

---

## 1. O fato operacional que vem primeiro: o nome do arquivo NÃO é o identificador

Esta é a única seção que, se ignorada, quebra o harness em silêncio.

Medido em disco, numa instalação com o preset `greek` ativo:

```
$ ls ~/.claude/agents/
trackfw-architect.md      ← nome do ARQUIVO: neutro, nunca muda

$ head -3 ~/.claude/agents/trackfw-architect.md
---
name: zeus-tf             ← identificador de ROTEAMENTO: renomeado
description: Zeus — Principal software architect for system design, ADRs and ...
```

O caminho de destino é montado em `internal/integrations/plan.go:87`:

```go
Destination: strings.ReplaceAll(installPath.Path, "{{id}}", item.ID),
```

`item.ID` é o id do catálogo (`architect`, `backend`, …). **O slug da identidade nunca chega ao
caminho.** Ele só é escrito dentro do arquivo, no campo `name:` do frontmatter
(`internal/integrations/render.go:103`).

**Consequência para o harness:** um harness que enumera `.claude/agents/*.md` e deriva o
identificador de roteamento do *basename* obtém `trackfw-architect`. Esse valor **não roteia para
lugar nenhum** — no Claude Code ele cai silenciosamente no agente genérico `general-purpose`, sem
erro, sem aviso, e o agente executa sem nenhuma das instruções de domínio pretendidas.

> **Regra:** para descobrir o `subagent_type` de um papel, **leia o campo `name:` do frontmatter do
> arquivo instalado**. Nunca derive do nome do arquivo, nunca derive do nome usado na prosa, nunca
> presuma um preset.

---

## 2. A cadeia de identificadores

```
item.ID                    "architect"           id do catálogo — lista FECHADA de 12
   │                                             internal/identity/preset.go:KnownAgentIDs
   ↓  ~/.trackfw/identity.json
slug                       "zeus"                escolhido por preset ou por Slugify
   │
   ↓  identity.AgentName(slug)  =  slug + "-tf"
name: no frontmatter       "zeus-tf"             ← o subagent_type real
   │
   ↓  só no alvo Codex (representação custom-agent-toml)
name = no TOML             "zeus_tf"             render.go:110 — ReplaceAll(name, "-", "_")
```

Três observações que importam:

- **O sufixo `-tf` é aplicado num único lugar** — `internal/identity/identity.go:AgentName`. Não é
  configurável; não existe flag para trocá-lo por `-corp` ou coisa parecida. Não procure.
- **O `Validate` rejeita um slug que já termine em `-tf`** (`identity.go`, última regra), justamente
  para impedir `zeus-tf-tf`.
- **O Codex é a quinta forma do mesmo identificador.** `render.go:110` troca `-` por `_`, então lá o
  agente se chama `zeus_tf`. Se o harness gerar artefatos para Codex, precisa da conversão.

### Os 12 ids são uma lista fechada

`internal/identity/preset.go:KnownAgentIDs()`, ordem estável:

```
architect  backend  frontend  qa  infra  security
dba        ux       code-quality  data  iac  tooling
```

`identity.Validate` recusa qualquer id fora dessa lista. **Não é possível criar um 13º papel pelo
`identity.json`** — identidade renomeia papéis existentes, não cria papéis novos. Um papel novo exige
um asset novo no catálogo (`internal/integrations/assets/agents/*.md`).

---

## 3. As três estratégias de nomeação

O wizard (`internal/commands/identity_wizard.go:34`) oferece três caminhos mutuamente exclusivos.

### 3.1 Preset temático — 10 tabelas fixas

O usuário escolhe um tema e os 12 papéis recebem nome de uma vez.

**As tabelas são HARDCODED, não derivadas.** `preset.go` documenta o porquê, e é uma decisão de ADR:
derivar o slug chamando `Slugify` em runtime faria o resultado depender de a normalização Unicode se
comportar de forma idêntica nos três CLIs (Go, Node.js, Python). Hardcodar remove essa dependência.
É por isso que `Ártemis` → `artemis` e `Aulë` → `aule` estão escritos à mão, não computados.

### 3.2 Custom — um nome por papel

O usuário digita 12 nomes livres; o slug é derivado por `identity.Slugify`
(`internal/identity/slug.go`), em 8 passos nesta ordem exata:

```
1. NFD + remoção de diacríticos      ("Métis" → "Metis")
2. minúsculas
3. espaço e '_' viram '-'
4. descarta tudo fora de [a-z0-9-]
5. colapsa '-' repetidos
6. remove '-' das pontas
7. resultado vazio  → ERRO
8. resultado > 40 caracteres → ERRO
```

> `Slugify` **nunca "conserta" entrada degenerada em silêncio** — ela devolve erro. Um nome que só
> contenha emoji, ou que gere slug de 41 caracteres, aborta o wizard antes de qualquer escrita.

O modo custom **só existe no wizard interativo**. Não há flag para custom em modo não-interativo
(ver §5) — em CI, ou é preset, ou é neutro.

### 3.3 Neutro — não escrever nada

`identity_wizard.go:120`: se a seleção for `""` ou `"neutral"`, o resultado é `identitySkipped` e
**nada é gravado em disco**. Na renderização, `render.go:237` faz curto-circuito:

```go
if !hasIdentity {
    return normalizeMarkdown(source), nil
}
```

A saída sem identidade é **idêntica byte a byte** à de antes de a feature existir — por construção,
não por coincidência. Os agentes ficam `trackfw-architect`, `trackfw-backend`, etc.

---

## 4. Tabela de equivalência — 12 papéis × 10 presets

Gerada mecanicamente a partir de `internal/identity/preset.go` (não transcrita à mão).
Cada célula mostra `display_name` → o `subagent_type` resultante.

| `item.ID` | neutro (`name:`) | greek | norse | potter | thrones | chaves | pioneers | starwars | tolkien | turma | egyptian |
|---|---|---|---|---|---|---|---|---|---|---|---|
| `architect` | `trackfw-architect` | Zeus → `zeus-tf` | Odin → `odin-tf` | Dumbledore → `dumbledore-tf` | Tyrion → `tyrion-tf` | Girafales → `girafales-tf` | Turing → `turing-tf` | Yoda → `yoda-tf` | Gandalf → `gandalf-tf` | Franjinha → `franjinha-tf` | Thoth → `thoth-tf` |
| `backend` | `trackfw-backend` | Apolo → `apolo-tf` | Thor → `thor-tf` | Snape → `snape-tf` | Jon → `jon-tf` | Madruga → `madruga-tf` | Ritchie → `ritchie-tf` | Han → `han-tf` | Aragorn → `aragorn-tf` | Cebolinha → `cebolinha-tf` | Rá → `ra-tf` |
| `frontend` | `trackfw-frontend` | Afrodite → `afrodite-tf` | Freya → `freya-tf` | Luna → `luna-tf` | Sansa → `sansa-tf` | Chiquinha → `chiquinha-tf` | Berners-Lee → `berners-lee-tf` | Leia → `leia-tf` | Arwen → `arwen-tf` | Magali → `magali-tf` | Ísis → `isis-tf` |
| `qa` | `trackfw-qa` | Ártemis → `artemis-tf` | Heimdall → `heimdall-tf` | Moody → `moody-tf` | Arya → `arya-tf` | Florinda → `florinda-tf` | Hamilton → `hamilton-tf` | Ahsoka → `ahsoka-tf` | Legolas → `legolas-tf` | Mônica → `monica-tf` | Hórus → `horus-tf` |
| `infra` | `trackfw-infra` | Ares → `ares-tf` | Tyr → `tyr-tf` | Hagrid → `hagrid-tf` | Brienne → `brienne-tf` | Barriga → `barriga-tf` | Torvalds → `torvalds-tf` | Chewbacca → `chewbacca-tf` | Gimli → `gimli-tf` | Cascão → `cascao-tf` | Ptah → `ptah-tf` |
| `security` | `trackfw-security` | Hades → `hades-tf` | Vidar → `vidar-tf` | Kingsley → `kingsley-tf` | Varys → `varys-tf` | Quico → `quico-tf` | Diffie → `diffie-tf` | Vader → `vader-tf` | Boromir → `boromir-tf` | Bidu → `bidu-tf` | Anúbis → `anubis-tf` |
| `dba` | `trackfw-dba` | Poseidon → `poseidon-tf` | Njord → `njord-tf` | Flitwick → `flitwick-tf` | Samwell → `samwell-tf` | Clotilde → `clotilde-tf` | Codd → `codd-tf` | R2-D2 → `r2-d2-tf` | Elrond → `elrond-tf` | Marocas → `marocas-tf` | Seshat → `seshat-tf` |
| `ux` | `trackfw-ux` | Atena → `atena-tf` | Idun → `idun-tf` | Tonks → `tonks-tf` | Margaery → `margaery-tf` | Popis → `popis-tf` | Norman → `norman-tf` | Padmé → `padme-tf` | Galadriel → `galadriel-tf` | Anjinho → `anjinho-tf` | Bastet → `bastet-tf` |
| `code-quality` | `trackfw-code-quality` | Hefesto → `hefesto-tf` | Bragi → `bragi-tf` | Hermione → `hermione-tf` | Stannis → `stannis-tf` | Nhonho → `nhonho-tf` | Knuth → `knuth-tf` | Obi-Wan → `obi-wan-tf` | Faramir → `faramir-tf` | Titi → `titi-tf` | Maat → `maat-tf` |
| `data` | `trackfw-data` | Métis → `metis-tf` | Mimir → `mimir-tf` | Trelawney → `trelawney-tf` | Bran → `bran-tf` | Godinez → `godinez-tf` | Hopper → `hopper-tf` | C-3PO → `c-3po-tf` | Bilbo → `bilbo-tf` | Chico → `chico-tf` | Osíris → `osiris-tf` |
| `iac` | `trackfw-iac` | Dédalo → `dedalo-tf` | Ivaldi → `ivaldi-tf` | Rowena → `rowena-tf` | Gendry → `gendry-tf` | Chaves → `chaves-tf` | Hashimoto → `hashimoto-tf` | Rey → `rey-tf` | Aulë → `aule-tf` | Piteco → `piteco-tf` | Imhotep → `imhotep-tf` |
| `tooling` | `trackfw-tooling` | Prometeu → `prometeu-tf` | Loki → `loki-tf` | Ollivander → `ollivander-tf` | Qyburn → `qyburn-tf` | Chapolin → `chapolin-tf` | McCarthy → `mccarthy-tf` | Babu Frik → `babu-frik-tf` | Celebrimbor → `celebrimbor-tf` | Nimbus → `nimbus-tf` | Khnum → `khnum-tf` |

### Paridade das tabelas — MEDIDA, não presumida

O trackfw tem três implementações do CLI (Go, Node.js, Python) e uma regra dura de paridade. Para
este documento as três tabelas de preset foram **extraídas em runtime de cada implementação** e
comparadas:

```
internal/identity/preset.go        →  go.json    5462 bytes
npm/src/identity/preset.js         →  node.json  5462 bytes
pypi/trackfw/identity/__init__.py  →  py.json    5462 bytes
                                      diff → idênticas (120 pares display_name/slug cada)
```

**As três tabelas são idênticas.** Você pode ler qualquer uma delas como fonte de verdade.
(Este parágrafo é o resultado de uma medição feita em 2026-09-02, não uma inferência a partir da
regra de paridade — a regra diz que *deveriam* ser iguais; a medição diz que *são*.)

### O que cada papel faz

Descrições neutras, de `internal/integrations/assets/agents/<id>.md`. Use isto para mapear os papéis
do trackfw contra os papéis da sua organização.

| `item.ID` | especialidade |
|---|---|
| `architect` | Arquitetura de sistema, ADRs e coordenação multi-agente governada |
| `backend` | APIs, lógica de domínio, integrações e acesso a dados |
| `frontend` | Interfaces acessíveis, localizadas e performantes |
| `qa` | Testes unitários, de integração, de contrato e ponta a ponta |
| `infra` | Cloud, Kubernetes, GitOps, CI/CD, confiabilidade e FinOps |
| `security` | Fronteiras de confiança, segredos, injeção, permissões, cadeia de suprimentos |
| `dba` | Modelagem, queries, índices, migrações, backup e recuperação |
| `ux` | Pesquisa, design de interação, design systems e acessibilidade |
| `code-quality` | Manutenibilidade, análise estática, refatoração e quality gates |
| `data` | Pipelines, qualidade de dados, analytics e MLOps |
| `iac` | Provisionamento declarativo: Terraform, Pulumi, OpenTofu, Ansible |
| `tooling` | Agentes, skills, MCP, contratos de tool-calling, configuração de assistente |

---

## 5. O que exatamente muda no artefato renderizado

> **Escopo da evidência.** As §5–§9 são lidas do **CLI Go**. A paridade medida na §4 cobre as
> **tabelas de preset** (idênticas nos 3 CLIs), não os pontos de aplicação nem o comportamento das
> flags. Se precisar do comportamento exato em Node.js ou Python, meça — não extrapole da §4.

Quando existe identidade para `item.ID`, `internal/integrations/render.go` altera **quatro** campos.
Todo o resto do arquivo — `model:`, `memory:`, `tools:`, e o corpo das instruções — passa intacto.

| # | Onde | Campo | Transformação |
|---|---|---|---|
| 1 | `render.go:103` | `name:` | `identity.AgentName(slug)` → `"zeus-tf"` |
| 2 | `render.go:104` | `description:` | `DisplayName + " — " + descrição original` |
| 3 | `render.go:105` | corpo | prefixo de saudação inserido como primeira linha |
| 4 | `render.go:237` | assinatura | última linha `^— <nome>, <título>$` tem o **nome** trocado; o título é preservado byte a byte |

A saudação tem duas formas (`render.go:247-249`):

```
sem apelido:  "You are Zeus."
com apelido:  "You are Zeus. Address the user as KG."
```

### Por que o campo 4 existe separado do campo 3

Os agentes terminam com uma linha de assinatura (`— Zeus, Principal Software Architect`). Ela é
reescrita **em separado** porque a seleção de subagente do Claude Code lê **apenas o frontmatter**,
nunca o corpo — então o frontmatter precisa da reescrita para o roteamento funcionar, e o corpo
precisa da reescrita para o agente saber quem ele é. São duas necessidades diferentes atendidas por
dois pontos de código diferentes. `rewriteSignatureLine` **nunca inventa** uma assinatura que já não
estivesse lá; se o padrão não casar, o arquivo volta inalterado.

### Duas rotas de renderização, ambas cobertas

- **Rota A** — `custom-agent-toml` (Codex), `cli-agent-json`, `agent-json`, `agent-directory`
  (Antigravity): trabalham sobre `name`/`description`/`body` já separados do frontmatter.
- **Rota B** — representação `subagent`: Claude, Gemini, Cursor, Copilot, Kiro IDE, Windsurf.
  Devolve o markdown cru e reescreve as linhas `name:`/`description:` **no lugar**.

Detalhe do Antigravity (`agent-directory`): a injeção de `tools:` é decidida **pelo `item.ID`, não
pelo nome renderizado** — de propósito, para que renomear um agente nunca altere as ferramentas que
ele recebe.

---

## 6. Armazenamento

```
$HOME/.trackfw/identity.json     modo 0600, escrita atômica (tmp + rename)
```

```json
{
  "schema_version": 1,
  "user_nickname": "KG",
  "agents": {
    "architect": { "display_name": "Zeus", "slug": "zeus" }
  }
}
```

Propriedades que o harness precisa saber:

- **Global por máquina, nunca por projeto.** Não existe `identity.json` de projeto. A mesma
  identidade vale para todos os repositórios daquele usuário. (Mesma forma da configuração de
  modelos, que também vive só no global `~/.trackfw/`.)
- **Não é versionado.** Não está no repositório, não viaja em clone. Uma máquina nova renderiza
  neutro até alguém rodar o wizard.
- **`user_nickname` é global**, um por máquina — não é por agente.
- **`schema_version` é validado estritamente.** `identity.Load` **falha** se o valor não for `1`; não
  há migração automática.
- **Arquivo ausente não é erro.** `Load` devolve `Config` zerada e `nil` — é o caminho de
  não-regressão e nunca deve aparecer como falha.

### O que `Validate` garante (`internal/identity/identity.go`)

```
id ∈ KnownAgentIDs()          senão: "agente desconhecido"
display_name != ""            senão: "display_name vazio"
slug ~ ^[a-z0-9]+(-[a-z0-9]+)*$
slug único entre os 12        senão: "slug duplicado"
slug não termina em "-tf"     senão: erro com a correção sugerida
```

A validação roda **antes** da tela de confirmação, que por sua vez roda **antes** do `Save`. Uma
colisão de slug aborta antes de qualquer coisa chegar ao disco.

---

## 7. Superfície de comando — e as três armadilhas

**Não existe `trackfw identity`.** O wizard é alcançável por exatamente dois caminhos:

| Comando | Chamada |
|---|---|
| `trackfw init` | `internal/commands/init.go:285` |
| `trackfw agents install` | `internal/commands/integrations_flags.go:208` |

Modo não-interativo (o que interessa a um harness / CI):

```bash
trackfw agents install --targets claude --identity-preset greek
trackfw agents update  --targets claude --identity-preset none
trackfw agents install --targets claude --identity        # força o wizard mesmo já havendo config
```

`--identity-preset` aceita: `none`, `neutral`, ou um dos 10 nomes de preset. **`custom` não é aceito**
— nomes livres só pelo wizard interativo.

### Armadilha 1 — `--identity-preset none` NÃO remove a identidade existente

`resolveIdentityPreset` (`init.go:39-49`) devolve `shouldSave = false` para `none`/`neutral`. Ou
seja: **não grava, mas também não apaga**. Logo abaixo, `integrations_flags.go:222` faz
`identity.Load` do disco — e, se o `identity.json` ainda existir, os agentes continuam saindo como
`zeus-tf`.

```bash
# reverter para nomes neutros de verdade:
rm ~/.trackfw/identity.json
trackfw agents update --targets claude
```

### Armadilha 2 — a identidade só chega aos artefatos por uma re-renderização

Editar `identity.json` na mão não muda nada por si só. O `identity.Load` acontece dentro do fluxo de
`install`/`update`. Depois de qualquer mudança de identidade, rode:

```bash
trackfw agents update --targets <alvo>
```

O comentário em `integrations_flags.go:220-221` registra a armadilha inversa, que já mordeu:
*"Identity must be resolved from disk before BuildPlans — skipping this silently reverts custom
agent names to the neutral defaults."*

### Armadilha 3 — `--identity-preset` não define apelido

Não há flag de apelido. Um preset aplicado por flag deixa `user_nickname` vazio, e a saudação sai
`"You are Zeus."` sem a segunda frase. Para ter apelido em modo não-interativo, escreva o
`identity.json` diretamente (respeitando §6) e rode `agents update`.

---

## 8. Assimetrias que já causaram confusão

- **Skills nunca são renomeadas.** `render.go:93` retorna cedo para `KindSkills`. Só agentes têm
  identidade. Uma skill continua `trackfw-architecture-skill` em qualquer preset.
- **A ordem do menu ≠ a ordem de `PresetNames()`.** O menu do wizard
  (`identity_wizard.go:174-188`) lista `pioneers` em 3º; `presetOrder` (`preset.go`) o coloca em 6º.
  Um harness que leia `PresetNames()` verá uma ordem diferente da que o usuário viu na tela — não
  case índice numérico com escolha do usuário; case pelo **nome** do preset.
- **O nome do arquivo é neutro, o `name:` interno não é** — §1. Vale repetir porque é o erro que
  falha em silêncio.
- **O Codex usa `_`, todos os outros usam `-`** — `zeus_tf` vs `zeus-tf`.

---

## 9. Receita para o harness — descobrir sem presumir

### 9.1 Os caminhos de instalação variam por alvo

`plan.go:87` substitui `{{id}}` num template que vem do catálogo
(`internal/integrations/assets/catalog.json`), e **esse template é diferente para cada alvo e
superfície**. Não presuma `~/.claude/agents/`. Escopo `global`; troque pelo par `project` quando for
instalação de projeto.

| alvo | superfície | destino de agente (escopo `global`) |
|---|---|---|
| claude | cli | `~/.claude/agents/trackfw-{{id}}.md` |
| codex | cli | `~/.codex/agents/trackfw-{{id}}.toml` |
| gemini | cli | `~/.gemini/agents/trackfw-{{id}}.md` |
| antigravity | current | `~/.gemini/config/agents/trackfw-{{id}}/agent.md` |
| antigravity | legacy-cli | `~/.gemini/antigravity-cli/agents/trackfw-{{id}}/agent.json` |
| cursor | ide | `~/.cursor/agents/trackfw-{{id}}.md` |
| copilot | ide | `~/.copilot/agents/trackfw-{{id}}.agent.md` |
| windsurf | ide | `~/.codeium/windsurf/skills/trackfw-agent-{{id}}/SKILL.md` |
| amazonq | cli | `~/.aws/amazonq/cli-agents/trackfw-{{id}}.json` |
| opencode | cli | `~/.config/opencode/agents/trackfw-{{id}}.md` |
| kiro | ide | `~/.kiro/agents/trackfw-{{id}}.md` |
| kiro | cli | `~/.kiro/agents/trackfw-{{id}}.json` |

Três coisas para notar: o **Windsurf** não tem agentes nativos — o `support_level` é `fallback` e os
agentes viram *skills* com prefixo `trackfw-agent-`, num diretório com `SKILL.md`; o **Antigravity**
usa um **diretório** por agente, não um arquivo; e a extensão varia entre `.md`, `.toml`, `.json` e
`.agent.md`. Um glob `*.md` erra em quatro dos doze pares alvo/superfície.

### 9.2 A descoberta correta: pergunte ao próprio trackfw

`trackfw agents list --json` devolve `deployments[].destination` — o caminho **real**, já resolvido
para o alvo, a superfície e o escopo. É a única fonte que não exige adivinhar nada:

```bash
trackfw agents list --targets claude --json > /tmp/tf-agents.json

python3 - <<'PY'
import json, os, re

for d in json.load(open('/tmp/tf-agents.json'))['deployments']:
    path = os.path.expanduser(d['destination'])
    if not os.path.exists(path):
        print(f"{d['item']:<14} -> (nao instalado)")
        continue
    text = open(path, encoding='utf-8').read()
    # markdown/subagent: name: x   |   TOML: name = "x"   |   JSON: "name": "x"
    m = (re.search(r'^name:\s*(\S+)', text, re.M)
         or re.search(r'^name\s*=\s*"([^"]+)"', text, re.M)
         or re.search(r'"name"\s*:\s*"([^"]+)"', text))
    print(f"{d['item']:<14} -> {m.group(1) if m else '(name nao encontrado)'}")
PY
```

Saída numa máquina com o preset `greek` e alvo Claude Code:

```
architect      -> zeus-tf
backend        -> apolo-tf
code-quality   -> hefesto-tf
data           -> metis-tf
dba            -> poseidon-tf
frontend       -> afrodite-tf
iac            -> dedalo-tf
infra          -> ares-tf
qa             -> artemis-tf
security       -> hades-tf
tooling        -> prometeu-tf
ux             -> atena-tf
```

(Saída real, executada em 2026-09-02 — não um exemplo ilustrativo.)

> **`agents list` sozinho NÃO responde a pergunta.** O bloco `items[]` do JSON traz o **id e o rótulo
> do catálogo** (`architect` / `"Architect"`), nunca o nome da identidade. Quem responde é o
> `destination` combinado com a leitura do artefato. Confirmado no `--json` de 2026-09-02.

### 9.3 Existe identidade nesta máquina?

```bash
test -f ~/.trackfw/identity.json && echo "identidade configurada" || echo "nomes neutros"
```

Útil como triagem, mas **não** substitui a §9.2: o `identity.json` pode ter mudado sem que os
artefatos tenham sido re-renderizados (armadilha 2 da §7). O que vale para roteamento é o que está
**no artefato**, não o que está no JSON de configuração.

**A regra final, em uma linha:** o `subagent_type` de um papel é o campo `name` do artefato instalado
daquele papel — leia-o, não o deduza do nome do arquivo, do preset, nem da prosa.

---

## Referências no repositório

| Assunto | Arquivo |
|---|---|
| Decisão de identidade | `docs/adr/ADR-2026-07-25-identidade-personalizavel-de-agentes.md` |
| Decisão do wizard unificado | `docs/adr/ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md` |
| Tabelas de preset (Go) | `internal/identity/preset.go` |
| Tabelas de preset (Node) | `npm/src/identity/preset.js` |
| Tabelas de preset (Python) | `pypi/trackfw/identity/__init__.py` |
| Persistência e validação | `internal/identity/identity.go` |
| Slugify | `internal/identity/slug.go` |
| Aplicação nos artefatos | `internal/integrations/render.go` |
| Wizard | `internal/commands/identity_wizard.go` |
| Flags não-interativas | `internal/commands/integrations_flags.go`, `internal/commands/init.go` |
| Guardrails de git/governança | `docs/portabilidade/2026-09-02-guardrails-de-git-e-governanca-para-harness-de-agente.md` |
