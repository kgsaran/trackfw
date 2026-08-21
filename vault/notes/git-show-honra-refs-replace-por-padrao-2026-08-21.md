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

## Mecanismo adicional: `.git/info/grafts`

O arquivo `.git/info/grafts` (mecanismo mais antigo, deprecated mas ainda suportado) cria
substituições similares. `--no-replace-objects` NÃO desabilita grafts. Se isso for considerado
vetor adicional, a mitigação seria `git -c core.grafts=/dev/null show ...` ou inicialização de
repositório com `core.grafts` explicitamente desapontando para `/dev/null`. Na prática, grafts são
raros e ausentes na maioria dos clones modernos.

## Estrutura do guard

O `trackfw-git-branch-guard.sh` não menciona `git replace` em nenhum bloco. Como o ataque pode
ser feito com escrita de arquivo direta (sem invocar `git replace`), o guard é irrelevante para
este vetor.

## Relevância futura

Sempre que `git show <sha>:<path>` for usado como mecanismo de ancoragem de conteúdo por sha,
verificar se `--no-replace-objects` é necessário. A ausência deste flag não é óbvia e não aparece
nas error messages — o conteúdo errado é servido silenciosamente.
