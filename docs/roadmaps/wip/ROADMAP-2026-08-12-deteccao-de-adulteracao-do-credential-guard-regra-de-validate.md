---
status: wip
date: 2026-08-12
req: "docs/req/REQ-2026-08-12-deteccao-de-adulteracao-do-credential-guard-ancorada-no-head-do-git-regra-de-validate-para-script-e-config.md"
squad: "Apolo, Ártemis, Hades, Hefesto"
---

# Roadmap: Deteccao de adulteracao do credential-guard — regra de validate

> Created: 2026-08-12 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-12-deteccao-de-adulteracao-do-credential-guard-ancorada-no-head-do-git-regra-de-validate-para-script-e-config.md`
ADR: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`

Três vias de neutralização do `trackfw-credential-guard.sh` estão medidas ou verificadas no código,
e apenas a primeira tem cobertura parcial hoje:

| Via | Cobertura atual |
|---|---|
| **Deleção** do script | ⚠️ `credential_guard_hook_resolvable`, e **só quando `validate` roda** |
| **Sobrescrita** com `exit 0` | ❌ passa em `os.Stat` **e** no bit `0111`; no Codex produz `hook: PreToolUse Completed`, o **mesmo rótulo de sucesso normal** |
| **Downgrade de `credential_guard.mode`** | ❌ uma linha de YAML, lida em runtime (`internal/generators/scaffold.go:1005`) |

## 🔴 Decisão de desenho ainda em aberto — resolver no ML-0A, não presumir

O ADR fala em **detecção ancorada no `HEAD` do git**. Mas existe uma segunda âncora que o ADR
**superseded rejeitou por um raciocínio que pode estar errado**, e isto precisa ser reavaliado antes
de implementar:

> O ADR superseded rejeitou "verificação de integridade de conteúdo" porque *"exige um valor de
> referência guardado fora do arquivo gerado — ou seja, exige exatamente o escopo global"*.

**Isso pode ser falso.** O **próprio binário do trackfw** contém o template do script
(`internal/generators/scaffold.go`) — ele **é** uma referência fora do arquivo gerado, e não depende
de escopo global nenhum. O `validate` pode **regenerar o script em memória** e comparar com o disco.

### As duas âncoras, com trade-offs reais

| Âncora | Cobre sobrescrita? | Falso positivo provável | Limite |
|---|---|---|---|
| **`HEAD` do git** | sim, se o script estiver versionado | script/`yaml` legitimamente editado; `trackfw update` regenerando após bump de versão | **não existe** antes do primeiro commit ou para arquivo não versionado |
| **Template do binário** | sim, sempre | **usuário em binário antigo com script gerado por versão nova** (e vice-versa) — divergência legítima por *drift* de versão | não cobre `trackfw.yaml`, que não é gerado por template |

Provavelmente a resposta é **as duas, para alvos diferentes**: template do binário para o **script**
(é gerado, tem forma canônica) e `HEAD` para o **`credential_guard.mode`** (é autoral, não tem forma
canônica). Mas isto é **hipótese de Zeus, não decisão** — o ML-0A avalia e o ADR é emendado com o
resultado.

**Por que isso não é reabrir decisão fechada:** o ADR rejeitou integridade de conteúdo por um motivo
específico (dependência do escopo global). Se o motivo não se sustenta, a rejeição merece
reavaliação — o mesmo padrão que já se aplicou duas vezes nesta sequência, quando premissas não
medidas foram testadas e caíram.

## Acceptance Criteria

- [ ] Decisão de âncora tomada com base em análise escrita, e o ADR **emendado** com o resultado.
- [ ] Regra nova de `validate` nos **3 CLIs** cobrindo as **três** vias.
- [ ] **Não dispara** sem âncora disponível (repo sem commits, arquivo não versionado, binário sem
      template correspondente) — explícito e testado.
- [ ] **Não dispara** por *drift* de versão legítimo, ou o comportamento nesse caso está **escrito e
      justificado**.
- [ ] **Não dispara neste repositório** — `trackfw validate` continua sem violações.
- [ ] Configurável por `rules:`; severidade default decidida e justificada.
- [ ] Cenário de falsificação com **prova negativa** e braço **autodiscriminante**.
- [ ] Documentação para o **usuário final**, fora do `cli-parity.md`, com "detecção ≠ prevenção".
- [ ] `make quality` verde.

### Escopo negativo

- **Não** reintroduz `failClosed` nem wrapper.
- **Não** transforma detecção em bloqueio de chamada de ferramenta.
- **Não** escreve em `$HOME` nem depende do escopo global.
- **Não** altera `scripts/trackfw-credential-guard.sh` nem o wiring de caminho dos hooks.

---

## Wave 0 — Decisão de âncora (1 ML, bloqueante)
> Dependências: nenhuma. **Bloqueia a implementação.**

