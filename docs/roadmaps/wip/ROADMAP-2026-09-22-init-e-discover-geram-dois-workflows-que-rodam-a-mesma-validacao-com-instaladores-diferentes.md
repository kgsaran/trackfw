---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md"
squad: ""
---

# Roadmap: `init` e `discover` geram dois workflows que rodam a mesma validação, com instaladores diferentes

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md -->
REQ: docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md

## Acceptance Criteria
- [ ] **AC1** — por que existem dois, **com evidência**; razão legítima → a REQ fecha documentando
- [ ] **AC2/AC7** — um único `trackfw validate` por evento, medido por **check-runs no mesmo SHA**
- [ ] **AC3** — o caminho de adoção que depende do workflow tocado **continua funcionando**
- [ ] **AC4-bis** — o comentário que cita decisão inexistente é corrigido
- [ ] **AC5** — migração de quem **já tem os dois** instalados
- [ ] **AC8** — o `doctor` acompanha na mesma entrega, sem achado falso novo
- [ ] **AC6** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — por que existem dois, e o que quebra se um sair
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — a arqueologia do AC1, e o inventário de quem depende de qual
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Arquivos de leitura:** `internal/generators/scaffold.go` (`generateGitHubActionsWorkflow`:2553) ·
`internal/generators/scaffold_doctor.go` (:32, :59, :333) · `internal/discover/discover.go`
(`InstallGates`:66) · `internal/generators/update.go` (:2189) · `docs/adr/`
**Método:** medir no código e no histórico. 🔴 **Não decidir o remédio** — a Wave 0 entrega a
evidência sobre a qual a decisão é tomada, e tem autoridade para **bloquear** o roadmap.

**Perguntas, em ordem de peso:**

1. 🔴 **Existe caminho de adoção que recebe `trackfw-validate.yml` e NUNCA receberia
   `trackfw-gate.yml`?** É a pergunta que decide tudo. `discover --init` (`InstallGates`) decide por
   `DiscoveryResult.CISystem`; o `gate.yml` sai de `generateGitHubActionsWorkflow`, guiado por
   `cfg.CI` do `trackfw.yaml`. **Se houver projeto que satisfaz um e não o outro, os dois são
   necessários** e o AC1 fecha a REQ documentando a razão.
2. **Os dois arquivos são funcionalmente equivalentes hoje?** Compare o conteúdo **gerado**, não a
   intenção: gatilhos, jobs, steps, e o que entra em `required_status_checks`.
   ⚠️ `governance-go-install` é contrato de required check — se ele só nasce de um dos dois, remover
   esse é quebrar merge protection de quem já depende dele. **Meça**, não presuma.
3. **Instalador:** `go install` exige toolchain Go; o script `curl | sh`, não. Existe adotante sem Go
   atendido **só** pelo `validate.yml`? E o inverso?
4. **O que o `doctor` cobra de cada um**, e o que passa a cobrar errado se um deixar de ser escrito.

**Ataque a premissa que eu trago — ela já esteve errada uma vez:**

🔴 A afirmação *"a coexistência está decidida na `ADR-2026-08-28`"* aparece em **dois** sítios
(`scaffold_doctor.go:333` e a cauda deste roadmap) e é **falsa** — medido: zero ocorrências de
`trackfw-validate.yml` naquela ADR. **Varra o repositório atrás de um terceiro sítio** que repita a
citação, e atrás de **qualquer** ADR que decida a coexistência por outro nome. Se existir decisão
real, é ela que governa, não a minha medição.

