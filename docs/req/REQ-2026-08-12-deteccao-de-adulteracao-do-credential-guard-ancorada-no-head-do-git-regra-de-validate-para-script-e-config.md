---
status: Done
date: 2026-08-12
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate.md"
---

# REQ: Deteccao de adulteracao do credential-guard ancorada no HEAD do git — regra de validate para script e config

> Date: 2026-08-12 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
decidiu: **não há prevenção técnica contra agente induzido com escrita irrestrita ao workspace** — e
o esforço vai para **detecção ancorada no `HEAD` do git**.

Três vias de neutralização do `trackfw-credential-guard.sh` estão **medidas ou verificadas no
código**, e **nenhuma** é fechada pelas mitigações já avaliadas e rejeitadas:

| Via | Como | Coberta hoje? |
|---|---|---|
| **Deleção** do script | `rm scripts/trackfw-credential-guard.sh` — o guard bloqueia materialização de credenciais, não `rm` | ⚠️ só por `credential_guard_hook_resolvable`, e **só quando `validate` roda** |
| **Sobrescrita** com `exit 0` | passa em `os.Stat` **e** no bit `0111`; no Codex produz `hook: PreToolUse Completed` — **o mesmo rótulo de sucesso normal** | ❌ **não coberta** |
| **Downgrade de `credential_guard.mode`** | uma linha no `trackfw.yaml`, lido em **runtime** (`internal/generators/scaffold.go:1005`) | ❌ **não coberta** |

O `HEAD` do git é âncora de confiança que **já existe** e **não depende do escopo global** — é
exatamente o que distingue esta abordagem da verificação de integridade rejeitada no ADR superseded,
que precisava de um valor de referência guardado fora do arquivo gerado.

**Alvo decidido por KG (2026-08-12): `trackfw validate`.** Superfície que já existe, com mecanismo de
severidade configurável (`rules:`), paridade nos 3 CLIs já estabelecida, e precedente direto — a
regra `credential_guard_hook_resolvable` e seu Cenário 47 de falsificação.

### A assimetria que favorece esta abordagem

O fallback do guard **global** é `block` (Emenda 6 do `ADR-2026-08-06`). Logo, a via que sobrevive
exige uma **edição positiva** no `trackfw.yaml` — a mais **diffável** das três. Detecção contra
`HEAD` é o instrumento certo justamente para mudanças positivas em arquivo versionado.

### ⚠️ O que precisa ser resolvido no desenho, não presumido

- **Arquivo legitimamente modificado.** Um desenvolvedor edita `trackfw.yaml` o tempo todo, e o
  script pode ser regenerado por `trackfw update`. **Divergir do `HEAD` não é, por si, adulteração.**
  O desenho precisa distinguir sinal de ruído — senão a regra vira falso positivo constante e é
  desligada, e aí perde-se o controle inteiro (o mesmo raciocínio que fez a regra anterior ignorar
  formatos de caminho desconhecidos).
- **O que nunca foi commitado** não tem `HEAD` para comparar: projeto antes do primeiro commit,
  script recém-gerado e ainda não versionado, `trackfw.yaml` novo. Precisa de resposta explícita —
  provavelmente "não violar", como as demais regras deste projeto fazem em caso de ausência.
- **Detecção não é prevenção**, e isso precisa chegar ao **usuário final** — não só ao
  `docs/cli-parity.md`, que é documento interno de paridade.

## Acceptance Criteria

- [ ] Regra nova de `validate`, nos **3 CLIs** (Go, Node.js, Python), que detecta divergência entre o
      conteúdo em disco e o `HEAD` para: **(a)** `scripts/trackfw-credential-guard.sh` e
      **(b)** a chave `credential_guard.mode` do `trackfw.yaml`.
- [ ] **Cobre as três vias**: deleção, sobrescrita e downgrade de `mode`.
- [ ] **Não dispara** quando não há `HEAD` para comparar (repo sem commits, arquivo não versionado) —
      comportamento explícito e testado, não acidental.
- [ ] **Não dispara** por modificação legítima que não altere o comportamento de segurança — o
      critério de discriminação precisa estar **escrito e justificado** no ADR ou no código, não
      implícito.
- [ ] Configurável por `rules:` como as demais; severidade default decidida e justificada.
- [ ] **Não dispara neste repositório** — `trackfw validate` continua sem violações
      (mesma verificação que o ML-1A do roadmap anterior exigiu).
- [ ] Mensagem **acionável**: qual arquivo, qual divergência, e o que fazer.
- [ ] **Cenário de falsificação** provando que a regra não é vácua, com **prova negativa**:
      desabilitar a regra tem de fazer o cenário **falhar**.
- [ ] O braço de detecção é **autodiscriminante** — não pode ser satisfeito por outra causa que não a
      sabotagem. *(Lição do ML-1A → ML-1B do `ROADMAP-2026-08-12-prova-negativa-...`.)*
- [ ] Documentação para o **usuário final**, fora do `cli-parity.md`, deixando explícito que isto é
      **detecção, não prevenção**.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** reintroduz `failClosed`, wrapper nem verificação de integridade com referência fora do
  repositório — rejeitados; reabrir exige emendar o ADR.
- **Não** altera `scripts/trackfw-credential-guard.sh` nem o wiring de caminho dos hooks.
- **Não** transforma detecção em bloqueio de chamada de ferramenta — o ADR decidiu que prevenção não
  é possível no escopo do trackfw.
- **Não** escreve em `$HOME` nem depende do escopo global.

## Linked ADR
ADR: docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate.md
