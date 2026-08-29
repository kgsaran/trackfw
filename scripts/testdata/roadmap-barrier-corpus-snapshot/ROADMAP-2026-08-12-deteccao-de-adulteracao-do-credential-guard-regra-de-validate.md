---
status: done
date: 2026-08-12
req: "docs/req/REQ-2026-08-12-deteccao-de-adulteracao-do-credential-guard-ancorada-no-head-do-git-regra-de-validate-para-script-e-config.md"
squad: "Apolo, Ártemis, Hades, Hefesto"
---

# Roadmap: Deteccao de adulteracao do credential-guard — regra de validate

> Created: 2026-08-12 | Status: done

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

- [x] Decisão de âncora tomada com base em análise escrita, e o ADR **emendado** com o resultado.
- [x] Regra nova de `validate` nos **3 CLIs** cobrindo as **três** vias.
- [x] **Não dispara** sem âncora disponível (repo sem commits, arquivo não versionado, binário sem
      template correspondente) — explícito e testado.
- [x] **Não dispara** por *drift* de versão legítimo, ou o comportamento nesse caso está **escrito e
      justificado**.
- [x] **Não dispara neste repositório** — `trackfw validate` continua sem violações.
- [x] Configurável por `rules:`; severidade default decidida e justificada.
- [x] Cenário de falsificação com **prova negativa** e braço **autodiscriminante**.
- [x] Documentação para o **usuário final**, fora do `cli-parity.md`, com "detecção ≠ prevenção".
- [x] `make quality` verde.

### Escopo negativo

- **Não** reintroduz `failClosed` nem wrapper.
- **Não** transforma detecção em bloqueio de chamada de ferramenta.
- **Não** escreve em `$HOME` nem depende do escopo global.
- **Não** altera `scripts/trackfw-credential-guard.sh` nem o wiring de caminho dos hooks.

---

## Wave 0 — Decisão de âncora (1 ML, bloqueante)
> Dependências: nenhuma. **Bloqueia a implementação.**

### ML-0A — `HEAD` × template do binário: qual âncora, para qual alvo
**Status:** ✅ Concluído (Hades; auditado e aprovado por Zeus em 2026-08-12)
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
- [x] As 5 perguntas respondidas, com **medido/verificado × avaliação** separados.
- [x] Recomendação explícita de âncora por alvo, com justificativa.
- [x] Nenhum arquivo de código modificado.

---

## Barreira B0 — Emenda ao ADR (Zeus) — ✅ CONCLUÍDA
> Dependências: ML-0A.

**ADR emendado (3 emendas).** Decisão: **âncora por alvo**.

| Alvo | Âncora | Severidade |
|---|---|---|
| `scripts/trackfw-credential-guard.sh` | **template do binário** | **`warn`** (ver abaixo) |
| `credential_guard.mode` | **`HEAD`**, comparação **semântica e direcional** | a decidir no ML-1A |

**O raciocínio do ADR superseded cai — parcialmente.** É **falso** para o script (concatenação pura
de constantes, sem interpolação por projeto: o binário sempre foi a referência externa) e
**verdadeiro** para o `mode` (valor autoral, sem forma canônica). O ADR **Accepted** repetia o mesmo
erro ao tratar "âncora no `HEAD`" como uma coisa só — emendado também, não apenas o superseded.

🔴 **Pré-requisito descoberto e verificado por Zeus:** **não existe gate de paridade byte-a-byte do
script do credential-guard entre os 3 stacks.** `check-attention-scripts-parity.sh` cobre apenas os
dois scripts de *attention*; `check-agent-hooks-parity.sh` só faz `grep` da string no **JSON do
hook**. Sem esse gate, o mesmo repo **dispara num CLI e fica silencioso nos outros**. Virou **ML-0B**,
antes da implementação.

**Severidade `warn` para o braço do script, decidida agora:** o script **não carrega marcador de
versão**, então a regra **não consegue** discriminar *drift* legítimo de adulteração. Mensagem
causalmente neutra. Embutir versão/hash no template é trabalho futuro — e é o que permitiria elevar
para `error`.

---

## Wave 0-bis — Pré-requisito: paridade do script (1 ML)
> Dependências: Barreira B0. **Bloqueia a Wave 1.**

### ML-0B — Gate de paridade byte-a-byte do script do credential-guard
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Ártemis (`artemis-tf`)
**Arquivos:** `scripts/` (gate novo ou extensão de `check-attention-scripts-parity.sh`) + `Makefile`
se necessário + cenário de falsificação correspondente.

Sem este gate, a âncora de template é insegura. Modelo pronto: `check-attention-scripts-parity.sh`
já faz exatamente isso para os dois scripts de attention — a extensão natural é incluir
`trackfw-credential-guard.sh`, **se** a mecânica do gate comportar (o script do guard é gerado por
caminho diferente do de attention — **verificar antes**, não presumir).

**Critérios de aceite:**
- [x] Os 3 templates do script do credential-guard comparados **byte-a-byte** entre Go, Node e Python.
- [x] Cenário de falsificação com **prova negativa**: corromper um dos três faz o gate reprovar; sem
      a sabotagem, passa.
- [x] `make quality` exit 0 com total de cenários incrementado.
- [x] Nenhum código de produto alterado (`internal/`, `npm/src/`, `pypi/trackfw/` intocados) — **a
      menos que** os 3 templates **já estejam divergentes hoje**. Se estiverem: **PARE e reporte a
      Zeus** — divergência pré-existente é achado, não conserto silencioso.

---

## Wave 1 — Implementação (1 ML)
> Dependências: Barreira B0.

### ML-1A — Regra de detecção nos 3 CLIs
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-12)
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
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Ártemis (`artemis-tf`)
**Arquivos:** `scripts/check-gates-falsify.sh`.

**Critérios de aceite:**
- [x] Baseline + detecção, casando a chave desta regra especificamente.
- [x] 🔴 **Prova de não-vacuidade:** desabilitar a regra faz o cenário **falhar**. Reportar a saída.
      ⚠️ **Reconstrua `bin/trackfw`** ao sabotar — `go build ./...` não regenera esse binário, e o
      cenário o usa. *(Erro que Zeus cometeu na auditoria do Cenário 47.)*
- [x] Braço de detecção **autodiscriminante** — não satisfazível por outra causa.
- [x] Âncora de manutenção documentada + instrução `RETARGET`.

---

## Wave 3 — Documentação e revisão (2 MLs, **paralelos**)
> Dependências: Wave 2. Arquivos disjuntos.

### ML-3A — Documentação
**Status:** ✅ Concluído — **executado por Zeus** após recusa de escopo de Hefesto (ver working context)
documentação de usuário final (README / `--help`). O item de usuário final é **requisito da REQ**, não
opcional: `cli-parity.md` é interno e não é lido por quem instala o trackfw.

### ML-3B — Revisão de segurança
**Status:** ✅ Concluído (Hades; auditado e aprovado por Zeus em 2026-08-12)
Avaliar se a regra entregue cobre as três vias de fato, e se cria falso senso de segurança.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Sequencialidade:** Waves 0→1→2 sequenciais; Wave 3 tem 2 MLs paralelos.
- **Paridade nos 3 CLIs** é inviolável na Wave 1 (altera código de produto).
- **Regra herdada desta sequência:** medir/decidir antes de construir. A Wave 0 existe porque o
  raciocínio que rejeitou a integridade de conteúdo **pode estar errado**, e implementar sobre ele
  sem checar repetiria o erro que já custou três MLs neste ciclo.
