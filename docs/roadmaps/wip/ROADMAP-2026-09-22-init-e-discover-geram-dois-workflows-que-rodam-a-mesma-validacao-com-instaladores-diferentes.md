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

**Gate da wave:**
```bash
trackfw barrier ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes --wave 0 --trust-local-gates
```

## Wave 1 — o remédio
> 🔴 **Dependências: Wave 0 auditada.** O conteúdo dos MLs depende do veredito do ML-0A e da decisão
> de produto sobre ele — **escrito depois, não antes.** Os três remédios possíveis (não instalar o
> segundo · desduplicar gatilho · cobertura de instalação só no CI do trackfw) levam a produtos
> diferentes, e escolher antes de medir é exatamente o erro que esta REQ já cometeu ao herdar uma
> citação falsa como decisão.

**Status:** ⬜ Pendente — aguardando Wave 0

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
