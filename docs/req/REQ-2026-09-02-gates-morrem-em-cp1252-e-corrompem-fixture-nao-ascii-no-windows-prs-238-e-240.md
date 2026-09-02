---
status: Done
date: 2026-09-02
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-02-governanca-dos-prs-238-e-240-do-reporter.md"
---

# REQ: Gates morrem em cp1252 e corrompem fixture não-ASCII no Windows — PRs #238 e #240

> Date: 2026-09-02 | Status: Done

## Nota sobre a autoria e sobre esta REQ

🔴 **O código destes dois PRs é de `lourivalgarciajunior`.** Esta REQ **não** descreve trabalho
nosso: ela existe porque a cadeia `REQ → ROADMAP` é exigida neste repositório e **nunca foi
publicada** — não há `CONTRIBUTING.md` nem template de PR, e a exigência vive na cabeça dos
mantenedores.

**A obrigação de governança é nossa, não dele.** Escrever a REQ pelo contribuidor é a saída coerente
enquanto a regra não está publicada. **Exceção pontual, decidida pelo KG, válida só para estes dois
PRs** — a partir dos próximos, o fluxo se aplica, e a
`REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing`
publica a regra para que a exigência deixe de ser retroativa.

## Motivation

### PR #238 — o gate morre em cp1252, e há uma segunda razão pior

`scripts/check-roadmap-barrier-contract.sh` tem um heredoc `python3` que imprime `evidence`/`failures`
do `barrier` — contendo os tokens de status do roadmap (`✅`, `⬜`). Em console cp1252 o `print()`
estoura com `UnicodeEncodeError` e o gate **morre no 11º check**.

**A segunda razão, do próprio autor, é mais séria e eu confirmei na linha 542:** a saída do heredoc
alimenta o `CORPUS_HASH`. Então **a codificação faz parte do dado**, e o **mesmo corpus daria hash
diferente por SO**.

**Um hash divergente é pior que um crash.** Crash é barulhento; hash divergente parece *"o corpus
mudou"* e manda alguém caçar uma alteração que não houve.

Correção: `sys.stdout.reconfigure(encoding="utf-8", errors="replace", newline="\n")`.

### PR #240 — o gerador de fixture corrompia a entrada, e a culpa caía no produto

`write_fixture_crlf` usava `sys.stdin.read()`, que decodifica pelo **locale** — cp1252 no Windows —
enquanto o heredoc chega em **UTF-8**. **Reproduzi os bytes exatos:**

```
entra:      e2 ac 9c              (U+2B1C, marcador de status do roadmap)
sai:        c3 a2 c2 ac c5 93     dupla codificação
com o fix:  e2 ac 9c              idêntico à entrada
```

A frase do autor é a melhor descrição de defeito de harness que este repositório já registrou:

> *"A fixture ia para o disco com dupla codificação, e o CLI reportava fielmente o lixo — o defeito
> parecia do produto e era do gerador de fixture."*

**É o tipo mais caro de defeito de teste:** o harness corrompe a entrada, o produto reporta a
corrupção corretamente, e a culpa cai no produto. Alguém depuraria o CLI por horas procurando um bug
que não existe — e o gate ficaria *"vermelho por motivo verdadeiro"*, o pior disfarce possível.

Correção: `sys.stdin.buffer.read().decode('utf-8')`.

## Acceptance Criteria

- [x] **AC1** — O gate não estoura sob console cp1252. **Verificado pelo autor**; o mecanismo é
      reproduzível em qualquer SO via `PYTHONIOENCODING=cp1252`.
- [x] **AC2** — A fixture escrita bate **byte a byte** com a entrada para conteúdo não-ASCII.
      **Reproduzido pelo arquiteto**, bytes acima.
- [x] **AC3** — 🔴 **Controle:** a fixture com CRLF continua correta para entrada ASCII — a correção
      não quebrou o caso que já funcionava. Coberto pelos cenários existentes do gate.
- [x] **AC4** — `CI` verde: `parity` **SUCCESS** nos dois PRs, e é o job que **executa** o gate
      corrigido. **A correção foi exercitada, não só compilada.**
- [ ] **AC5** — Rebase sobre a `main`: os PRs foram abertos **antes** do rename dos job ids e do
      `required_status_checks`, então os checks `governance-install-script` e `governance-go-install`
      **não existiam**. Precisam existir para satisfazer os 9 exigidos.
- [ ] **AC6** — 🔴 **`errors="replace"` no `CORPUS_HASH` não é suficiente sozinho.** O ML-0A da
      `REQ-2026-09-02-heredoc-python3-...` mediu: `replace` **não corrige a não-determinação do
      hash** — só a torna **silenciosa**. O PR #238 fecha o crash; **a determinação do hash entre
      SOs continua aberta** e fica registrada ali, não aqui.

## Negative Scope

- **Não** corrigir os outros 38 gates — é a `REQ-2026-09-02-heredoc-python3-...`, e o ML-0A mediu que
  neles o não-ASCII está no `echo` do bash, que **não estoura**.
- **Não** reescrever o código dos PRs. A autoria é dele; auditamos e verificamos.
- **Não** estabelecer precedente: **exceção pontual**, só para estes dois.

## Linked ADR

ADR: <!-- nenhum. Correção de codificação em ferramenta; nenhuma decisão arquitetural. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-02-governanca-dos-prs-238-e-240-do-reporter.md`
