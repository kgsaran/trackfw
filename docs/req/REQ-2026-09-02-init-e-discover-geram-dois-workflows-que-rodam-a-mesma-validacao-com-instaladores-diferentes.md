---
status: Done
date: 2026-09-02
author: "zeus-tf"
adr: "docs/adr/ADR-2026-09-29-o-produto-entrega-um-workflow-de-governanca-por-projeto-e-nunca-adiciona-um-segundo-ao-lado-do-existente.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md"
---

# REQ: `init` e `discover` geram dois workflows que rodam a mesma validação, com instaladores diferentes

> Date: 2026-09-02 | Status: Done

## Motivation

Achado ao corrigir a colisão de job id (ML-1A da
`REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw`). Comparando o que cada
workflow gerado **executa**:

| workflow | gerado por | instala com | executa |
|---|---|---|---|
| `trackfw-gate.yml` | `init` / `update` | script de instalação | `trackfw validate` |
| `trackfw-validate.yml` | `discover` | `go install` | `trackfw validate` |

**São a mesma verificação.** A única diferença real é o método de instalação.

Um projeto que roda `init` **e** `discover` — o caminho normal de adoção — recebe **dois workflows
que validam a mesma coisa**. E o `trackfw-validate.yml` dispara em `push` **e** `pull_request`, então
um PR produz **três check-runs** para uma verificação.

## Por que isto importa mais do que "CI redundante"

**1. Custo em todo adotante.** Não é ineficiência deste repositório: é do que entregamos. Cada
projeto que adota o trackfw paga dois jobs por push.

**2. Foi o que escondeu a colisão de nome.** Os três check-runs homônimos que bloquearam o
`required_status_checks` existem **porque há dois workflows**. A colisão era o sintoma; a duplicação
é a causa.

**3. Ambiguidade de qual é o portão.** Com dois workflows equivalentes, qual entra no
`required_status_checks`? Exigir os dois dobra o custo sem dobrar a garantia; exigir um deixa o outro
como ruído verde que ninguém lê.

## A pergunta que a REQ precisa responder antes de corrigir

🔴 **Por que existem dois?** A resposta pode ser legítima e não é óbvia:

- O `discover` serve a repositório **que já existe** e talvez não tenha rodado `init`;
- Os instaladores diferentes podem atender públicos diferentes (`go install` exige Go; o script, não);
- Pode ser sedimentação histórica — dois caminhos que cresceram sem que ninguém comparasse.

**Descobrir isso é a Wave 0.** Unificar sem entender arriscaria remover um caminho que atende um caso
real de adoção.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
       → Wave 0: janela exclusiva do `discover --init` **transiente** (fecha no 1º `update`); instalador não discrimina público (`setup-go@v7` provisiona). **Não há razão legítima** → o AC1 **não** fechou a REQ, como previsto
      de `discover` em repo sem `init`). **Se houver razão legítima, a REQ fecha documentando-a** —
      não force unificação.
- [x] **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
       → D1/D2: canônico `trackfw-gate.yml`; os **dois** sítios de escrita passam a olhar o disco antes
- [x] **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
       → contra-braço medido em fixture `fx-b`: sem `gate.yml`, o `discover --init` **continua** escrevendo o `validate.yml` — brownfield intacto
      funcionando**. Remover CI de quem não rodou `init` seria trocar redundância por lacuna.
- [x] ~~**AC4** — Paridade nos 3 CLIs~~ → 🔴 **OBSOLETO pela v8.0.0**: existe **uma** implementação
       → obsoleto pela v8 (implementação única em Go)
      em Go, entregue por três canais. Não há paridade a manter. Substituído pelo **AC4-bis**.
- [x] **AC4-bis** — 🔴 **O comentário do código que afirma decisão inexistente é corrigido.**
       → 6 sítios: 2 corrigidos (ML-1A), 4 retratados com data (ML-1C), originais preservados; 7º não existe
      `scaffold_doctor.go:333` afirma *"both can coexist in the same project (ADR-2026-08-28)"*;
      a `ADR-2026-08-28` tem **zero** ocorrências de `trackfw-validate.yml` — ela decide pino de
      versão e `TRACKFW_VERSION`. Medido: o único ADR que cita o arquivo é a `ADR-2026-09-18`, e lá
      como caminho de exemplo. **A coexistência nunca foi decidida**; o comentário fabrica a decisão
      e vinha travando a correção por engano.
