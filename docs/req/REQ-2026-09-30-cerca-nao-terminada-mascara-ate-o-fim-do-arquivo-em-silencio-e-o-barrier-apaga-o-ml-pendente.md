---
status: Open
date: 2026-09-30
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md"
---

# REQ: cerca nao terminada mascara ate o fim do arquivo em silencio e o barrier apaga o ML pendente

> Date: 2026-09-30 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Origem: **#476** (`lourivalgarciajunior`, consumidor externo, com medição e sonda de dois braços).

### O contrato promete, o produto não cumpre

`docs/cli-parity.md`, § *Roadmap parsing rules*, regra **6**:

> *"an unterminated fence is a usage error (**exit 2**) with an explicit message naming the offending
> line number — **never a silent pass**."*

`FenceMask` (`internal/roadmapdoc/roadmapdoc.go:311`) devolve `[]bool` e **descarta** o estado
`fenced` ao sair do laço. Cerca aberta → mascara **até o fim do arquivo**, sem erro, sem linha, sem
sinal.

### Reproduzido por mim em 2026-09-30, fixture própria, diferença de UMA linha

```
braço C (cerca fecha)      mls_complete = BLOCKED  ["ML-1B: not complete (status: ⬜ Pendente)"]
                           acceptance_evidence = BLOCKED  ["ML-1B: 1 unmet acceptance criteria"]
braço D (cerca NÃO fecha)  mls_complete = PASSED
                           acceptance_evidence = PASSED
```

🔴 **O ML pendente não é tolerado — ele é apagado da análise.** O `barrier` não vê um microlote
incompleto; ele vê um roadmap que não tem aquele microlote. Com `validate` limpo, o veredito é
`passed` / `rc=0`, e a wave libera a seguinte com trabalho em aberto dentro dela.

### A detecção JÁ EXISTE — só não no caminho que importa

```
$ git grep -n "unterminated" -- internal | grep -v _test
internal/roadmapdoc/roadmapdoc.go:674:  "unterminated gates fence starting at line %d"
```

`ParseGates` detecta e produz a mensagem **no formato que a regra 6 pede**. Medido por mim: uma
fixture cuja cerca aberta é a do bloco de gates sai **`rc=2`** com
`trackfw barrier: unterminated gates fence starting at line 23`. A mesma fixture com a cerca aberta
**fora** do bloco de gates sai silenciosa.

Não é um contrato sem implementação: é uma implementação que cobre **um** caminho de dois.

### 🔴 A superfície é maior que o `barrier` — e não é uniforme

`FenceMask` tem **10 call sites**, medidos:

| call site | comando | pode sair 2? |
|---|---|---|
| `internal/commands/barrier.go:164` | `barrier` | sim |
| `internal/generators/roadmap.go:617` | `roadmap show` | sim |
| `internal/generators/roadmap_show_json.go:128` | `roadmap show --json` | sim |
| 🔴 `internal/serve/api_board.go:168` | `serve` | **NÃO — é servidor HTTP** |
| `roadmapdoc.go` (748, 798, 842, 866, 909) | internos | depende do chamador |

O `serve` não pode "sair 2": ele serve um board sobre um acervo que pode conter um arquivo malformado,
e derrubar o servidor por causa de um roadmap é pior que a doença. **Qual é o comportamento correto em
cada superfície é entregável da Wave 0**, não presunção desta REQ.

### Exposição hoje no acervo: 2 arquivos, e conferi

```
scripts/testdata/roadmap-barrier-corpus-snapshot   144 arquivos   1 com cerca aberta (linha 460)
docs/roadmaps                                      235 arquivos   2 com cerca aberta (460, 1044)
```

Reimplementei a regra de cerca a partir do contrato, em Python, independente do Go — os dois arquivos
coincidem com os que o #476 reporta. A cauda mascarada dos dois é **só prosa** (zero `### ML-`, zero
`**Status:**`, zero `- [ ]`), então **hoje o defeito não esconde trabalho de ninguém**. O que existe é
o mecanismo, intacto, esperando a primeira cauda com conteúdo.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0:** o comportamento correto **por superfície**, mapeado nos 10 call sites.
      `serve` não pode sair 2; diga o que ele faz. Se a medição indicar que algum call site não deve
      mudar, **diga e justifique**
- [ ] **AC2** — cerca não terminada em comando CLI → **exit 2** com mensagem nomeando **a linha de
      abertura**, no formato que a regra 6 escreve e que o `ParseGates` já usa como molde
- [ ] **AC3** — 🔴 **Falsificação nas duas direções:** cerca aberta → reprova nomeando a linha ·
      **roadmap bem-formado → continua passando**. O segundo braço é o que impede a correção de
      transformar todo roadmap em erro de uso
- [ ] **AC4** — 🔴 **O braço D da sonda fecha:** o roadmap com cerca aberta e ML pendente **deixa de
      sair `passed`**. Medido por `barrier --json`, não por leitura de código
- [ ] **AC5** — 🔴 **Os 2 arquivos do acervo corrigidos no MESMO PR.** A detecção os transforma em
      `exit 2`; entregar a detecção sem o conserto é quebrar o acervo de propósito. São 2 linhas
- [ ] **AC6** — o `serve` **não quebra** com um roadmap de cerca aberta no acervo — braço explícito
- [ ] **AC7** — `docs/cli-parity.md`: a regra 6 deixa de ser promessa e passa a descrever o
      comportamento real, **por superfície**
- [ ] **AC8** — `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- **Não** reabre o **#470** (`ParseWaves` fora da máscara). 🔴 **Causa diferente, verificado por
  efeito:** o PR #475 está mergeado na `main`, e a sonda do braço D — rodada por mim com o binário
  da `main` **já contendo** o #475 — continua saindo `mls_complete: passed`. A Regra Dura autoriza
  separar quando a causa é outra, e aqui está medido, não presumido.
- **Não** muda a gramática de cerca (`DetectFenceMarker`, a regra CommonMark de fechamento, o achado
  de 2026-08-29 sobre conteúdo após a crase de fechamento). Isto é **detecção de cerca que nunca
  fecha**, não reinterpretação de qual linha fecha.
- **Não** introduz leniência com prazo para esta classe. A regra 6 diz *"never a silent pass"*, e o
  modo lenient deste projeto existe para **violations do `validate`**, não para **usage error** de
  entrada malformada. Um arquivo que o produto não consegue interpretar não é dívida de governança:
  é entrada inválida.
- **Não** varre o acervo do consumidor. Os 233 arquivos do fork são medição **dele**, citada como
  corroboração; corrigi-los é dele.

## Linked ADR
ADR: <!-- avaliar na Wave 0: o comportamento por superfície (CLI sai 2, serve degrada) é decisão de
produto e provavelmente merece registro. -->
