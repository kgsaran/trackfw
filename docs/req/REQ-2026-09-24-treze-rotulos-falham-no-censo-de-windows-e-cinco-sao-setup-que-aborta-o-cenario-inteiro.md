---
status: Open
date: 2026-09-24
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md"
---

# REQ: treze rótulos falham no censo de Windows, e cinco são `setup` que aborta o cenário inteiro

> Date: 2026-09-24 | Status: Open
| Linear Issue:
| Jira Issue:

Origem: censo `36036473391` em `main` (2026-09-24), o **primeiro** rodado com o instrumento
consertado pelas REQs de 2026-09-23 e 2026-09-24.

## Motivation

O cluster de Windows deste projeto foi, por meses, uma **estimativa**. A REQ-2026-09-23 consertou o
instrumento e a REQ-2026-09-24 removeu os abortos que ele escondia. **Agora existe número**, e é
pequeno o bastante para ser tratado por causa em vez de por lote.

### A linha de base, medida

```
run 36036473391 · main · 8/8 shards · SEM TOTAL INCOMPLETO
OK=347 · FAIL=11 · rótulos ausentes: 3
```

| | valor |
|---|---|
| rótulos **distintos** em `FAIL` | **13** |
| dos quais `setup-*` | **5** |
| rótulos **ausentes** | **3** (dois do mesmo cenário) |

**Os 13 em FAIL:**
```
setup-s75 · setup-s76 · setup-s87-baseline · setup-s158-baseline · setup-s175-baseline
credential-guard-script-integrity/detected
git-branch-guard-global-script-integrity/detected
git-branch-guard-dedup/baseline-skips-project-entry
git-branch-guard-dedup/double-slash-tolerance
git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload
sandbox-gap-e/direction-a-detected
sandbox-walkdir-reintroduced/direction-b-detected
scaffold-update-chmod-removed/direction-c-detected
```

**Os 3 ausentes:**
```
release-tag-parity/success/baseline-clean
release-tag-parity/forge-commit-diverges-update-ref/baseline-clean
sandbox-gap-e/direction-a-baseline
```

### 🔴 A hipótese que organiza o trabalho — e que a Wave 0 deve REFUTAR ou confirmar

**Um `setup` que falha aborta o cenário inteiro.** Os 5 `setup-*` são candidatos a causa comum dos
demais, não defeitos independentes:

| setup | gate |
|---|---|
| `setup-s75`, `setup-s76` | `check-release-tag-parity.sh failed against the UNMODIFIED Go binary` |
| `setup-s87-baseline`, `setup-s158-baseline` | `check-release-tag-parity.sh já reprova com binário real` |
| `setup-s175-baseline` | `check-update-parity.sh já reprova com o binário real` |

E os 2 ausentes de `release-tag-parity/*/baseline-clean` são **do mesmo gate** dos quatro primeiros.

🔴 **Tratar 13 como 13 seria repetir o erro do `IsAbs`** — 14 estimados, 2 entregues — que este
projeto já pagou e documentou.

### A varredura de issues achou o mecanismo, e ele tem medição de terceiro

O **#307** (consumidor externo, com medição) nomeia `check-release-tag-parity.sh` entre os **3 gates
de PATH curado** que falham no Windows, e descreve o mecanismo:

- no Git for Windows sem `winsymlinks`, o `ln -s` **degrada para cópia**;
- o `python3` copiado **não inicia** (`error while loading shared libraries`);
- 🔴 **e a guarda de vacuidade atribui a falha ao `git`**, quando é o `python3`:
  > `vacuity guard failed — git does not resolve on NO_FORGE_PATH … for a native child process`

Se a Wave 0 confirmar, **4 dos 13 rótulos são o #307**, que já está medido — e esta REQ o absorve em
vez de redescobri-lo.

⚠️ O **#308** (MSYS expande `{owner}/{repo}` ao reconstruir `argv` de pai nativo) é da mesma família
de fronteira MSYS e é candidato para os rótulos de `git-branch-guard-dedup/*`. **Candidato, não
membro** — entra com medição.

## Acceptance Criteria

- [ ] **Triagem por causa, não por rótulo**: cada um dos 13 `FAIL` e dos 3 ausentes recebe um
      veredito de **mecanismo**, e os rótulos são agrupados por causa. 🔴 Para cada grupo, a frase de
      fechamento: *"corrijo esta causa, exatamente estes rótulos fecham, e nenhum outro"*
- [ ] **A hipótese do `setup` é medida, não assumida** — quantos dos 8 rótulos não-`setup` fecham
      quando o `setup` do mesmo cenário passa? Se a resposta for "nenhum", a hipótese cai e isso fica
      escrito
- [ ] **O #307 é absorvido ou descartado com medição** — se o mecanismo for o mesmo, os rótulos dele
      entram nesta REQ e a issue é referenciada; se não for, a diferença fica escrita
- [ ] Cada grupo corrigido tem **falsificação nas duas direções**, exercitada no Windows
- [ ] 🔴 **Recontagem no CI ao fim de cada wave**, com o delta **atribuído** ao grupo corrigido —
      sem isso não se sabe qual correção funcionou
- [ ] `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- **Não** trata o **#421** (bit de execução em NTFS `noacl`). Mecanismo medido e distinto:
  permissão, não caminho. Tem issue própria.
- **Não** trata os dois classificados como outra causa na REQ anterior, ambos com medição escrita:
  `.venv/bin/python` (venv no Windows usa `Scripts/`, não symlink em `bin/`) e `trackfw barrier`
  saindo **2** onde o cenário espera **1**.
- **Não** recalibra `falsify-scenario-weights.json` nem mexe no intervalo de queda esperada
  (`~442`) do censo. 🔴 A queda medida é **501**, fora do intervalo, e isso é **correto** sobre este
  corpus — a base ARM64 de 2026-09-08 é pré-v8. Alargar o intervalo para calar o aviso seria maquiar.
- **Não** promete "Windows verde". O entregável de cada wave é **um grupo fechado e a contagem
  re-medida**.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`
