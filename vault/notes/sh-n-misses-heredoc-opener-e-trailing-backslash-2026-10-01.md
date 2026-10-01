# sh -n misses heredoc opener and bare trailing `\` — measured 2026-10-01

> Hades · 2026-10-01 · REQ #491 · context: `barrier` per-line gate detection

## O que foi medido

`sh -n -c '<cmd>'` foi testado contra vetores sintéticos em bash 3.2 (`/bin/sh` macOS) e `/bin/dash`:

| Vetor | `sh -n` exit | Tipo |
|---|---|---|
| `cat <<EOF` (heredoc opener sem corpo) | 0 | **FN** — ambos |
| `echo word \` (trailing-`\` puro) | 0 | **FN** — ambos |
| `prod=$(git ls-files ...\` | 2 | TP — `$(` unclosed capturado |
| `| xargs ...` | 2 | TP — pipe inicial |
| `if [ -n "$x" ]; then` | 2 | TP — `if` sem `fi` |
| `fi` (sozinho) | 2 | TP — `fi` sem `if` |
| `esperado="scaffold.go` | 2 | TP — aspas abertas |
| `grep -q '<<EOF' f` | 0 (correto) | OK — `<<` dentro de aspas simples como argumento, não heredoc |
| `x=$((a<<b))` | 0 (correto) | OK — `<<` é bitshift aritmético |
| `grep x f &&` (trailing operator) | 2 | **TP** — bash 3.2 e dash capturam operador pendente |

## Implicação para implementação de (b)

Para detectar os dois FN em (b) (`barrier` opção de detectar fragmentos):

1. **Trailing-`\`:** complementar com `cmd.endswith('\\')`. Zero FP no corpus (todas as backslashes no corpus vêm acompanhadas de `$(` unclosed, que `sh -n` já captura). FP teórico: `echo "text\\"` (backslash literal no fim de string) — raro na prática.

2. **Heredoc opener:** passa `sh -n` (exit 0). Consequência per-line medida: `sh -c 'cat <<EOF'` recebe body vazio, sai 0; body lines executam como comandos separados; delimitador `EOF` causa "command not found" (exit 127). O heredoc body não fica silencioso — executa de forma errada e bloqueia pelo delimitador. Regex simples (`<<WORD`) tem FP em `grep -q '<<EOF' f` (aspas simples) e `x=$((a<<b))` (bitshift). No corpus atual: zero heredocs. Decisão: omitir do supplement inicial, documentar como resíduo.

3. **Spawn failure:** se `sh -n` falhar em spawnar (ENOENT), deve retornar `not_evaluated`, nunca "malformed".

## Corpus

Nos 3 universos medidos (docs/roadmaps, scripts/testdata, internal/roadmapdoc/testdata): zero instâncias de trailing-`\` puro ou heredoc opener. O único bloco com `sh -n` FAIL (2026-08-28 `done/`) tem backslashes sempre acompanhadas de `$(` unclosed — `sh -n` captura por `$(` antes de chegar ao `\`.

## Links

- `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md` — documenta o problema completo
- `docs/seguranca/2026-10-01-wave0-gate-por-linha.md` — Wave 0 parecer completo com medição
