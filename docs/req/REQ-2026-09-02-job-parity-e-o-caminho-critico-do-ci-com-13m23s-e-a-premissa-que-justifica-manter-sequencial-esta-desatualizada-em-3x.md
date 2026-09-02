---
status: Open
date: 2026-09-02
author: "kgsaran"
adr: ""
roadmap: ""
---

# REQ: O job `parity` é o caminho crítico do CI com 13m23s, e a premissa que justifica mantê-lo sequencial está desatualizada em 3x

> Date: 2026-09-02 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

### O que foi medido

Run `33679814232` (`Quality` sobre o merge do PR #250 na `main`, 2026-09-02), durações reais por job:

```
13m23  parity                        20:33:25 → 20:46:48
 7m16  windows-full-suites           (paralelo, não bloqueia)
 1m37  windows-defect-reproduction
 1m33  python (3.12)
 1m26  python (3.10)
 1m02  windows-integrations-resolve
 0m39  node
 0m34  go
 0m24  package-smoke
```

**Todo o resto do pipeline termina em ~1m35. O `parity` responde por ~90% do tempo de parede de um
PR.**

### A premissa registrada no workflow caducou

`.github/workflows/quality.yml` justifica a escolha sequencial assim, textualmente:

> *"Tempo local medido em 2026-08-05 (Apple Silicon): ~4m15s total, dos quais
> `check-gates-falsify.sh` sozinho responde por ~3m05s. Mantido sequencial (sem job paralelo) porque
> o total fica abaixo de dois dígitos de minutos e o job `parity` já roda depois de
> go/node/python/package-smoke/windows-integrations-resolve (`needs:` acima), então **não é o gargalo
> do pipeline de PR**."*

As duas metades da justificativa deixaram de valer:

- **"abaixo de dois dígitos de minutos"** — são 13m23s no CI hoje;
- **"não é o gargalo"** — é o gargalo, e é praticamente o pipeline inteiro.

O alvo `parity:` do `Makefile` tinha ~14 gates quando aquela nota foi escrita e hoje tem **45**. Só
nesta sprint entraram `check-output-encoding-declared.sh`, `check-shell-posix-portability.sh`,
`check-atomic-write-anti-divergence.sh`, `check-ref-separator-portability.sh` e
`check-doctor-remote-parity.sh`.

**A decisão original estava certa quando foi tomada.** O defeito não é a escolha — é que a medição
que a sustentava não foi reavaliada enquanto a população triplicava. É o mesmo padrão de
`vault/notes/uniao-disco-agents-mascara-gate-por-presenca-2026-08-29.md`: a premissa envelhece em
silêncio porque nada a revalida.

### 🔴 A armadilha que quebraria todos os PRs

`parity` é um dos **9 checks obrigatórios** da branch protection da `main`
(`required_status_checks`, com `enforce_admins: true`), e a correspondência do GitHub é **por nome**.

Uma matriz faz os checks se chamarem `parity (1)`, `parity (2)`, … e o check exigido `parity`
**nunca mais aparece** — a proteção passa a bloquear **todo** PR, indefinidamente, sem mensagem que
aponte a causa.

Qualquer divisão exige um job agregador chamado **exatamente** `parity`, com `needs:` nos shards e
`if: always()`, que falhe se qualquer shard falhar. **Isto não é detalhe de implementação: é
pré-condição.**

### Por que não decidir o mecanismo agora

Falta o dado que dimensiona a solução: **o tempo de cada gate individualmente**. O log não é
cronometrado. Sem ele, qualquer divisão em shards é chute, e o shard mais lento passa a mandar no
total — trocaríamos 13m23 por 11m sem saber.

E há um risco que só a medição responde: os gates compartilham `bin/trackfw`, `$HOME` e o diretório
de trabalho do repositório. O `check-gates-falsify.sh` **copia e muta** gates; o `check-barrier.sh`
**executa git**. Execução paralela pode fazê-los interferir e produzir *flake* — que num gate é
**pior que lentidão**, porque destrói a confiança no sinal e treina todo mundo a re-rodar em vez de
investigar.

Precedente direto desta sprint: no ML-1B o mecanismo escrito no roadmap (`PYTHONUTF8=1`) foi
**falsificado pela medição** e trocado. Medir primeiro poupou aplicar a correção errada em 37
arquivos.

## Acceptance Criteria

- [ ] **AC1** — Cada um dos 45 gates do alvo `parity:` tem tempo de execução medido e registrado,
      em ambiente comparável ao runner do CI. A tabela vai para o repositório, não só para o
      relatório.
- [ ] **AC2** — 🔴 **Segurança de paralelismo medida, não presumida:** para cada gate, determinar se
      ele escreve em recurso compartilhado (`bin/trackfw`, `$HOME`, working tree, `.trackfw-log`,
      arquivos em `scripts/`). Falsificação: rodar os candidatos concorrentemente **N vezes** e
      mostrar que o veredito não muda. **Um único flake reprova o candidato.**
- [ ] **AC3** — A escolha entre **matriz de shards** e **`make -j`** é justificada pela medição do
      AC1+AC2, com o descartado dizendo **por que** foi descartado.
- [ ] **AC4** — 🔴 **Um job chamado exatamente `parity` continua existindo e continua reprovando
      quando qualquer shard reprova.** Falsificação nas duas direções: com um gate quebrado de
      propósito, o `parity` fica **vermelho**; com a árvore íntegra, fica verde. Sem isso, a
      proteção da `main` bloqueia todo PR.
- [ ] **AC5** — 🔴 **Cobertura preservada:** o conjunto de gates executados depois é **idêntico** ao
      de antes. Verificado por comparação de conjunto — nenhum gate pode sumir da distribuição.
      Um shard que não executa nada é reprovação, não sucesso (guarda de vacuidade).
- [ ] **AC6** — Ganho real medido no CI, em run comparável — não estimado a partir da soma local.
      **"Não deu ganho suficiente para justificar a complexidade" é resultado válido** e encerra a
      REQ com a medição registrada.
- [ ] **AC7** — O comentário do `quality.yml` é reescrito com a medição nova **e** com a data,
      para que a próxima revisão saiba quando a premissa foi vista pela última vez.

## Negative Scope

- **Não** remover gates nem mover gate para fora do `parity` para ganhar tempo. Reduzir cobertura
  não é otimizar — é desligar o controle e chamar de melhoria.
- **Não** mexer no `needs:` do job nesta REQ sem medir o trade-off: ele existe por economia de
  runner (não queimar 13 minutos num PR que já quebrou no `go`), e o ganho seria de ~1m35. Se a
  medição mostrar que compensa, vira decisão explícita — não efeito colateral.
- **Não** otimizar o `check-gates-falsify.sh` por dentro nesta REQ. Se a medição mostrar que ele é o
  dominante e que dá para acelerá-lo sem perder cenário, é **REQ própria** — mexer nele é mexer no
  gate que falsifica os outros.
- **Não** alterar a lista de `required_status_checks` como forma de contornar o AC4. A solução é o
  job agregador, não afrouxar a proteção.
- **Não** tocar nos jobs de Windows (`windows-full-suites`, `windows-defect-reproduction`). Rodam em
  paralelo, não estão no caminho crítico, e os 2 vermelhos são de diagnóstico da issue #216.

## Linked ADR
<!-- Otimização de pipeline com preservação de contrato de proteção de branch; sem decisão de
     arquitetura de produto. Se a escolha for matriz de shards, o formato do job vira convenção do
     repositório e pode merecer ADR curta — decidir na Wave 0. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap NÃO criado ainda: `trackfw roadmap move` escreve em docs/roadmaps/.trackfw-log, que é o
     arquivo em conflito nos PRs #238/#240 do reporter externo. Criar antes de o #240 mergear
     geraria um terceiro conflito no PR dele. Criar após o merge. -->
Roadmap:
