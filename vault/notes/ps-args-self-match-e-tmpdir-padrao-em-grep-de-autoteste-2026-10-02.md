# Dois falsos positivos em autotestes de scripts: self-match de `ps -o args=` e padrão no TMPDIR

> 2026-10-02 · achados durante ML-1A do ROADMAP-2026-10-02 (limite de tempo por chunk)

## 1. `grep -qF "chunk-timeout"` casa o próprio diretório temporário

**Sintoma:** arm 2 do `check-falsify-chunk-timeout.sh` falhava com "linha FAIL timeout apareceu
mesmo com chunks normais" mesmo quando o driver saía rc=0 e sem nenhum timeout real.

**Causa:** o diretório de trabalho criado por
```bash
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/check-falsify-chunk-timeout.XXXXXX")
```
produz um caminho como `.../check-falsify-chunk-timeout.Txx2nX/arm2/gen.py`. O driver anuncia no
`stderr` o caminho efetivo de `TRACKFW_FALSIFY_GEN`:
```
run-gates-falsify-parallel: TRACKFW_FALSIFY_GEN setada -- valor efetivo='.../check-falsify-chunk-timeout.Txx2nX/arm2/gen.py'
```
Essa linha contém `chunk-timeout` como parte do nome do diretório. O `grep -qF "chunk-timeout"`
sobre `arm2_err` sempre casava.

**Correção:** usar o padrão completo e exclusivo da linha de erro real:
```bash
if grep -qF "FAIL [falsify-driver/chunk-timeout]" "$arm2_err" 2>/dev/null; then
```
`FAIL [falsify-driver/chunk-timeout]` nunca aparece num caminho de arquivo, só na saída de timeout
do driver.

**Regra:** ao criar diretório temporário com `mktemp -d ".../<script-name>.XXXXXX"`, qualquer
`grep` sobre arquivos naquele diretório (ou sobre saídas que incluem o caminho) deve usar o padrão
**mais específico possível** — preferencialmente a linha exata que só o caminho de erro emite,
não um fragmento do nome do script.

---

## 2. `ps -A -o args= | awk '/sleep 9981743/'` casa a si mesmo

**Sintoma:** o check de sobreviventes em arm 3 reportava 1-3 processos `sleep 9981743` mesmo depois
de confirmar que o driver havia matado o grupo. Os processos por PGID mostravam 0 sobreviventes.

**Causa:** `ps -A -o args=` mostra a linha de comando completa de cada processo. No ambiente do
Claude Bash tool, o shell pai (zsh) tem como `args` o comando inteiro que foi passado via `-c '...'`,
e esse texto inclui `sleep 9981743` como parte do padrão awk embutido no script. Em terminal normal,
o processo `awk '/sleep 9981743/{c++} END{...}'` também aparece no snapshot, com seu próprio
argumento contendo o padrão.

Ambas as situações produzem falso positivo: o awk/shell que EXECUTA a verificação aparece no
snapshot como se fosse um `sleep` sobrevivente.

**Correção:** filtrar por `comm=` (basename do executável) antes de checar `args=`:
```bash
_ps_snap="$SCRATCH/ps_snap.txt"
ps -A -o comm= -o args= > "$_ps_snap" 2>/dev/null || true
arm3_survivors=$( awk '$1=="sleep" && $0~/9981743/{c++} END{print c+0}' "$_ps_snap" )
```
Isso conta apenas processos cujo executável é literalmente `sleep` — awk, bash e zsh não passam
no filtro `$1=="sleep"`.

O mesmo padrão vale para o cleanup de survivors em contra-braços (arm 5):
```bash
ps -A -o comm= -o pid= -o args= > "$_ps_snap5" 2>/dev/null || true
_surv=$( awk '$1=="sleep" && $0~/9981744/{print $2}' "$_ps_snap5" )
```

**Regra geral:** quando checar processos por padrão de args em autoteste:
1. Nunca usar `ps | awk` em pipeline: o awk é capturado no snapshot do pipe e casa contra si.
2. Capturar `ps` para arquivo PRIMEIRO, depois rodar awk no arquivo. Isso garante que awk (que
   tem o padrão em sua linha de comando) não aparece no snapshot.
3. Adicionar `comm=` como primeiro filtro para restringir ao executável real: `$1=="sleep"`,
   `$1=="python3"`, etc.

**Verificação rápida:** se `count > 0` com zero chunks rodando, o teste de sobrevivência está
com falso positivo por auto-referência.
