# `grep -qF` com `\n` embutido no padrão casa QUALQUER linha (falso positivo vácuo)

**Contexto:** `scripts/check-gates-falsify.sh`, Cenário 58 (ROADMAP-2026-08-16-handler-global-de-
erro-nos-entrypoints-node-e-python), braço baseline Node — provar que o `bin/trackfw` real
(com o handler global de erro) NÃO vaza frames de stack no repro real de `agents update --force`
sobre artefato unmanaged.

## Sintoma

```bash
out=$'Destino (project):\n  .codex/agents/trackfw-iac.toml\nError: Unmanaged artifact ...'
if grep -qF $'\n    at ' <<<"$out"; then
  echo "TRIGGERED"   # <- disparava mesmo com $out limpo, sem nenhum frame de stack
fi
```

O braço baseline falhava alegando vazamento de stack numa saída que, impressa literalmente, não
continha nenhuma linha `    at ...`.

## Causa raiz

Este ambiente resolve `grep` para `ugrep` (não GNU grep). Com `-F` (fixed strings), tanto GNU grep
quanto `ugrep` tratam um padrão contendo `\n` embutido como **múltiplos padrões separados por
linha**, unidos por OR — documentado no próprio grep(1): "If PATTERN contains a newline, it will
match a multi-line pattern **or** be interpreted as multiple patterns, one per line."
`ugrep` explicitou isso ao printar o erro decomposto: `\Q\E|\Q    at \E` — ou seja, o padrão
`$'\n    at '` virou DOIS padrões fixos: a **string vazia** (tudo antes do primeiro `\n`) e
`"    at "` (depois). Uma string vazia como padrão fixo **casa com qualquer linha não-vazia** —
então `grep -qF $'\n    at '` é vacuamente verdadeiro contra praticamente qualquer entrada
multi-linha, independente de conter ou não o frame de stack real.

Em uma primeira tentativa isolada, `ugrep` chegou a devolver **exit 2** (erro de parse, "empty
subexpression") em vez de casar — inconsistente entre invocações/contexto de shell. Não confiar no
exit code de `grep -F` com `\n` embutido para decidir a lógica de um `if`: o comportamento observado
foi tanto "erro" quanto "match vácuo" dependendo do contexto exato de invocação.

## Correção

Nunca usar `-F` com um padrão que contenha um `\n` literal quando a intenção é buscar uma
substring exata que atravessa uma quebra de linha. Alternativas:
- Se o marcador de interesse não depende de estar no início de uma linha (caso deste cenário —
  frames de stack V8 são sempre `    at ...`, 4 espaços de indentação são específicos o
  suficiente), usar só o trecho sem a quebra de linha: `grep -qF "    at "`.
- Se a adjacência de linha importar de verdade, usar `grep -Pzo` (NUL-separated, modo multi-linha)
  ou `awk`/`perl -0777`, nunca `grep -F` puro.

## Como detectar este bug em outros cenários do harness

Qualquer `grep -qF $'...\n...'` (ou equivalente `printf '...\n...'` passado como padrão fixo) neste
repositório está sujeito ao mesmo problema. Buscar por `-qF \$'.*\\\\n` nos scripts de
`check-gates-falsify.sh`/gates de paridade antes de copiar este padrão para um cenário novo.

Ver [index](index.md).
