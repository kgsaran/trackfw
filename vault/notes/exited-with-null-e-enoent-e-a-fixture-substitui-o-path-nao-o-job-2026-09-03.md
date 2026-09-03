---
title: "`exited with null` é ENOENT, e quem substitui o $PATH é a fixture — não o job de CI"
tags: [windows, pathext, ci, fixture, diagnostico, gotcha]
date: 2026-09-03
related: [[job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30]]
---

## Sintoma

No job `windows-full-suites`, duas mensagens que parecem problema de ambiente:

```
git symbolic-ref --short HEAD exited with null
git not found in current $PATH
```

A leitura natural é *"o home sintético do job quebrou o `git`"*. Foi essa a leitura do arquiteto ao
escrever o ML-1D, e ela está **errada**.

## Causa raiz

**`exited with null` não é "exit code null" — é ENOENT.** A string sai de
`npm/src/ship/runner.js:124`:

```js
`git ${args.join(' ')} exited with ${result.status}`
```

`result.status === null` é o que o `spawnSync` devolve quando **não encontra o executável**.

E o executável não é encontrado porque a **fixture substitui o `$PATH` inteiro**:

```js
npm/tests/ship.test.js:867,944    env: { PATH: tmpBin, HOME: tmpDir, ... }
```

`tmpBin` contém um symlink **sem extensão** chamado `git`. No Windows o `PATHEXT` exige `git.exe`.
O par Python é literal:

```python
pypi/tests/test_barrier.py:446    os.symlink(git_path, Path(curated) / "git")
```

## Por que o diagnóstico errado é caro

**O job de CI não podia ser a causa, porque a fixture descarta o `$PATH` do job.** Qualquer conserto
no `quality.yml` — semear `git` no PATH, ajustar o home — seria inerte: o processo filho nunca vê
aquele ambiente.

Pior: um conserto no job daria a impressão de ter funcionado se a contagem mudasse por outro motivo,
e o defeito real continuaria em duas fixtures.

## 🔴 Correção (ML-1E, 2026-09-03): o symlink NÃO é o bloqueio

A primeira redação desta nota dizia que *"o symlink ainda por cima exige Developer Mode para ser
criado"*. **É falso, e a evidência está no próprio log citado acima:** `git ... exited with null` é
emitida por `npm/src/ship/runner.js:124`, **dentro do produto já spawnado**. Se o `symlinkSync`
tivesse sido negado, o teste morreria **antes**, e essa mensagem não existiria.

**O bloqueio é só o nome sem extensão.** E a diferença muda o remédio: no Windows o processo roda com
o caminho do **alvo**, então o symlink mantém o wrapper do Git for Windows achando a instalação dele
— enquanto a **cópia**, que a premissa falsa exigiria, pode quebrá-lo. É a mesma classe do shim
assinado do `/usr/bin/git` do macOS, que o ML-1A mediu ao quebrar o POSIX na primeira tentativa.

Deixar a linha falsa faria o próximo agente projetar contra uma restrição que não existe, e escolher
cópia onde symlink é melhor.

## É a MESMA classe do binário sem `.exe`

O ML-1A corrigiu 17 testes Go onde `filepath.Join(dir,"trackfw")` não resolvia por falta de
extensão. **`npm/tests/ship.test.js` e `pypi/tests/test_barrier.py` são a mesma classe**, em outros
dois runtimes — e ficaram órfãos porque o ML-1A foi escopado como "só Go" e o ML-1D como "só
workflow".

**Lição de roteamento:** um defeito classificado pelo *sintoma* (mensagem de `git`) foi para o ML de
infraestrutura; classificado pelo *mecanismo* (PATHEXT), pertencia ao ML de fixtures. A atribuição
errada criou um ML que não podia entregar e deixou duas metades sem dono.

## Hipótese medida e descartada, para ninguém repetir

Semear `.gitconfig` de identidade no home sintético (home vazio → `git commit` = *"Author identity
unknown"*) **não desbloqueia nada**: todo helper que commita nos 3 runtimes seta identidade **local**
antes do primeiro commit — `internal/validator/validator_test.go:26`,
`pypi/tests/test_credential_guard_integrity.py:27`, `npm/tests/credential_guard_integrity.test.js:36`
— e os testes que exigem config global limpa pinam `GIT_CONFIG_GLOBAL` num arquivo vazio próprio.
Semear só acrescentaria variável não medida ao instrumento.

## Como foi descoberto

ML-1D da Wave 0 de Windows (`ROADMAP-2026-09-03-desmascarar-as-falhas-de-windows-...`). O agente
recebeu a tarefa de consertar o ambiente do job, **mediu antes de consertar**, e devolveu a
falsificação da premissa em vez do conserto pedido. Terceira das quatro premissas do arquiteto
derrubada na mesma wave.
