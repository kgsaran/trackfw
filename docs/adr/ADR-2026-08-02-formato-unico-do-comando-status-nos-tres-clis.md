---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Formato unico do comando status nos tres CLIs

> Date: 2026-08-02 | Status: Accepted

## Context

O comando `status` tem **duas implementações completamente diferentes**, não uma com um bloco
faltando. Capturado em 2026-08-02:

**Go e Node** — visão acionável, com moldura:

```
── trackfw status ──────────────────────

🔄 WIP (0)

❌ Blocked (0)

✅ Done (last 5)
   v2.7.0-trackfw-serve-ui-2026-06-14.md

────────────────────────────────────────
```

**Python** — visão de inventário, sem moldura:

```
Governance Status
─────────────────
ADRs:      20
REQs:      66 (0 Open, 66 Closed)
Roadmaps:
  backlog:  0
  wip:      0
  ...
```

Nenhuma das duas é redundante: a de Go/Node responde "o que exige ação agora"; a do Python
responde "qual o tamanho e a distribuição do projeto". São informações complementares que
acabaram em CLIs diferentes.

### Dois defeitos encontrados ao comparar

1. **O Python omite o estado `analyzing`.** `pypi/trackfw/commands/status.py` enumera
   `["backlog", "wip", "blocked", "done", "abandoned"]` — cinco dos **seis** estados — em três
   pontos (linhas ~73, ~81, ~141). Um roadmap em `analyzing/` fica **invisível na contagem**, em
   silêncio.
2. **O Python agrupa REQs em `Open` vs `Closed`**, mas os status reais são `Open`, `Done` e
   `Closed`. O agrupamento apaga a distinção entre REQ **entregue** e REQ **encerrada sem
   entrega**.

### Restrição que NÃO se aplica

Considerei tratar a mudança como *breaking change*. **KG corrigiu:** o trackfw ainda não tem
usuários externos. Não há saída sendo consumida por script de terceiro, não há migração a
proteger. O custo de mudar o formato é **interno** — testes, fixtures e os três CLIs.

Isso remove o principal argumento contra convergir, e favorece corrigir na origem.

### Sobre i18n

Os rótulos do `status` (`WIP`, `Blocked`, `Done (last 5)`) são **hardcoded em inglês**. O bloco
`status` do i18n contém apenas `description`. Existem três locales (`en-US`, `es-ES`, `pt-BR`),
mas a saída do comando não passa por eles.

## Decision

**Um único formato nos três CLIs, somando as duas visões.** A visão de inventário abre a saída; a
acionável vem em seguida, dentro da moldura que Go e Node já usam.

```
── trackfw status ──────────────────────

📊 Inventory
   ADRs        20
   REQs        66  (0 Open · 65 Done · 1 Closed)
   Roadmaps    75
     backlog 0 · analyzing 0 · wip 0
     blocked 0 · done 75 · abandoned 0

🔄 WIP (0)

❌ Blocked (0)

✅ Done (last 5)
   v2.7.0-trackfw-serve-ui-2026-06-14.md

────────────────────────────────────────
```

Decorrências vinculantes:

1. **Os seis estados de roadmap são enumerados**, incluindo `analyzing`. Nenhuma lista de estados
   fica hardcoded parcialmente.
2. **REQs são discriminadas por status real** — `Open`, `Done`, `Closed` — em vez de agrupadas.
3. **A saída é byte-idêntica nos três CLIs**, verificada com fixture compartilhada, e o gate de
   falsificação passa a cobrir isso.
4. **Os rótulos permanecem hardcoded em inglês**, como hoje. `Inventory`, não `Inventário`:
   misturar idiomas com `WIP`/`Blocked`/`Done` seria pior que o estado atual. Passar a saída do
   `status` por i18n é mudança de escopo próprio.
5. A seção `⏳ REQs blocked by not-accepted ADRs`, que Go e Node já exibem condicionalmente,
   **passa a existir também no Python**, com o mesmo texto.

## Consequences

**Positivas**

- Elimina a maior divergência de superfície restante entre os CLIs: hoje `trackfw status` responde
  coisas diferentes conforme o runtime instalado.
- Fecha dois defeitos silenciosos do Python — `analyzing` invisível e a distinção `Done`/`Closed`
  perdida.
- Nenhuma informação é perdida: as duas visões coexistem.

**Negativas / aceitas**

- **A saída do Python muda por completo**, e a de Go/Node ganha um bloco. Todas as fixtures e
  asserções de teste dos três CLIs que dependem do formato precisam ser reescritas. É o custo
  real deste ciclo, e é interno.
- Saída mais longa. Aceito: quem quer só o inventário tem `--json`; quem quer só o board tem o
  dashboard.
- O `status` continua fora do i18n, agora com mais rótulos hardcoded. Dívida registrada, não
  ampliada em natureza.

## Alternatives Considered

**Substituir a saída do Python pela de Go/Node** — era a opção 1 apresentada a KG, e a mais
simples. **Rejeitada por ele:** descartaria a visão de inventário, que é informação útil e não
existe em lugar nenhum além do Python. Convergir preserva as duas.

**Adiar e documentar como divergência aceita** — opção 3 apresentada. **Rejeitada:** é a maior
divergência de superfície do projeto, e "documentar" não a torna menos confusa para quem usa
runtimes diferentes.

**Passar toda a saída do `status` por i18n neste ciclo** — corrigiria também os rótulos
hardcoded. **Rejeitado:** multiplica o escopo por três locales × N rótulos × 3 CLIs, misturando
convergência de formato com internacionalização. Fica como candidato próprio.

**Fazer o Go/Node adotarem o formato do Python** — também produziria unidade. **Rejeitado:** a
visão acionável é a mais usada no dia a dia e a moldura de Go/Node é a mais legível; descartá-la
seria regressão de usabilidade.
