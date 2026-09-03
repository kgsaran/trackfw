---
title: isolação de home no Windows — o defeito é divergência de CANAL, não vacuidade (a nota de 2026-08-30 está desatualizada)
tags: [windows, teste, fixture, gotcha, homedir]
date: 2026-09-03
related: [[job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30]], [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## A afirmação que ficou obsoleta

A nota `job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30` (seção "Vetor de ameaça")
afirma que a isolação de `$HOME` das suítes é **vácua** no Windows, porque "a produção não lê `HOME`
lá". O comentário AC12 em `.github/workflows/quality.yml` (~linhas 159-200) repete a mesma frase.

**Isso deixou de ser verdade em 2026-09-01**, com o commit `c88b81e`, que introduziu o shim de home
nos 3 CLIs:

- `internal/homedir/homedir.go:32` — `if h := os.Getenv("HOME"); h != "" { return h, nil }`, **em
  qualquer plataforma**, antes de cair em `os.UserHomeDir()`.
- `pypi/trackfw/homedir.py:58` — mesma preferência por `HOME`, com guarda `sys.platform == "win32"`.
- `npm/src/homedir.js` — equivalente.

Ou seja: a produção que passa pelo shim **lê `HOME` no Windows**. A isolação não é vácua.

## O que o defeito realmente é

**Divergência de canal dentro do mesmo processo.** A produção resolve a home por `HOME` (shim);
qualquer teste, fixture ou gate que calcule a expectativa pelo **primitivo da plataforma**
(`os.path.expanduser("~")` / `os.UserHomeDir()` / `os.homedir()`, que no Windows leem
`%USERPROFILE%` e **nunca** consultam `HOME`) enxerga **outra** home. Duas homes, um processo.

A evidência do run `33742756936` diz exatamente isso, e a direção importa:

```
['C:\...\trackfw-pypi-test-home-h4vri31t\adr'] != ['D:\a\_temp\winhome\adr']
   ^ actual: home da FIXTURE (a produção achou via HOME)   ^ expected: home do JOB (via %USERPROFILE%)
```

Se a isolação fosse vácua, o **actual** seria a home do job — é o oposto do que foi medido.

## O conserto e por que não sobre-isola

Apontar `HOME` **e** `USERPROFILE` para o **mesmo** diretório sintético (`pypi/tests/conftest.py`,
`internal/validator/main_test.go`). Colapsar os dois canais não cega a produção: não há terceiro
canal que ela leia legitimamente (`HOMEDRIVE`/`HOMEPATH` só entram no `ntpath.expanduser` quando
`USERPROFILE` está ausente — e passa a estar presente). Em POSIX `%USERPROFILE%` é inerte: nem o
stdlib nem o produto o leem. Controle medido em macOS: `pytest pypi/tests/` 1604 passed antes e
1604 passed depois; `go test ./internal/validator/` ok nas duas.

## O achado que generaliza

Quando um shim de plataforma é introduzido no produto, **toda nota e todo comentário de CI que
descrevia o defeito pré-shim vira armadilha** — continua plausível, e o próximo agente conserta o
sintoma errado (ex.: aqui, "fazer o teste ler `USERPROFILE`" em vez de "fazer os dois canais
concordarem" — o primeiro reintroduz a divergência). Ao ler diagnóstico herdado que nomeia uma
primitiva de plataforma, **conferir se ainda existe chamada direta a ela na produção** antes de agir.

## Residual conhecido

A correção é **por sessão**, não **por teste**. Isolações por teste que sobrescrevem só `HOME`
(`globalGuardHome` em `internal/validator/validator_git_branch_guard_test.go:237`; `setUp`s em
`pypi/tests/test_git_branch_guard_validator.py` etc.) deixam `USERPROFILE` apontando para a home de
sessão no Windows. É estritamente melhor que hoje (o vazamento cai num diretório descartável, não no
perfil real do runner), mas não é isolação por teste. O CLI Node não tem equivalente de sessão — cada
arquivo de teste seta `HOME` por conta própria e só `npm/tests/context_cli.test.js:66` seta as duas.
