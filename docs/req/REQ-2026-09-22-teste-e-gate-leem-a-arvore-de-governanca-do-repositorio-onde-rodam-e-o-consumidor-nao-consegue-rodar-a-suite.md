---
status: Open
date: 2026-09-22
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md"
---

# REQ: teste e gate leem a árvore de governança do repositório onde rodam, e o consumidor não consegue rodar a suíte

> Date: 2026-09-22 | Status: Open
| Linear Issue:
| Jira Issue:

Origem: **issue #396**, reportado por consumidor externo, com medição própria.
Mesma classe declarada pelo reportante: **#277** e o achado 16 do **#216** (já fechado).

## Motivation

O trackfw se propõe a governar **o projeto de quem o instala**. Quando um teste ou um gate do
produto lê a árvore de governança **do repositório onde está rodando** e presume o layout ou o
conteúdo do mantenedor, a suíte fica **inalcançável para o consumidor** — que é exatamente a
proposta de valor sendo negada.

### O caso medido (#396)

`TestCorpusMeasurement_ReportOnly` (`internal/roadmapdoc/roadmapdoc_test.go`) declara-se, no próprio
cabeçalho da seção, **"report-only, never fails"** — e faz `t.Fatalf`:

```go
// ── Corpus measurement — report-only, never fails ──────────────  (linha 242)
...
doneDir := filepath.Join(repoRoot(t), "docs", "roadmaps", "done")
entries, err := os.ReadDir(doneDir)
if err != nil {
    t.Fatalf("ReadDir %s: %v", doneDir, err)          // linha 253
}
```

Num consumidor com `roadmap_namespacing: by_agent`, os roadmaps vivem em
`docs/roadmaps/<agente>/done/` e o caminho plano **não existe**. Efeito medido pelo reportante, em
cascata:

| job | efeito |
|---|---|
| `go` | **reprova** |
| `windows-full-suites` | reprova no ratchet — `NEW Go assertion failure not in known list` |
| `parity-falsify-shard` | **pulado** (depende de `go`) |
| `parity-other-gates` | **pulado** (depende de `go`) |

🔴 **E o teste não mede nada do produto nesse caso.** Medição do reportante: com
`docs/roadmaps/done/` **vazio** (só um `.gitkeep`), ele **passa**, e o `validate` segue com 0
violações. Ele só exige que o repositório onde roda tenha o layout plano.

### Por que é uma REQ e não um fix pontual

O reportante nomeia a família: é a mesma classe do corpus do `barrier-contract` (**#277**, onde 108
de 144 basenames do snapshot estão ausentes num fork, tornando `make quality` inalcançável) e das
REQs lidas por `TestExtractRefPath` (#216, fechado).

A Regra Dura de Causa Raiz deste projeto é explícita: **mesmo sintoma investiga junto; só se separa
com a medição escrita.** O sintoma é idêntico — *"o consumidor não consegue rodar a suíte"*. A Wave 0
desta REQ é a medição que decide se a causa é uma ou são várias.

🔴 Tratar #396 isoladamente produziria o padrão que este projeto já mediu **59 vezes**: defeito
localizado, corrigido num sítio, e os irmãos empurrados para uma fila onde se perdem.

### Medição inicial do arquiteto (aproximada — a enumeração real é da Wave 0)

```
$ grep -rn "repoRoot(" --include='*_test.go' internal/ | grep -v "func repoRoot" | wc -l
1
$ grep -rln 'docs/roadmaps\|docs/req\|docs/adr' scripts/*.sh | wc -l
14
```

O `1` é o sítio do #396. Os `14` **não** são todos defeito — um gate do upstream pode legitimamente
auditar a governança do próprio repositório; o defeito é **exigir isso de quem consome**. Separar as
duas coisas é o entregável da Wave 0.

## Acceptance Criteria

- [ ] **Enumeração real** da população, pelo critério "lê árvore de governança do repositório onde
      roda", classificando cada sítio em: **(a)** exige do consumidor → defeito · **(b)** audita o
      upstream e é legítimo · **(c)** usa fixture própria e já está correto
- [ ] Todo sítio **(a)** corrigido — por fixture própria, por resolução via config
      (`roadmap_dir` / `roadmap_namespacing`), ou por skip declarado quando a medição não se aplica
- [ ] 🔴 **Nenhum artefato declara o que não sustenta** — se o cabeçalho diz *"never fails"*, ele não
      pode conter `t.Fatalf`. Regra Dura de Reconciliação
- [ ] **Falsificação nas duas direções:** o teste/gate reprova quando o produto regride **e** deixa
      de reprovar o consumidor que apenas tem outro layout
- [ ] Prova de que a suíte roda num consumidor com `roadmap_namespacing: by_agent` — **sem** que ele
      precise importar governança do mantenedor
- [ ] `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- **Não** remove a capacidade de o upstream auditar a própria governança. Gate que valida
  `docs/roadmaps/**` deste repositório continua existindo — a separação é entre *"o produto
  funciona"* e *"a governança deste repo está íntegra"*, não a eliminação do segundo.
- **Não** migra layout de roadmaps de ninguém, nem altera `roadmap_namespacing` deste repositório.
- **Não** corrige os defeitos de Windows/CI (#307, #353, #363, #364) — mecanismo diferente, mesmo
  que alguns se manifestem nos mesmos jobs.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md`
