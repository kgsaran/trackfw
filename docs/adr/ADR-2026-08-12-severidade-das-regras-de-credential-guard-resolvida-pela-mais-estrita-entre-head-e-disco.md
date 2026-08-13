---
status: Accepted
date: 2026-08-12
author: "Zeus (Arquiteto)"
---

# ADR: Severidade das regras de credential-guard resolvida pela mais estrita entre HEAD e disco

> Date: 2026-08-12 | Status: Accepted

REQ: `docs/req/REQ-2026-08-12-ancorar-a-configuracao-rules-no-head-para-as-regras-de-credential-guard-impedir-auto-silenciamento.md`
Parecer: `docs/seguranca/2026-08-12-mecanismo-rules-ancorada-no-head.md`

## Context

As regras de credential-guard podem ser **desligadas pela mesma edição não commitada que deveriam
denunciar**: `ruleSeverity()` (`internal/validator/validator.go:107`) resolve severidade lendo
`rules:` do `trackfw.yaml` **em disco**, nunca do `HEAD`.

Isso é pior que os limites que o `ADR-2026-08-12` (detecção) já aceita: lá o adversário **commita**,
e sobra o **diff visível em revisão**. Aqui **não há commit, logo não há rastro**.

### O parecer ampliou o problema — `rules:` não é o único canal

Verificado por Zeus:

| Canal | O que faz | Versionado? |
|---|---|---|
| **`rules:`** no `trackfw.yaml` | rebaixa a severidade da regra | **sim** — fechável exigindo commit |
| **`.trackfw-baseline.json`** | tolera violações pré-existentes (ratchet) | **não** — `.gitignore:14-15`, **deliberadamente** |
| **`governance_mode: lenient`** | converte **toda** a saída do `validate` em warning | sim, mas o *blast radius* é o validador inteiro |

O canal do **baseline** é o mais insidioso: "exigir commit" **não se aplica** a um arquivo que o
projeto decidiu não versionar. Precisa de tratamento próprio, não do mesmo mecanismo.

## Decision

**1. Mecanismo para o canal `rules:` — M4: resolução pela severidade mais estrita entre `HEAD` e
disco, com branch guardado por nome de regra.**

Dentro de `ruleSeverity()`, apenas para as regras de credential-guard: resolver a severidade
comparando o valor em disco com o do `HEAD` e **adotar o mais estrito**. Reaproveita
`headTrackfwYAML()`, que já existe (`validator_credential_guard_integrity.go:117`; equivalente Node
via `git show HEAD:./trackfw.yaml`).

**Por que M4 e não os outros** — cada um foi rejeitado pela pergunta *"o que impede o adversário de
desligar isto também?"*:

- **M1, meta-regra** que avisa quando `rules:` diverge do `HEAD`: **recursivo** — a meta-regra é
  configurável pelo mesmo `rules:`. Empurra o problema um nível.
- **M2, hash externo em `~/.trackfw/`**: reabriria o debate de escopo global que o `ADR-2026-08-12`
  já fechou com medição.
- **M3, ancorar `ruleSeverity()` no `HEAD` globalmente**: funciona, mas altera o comportamento de
  **~40 regras** e quebra configuração legítima de quem usa `rules:` por motivos sem relação com
  segurança. **Viola a restrição de escopo do roadmap.**
- **M4**: zero delta para as outras ~38 regras. É a única que fecha o canal **sem** recursão e
  **sem** blast radius.

**2. Escopo ampliado: o carve-out do `.trackfw-baseline.json` entra neste roadmap.** Fechar `rules:`
e deixar o baseline aberto entregaria a sensação de correção sem a correção — o adversário
simplesmente troca de canal. O mecanismo é **diferente** (o arquivo não é versionado, então não há
`HEAD` para comparar): as regras de credential-guard **não podem ser toleradas via baseline**.

**3. `governance_mode: lenient` fica FORA, como canal aberto documentado.** O *blast radius* é o
validador inteiro e existe caso de uso legítimo (onboarding, com `lenient_until`). Decidir aqui, no
fim de um roadmap sobre outra coisa, seria decisão apressada sobre feature que não estudamos.
**REQ própria.** Até lá, é limite conhecido e **precisa estar documentado** — não escondido.

**4. Sem `HEAD`, cai no disco.** Repositório sem commits ou `trackfw.yaml` não versionado: a
resolução usa o disco e o buraco existe. **Aceito e escrito**, pela mesma razão que o resto desta
linha de trabalho aceita limites: sem `HEAD` não há âncora, e inventar uma reabriria o escopo global
já rejeitado.