**Critérios de aceite:**
- [x] Resposta à pergunta 1 **com o comando e a saída** — é a que decide o desenho
- [x] Tabela de equivalência dos dois workflows **gerados**, campo a campo
- [x] Inventário dos sítios que citam a decisão inexistente (esperado ≥ 2; **achados: 5**)
- [x] Veredito explícito: **os dois são necessários, ou não** — e o que falsificaria a resposta
- [x] 🔴 Se a medição refutar qualquer premissa da REQ, **diga**; Wave 0 que só concorda não mediu
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
trackfw barrier ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes --wave 0 --trust-local-gates
```

## Wave 1 — o remédio
> 🔴 **Dependências: Wave 0 auditada** (ML-0A ✅, barrier `passed`).
> **Decisão do KG em 2026-09-29:** remédio **"não instalar o segundo"**, canônico **`trackfw-gate.yml`**.
> Registrada em `docs/adr/ADR-2026-09-29-o-produto-entrega-um-workflow-de-governanca-por-projeto-...md`
> (D1–D5). **A ADR é o contrato destes MLs** — leia-a antes de agir.

> **Paralelismo:** ML-1A e ML-1C tocam arquivos **disjuntos** → paralelos.
> 🔴 ML-1B toca `scaffold_doctor.go`, que o ML-1A também toca → **sequencial após o 1A**, nunca junto.

### ML-1A — o gerador pergunta antes de escrever, e o `doctor` acompanha
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos:** `internal/generators/scaffold.go` · `internal/generators/scaffold_doctor.go`
(+ testes dos dois)
**Implementa:** **D2** e **D4** da ADR · AC2, AC7, AC8 · sítios **1** e **2** do AC9

**Ações:**
1. `generateGitHubActionsWorkflow` (`scaffold.go:2553`): **não escrever** o `trackfw-gate.yml` quando
   `.github/workflows/trackfw-validate.yml` já existe. Espelhe o cuidado que
   `refreshDiscoverGitHubActionsWorkflowIfPresent` (`update.go:2189`) já tem — **a assimetria é o
   defeito**. Emita a razão ao usuário (não falhe): o arquivo existente é nomeado na mensagem.
2. `doctor` (`scaffold_doctor.go`): com `validate.yml` **presente** e `gate.yml` **ausente**, **não**
   acusar `scaffold-missing` para o `gate.yml`. Hoje ele cobra sempre que `cfg.CI == "github-actions"`.
   🔴 **Sem isto, o passo 1 troca uma duplicação por um achado falso** — é dependência dura, não detalhe.
3. Corrigir os comentários dos **sítios 1 e 2** (`scaffold_doctor.go:29-31` e `:334-335`): a
   `ADR-2026-08-28` **não** decide a coexistência. Aponte para a `ADR-2026-09-29` e o que ela decide.

**Critérios de aceite:**
- [x] `validate.yml` presente → `gate.yml` **não** é escrito, e a razão é dita
- [x] 🔴 **Contra-braço:** `validate.yml` **ausente** → o `gate.yml` **continua** sendo escrito
      (o comportamento de projeto novo não pode ter regredido)
- [x] `validate.yml` presente + `gate.yml` ausente → `doctor` **silencioso** quanto ao `gate.yml`
- [x] 🔴 **Contra-braço do `doctor`:** `gate.yml` ausente **e** `validate.yml` também ausente, com
      `ci: github-actions` → `doctor` **continua acusando** (o achado verdadeiro não pode ter sumido)
- [x] `gate.yml` presente e **defasado** → `doctor` continua acusando divergência de template
- [x] Nenhum comentário do arquivo atribui a coexistência à `ADR-2026-08-28`
- [x] `go build ./...` · `go test ./internal/generators/...` verdes
- [x] 🔴 **Regra Dura de Reconciliação:** por teste novo, **uma frase** dizendo qual conclusão do ML
      aquele teste afirma

### ML-1B — o `doctor` nomeia a duplicação de quem já tem os dois
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos:** `internal/generators/scaffold_doctor.go` (+ teste)
**Implementa:** **D3** da ADR · AC5

**Ações:**
1. Com **os dois** workflows presentes, o `doctor` emite achado de **migração**: nomeia os dois
   arquivos, diz que executam o mesmo `trackfw validate`, e aponta o `gate.yml` como canônico.
2. 🔴 **Não remova, não proponha comando que remova.** O nome do job do arquivo a remover pode ser um
   required check, e o produto **não tem como verificar isso** lendo o repositório. O achado informa;
   a remoção é do consumidor. A ADR declara esse residual — respeite-o.
3. A mensagem deve dizer **o que o consumidor precisa checar antes** de remover: se o job id
   (`governance-go-install` / `governance-install-script`) está no `required_status_checks` dele.

**Critérios de aceite:**
- [x] Os dois presentes → achado de migração, nomeando ambos os arquivos e ambos os job ids
- [x] 🔴 **Contra-braço:** apenas **um** presente → **nenhum** achado de migração
- [x] O achado **não** é `error`/bloqueante e **não** sugere remoção incondicional
- [x] A mensagem menciona a checagem do `required_status_checks` antes de remover
- [x] `go test ./internal/generators/...` verde
- [x] Regra Dura de Reconciliação: uma frase por teste novo

### ML-1C — os 4 sítios restantes da citação falsa
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — **paralelo ao ML-1A** (arquivos disjuntos)
**Arquivos:** `internal/generators/discover_workflow_trigger_test.go` ·
`docs/seguranca/2026-09-28-triagem-issues-abertas.md` ·
`docs/req/REQ-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-...md`
🔴 **NÃO toque em `scaffold.go` nem em `scaffold_doctor.go`** — são do ML-1A, em execução paralela.
**Implementa:** AC4-bis · sítios **3, 4, 5, 6** do AC9

**Ações:**
1. **Sítio 3** — `discover_workflow_trigger_test.go:17`: o comentário afirma *"a coexistência dos DOIS
   arquivos é decidida e está escrita (ADR-2026-08-28)"*. 🔴 Reescreva apontando a `ADR-2026-09-29` e o
   que ela decide de fato. **Verifique se a asserção do teste ainda afirma algo verdadeiro** depois do
   ML-1A — se o teste passou a afirmar o contrário do produto, diga; não o conserte por conta própria.
2. **Sítios 4 e 5** — parecer de 2026-09-28: são registro histórico. **Não reescreva o passado** —
   acrescente nota de retratação datada, apontando esta ADR. Apagar evidência de como o erro
   propagou destrói o valor do registro.
3. **Sítio 6** — `REQ-2026-09-28`, escopo negativo: o parágrafo usa a citação falsa para concluir que
   o #451 é "REQ própria". Acrescente retratação: o #451 **foi absorvido** na `REQ-2026-09-02` em
   2026-09-29, a premissa era falsa, e a conclusão de governança estava errada. **Preserve o texto
   original** e marque-o como retratado.

**Critérios de aceite:**
- [x] `grep -rn 'ADR-2026-08-28'` não devolve nenhum sítio que atribua a **coexistência** a ela,
      exceto dentro de texto explicitamente marcado como retratado
- [x] Nenhum sítio histórico foi apagado — os 3 estão retratados, não reescritos
- [x] Sítio 6 aponta a absorção do #451 na `REQ-2026-09-02`
- [x] 🔴 **Varredura própria:** confirme se existe um **7º** sítio (a Wave 0 disse 5, eu achei o 6º).
      Diga o comando e o total

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa e auditada.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler a `ADR-2026-09-29` e a REQ, derivar o esperado, medir com
fixture própria em diretório temporário.
**Alvos:** os 4 contra-braços do ML-1A/1B (o comportamento de projeto novo e o achado verdadeiro do
`doctor` não podem ter regredido) · o achado de migração não sugere remoção · nenhum required check
declarado deste repositório deixou de ser produzido.
**Critérios de aceite:**
- [ ] Veredito explícito: sobrou caminho pelo qual um projeto novo receba **dois** workflows?
- [ ] Veredito explícito: algum consumidor perde check que já exigia?
- [ ] Se a medição refutar a ADR, **diga**

---

## Correção de 2026-09-27 — a cópia do PRÓPRIO repositório ficou para trás

### Como apareceu

Ao cruzar os 4 PRs mergeados em 2026-09-27 com as issues que eles aparentavam fechar (nenhum trouxe
palavra-chave), a auditoria mediu que o **#456 corrigiu o template embutido e não a cópia versionada
deste repositório**:

```
internal/generators/scaffold_doctor.go   push: branches: [main]     ← corrigido pelo #456
.github/workflows/trackfw-validate.yml   on: [push, pull_request]   ← ficou para trás
```

🔴 **É a mesma forma do defeito que a `REQ-2026-09-23` fechou horas antes** — literal corrigido, cópia
versionada não. Outro par, o mesmo mecanismo.

### Por que NÃO estendi a guarda de paridade aos workflows

A guarda criada na `REQ-2026-09-23` (`TestScripts_LiteralMatchesVersionedCopy`) cobre **5 pares de
scripts** e **nenhum workflow** — medido: `grep -cE 'workflow|\.yml'` no teste dá **0**.

A tentação era estender. Medi a população antes:

```
trackfw-gate.yml       já com o gatilho novo
trackfw-validate.yml   🔴 gatilho antigo   ← ÚNICO divergente
```

**Um par divergente, não uma família.** A guarda de scripts nasceu de um defeito que **já tinha
acontecido duas vezes**; construir a mesma maquinaria para um caso único é maquinaria que não se
paga. Fica **declarado como dívida**: quando aparecer o segundo par de workflow divergente, a guarda
se justifica.

⚠️ **Registro uma recomendação minha que corrigi no meio do caminho.** Eu havia dito ao KG que *"o par
literal↔workflow é mesma causa, e a Regra Dura manda"* — sugerindo reabrir a `REQ-2026-09-23` pela
terceira vez. A Regra Dura manda para **mesma causa com múltiplos sítios**; com um sítio só, o que
ela exige é **corrigir**, não construir guarda. Eu estava aplicando a regra pelo formato, não pelo
conteúdo.

### Verificação

- `python3 scripts/check-required-status-checks.py --scope dw` → `[OK] declared=9, workflow_checks=45, D\W=∅`
  🔴 **Este era o risco real da mudança:** `governance-go-install` é contrato de
  `required_status_checks`. Com o gatilho novo ele continua sendo produzido em `pull_request`, que é
  onde os required checks são avaliados.
- YAML parseado com `yaml.safe_load` — não por inspeção visual
- `make quality` `exit=0`, **1367** `^OK `, **0** `: FALHA`
- `trackfw validate` 170 warnings, 0 violations

### O que continua aberto nesta REQ

A **sugestão 1 da issue #451** — *detectar workflow de governança já existente e não instalar um
segundo, ou avisar que vai substituir* — **não** foi endereçada, nem aqui nem pelo #456. É o que
sobra, e é a parte que protege o consumidor que já está onboardado.

⚠️ E a redução é de **3 para 2** execuções por push em PR, não para 1. A issue pede economia de cota;
2 ainda é duplicata.

## 🔴 RETRATAÇÃO de 2026-09-29 — a frase seguinte estava errada, e travou a correção

O parágrafo acima terminava assim:

> *"A coexistência dos dois arquivos está decidida na `ADR-2026-08-28`, então reduzir para 1 é
> mudança de decisão, não de implementação."*

**Falso.** Medido em 2026-09-29: a `ADR-2026-08-28` tem **zero** ocorrências de
`trackfw-validate.yml` — ela decide que o template de CI nasce pinado na versão que o gerou e que o
`install.sh` honra `TRACKFW_VERSION`. O único ADR que cita o arquivo é a `ADR-2026-09-18`, e lá como
caminho de exemplo. **A coexistência nunca foi decidida por ninguém.**

🔴 **Como o erro entrou, que é o que importa:** a citação nasceu num **comentário de código**
(`scaffold_doctor.go:333`), foi lida como decisão, e copiada para cá. Daqui passou a ser a razão pela
qual a correção não avançava — *"é mudança de decisão"*. Uma citação de ADR que ninguém conferiu
virou governança por três semanas, e quem a derrubou foi um **consumidor externo**
(`lourivalgarciajunior`, no #451), não nós.

**Lição operável:** citação de ADR dentro de comentário de código não é decisão — é afirmação a
verificar. Ao encontrar uma, abra o ADR e confira antes de construir sobre ela.
