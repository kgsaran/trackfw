---
domain: security
date: 2026-08-21
tags: [git, security, content-addressing, refs-replace]
---

# `git show <sha>:<path>` honra `refs/replace/` por padrão — contorna ancoragem por sha

## O achado

`git show <sha>:<path>` honra `refs/replace/` refs por padrão. Um atacante com acesso local de
escrita pode criar `.git/refs/replace/<forge-sha>` apontando para um commit forjado com uma simples
escrita de arquivo — sem invocar nenhum comando git, portanto o guard de branch não é relevante.
Com esse ref no lugar, `git show <forge-sha>:CHANGELOG.md` retorna o conteúdo do commit forjado,
não o conteúdo do commit do forge.

Medido diretamente (ML-3A, 2026-08-21): fixture em
`scratchpad/ml3a-fixture/`, resultado conclusivo — "VULNERABLE: git show honored the replace ref".

## Por que importa

A ADR-2026-08-21 afirma: "Objetos git são endereçados por conteúdo — dado um sha, o conteúdo é
criptograficamente determinado." Isso é verdade para o object store bruto, mas `git show` não lê o
object store bruto — passa pela camada de substituição de objetos (`refs/replace/`), que transparentemente
redireciona a leitura. O sha vem do forge; o conteúdo servido pode ser controlado localmente pelo
atacante.

Isso re-abre os dois danos que a ADR-2026-08-21 se propôs a fechar:
- P3 (versão): version files no commit forjado localmente definem a versão publicada na tag
- P4 (mensagem): CHANGELOG.md do commit forjado vira o `tagMessage` publicado sob a identidade real

## O fix

Adicionar `--no-replace-objects` ao invocation de `git show` nas três implementações:

- Go (`internal/commands/release.go`, `defaultReleaseReadCommittedFile`):
  `exec.Command("git", "--no-replace-objects", "show", sha+":"+path)`
- Node.js (`npm/src/release/runner.js`, `defaultReadAtCommit`):
  `spawnSync('git', ['--no-replace-objects', 'show', ...])`
- Python (`pypi/trackfw/release/runner.py`, `default_read_at_commit`):
  `subprocess.run(["git", "--no-replace-objects", "show", ...])`

`GIT_NO_REPLACE_OBJECTS=1` como variável de ambiente do subprocess tem o mesmo efeito.

## Mecanismo adicional: `.git/info/grafts` — MEDIDO, NÃO É UM VETOR

**Medição executada (ML-4A, 2026-08-21):** fixture isolada (fora do repo trackfw, sem hooks),
dois commits A (legítimo, "legitimate content") e B (forjado, "FORGED content"). Graft instalado
em `.git/info/grafts` declarando A com pai virtual B. Resultado:

- `git show SHA_A:CHANGELOG.md` → "legitimate content" (inalterado pelo graft)
- `git log SHA_A --oneline` → mostra B como pai (graft age aqui)
- `git cat-file -p SHA_A` → ponteiro `tree` idêntico antes e depois do graft

**Por quê:** grafts substituem a **lista de pais** de um commit (parent-chain traversal — `git
log`, `git rev-list`). `git show <sha>:<path>` resolve o objeto pelo sha, lê o ponteiro `tree`,
e percorre a árvore de arquivos — esse caminho nunca toca os pais. Grafts não têm como redirecionar
`git show <sha>:<path>`.

**Taxonomia das camadas de indireção do git:**
- `refs/replace/` — substitui **identidade do objeto**: git trata `<sha>` como se fosse outro
  objeto antes de qualquer leitura. Quebra a garantia "sha determina conteúdo". **Bloqueado por
  `--no-replace-objects`.** Vetor confirmado.
- `.git/info/grafts`, `.git/shallow` — alteram **metadados do grafo** (lista de pais, profundidade
  de histórico). Nunca tocam a árvore do objeto requisitado. Não são vetores para `git show
  <sha>:<path>`. Grafts são deprecated no git 2.50.1.
- `.git/objects/info/alternates`, `GIT_ALTERNATE_OBJECT_DIRECTORIES` — adicionam **fontes de
  objetos**. Não podem forjar: objetos são content-addressed, então um alternate só pode fornecer
  um objeto cujo conteúdo hash para o sha requisitado. Pior caso = objeto ausente → recusa (caminho
  já coberto pelo Scenario 15). Não são vetores de forjaria.

**Conclusão:** grafts não precisam de cobertura adicional nem de mitigação com `core.grafts`.
A única rota de grafts para o fluxo de leitura de árvore seria `git replace --convert-graft-file`,
que produz entradas `refs/replace/` — já bloqueadas pela flag.

## Estrutura do guard

O `trackfw-git-branch-guard.sh` não menciona `git replace` em nenhum bloco. Como o ataque pode
ser feito com escrita de arquivo direta (sem invocar `git replace`), o guard é irrelevante para
este vetor.

## Observação: `os.Environ()` bruto em `defaultReleaseReadCommittedFile`

`internal/commands/release.go`'s `exec.Command` herda o `os.Environ()` bruto — sem passar por
`cleanGitEnv()` (que existe em `internal/validator/validator_git_exec.go` e remove variáveis
`GIT_*`). Para o padrão `git show <sha>:<path>` (sha-addressed), redirecionamento via `GIT_DIR`
faz o objeto ficar ausente (recusa) em vez de forjado — menos crítico que o caso `HEAD:path`
(ref-addressed) fechado pelo `cleanGitEnv` do validador (Cenário 54 em check-gates-falsify.sh).
Não corrigido em ML-4A; reportado para `hades-tf`/ML-4B.

## Relevância futura

Sempre que `git show <sha>:<path>` for usado como mecanismo de ancoragem de conteúdo por sha,
verificar se `--no-replace-objects` é necessário. A ausência deste flag não é óbvia e não aparece
nas error messages — o conteúdo errado é servido silenciosamente.
