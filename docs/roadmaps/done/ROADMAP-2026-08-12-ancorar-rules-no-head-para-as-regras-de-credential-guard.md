---
status: done
date: 2026-08-12
req: "docs/req/REQ-2026-08-12-ancorar-a-configuracao-rules-no-head-para-as-regras-de-credential-guard-impedir-auto-silenciamento.md"
squad: "Hades, Apolo, Ártemis"
---

# Roadmap: Ancorar rules no HEAD para as regras de credential-guard

> Created: 2026-08-12 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-12-ancorar-a-configuracao-rules-no-head-para-as-regras-de-credential-guard-impedir-auto-silenciamento.md`

Achado da revisão final do roadmap anterior (ML-3B): **a regra `credential_guard_mode_downgrade` pode
ser desligada pela mesma edição que ela deveria denunciar.**

`ruleSeverity()` (`internal/validator/validator.go:107`) resolve severidade lendo `rules:` do
`trackfw.yaml` **em disco**, nunca do `HEAD`. Uma **única edição não commitada**:

```yaml
credential_guard:
  mode: warn                              # rebaixa o controle
rules:
  credential_guard_mode_downgrade: off    # e desliga quem avisaria
```

**derrota a detecção sem deixar rastro.** Isso é **pior** que os limites que o `ADR-2026-08-12` já
aceita: lá o adversário **commita**, e o que sobra é o diff visível em revisão. Aqui **não há
commit**, logo **não há rastro**, e a própria regra é desativada pelo arquivo que ela vigia.

## 🔴 A armadilha de desenho: recursão

A solução ingênua — *"crio uma regra que avisa quando o `rules:` em disco é mais fraco que no
`HEAD`"* — **tem o mesmo furo**: essa meta-regra também seria configurável por `rules:`, e o
adversário a desliga na mesma edição. Empurra o problema um nível, não resolve.

**Qualquer mecanismo proposto precisa responder:** *"e o que impede o adversário de desligar isto
também?"* Se a resposta for "outra regra configurável", está errado.

## Restrição de escopo que não pode ser violada

**Não alterar a resolução de `rules:` das demais regras.** `ruleSeverity()` é maquinaria
**compartilhada por todas** as regras do validador. Mudança de escopo amplo aqui afeta comportamento
de usuários que configuraram `rules:` legitimamente, por motivos que não têm nada a ver com
segurança. Se o mecanismo escolhido exigir tocar no caminho compartilhado, isso precisa ser
**justificado explicitamente**, não presumido.

## Acceptance Criteria

- [x] A severidade das regras de credential-guard **não** é rebaixável por edição **não commitada**
      do `rules:`.
- [x] O mecanismo **não é recursivamente desligável** — responde à pergunta da armadilha acima.
- [x] **Desligar de forma legítima e commitada continua funcionando** — o objetivo é impedir o
      rebaixamento **silencioso**, não remover a configurabilidade.
- [x] Comportamento **sem `HEAD`** (repo sem commits, `trackfw.yaml` não versionado) decidido
      **conscientemente** e escrito: sem `HEAD` a resolução cairia no disco e o buraco volta.
- [x] Resolução de `rules:` das **demais** regras **inalterada** — ou alteração justificada.
- [x] **Paridade nos 3 CLIs** (Go, Node.js, Python).
- [x] Cenário de falsificação: a edição combinada (`mode: warn` + regra `off`, **não commitada**)
      **continua sendo reportada**. Com prova de não-vacuidade e braço autodiscriminante.
- [x] `docs/cli-parity.md` e `README.md` atualizados **removendo** o limite, se resolvido.
- [x] `make quality` verde; `trackfw validate` sem violações **neste repositório**.

### Escopo negativo

- **Não** reabre prevenção × detecção (`ADR-2026-08-12`).
- **Não** altera as âncoras já decididas (template do binário para o script; `HEAD` para o `mode`).
- **Não** transforma detecção em bloqueio de chamada de ferramenta.
- **Não** trata o item secundário da REQ (cobertura de deleção condicional ao wiring) — fica para
  roadmap próprio se a Barreira B0 concluir que vale.

---

## Wave 0 — Mecanismo (1 ML, bloqueante)
> Dependências: nenhuma. **Bloqueia a implementação.**

### ML-0A — Qual mecanismo, e por que ele não é recursivamente desligável
**Status:** ✅ Concluído (Hades; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Hades (`hades-tf`)
**Entregável:** `docs/seguranca/2026-08-12-mecanismo-rules-ancorada-no-head.md` (novo).
**Não modifica código.**

**Perguntas:**
1. **Enumere os mecanismos possíveis** e, para cada um, responda: *"o que impede o adversário de
   desligar isto também?"* Rejeite os que só empurram o problema um nível.
2. **Qual você recomenda**, e qual o custo em maquinaria compartilhada? Se exigir mexer em
   `ruleSeverity()` para **todas** as regras, isso é aceitável? Justifique contra a restrição de
   escopo acima.
3. **Sem `HEAD`, o que fazer?** Cair no disco reabre o buraco; ignorar `rules:` por completo quebra
   configuração legítima de quem não versiona `trackfw.yaml`. Recomende **e assuma o trade-off**.
4. **Desligamento legítimo commitado** continua possível no seu mecanismo? Demonstre com o fluxo.
5. **Existe um caminho mais simples que ninguém considerou?** Ex.: não tornar estas regras
   configuráveis; ou reportar o rebaixamento de `rules:` como parte da **própria** mensagem da regra
   de `mode`, em vez de regra separada. Avalie honestamente — **"não vale o custo, documentar o
   limite"** é conclusão aceitável e deve ser dita se for o caso.

**Critérios de aceite:**
- [x] Mecanismos enumerados, cada um com a resposta à pergunta da recursão.
- [x] Recomendação explícita e acionável, com o custo declarado.
- [x] Posição sobre "sem `HEAD`" e sobre desligamento legítimo.
- [x] Nenhum arquivo de código modificado.

---

## Barreira B0 — ADR do mecanismo (Zeus) — ✅ CONCLUÍDA

**ADR aceito:** `docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md`

**Mecanismo: M4** — dentro de `ruleSeverity()`, branch guardado **por nome de regra**, resolvendo a
severidade pela **mais estrita entre `HEAD` e disco**. Reaproveita `headTrackfwYAML()`, que já existe.
**Zero delta** para as outras ~38 regras.

Os outros três foram rejeitados pela pergunta da recursão: **M1** (meta-regra) é recursivo; **M2**
(hash externo) reabriria o escopo global já fechado por medição; **M3** (âncora global) altera ~40
regras e viola a restrição de escopo.

**🔴 Escopo AMPLIADO — o parecer encontrou mais dois canais, e um entra:**

| Canal | Decisão |
|---|---|
| `rules:` no `trackfw.yaml` | **fechar** com M4 |
| `.trackfw-baseline.json` | **fechar** — carve-out: regras de credential-guard **não** são toleráveis via baseline |
| `governance_mode: lenient` | **FORA** — REQ própria; limite documentado |

**Por que o baseline entra:** fechar `rules:` e deixar o baseline aberto entregaria a **sensação** de
correção sem a correção — o adversário troca de canal. E o mecanismo é **diferente**: o arquivo é
`.gitignore`d **deliberadamente** (`.gitignore:14-15`, verificado por Zeus), então "exigir commit"
**não se aplica** — a regra é que essas violações não podem ser toleradas via baseline.

**Por que `lenient` fica fora:** *blast radius* é o validador inteiro e há caso de uso legítimo
(onboarding, com `lenient_until`). Decidir no fim de um roadmap sobre outra coisa seria decisão
apressada. **Mas fica documentado no `README.md`, não só no `cli-parity.md`** — enquanto ele existir,
o problema está **reduzido, não resolvido**.

## Barreira B0 — registro original
> Dependências: ML-0A. Zeus decide e registra em ADR antes de liberar a implementação. Inclui decidir
> se o item secundário da REQ (cobertura de deleção) entra em roadmap próprio.

---

## Wave 1 — Implementação (1 ML)
> Dependências: Barreira B0.

### ML-1A — Mecanismo nos 3 CLIs (M4 + carve-out do baseline)
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-12)
(ver seção "Consequência esperada" no handoff em `docs/agents-working-context.md`; todo o resto —
`go build`, `go test`, `npm test`, `pytest`, `go vet`, `trackfw validate` neste repo, e todos os
scripts de `parity` exceto `check-gates-falsify.sh` Scenario 50 — está verde) · **Agente:** Apolo
(`apolo-tf`)
**Escopo ampliado na Barreira B0:** além do M4 para `rules:`, implementar o carve-out do
`.trackfw-baseline.json` — as regras de credential-guard **não** podem ser toleradas via baseline.
**Arquivos:** `internal/validator/` + equivalentes em `npm/src/` e `pypi/trackfw/` + testes dos 3.

⚠️ **Armadilhas já pagas nesta linha de trabalho:** `os.Getwd()` do Go devolve caminho **symlinkado**
(use `filepath.EvalSymlinks` se a mensagem embutir caminho); a regra **não pode disparar neste
repositório**; **não alterar** a mensagem de sucesso do `validate` (Cenário 29 a fixa byte-a-byte).

---

## Wave 2 — Prova negativa (1 ML)
> Dependências: Wave 1. **Separada de propósito.**

### ML-2A — Cenário de falsificação
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-12)
**Arquivos:** `scripts/check-gates-falsify.sh`.

O cenário decisivo: **edição combinada não commitada** (`mode: warn` + regra `off`) **continua sendo
reportada**.

⚠️ Ler `vault/notes/armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md` **antes** —
as quatro armadilhas (prova que não prova; `warning` × `assert_fails_with`; isolamento de git em
fixture; reconstruir `bin/trackfw` ao sabotar) foram todas pagas nesta linha de trabalho.

---

## Wave 3 — Documentação e revisão (2 MLs)
> Dependências: Wave 2.

### ML-3A — Documentação
**Status:** ✅ Concluído (Zeus)
`docs/cli-parity.md` + `README.md` — **removendo** o limite documentado, se resolvido.
> Atribuído a Zeus porque Hefesto recusou tarefa equivalente por escopo em 2026-08-12, apesar de tê-la
> executado em 4 PRs anteriores. Enquanto `~/.claude/agents/hefesto-tf` não for reconciliado, este
> tipo de ML não é despachado para ele.

### ML-3B — Revisão de segurança
**Status:** ✅ Concluído (Hades; auditado por Zeus em 2026-08-12) — **achado bloqueante**, ver ML-1B
decisão de Zeus. Entregável: `docs/seguranca/2026-08-12-estado-final-ancoragem-no-head.md`.
**Agente:** Hades (`hades-tf`)
Avaliar se o mecanismo entregue **de fato** impede o auto-silenciamento, e se não criou brecha nova.

---

## Wave 1-bis — Correção do bypass por `GIT_DIR`/`GIT_WORK_TREE` (1 ML)
> Dependências: ML-3B. **Bloqueia o PR.**

### ML-1B — Limpar o ambiente nas invocações de `git` do validador
**Status:** ✅ Concluído (Apolo; auditado e aprovado por Zeus em 2026-08-12)
**Agente:** Apolo (`apolo-tf`)

Brecha provada por PoC no ML-3B e **reproduzida por Zeus**: `GIT_DIR`/`GIT_WORK_TREE` herdadas do
ambiente redirecionam a resolução do `HEAD`, fazendo a severidade cair no disco **em silêncio** —
derrota o M4 inteiro sem commit e sem editar o `trackfw.yaml`.

**Não é limite aceitável.** A correção é pequena e um controle derrotado por duas variáveis de
ambiente não é controle. Ver **Emenda 3** do ADR.

---

## Wave 2-bis — Cenário do bypass por ambiente (1 ML)
> Dependências: ML-1B. **Bloqueia o PR.**

### ML-2B — Falsificação do bypass por `GIT_*`
**Status:** ✅ Concluído (Ártemis; auditado e aprovado por Zeus em 2026-08-13)

O ML-1B fechou o bypass, mas **nenhum cenário prova que ele continua fechado**. Sem isso, uma
regressão futura reabre o buraco em silêncio — exatamente o que este roadmap inteiro combate.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus cria branch, commita e faz push.
- **Sequencialidade:** Waves 0→1→2→3.
- **Paridade nos 3 CLIs** é inviolável na Wave 1.
- **Conclusão aceitável:** se o ML-0A concluir que **nenhum** mecanismo vale o custo, o roadmap
  termina na Barreira B0 com o limite documentado — e isso é resultado, não fracasso.