- [x] **AC5** — Migração para quem **já tem os dois** instalados: o `update` remove o obsoleto, ou o
       → `doctor` emite `scaffold-workflow-duplicated`, **advisory**: nomeia os 2 arquivos e os 2 job ids, e condiciona a remoção à checagem do `required_status_checks` (D3 — o produto não pode verificar isso)
      `doctor` acusa. **Deixar os dois em repositório existente e só corrigir o gerador resolveria
      apenas para projeto novo.**
- [x] **AC6** — `make quality` e **CI** verdes.
       → `make quality` RC=0 (1390 OK, 347 falsificações, 0 FAIL) · **PR #482: 20 checks, 0 falhas**

## 🔴 AMPLIADA em 2026-09-29 — o #451 é esta causa, relatado de fora

Origem: **#451** (consumidor externo) e o comentário de `lourivalgarciajunior` na mesma issue.
Absorvido aqui por **mesma causa, mesmo mecanismo** — não vira REQ nova.

### O que o relato de fora acrescenta à medição

**1. A contagem caiu de 3 para 2, e a causa é nossa.** O PR **#456** trocou o gatilho de lista por
`push: branches:[main]` + `pull_request:` nas duas variantes do template. Medido pelo consumidor, no
**mesmo SHA**:

```
SHA 47d4a6b6 (ponta de branch com PR aberto)
  trackfw-gate.yml       pull_request   1
  trackfw-validate.yml   pull_request   1     → 2 execuções de `trackfw validate`
SHA 08b514d9 (main, pós-merge)
  trackfw-validate.yml   push           1
  trackfw-gate.yml       —              não dispara em push
```

Metade do defeito do #451 fechou sozinha. **O que resta é o núcleo desta REQ:** dois arquivos, mesmo
comando, mesmo veredito, no mesmo evento.

**2. O custo tem nome, e é restrição declarada.** O consumidor é repositório **privado em plano
free**: 2.000 min/mês. Este projeto já descartou job-splitting por medição, com o motivo escrito no
topo do `quality-gates.yml`: *"Cota é a causa-raiz desta família de defeitos."* Entregar duas
execuções do mesmo veredito vai contra a nossa própria decisão — e o custo não é o `validate`, é o
`checkout` + instalação do binário, duas vezes.

**3. A assimetria no gerador, verificada por mim em 2026-09-29:**

| sítio | comportamento |
|---|---|
| `scaffold.go:2553` `generateGitHubActionsWorkflow` | escreve `trackfw-gate.yml` **incondicionalmente** |
| `update.go:2189` `refreshDiscoverGitHubActionsWorkflowIfPresent` | **só atualiza** `trackfw-validate.yml` se já existir; nunca cria |

O caminho cuidadoso já existe no código. 🔴 **A assimetria é o defeito** — um lado pergunta, o outro não.

⚠️ E ela puxa o `doctor`: hoje ele cobra `trackfw-gate.yml` sempre que `ci: github-actions`. Pular a
escrita **sem** mexer no `doctor` troca uma duplicação por um achado falso. É uma dependência dura
entre MLs, não um detalhe.

### AC novo

- [x] **AC7** — 🔴 **Um único `trackfw validate` por evento no repositório do consumidor**, medido
       → medido com binário compilado, não por leitura de template: `gate.yml` presente → `discover --init` produz **1** workflow
      por contagem de check-runs no mesmo SHA — não por leitura do template
- [x] **AC8** — 🔴 **O `doctor` acompanha a decisão na mesma entrega.** Nenhum achado falso novo, e
       → D4 na mesma entrega; contra-braços vivos: ambos ausentes → acusa · `gate.yml` defasado → `scaffold-divergent`
      o contra-braço: o achado **verdadeiro** que ele já dá continua saindo

## Wave 0 (2026-09-29) — duas premissas minhas caíram, e a citação falsa era maior

### 🔴 Premissa refutada: "a única diferença real é o método de instalação"

**Falso.** Os dois workflows gerados divergem no que mais importa — o **job id**, que é o nome do
check:

| | `trackfw-gate.yml` | `trackfw-validate.yml` |
|---|---|---|
| **job id / nome do check** | `governance-install-script` | `governance-go-install` |
| `pull_request` | `branches: [main]` | irrestrito |
| `push` | ausente | `branches: [main]` |
| `timeout-minutes` | 10 | ausente |
| instalação | `curl \| sh` + `TRACKFW_VERSION` | `go install @vX.Y.Z` via `setup-go@v7` |

