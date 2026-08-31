"""Detecção de terminal interativo, confiável no Windows.

Por que existe: `sys.stdin.isatty()` devolve True para `NUL` no Windows, porque
`NUL` é um character device e o Windows classifica character device como TTY. O
resultado é `trackfw init` entrando no wizard de identidade em contexto não
interativo e morrendo com `EOF when reading a line`.

Go e Node não têm o problema:

  Go    cbterm.IsTerminal -> windows.GetConsoleMode(handle, &st) == nil
        (charmbracelet/x/term@v0.2.2/term_windows.go:16)
  Node  process.stdin.isTTY -> tipo de handle do libuv

Este módulo usa **o mesmo GetConsoleMode do Go**, de propósito: o que um console
real fizer para o Go, fará para o Python. Não é uma heurística paralela.

O `isatty()` continua sendo a base — o `GetConsoleMode` só **estreita** o resultado,
e só no Windows. Em Linux e macOS nada muda.

Falha para False em qualquer exceção: teste e pipeline substituem `sys.stdin` por
objetos sem `fileno()`, e nesse caso o comportamento seguro é não promptar.
"""

import sys


def _windows_is_console(stream) -> bool:
    """True se o handle do stream for um console de verdade.

    Espelha `isTerminal` do charmbracelet/x/term: `GetConsoleMode` ter sucesso é a
    definição de console no Windows. `msvcrt.get_osfhandle` traduz o fd do CRT que
    o Python expõe para o HANDLE do sistema que o Go já recebe pronto de `Fd()`.
    """
    try:
        import ctypes
        import msvcrt
        from ctypes import wintypes

        handle = msvcrt.get_osfhandle(stream.fileno())
        mode = wintypes.DWORD()
        return bool(
            ctypes.windll.kernel32.GetConsoleMode(
                wintypes.HANDLE(handle), ctypes.byref(mode)
            )
        )
    except Exception:  # noqa: BLE001 — sem handle utilizável não há console
        return False


def _is_interactive(stream) -> bool:
    try:
        if not stream.isatty():
            return False
    except Exception:  # noqa: BLE001 — stream substituído sem isatty()
        return False
    if sys.platform == "win32":
        return _windows_is_console(stream)
    return True


def stdin_is_interactive() -> bool:
    """Substitui `sys.stdin.isatty()` na decisão de promptar."""
    return _is_interactive(sys.stdin)


def stdout_is_interactive() -> bool:
    """Substitui `sys.stdout.isatty()` na decisão de emitir cor."""
    return _is_interactive(sys.stdout)
