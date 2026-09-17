---
status: Open
date: 2026-08-29
author: claude
adr: "docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md"
roadmap: "docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-isatty-do-python-devolve-true-para-nul-no-windows.md"
---

# REQ: isatty do Python devolve True para NUL no Windows

> Date: 2026-08-29 | Status: Open

## Motivation

`sys.stdin.isatty()` devolve `True` para `NUL` no Windows. `NUL` e um character device, e o Windows
reporta character device como TTY. Medido:

```
                     stdin de /dev/null
python  sys.stdin.isatty()      True
node    process.stdin.isTTY     undefined
```

Consequencia: `trackfw init` do Python entra no wizard de identidade em contexto **nao interativo**
e morre com `trackfw init: EOF when reading a line`. A guarda existe —
`pypi/trackfw/commands/init.py:117` faz `if not skip_identity_wizard and sys.stdin.isatty()` — ela
so nao funciona nesta plataforma.

Go e Node nao tem o problema, e por motivos diferentes de sorte:

- Go usa `cbterm.IsTerminal`, que no Windows e literalmente
  `windows.GetConsoleMode(handle, &st) == nil`
  (`charmbracelet/x/term@v0.2.2/term_windows.go`).
- Node usa `process.stdin.isTTY`, que vem do tipo de handle do libuv.

O `isatty()` do Python e o unico que confia na classificacao de character device.

### Por que aparece so agora

Estava mascarado enquanto o Python lia a home real: `_identity_file_exists(home)` era verdadeiro,
`skip_identity_wizard` virava `True` e o wizard nunca rodava. Depois de
`REQ-2026-08-29-node-e-python-ignoram-home-no-windows` o Python passou a ver home vazia de verdade,
e o caminho do wizard passou a ser exercitado.

### O que isso bloqueia

`scripts/check-artifact-parity.sh` roda `python3 -m trackfw init` num fixture e aborta com
`set -euo pipefail` quando o init sai nao-zero. Enquanto isso existir, o gate nao passa no Windows e
o **ML-2A** de `REQ-2026-08-29-slug-de-artefato-no-python-diverge-de-go-e-node` segue bloqueado —
pela terceira parede: primeiro o CRLF, depois a home, agora esta.

## Superficie

| Runtime | Forma | Sites | Confiavel no Windows |
|---|---|---|---|
| Python | `sys.stdin.isatty()` | 7 | **nao** |
| Python | `sys.stdout.isatty()` (cor em `validate.py:24`) | 1 | **nao** |
| Node | `process.stdin.isTTY` | 8 | sim |
| Go | `cbterm.IsTerminal` | 5 | sim |

## Acceptance Criteria

- [x] Com stdin nao interativo, `trackfw init` do Python **conclui** em vez de promptar, verificado
      em execucao real
      → **(a) ENTREGUE.** 2026-09-10, execução real: `python3 -m trackfw init < /dev/null` →
      `exit 0`, **21 arquivos** produzidos. Não promptou.
- [x] O comportamento casa com o do Go **por construcao**: mesma chamada de sistema
      (`GetConsoleMode`), nao uma heuristica paralela
      → **(a) ENTREGUE.** O Go chega ao `GetConsoleMode` por
      `cbterm.IsTerminal(uintptr(os.Stdin.Fd()))`, e o próprio `pypi/trackfw/tty.py:10` documenta a
      cadeia: `Go cbterm.IsTerminal -> windows.GetConsoleMode(handle, &st) == nil`.
      🔴 **Quase reportei o contrário.** Meu primeiro teste foi `git grep GetConsoleMode -- internal`,
      que devolve **zero** — porque no Go a chamada está **dentro da biblioteca**, não no nosso
      código. Grep literal por nome de syscall não mede uso de syscall.
- [x] `sys.stdout.isatty()` de `validate.py` recebe o mesmo tratamento — hoje ele emitiria cor para
      dentro de arquivo redirecionado
      → **(a) ENTREGUE.** `pypi/trackfw/commands/validate.py:25`:
      `return hasattr(sys.stdout, "isatty") and stdout_is_interactive()` — consulta o helper, não o
      `isatty()` cru.
- [x] O caminho POSIX nao muda: em Linux e macOS continua sendo `isatty()` puro
      → **(a) ENTREGUE por leitura de fonte**, `pypi/trackfw/tty.py:53-58`:
      ```python
      if not stream.isatty(): return False
      if sys.platform == "win32": return _windows_is_console(stream)
      return True
      ```
      O estreitamento é guardado por `sys.platform == "win32"`; fora dele o resultado é o `isatty()`.
      🔴 **Limite declarado:** isto é leitura de fonte, **não execução em Linux ou macOS** — esta
      máquina é Windows. A construção é inequívoca, mas a medição naquelas plataformas não existe.
- [x] Gate impede regressao e **falha** com um site restaurado — nao-vacuidade verificada
      → **(a) ENTREGUE.** Falsificado em 2026-09-10 restaurando o sítio (`return True` no lugar de
      `return _windows_is_console(stream)`): `check-tty-detection.sh` → `exit 1`, com a mensagem
      `isatty() mente (True) e stdin_is_interactive() repetiu a mentira (True)`. Árvore intacta →
      `exit 0`.
      🔴 **Três mutações anteriores minhas não valeram, e cada uma por um motivo diferente:** a 1ª
      atingiu um comentário, a 2ª um docstring, e a 3ª — na chamada real — mudava o retorno para
      `False`, que é **a resposta certa para `NUL`**. Só a mutação que restaura o comportamento
      **antigo** exercita o discriminante. Chamar o gate de cego em qualquer uma das três teria sido
      um falso achado publicado.
- [x] `check-artifact-parity.sh` passa, desbloqueando o ML-2A do slug
      → **(a) ENTREGUE.** `exit 0` — `9 artifact types × 3 runtimes`.
- [ ] Sem regressao na suite pypi por lista nomeada contra 105 falhas
      → **(b) NAO ENTREGUE** — convertido de `(d)` em 2026-09-11 pela
      `REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados` (AC5). A afirmação sobre agosto é
      **irrecuperável**: um baseline tirado hoje não prova "sem regressão desde agosto", só daqui
      para a frente — e daqui para a frente quem verifica é o ratchet do upstream, no nosso CI.
      Registro de 2026-09-10, que continua verdadeiro: O AC exige comparação contra **lista nomeada**, e a lista das
      105 falhas de 2026-08-29 **não foi versionada**. Rodar a suite hoje daria um número, e comparar
      número com número é o que o próprio AC recusa.
      **O que faltaria:** a lista de falhas por nome daquela corrida.

## Risco declarado

**Nao consigo verificar o caso positivo nesta maquina.** Esta sessao nao tem console anexado: ate a
execucao "interativa" mede `isatty()=False`. Consigo provar que o falso positivo some, nao que um
terminal de verdade continua promptando.

A mitigacao e casar com o Go por construcao, usando o mesmo `GetConsoleMode`: o que um console real
fizer para o Go, fara para o Python. Um teste manual num terminal de verdade fecha o buraco e fica
registrado como pendencia da REQ.

## Linked ADR

ADR: docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-isatty-do-python-devolve-true-para-nul-no-windows.md