🔴 **Job id diferente = contrato de `required_status_checks` diferente.** Verificado por mim neste
repositório: **os dois** estão declarados em `.github/required-status-checks.txt` (linhas 29-30).
Num consumidor que configurou `governance-go-install` como required check, **remover o
`trackfw-validate.yml` quebra a branch protection** — o job do `gate.yml` tem outro nome e não
satisfaz o check exigido. Isso não é detalhe de implementação: é a diferença entre corrigir e
derrubar o merge protection de quem já adotou.

### A janela exclusiva existe, mas é transiente

Medido com fixture real: `discover --init` em projeto **sem** `trackfw.yaml` escreve
`trackfw-validate.yml` e **não** o `gate.yml` — mas grava `ci: github-actions`, então o **primeiro
`trackfw update` escreve o `gate.yml`** e fecha a janela. É o mecanismo do #451. Com `trackfw.yaml`
já presente, o `discover --init` faz early-return e não escreve workflow nenhum.

**Portanto o AC1 NÃO fecha a REQ**: não há razão legítima de produto para a coexistência. Há
sedimentação histórica, mascarada por uma citação falsa de ADR.

### O instalador não discrimina público

`actions/setup-go@v7` provisiona o toolchain; não há runner gerenciado do GitHub em que o
`go install` falhe por ausência de Go. O argumento *"os dois instaladores atendem públicos
diferentes"* **não se sustenta no ambiente onde os workflows rodam**.

### A citação falsa: 6 sítios, não 2

| # | sítio | o que afirma |
|---|---|---|
| 1 | `scaffold_doctor.go:29-31` | *"ADR-2026-08-28 names this exact case as the motivation"* |
| 2 | `scaffold_doctor.go:334-335` | *"both can coexist (ADR-2026-08-28)"* — **fonte primária** |
| 3 | `discover_workflow_trigger_test.go:17` | *"a coexistência é decidida e está escrita"* — em **comentário de teste** |
| 4 | `docs/seguranca/2026-09-28-triagem-issues-abertas.md:221` | propagação |
| 5 | idem `:245` | propagação |
| 6 | 🔴 `REQ-2026-09-28-...init-reexecutado...md:118` | **usou a citação para justificar que o #451 era REQ própria** |

🔴 **O sítio 6 é meu, e é o mais caro.** Na REQ do #445, no escopo negativo, escrevi que o #451
*"pede uma decisão de qual artefato instalar, governada pela ADR-2026-08-28"* e concluí **"REQ
própria"** — aplicando a Regra Dura de Causa Raiz com uma premissa falsa. A citação inventada não
só travou a correção: ela **produziu uma decisão de governança errada**, justamente na regra que
existe para impedir trabalho paralelo sobre a mesma causa. Corrigido hoje por absorção nesta REQ,
mas por outro caminho — não porque alguém conferiu a citação.

### AC novo

- [x] **AC9** — 🔴 **Os 6 sítios corrigidos**, incluindo o comentário de teste (sítio 3) e o escopo
       → inclui o sítio 3 (comentário de teste) e o sítio 6 (escopo negativo da `REQ-2026-09-28`, que era meu)
      negativo da `REQ-2026-09-28` (sítio 6). Comentário de teste que afirma decisão inexistente
      viola a Regra Dura de Reconciliação quando o produto mudar.
- [x] **AC10** — 🔴 **Nenhum consumidor perde o check que já exigia.** A mudança preserva, ou migra
       → o CI do PR #482 produziu **os dois** job ids, `check-required-checks` verde — caso real, não fixture
      explicitamente, o nome do job que está em `required_status_checks` — medido, não presumido.

## Negative Scope

- **Não** reverter os job ids únicos do ML-1A. Eles corrigem a ambiguidade **independentemente** de
  haver um ou dois workflows.
- **Não** decidir quais checks entram no `required_status_checks` — é a Wave 2 da REQ irmã.

## Linked ADR

ADR: `docs/adr/ADR-2026-09-29-o-produto-entrega-um-workflow-de-governanca-por-projeto-e-nunca-adiciona-um-segundo-ao-lado-do-existente.md`

> A Wave 0 respondeu: **não** atendem públicos distintos. A janela exclusiva é transiente e o
> argumento do instalador não se sustenta. A decisão foi registrada — pela primeira vez.

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md`
