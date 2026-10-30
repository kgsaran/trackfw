# #634 medido numa conta Windows sem privilégio de symlink (2026-10-30)

ML-1B do `ROADMAP-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md`
(AC4 da REQ). Medido por `zeus-tf` na VM `Windows-Lab` (Windows 11 ARM64, `go1.27.0 windows/arm64`).

## Ambiente

- A conta `Lab`, usada no SSH, **tem** `SeCreateSymbolicLinkPrivilege` (`whoami /priv` → `Ativada`):
  não reproduz o issue.
- Criada a conta padrão `tfnopriv`, membro só de `Usuários` (`S-1-5-32-545`), fora de
  `Administradores`. Modo de Desenvolvedor desligado
  (`AppModelUnlock\AllowDevelopmentWithoutDevLicense` ausente).
- Política: `SeCreateSymbolicLinkPrivilege = *S-1-5-32-544` (só Administradores). Para rodar por
  tarefa agendada, a `tfnopriv` recebeu `SeBatchLogonRight`, que não concede symlink.
- Execução: tarefa agendada como `tfnopriv` (`RunLevel Limited`), clone em `C:\work\tf634`,
  `PATH` com Git Bash (`C:\Program Files\Git\bin`, `usr\bin`) e Go, `GOFLAGS=-buildvcs=false`.
  O `whoami /priv | findstr Symbolic` da própria tarefa não imprime nada: o token não tem o privilégio.

## Resultado

| árvore | comando | resultado |
|---|---|---|
| `origin/main` `45ed483` | `go test ./internal/generators/ -run TestAttentionScripts_FallbackWithoutJQ -count=1 -v` | **FAIL** — `scaffold_test.go:432: Arquivo de atenção não foi encontrado no modo fallback: open docs\roadmaps\.trackfw-attention.json` (a mesma linha do issue) |
| `45ed483` + `internal/generators/scaffold_test.go` do ML-1A (sha256 `e42f91c3…6a93fe98`, conferido na VM) | o mesmo | **PASS** (1,85 s) |

A única diferença entre as duas execuções é o arquivo de teste: mesma conta, mesmo clone, mesmo
commit de base.
