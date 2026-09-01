---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: Gate de shell detecta reversão na grafia literal, mas não a semântica — endurecer para checagem comportamental

> Date: 2026-09-01 | Status: Open

## Motivation

Achado **bloqueante** do `hades-tf` na barreira final da
`REQ-2026-09-01-mesmo-gate-de-wave-...`, reproduzido por execução e **reconfirmado pelo arquiteto**.

O `scripts/check-shell-posix-portability.sh` tem **duas metades com tratamentos opostos**, e cada uma
tem um buraco diferente:

| metade | mecanismo | buraco |
|---|---|---|
| **positiva** — `assert_count` exige a assinatura `sh -c` presente | **não exclui comentários** | a assinatura viva pode estar **comentada** e ainda satisfazer o grep |
| **negativa** — `assert_no_code_match` proíbe `shell: true` | regex **literal** | grafia equivalente evade: `{["shell"]: true}` em JS, `**{"shell": True}` em Python |

**Reproduzido pelo arquiteto**, com regressão real e não espantalho:

```
// spawnSync('sh', ['-c', command], {      ← assinatura viva COMENTADA
   spawnSync(command, {["shell"]: true,    ← shell:true real, outra grafia

node --check  →  sintaxe válida
gate          →  OK — 10 assinaturas confirmadas   (exit 0)
```

`hades-tf` executou `spawnSync('echo hi', {["shell"]: true})` e obteve `exit 0` interpretado pelo
shell do SO — **não é grafia inerte, é `shell:true` funcional**.

## O que é o defeito, e o que não é

**Não é vulnerabilidade explorável por atacante externo.** É falha de **defesa em profundidade contra
reversão** — o mesmo vetor de *"agente com escrita irrestrita induzido a desfazer um controle"* já
nomeado no `ADR-2026-08-12`.

**O que era bloqueante foi o contrato mentir sobre isso.** O `docs/cli-parity.md` afirmava que o gate
*"reprova se qualquer dos dois arquivos reverter para `shell: true`"* — falso. Já corrigido para
`partial=`, nomeando as duas evasões. **Esta REQ trata o gate; o contrato já está honesto.**

### A observação que vale além deste gate

As duas metades foram revisadas por pessoas diferentes, e **as duas revisões estavam certas no que
examinaram**: `hefesto-tf` verificou que a metade negativa exclui comentários — correto; `hades-tf`
verificou que a positiva **não** exclui — também correto. **O buraco estava na assimetria entre elas,
que nenhuma das duas revisões tinha motivo para olhar.**

Um gate com duas metades de tratamento oposto é mais frágil que dois gates separados, porque a
inconsistência não aparece em nenhuma leitura parcial.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Checagem comportamental, não textual.** O gate observa **qual interpretador é de
      fato usado** (mock/instrumentação em runtime), em vez de casar texto. Regex sobre código é
      derrotada por grafia equivalente **por construção** — endurecer o regex só move a fronteira.
- [ ] **AC2** — 🔴 **Falsificação com as duas evasões reproduzidas aqui**: assinatura viva comentada;
      e `{["shell"]: true}` / `**{"shell": True}`. Ambas devem **reprovar**. São o corpus mínimo — não
      o suficiente.
- [ ] **AC3** — **Controle:** a árvore correta continua passando, e código legítimo que **mencione**
      `shell: true` em comentário ou prosa **não** reprova. Foi por isso que a exclusão de comentários
      existe; endurecer não pode ressuscitar o falso positivo.
- [ ] **AC4** — As duas metades passam a ter **tratamento consistente** quanto a comentários, ou a
      inconsistência é justificada por escrito.
- [ ] **AC5** — 🔴 **A mesma pergunta aplicada aos outros gates de regex.** `hefesto-tf` já observou
      que a limitação *"regex não pega reintrodução por atribuição"* é **genérica de todos os gates
      `grep` do repositório**. Enumerar quais fazem asserção de **ausência** por regex e dizer, para
      cada um, se a evasão por grafia equivalente é aceitável. **Esta AC vale mais que as quatro
      primeiras** — o defeito é da forma, não deste gate.
- [ ] **AC6** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** reabrir a correção do item 7 — ela está certa e verificada por execução nos 3 CLIs.
- **Não** remover a exclusão de comentários da metade negativa: ela impede que a documentação do
  antipadrão reprove a árvore correta, que é falso positivo real e já observado.

## Linked ADR

ADR: <!-- avaliar na análise: se a AC5 mostrar que asserção-de-ausência-por-regex é padrão
disseminado, a postura "gate de ausência exige checagem comportamental" vira decisão de projeto. -->

## Linked Roadmap

Roadmap:
