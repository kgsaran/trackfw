---
status: wip
date: 2026-09-03
squad: ares-tf
req: "docs/req/REQ-2026-09-03-setenta-e-tres-das-duzentas-e-quarenta-e-seis-falhas-de-windows-nao-medem-nada-e-contaminam-qualquer-estimativa.md"
---

# Roadmap: Desmascarar as falhas de Windows que não medem nada, e recontar

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-03-setenta-e-tres-das-duzentas-e-quarenta-e-seis-falhas-de-windows-nao-medem-nada-e-contaminam-qualquer-estimativa.md

## Diagnóstico

Triagem do run `33742756936`: **246 falhas** — Go 86, Node 56, Python 104. **73 delas não são
defeitos, são bloqueios de medição**: nelas o código de produto **nunca executou**.

**30% das falhas não dizem nada sobre o produto.** Priorizar por essa contagem é priorizar por ruído.

**O entregável desta wave é uma contagem confiável, não gates verdes.** Os remendos são o meio.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Acceptance Criteria

- [ ] Os 4 bloqueios de medição removidos, cada um falsificado nas duas direções
- [ ] 🔴 Nenhum teste marcado `skip` para "desmascarar" — trocar bloqueio visível por invisível é
      pior que o defeito
- [ ] 🔴 Re-rodar e **recontar**: a nova contagem por runtime substitui a de hoje como base
- [ ] `make quality` verde

## Wave 1 — Os quatro bloqueios (paralelo — arquivos disjuntos)
> Dependências: nenhuma. Os 4 MLs tocam arquivos disjuntos e rodam **simultaneamente**.

### ML-1A — Binário de teste sem `.exe` no Windows
**Status:** ✅ Concluído · **Agente:** `ares-tf`
**Files affected:** `internal/commands/barrier_contract_test.go`, `barrier_test.go`, `root_test.go`
**Só Go.** `filepath.Join(dir,"trackfw")` + `exec.Command`: no Windows o `LookPath` exige extensão
conhecida. O arquivo existe e **não é executável** — 17 testes onde o produto nunca rodou.
**Critérios de aceite:**
- [x] Os 17 executam o binário no Windows
- [x] 🔴 Falsificação: revertendo o remendo, os 17 voltam a falhar **pelo mesmo motivo**
- [x] Nada fora dos arquivos de teste listados

### ML-1B — Isolação de home é vácua no Windows
**Status:** ✅ Concluído · **Agente:** `ares-tf`
**Files affected:** `pypi/tests/conftest.py`, `internal/validator/main_test.go`
O job já mediu (`ITEM 2 = REPRODUCED`) que **no Windows `%USERPROFILE%` vence nos 3 runtimes**.
`conftest.py:30` isola só `HOME` → os testes veem o home sintético do job, não o da fixture.
**Critérios de aceite:**
- [x] `HOME` **e** `%USERPROFILE%` isolados
- [x] 🔴 Falsificação: sem a correção, o assert de home bate no caminho errado
- [x] 🔴 **Controle:** em POSIX o comportamento **não muda** — comparar antes/depois

### ML-1C — CRLF do checkout, metade FIXTURE apenas
**Status:** ✅ Concluído · **Agente:** `ares-tf`
**Files affected:** `.gitattributes`
`core.autocrlf=true` é default no runner; goldens versionados chegam com `\r\n`.
🔴 **Só a metade fixture.** A metade de **produto** — parser de frontmatter cego a CRLF, que emitiu
frontmatter duplicado em `TestRenderOpenCodeAgent` — é **decisão de arquitetura** e **não entra**.
**Critérios de aceite:**
- [x] `eol` fixado para os goldens; a metade fixture do grupo CRLF some
- [x] 🔴 Falsificação: sem a regra, os goldens voltam a divergir
- [x] 🔴 Nenhuma linha de produto tocada — o defeito do parser permanece **visível**

### ML-1D — `git`/branch ausentes no home sintético
**Status:** ✅ Concluído · **Agente:** `ares-tf`
**Files affected:** `.github/workflows/quality.yml`
`git symbolic-ref --short HEAD exited with null`; `git not found in current $PATH`. Vive no
workflow, **não nos 3 CLIs** — zero trabalho de paridade.
**Critérios de aceite:**
- [x] Os testes que dependem de `git` deixam de quebrar por ambiente
- [x] 🔴 **Controle:** nenhum teste passa a ser pulado para conseguir isso

### ML-1E — As metades órfãs da classe PATHEXT, em Node e Python
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Files affected:** `npm/tests/ship.test.js`, `pypi/tests/test_barrier.py`
**Origem:** achado do ML-1D, que **falsificou a premissa do próprio ML**.

As mensagens `git ... exited with null` e `git not found in current $PATH` **não vêm do home
sintético**. `exited with null` é **ENOENT** do `spawnSync` (`npm/src/ship/runner.js:124`), e o
executável não é achado porque a **fixture substitui o `$PATH` inteiro** por um `tmpBin` que contém
um symlink **sem extensão** chamado `git`. O `PATHEXT` do Windows exige `git.exe`.

```
npm/tests/ship.test.js:867,944   env: { PATH: tmpBin, HOME: tmpDir, ... }
pypi/tests/test_barrier.py:446   os.symlink(git_path, Path(curated) / "git")
```

🔴 **É a MESMA classe do ML-1A**, em outros dois runtimes. Ficou órfã por **erro meu de atribuição**:
classifiquei pelo *sintoma* (mensagem de `git`) e mandei para o ML de infraestrutura; pelo
*mecanismo* (PATHEXT) pertencia ao ML de fixtures. O ML-1A foi escopado "só Go" e o ML-1D "só
workflow" — ninguém ficou com isto.

O ML-1A já resolveu o par Go com `placeExecutableInPath` (`internal/commands/ship_test.go`), que
mantém **symlink em POSIX** e usa hardlink→cópia só no Windows. 🔴 **Reaproveite a forma, não
copie cega:** ele mediu que trocar o symlink por cópia **quebra o POSIX** — em macOS o `/usr/bin/git`
é um shim assinado que morre fora do diretório dele.

**Critérios de aceite:**
- [x] Node e Python montam o `git` da fixture de forma resolvível no Windows
- [x] 🔴 **Controle POSIX:** `npm test` e `pytest` com os mesmos números de antes — o remendo do Go
      quebrou o POSIX na primeira tentativa; não repita
- [x] 🔴 Nenhum teste marcado `skip`
- [x] Nada fora dos 2 arquivos


## Auditoria da Wave 1 — arquiteto, 2026-09-03

```
make quality QUALITY_EXIT=0, zero FAIL · validate exit 0
controle POSIX: Node 859 pass / 0 fail / 0 skipped · Python 1604 passed  (identicos ao baseline)
AC7 verificado: ZERO skip introduzidos nos 5 MLs (git diff -U0 | grep skip -> 0)
```

🔴 **QUATRO das minhas CINCO premissas foram derrubadas por medição.** Este é o resultado da wave, e
vale mais que os remendos:

| ML | veredito | o que a medição mostrou |
|---|---|---|
| 1A | **confirmado** | + 2 bloqueios extras. O melhor: `TestUnknownCommand_NeverExecutesExternalBinary` **passava por vacuidade** — o plugin falso era `#!/bin/sh`, sem PATHEXT nem shebang no Windows, então não poderia rodar nem que o trackfw tentasse |
| 1B | **derrubado** | Não é vacuidade, é **divergência de canal**. O commit `c88b81e` fez a produção ler `HOME` em qualquer plataforma; quem lê `%USERPROFILE%` é o **teste**. 🔴 O remendo "óbvio" que eu teria mandado **reintroduziria** o defeito |
| 1C | **derrubado** | Fixar `eol` nos goldens **cura ZERO** — o CRLF entra pelos **dois lados**, porque o asset embedado também chega com `\r\n`. O agente chegou a pinar e **removeu**, porque apagaria o `\r` que é a **evidência** do parser cego |
| 1D | **derrubado** | As mensagens de `git` não vêm do home sintético. `exited with null` é **ENOENT**, e quem descarta o `$PATH` é a **fixture** — conserto no `quality.yml` seria **inerte**, o filho nunca vê aquele ambiente |
| 1E | **confirmado** | + achou que a **minha nota de vault** afirmava falsamente que symlink exige Developer Mode. A evidência contra estava no log que eu citei: a mensagem vem de **dentro do produto já spawnado** |

**Dois erros meus de método, expostos:**

1. **Classifiquei um defeito pelo sintoma em vez do mecanismo.** A mensagem de `git` me levou ao ML
   de infraestrutura; pelo PATHEXT pertencia ao ML de fixtures. Resultado: um ML que **não podia
   entregar** e duas metades órfãs (daí o ML-1E).
2. **Escrevi restrição falsa em nota de vault**, e teria feito o próximo agente escolher **cópia**
   onde symlink é melhor.

🔴 **Correção de expectativa para o ML-2A:** a Wave 0 desmascara **menos** que os 73 estimados. O
ML-1C mediu **18 → 14** falhas no grupo dele: só ~4 por runtime eram medição bloqueada, **o resto é
defeito de produto e PERMANECE na contagem.** O número novo será pior que o esperado, e mais honesto.

**Três achados que o ML-2A precisa para não rotular errado:**
- `monkeypatch.setattr("os.path.expanduser", ...)` (9 sítios em 3 arquivos Python) fica
  **inalcançável** no Windows — o `home_dir()` retorna `HOME` antes do `expanduser`. **O patch falha
  em silêncio**; é fatia das 104 falhas Python e **não** é defeito de fixture.
- A raiz do `.gitattributes` **não pode** carregar a regra `eol=lf` de que ela própria precisa: 3
  testes exigem igualdade byte-a-byte com constante de **produto** nos 3 CLIs, e o conserto vazaria
  caminhos deste repo para o `.gitattributes` de todo adotante. Reprova em **2 de 3 runtimes** —
  Python passa por universal newlines, então medir num runtime só dá conclusão errada.
- `TestShip_Integration_GracefulDegradation_RealBinary` e os pares Node/Python montam `PATH` de
  **entrada única**, sem `System32`. **Não assumir que ficam verdes** — há um segundo bloqueio não
  medido ali.

**Residual sem dono, declarado:** `internal/commands/barrier.go:743` hard-coda
`exec.Command("sh","-c",command)` — defeito de **produto** no runner de gates, deixado **visível** de
propósito.

## Wave 2 — Recontar
> Dependências: Wave 1 mergeada **e CI executado**.

### ML-2A — Re-rodar, recontar e registrar a base nova
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
**É o entregável da REQ.** A contagem nova por runtime, com o delta explicado: quantas sumiram por
desmascaramento e quantas restam como defeito real.
**Critérios de aceite:**
- [ ] Contagem nova registrada, por runtime, com o delta
- [ ] 🔴 A triagem de causas-raiz é **revisada** contra a contagem nova — grupos podem ter mudado de
      tamanho, e o grupo B (~56, mecanismo desconhecido) pode ter encolhido ou não

## Verificação que só o CI fecha

O run de Windows na `main` depois do merge. **Nenhuma métrica local substitui.**

## Barreira final

Arquiteto. **Sem `hades-tf`** — não há superfície de ataque: são arquivos de teste, `.gitattributes`
e workflow. `hefesto-tf` se algum ML crescer além dos arquivos listados.