### ML-0A — `HEAD` × template do binário: qual âncora, para qual alvo
**Status:** 🔄 Em andamento
**Agente:** Hades (`hades-tf`)
**Entregável:** `docs/seguranca/2026-08-12-ancora-de-deteccao-de-adulteracao.md` (novo).
**Não modifica código.**

**Ações:** avaliar as duas âncoras contra as **três vias**, respondendo:
1. O raciocínio do ADR superseded (*"integridade exige escopo global"*) **se sustenta**, dado que o
   binário contém o template? Se não, dizer explicitamente.
2. Qual âncora para o **script**? Qual para o **`credential_guard.mode`**? Por quê.
3. **Falso positivo é o risco dominante** — uma regra ruidosa é desligada e aí o controle todo se
   perde. Para cada âncora, qual a taxa esperada de falso positivo em uso normal, e como discriminar?
4. O que fazer quando **não há âncora** (sem commit, arquivo novo, drift de versão)?
5. Um adversário que **também** adultera a âncora (commita a sobrescrita, ou usa binário adulterado)
   derrota a detecção? Isso muda a escolha?

**Critérios de aceite:**
- [ ] As 5 perguntas respondidas, com **medido/verificado × avaliação** separados.
- [ ] Recomendação explícita de âncora por alvo, com justificativa.
- [ ] Nenhum arquivo de código modificado.

---

## Barreira B0 — Emenda ao ADR (Zeus)
> Dependências: ML-0A. Zeus emenda o ADR com a decisão de âncora antes de liberar a implementação.

---

## Wave 1 — Implementação (1 ML)
> Dependências: Barreira B0.

### ML-1A — Regra de detecção nos 3 CLIs
**Status:** ⬜ Pendente
**Agente:** Apolo (`apolo-tf`)
**Arquivos:** `internal/validator/` + equivalentes em `npm/src/` e `pypi/trackfw/` + testes dos 3.

Implementar conforme a âncora decidida na Barreira B0. Seguir o padrão da regra
`credential_guard_hook_resolvable` (`internal/validator/validator_credential_guard.go`), que é o
precedente direto: `applyRule`/`applyRuleTagged`, mensagem acionável, e **silêncio** quando não há o
que avaliar.

⚠️ **Armadilha já paga nesta sequência:** `os.Getwd()` do Go devolve caminho **symlinkado**; Node e
Python devolvem o físico. Se a mensagem embutir caminho absoluto, use `filepath.EvalSymlinks` —
divergência que **nenhum gate pega**, porque o Cenário 29 fixa só a mensagem de sucesso.

**Critérios de aceite:** os da REQ, mais paridade nos 3 CLIs e `trackfw validate` limpo neste repo.

---

## Wave 2 — Prova negativa (1 ML)
> Dependências: Wave 1. **Separada de propósito** — foi a separação que expôs o braço não
> autodiscriminante no `ROADMAP-2026-08-12-prova-negativa-...` (ML-1A → ML-1B).

### ML-2A — Cenário de falsificação
**Status:** ⬜ Pendente
**Agente:** Ártemis (`artemis-tf`)
**Arquivos:** `scripts/check-gates-falsify.sh`.

**Critérios de aceite:**
- [ ] Baseline + detecção, casando a chave desta regra especificamente.
- [ ] 🔴 **Prova de não-vacuidade:** desabilitar a regra faz o cenário **falhar**. Reportar a saída.
      ⚠️ **Reconstrua `bin/trackfw`** ao sabotar — `go build ./...` não regenera esse binário, e o
      cenário o usa. *(Erro que Zeus cometeu na auditoria do Cenário 47.)*
- [ ] Braço de detecção **autodiscriminante** — não satisfazível por outra causa.
- [ ] Âncora de manutenção documentada + instrução `RETARGET`.

---

## Wave 3 — Documentação e revisão (2 MLs, **paralelos**)
> Dependências: Wave 2. Arquivos disjuntos.

### ML-3A — Documentação
**Status:** ⬜ Pendente · **Agente:** Hefesto (`hefesto-tf`) · **Arquivos:** `docs/cli-parity.md` **e**
documentação de usuário final (README / `--help`). O item de usuário final é **requisito da REQ**, não
opcional: `cli-parity.md` é interno e não é lido por quem instala o trackfw.

### ML-3B — Revisão de segurança
**Status:** ⬜ Pendente · **Agente:** Hades (`hades-tf`) · **Entregável:** `docs/seguranca/` (novo).
Avaliar se a regra entregue cobre as três vias de fato, e se cria falso senso de segurança.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Sequencialidade:** Waves 0→1→2 sequenciais; Wave 3 tem 2 MLs paralelos.
- **Paridade nos 3 CLIs** é inviolável na Wave 1 (altera código de produto).
- **Regra herdada desta sequência:** medir/decidir antes de construir. A Wave 0 existe porque o
  raciocínio que rejeitou a integridade de conteúdo **pode estar errado**, e implementar sobre ele
  sem checar repetiria o erro que já custou três MLs neste ciclo.
