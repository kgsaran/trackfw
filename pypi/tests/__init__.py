"""Pacote de testes do trackfw.

Força UTF-8 na saída antes de qualquer módulo de teste ser importado.

Os testes chamam funções de biblioteca direto (`scaffold`, `generate_claude_commands`,
`validate`…), sem passar pelo `main()` do CLI — então não herdam o
`_force_utf8_output()` do entry point. Como essas funções imprimem `✓`, `⚠` e texto
acentuado, num console cp1252 elas estouram `UnicodeEncodeError` e derrubam 16 testes
que nada têm a ver com codificação.

Sem isto a suíte só roda com `PYTHONUTF8=1` na frente, que é justamente o atrito
que `_force_utf8_output` existe para eliminar.
"""

from trackfw.cli import _force_utf8_output

_force_utf8_output()