**5. Desligamento legítimo continua funcionando — desde que commitado.** É o ponto: o objetivo não é
remover configurabilidade, é impedir o rebaixamento **silencioso**. Quem quiser desligar a regra,
commita a mudança — e aí ela vira diff revisável, que é exatamente o rastro que este ADR existe para
preservar.

### Emenda 1 (2026-08-12, ML-1A) — são TRÊS regras, não duas, e isso tem custo de migração

O texto original citava `credential_guard_mode_downgrade` e `credential_guard_script_integrity`. O
prompt do ML-1A nomeou **três**, incluindo `credential_guard_hook_resolvable` — o agente **seguiu o
prompt e sinalizou a divergência** em vez de escolher em silêncio. **Confirmado: são as três.**

**Justificativa:** o auto-silenciamento não é específico de uma regra — **qualquer** regra de
credential-guard desligável por edição não commitada tem o mesmo furo. Fechar duas e deixar a
terceira aberta seria a mesma troca-de-canal que motivou incluir o carve-out do baseline.

🔴 **Consequência de migração, que o agente levantou e é real:** um projeto que hoje **tolera** uma
violação de `credential_guard_hook_resolvable` via `.trackfw-baseline.json` passa a ter uma violação
**não suprimível**. Isso é **intencional** — é o carve-out funcionando — mas é mudança de
comportamento para quem já usa. A saída legítima existe e é auditável: **desligar a regra com
`rules:` commitado**, ou corrigir o wiring.

**Precisa estar no `README.md`**, não só no `cli-parity.md` — é o tipo de mudança que aparece como
"quebrou do nada" num `trackfw update`.

### Emenda 2 (2026-08-12, ML-1A) — o Cenário 50 fica obsoleto POR DESIGN

O braço de não-vacuidade do Cenário 50 (`scripts/check-gates-falsify.sh`) prova o comportamento
**pré-ADR**: que um `rules: credential_guard_mode_downgrade: off` **não commitado** desliga a regra.
Este ADR torna isso **falso por design**, então o cenário **fica vermelho** — corretamente.

Não é regressão: é o gate fazendo o trabalho dele, detectando que uma premissa encodificada mudou.
A atualização do cenário é escopo do **ML-2A**; até lá, o `make quality` está **conhecidamente
vermelho**, declarado no commit do ML-1A. **O PR não pode ser aberto com o gate vermelho.**

## Consequences

**Positivas**
- Fecha o canal mais direto sem tocar em ~38 outras regras.
- Reaproveita infraestrutura existente (`headTrackfwYAML`) nos 3 stacks.
- Preserva a configurabilidade legítima, exigindo apenas que ela seja **auditável**.

**Negativas / riscos aceitos**
- **`governance_mode: lenient` continua aberto.** É o canal com maior alcance e fica para REQ
  própria. **Enquanto isso, o problema não está resolvido — está reduzido.** Isso precisa estar no
  `README.md`, não só no `cli-parity.md`.
- **Sem `HEAD` o buraco permanece.** Projeto novo, antes do primeiro commit, não tem proteção.
- **Custo por execução:** `git show HEAD:./trackfw.yaml` passa a rodar na resolução de severidade
  destas regras. Aceitável — já é o custo pago pela própria regra `credential_guard_mode_downgrade`.
- **Assimetria de comportamento entre regras.** Duas regras passam a resolver severidade de forma
  diferente das outras ~38. Isso **precisa estar documentado no código**, senão parece bug para quem
  ler depois.

## Alternatives Considered

**M1 (meta-regra), M2 (hash externo), M3 (âncora global)** — rejeitados acima, cada um por um motivo
distinto: recursão, reabertura de debate fechado, e blast radius.

**"Não vale o custo — documentar e parar."** Era conclusão explicitamente aceitável no roadmap, e foi
**rejeitada pelo parecer com razão**: M4 tem custo baixo, escopo contido e infraestrutura pronta.
Parar aqui deixaria aberto o canal mais fácil de explorar, tendo a correção a um branch de distância.

**Tornar as regras de credential-guard não configuráveis.** Rejeitado: quebra quem tem motivo
legítimo para desligá-las (ex.: projeto que não usa credential-guard), e o objetivo é impedir
silenciamento **sem rastro**, não impedir configuração.
