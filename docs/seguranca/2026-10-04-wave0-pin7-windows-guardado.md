# Wave 0 — Threat model: pin7, discriminante de plataforma e vacuidade do braço guardado

> REQ: docs/req/REQ-2026-10-04-pin7-do-gate-de-pins-exige-violacao-que-a-regra-declina-por-desenho-no-windows-e-aborta-os-pins-seguintes.md
> Issue: #421 | Data: 2026-10-04 | Autor: hades-tf

---

## 1. Enumeração de superfícies

O roadmap declara duas superfícies a analisar:

1. **Discriminante de plataforma** — como o gate determina o GOOS do binário `$GO_BIN` sob teste.
2. **Vacuidade do braço guardado** — o braço Windows do pin7 pode passar por razão espúria.

**A lista está fechada.** Evidência:

```
# superfícies que poderiam criar a mesma família de falha
$ grep -n 'CurrentGOOS' internal/validator/*.go
internal/validator/goos.go:21:          var CurrentGOOS = runtime.GOOS
internal/validator/validator_credential_guard.go:493:  case CurrentGOOS != "windows" && info.Mode()&0111 == 0:
```

Há uma única guarda `CurrentGOOS` no validador. Não existe guarda equivalente para os
pins 8–20; verificado por `grep -n 'chmod' scripts/check-validate-rule-pins.sh`:

```
442: chmod +x cg-claude-present            (pin16 — fixture "presente"; silent)
447: chmod 644 cg-claude-noexec            (pin7  — única fixture noexec; afeta só este pin)
461: chmod +x cg-claude-relativo           (pin21 — git_branch_guard; silent)
...
```

`chmod 644` aparece apenas no pin7. As fixtures dos pins 8–15 usam: ausência de arquivo (pin6),
diretório no lugar do arquivo (pin14), JSON inválido (pin13), UTF-16 (pin15), caminhos
não-absolutos/relativos (pins 9–12) — nenhum depende de bit de execução. A superfície é fechada.

**Pin14 (unreadable) medido na VM** para confirmar que não é segunda vítima:

```
# fixture: settings.json é um diretório (mkdir -p .../settings.json)
rc=0 (bash ||true)
result.json: {"violations":[{"rule":"credential_guard_hook_resolvable",
  "message":".claude/settings.json (Claude Code) could not be read ..."},...]}
matching violations: 1   ← acusa corretamente no Windows
```

`readRegularFile` detecta directory-not-file por `errNotRegularFile` (sem dependência do bit
de execução); funciona em Windows/NTFS/noacl.

---

## 2. Modelo de ameaça

O adversário é o **implementador apressado** que:

**A — Usa o discriminante errado (sufixo `.exe`).**
O braço guardado seria:
```bash
case "$GO_BIN" in *.exe) # Windows ;; *) # POSIX ;; esac
```
Na VM, `go build -o "$TMP/trackfw-go" ./cmd/trackfw` produz `trackfw-go` **sem** `.exe`,
mesmo em Windows:

```
$ go build -C /c/Users/Lab/wt421 -o "$TMP/trackfw-go" ./cmd/trackfw
$ ls "$TMP"
trackfw-go          ← sem .exe
$ go version -m "$TMP/trackfw-go" | grep GOOS
  build   GOOS=windows    ← é um binário Windows, mas o nome não tem .exe
```

Com o discriminante de sufixo, o build interno no Windows cai no braço POSIX. O pin7 espera
uma violação; a regra é silenciosa por design; pin7 falha com "none found". O build interno
é o caminho padrão quando `GO_BIN` não está definido (script :43). A falha é ruidosa (mesma
mensagem de hoje), mas o pin **não é corrigido** — o discriminante simplesmente não funciona
para este caso.

**B — Escreve um braço guardado que passa vacuosamente.**
O braço assertaria `len(matching) == 0`. Isso seria verdade também se:
- a regra fosse renomeada de `credential_guard_hook_resolvable` para outro identificador;
- o JSON de saída fosse `{}` (sem chave `violations`);
- a fixture `noexec` não existisse no diretório esperado (a fixture é construída em memória
  durante o run; se a construção falhar silenciosamente, `load_cg` lança `FileNotFoundError`
  — ruidoso, mas não é o único vetor).

---

## 3. Falsificação em ambas as direções

### Superfície 1 — Discriminante

| Direção | Cenário | Comportamento | Barulhento? |
|---|---|---|---|
| FN: diz "posix" em Windows | `.exe`-suffix, build interno sem sufixo | pin7 POSIX: espera violation, regra silenciosa → falha "none found" | Sim |
| FP: diz "windows" em POSIX | binário POSIX nomeado `.exe` em Linux | pin7 guardado: afirma silêncio, mas regra ACUSA → falha "expected silence" | Sim |
| FN: diz "posix" em Windows | `go env GOOS` com `GOOS=linux` exportado no shell | `GOOS=linux go env GOOS` → `linux`; discriminante diz "posix" em Windows | Sim |

Medido na VM:
```
$ GOOS=linux go env GOOS
linux
```
A variável de ambiente `GOOS` contamina o candidato 5 (`go env GOOS`).

**O discriminante recomendado `go version -m "$GO_BIN"` é imune a estas três falhas:**

```
# macOS, binário darwin
$ BIN_GOOS=$(go version -m bin/trackfw \
    | awk '$1=="build" && $2 ~ /^GOOS=/{sub(/GOOS=/,"",$2); print $2}')
$ printf '%q\n' "$BIN_GOOS"
darwin

# macOS, binário windows cross-compilado
$ BIN_GOOS=$(go version -m bin/trackfw.exe \
    | awk '$1=="build" && $2 ~ /^GOOS=/{sub(/GOOS=/,"",$2); print $2}')
$ printf '%q\n' "$BIN_GOOS"
windows

# VM Windows, binário sem .exe (build interno)
$ BIN_GOOS=$(go version -m "$TMP/trackfw-go" \
    | awk '$1=="build" && $2 ~ /^GOOS=/{sub(/GOOS=/,"",$2); print $2}')
$ printf '%q\n' "$BIN_GOOS"
windows

# VM Windows: awk portável (sem \r), BSD e gawk
result: windows    (printf %q — sem \r, sem sufixo espúrio)
```

Build com `-ldflags="-s -w"` (strip de símbolos): GOOS sobrevive.
```
$ go build -ldflags="-s -w" -o /tmp/trackfw-stripped ./cmd/trackfw
$ go version -m /tmp/trackfw-stripped | grep GOOS
  build   GOOS=darwin
```

**Caso de falha do discriminante recomendado:** `go version -m "$GO_BIN"` retorna vazio quando
`$GO_BIN` não existe ou não é um binário Go (`2>/dev/null` suprime o erro). `BIN_GOOS` fica
vazio, e o `[[ "$BIN_GOOS" == "windows" ]]` é falso: cai no braço POSIX. Isso é a direção
**segura** (barulhenta em Windows, correta em POSIX).

Recomendação: testar `[[ -z "$BIN_GOOS" ]]` após a extração e abortar com mensagem explícita
em vez de cair silenciosamente no braço POSIX:
```bash
BIN_GOOS=$(go version -m "$GO_BIN" 2>/dev/null \
  | awk '$1=="build" && $2 ~ /^GOOS=/{sub(/GOOS=/,"",$2); print $2}')
[[ -n "$BIN_GOOS" ]] || { echo "[pin7] ABORT: go version -m '$GO_BIN' returned empty GOOS"; exit 1; }
```

**Disponibilidade de `go`:** quando `GO_BIN` é fornecido externamente (`:44` do script), `go`
pode não estar em PATH. O script não verifica. `go version -m` falharia silenciosamente; com a
guarda acima, abortaria ruidosamente.

### Superfície 2 — Vacuidade do braço guardado

Medição direta na VM (fixtures corretas, executadas):

```
$ ( cd "$TMP/cg-noexec" && /c/Users/Lab/wt421/bin/trackfw.exe validate --json ) > result.json 2>result.stderr
$ rc=$?; echo "rc=$rc"
rc=0
$ python3 - result.json <<PY
import json, sys
with open(sys.argv[1], encoding='utf-8') as f:
    payload = json.load(f)
print("keys:", sorted(payload.keys()))
print("type(violations):", type(payload.get("violations")).__name__)
matching = [v for v in payload.get("violations",[]) if v.get("rule") == "credential_guard_hook_resolvable"]
print("matching:", len(matching))
PY
keys: ['summary', 'violations', 'warnings']
type(violations): list
matching: 0
```

O JSON tem as chaves `summary`, `violations`, `warnings`. `violations` é uma lista. Zero
violações de `credential_guard_hook_resolvable` para a fixture `noexec`.

**Vetor de vacuidade A — regra renomeada:** se `credential_guard_hook_resolvable` for renomeado,
`matching == []` também para o braço POSIX de pin6. Pin6 (`cg-claude-absent`, acusa
"script does not exist") precede pin7 no laço `expect_violation`. Se pin6 passou, a string
`"credential_guard_hook_resolvable"` ainda é emitida pelo binário. Pin6 é a prova de vida
estrutural da regra — não é necessário código extra no braço guardado para este vetor.

**Vetor de vacuidade B — JSON `{}`:** `payload.get("violations", [])` retorna `[]`. Coberto pelo
mesmo argumento: pin6 precede pin7 e lê do mesmo arquivo JSON via `load_cg`; se o JSON fosse
`{}`, pin6 também falharia. Invariante de ordenação do laço é suficiente.

**Vetor de vacuidade C — fixture `noexec` corrompida ou ausente:** se a fixture não for
construída (o `mkdir` falhou, o `printf` falhou, o `python3` para o settings.json falhou), o
`load_cg` para `cg-claude-noexec-go.json` lançaria `FileNotFoundError`. Isso aborta o script
antes de chegar ao pin7. Ruidoso, não vacuoso.

**Vetor de vacuidade D — `rc != 0` com `violations: []`:** o validate pode retornar rc=1 por
outra regra e ter zero violações desta. O braço guardado deve confirmar que `rc == 0` **e**
`matching == []`. `rc=0` prova que validate não achou nenhuma violação em nenhuma regra para
esta fixture; é um fechamento mais forte que verificar só a regra específica.

**Checagem de schema mínima:** a presença da chave `violations` como lista é garantida pela
estrutura do JSON do binário (confirmada na medição acima: `type(violations): list`). Um braço
guardado que faz `payload.get("violations", [])` não falha em JSON inesperado — mas o sentido
de "passou" seria errado. A guarda `assert "violations" in payload` é opcional mas recomendada.

**Resumo das conferências anti-vacuidade para o implementador:**
1. `rc == 0` (validate retornou sem violações em nenhuma regra para esta fixture).
2. `matching == []` (zero violações desta regra).
3. Pin6 passou antes de pin7 no mesmo laço — garante que a regra está viva e o JSON é válido.
   Nenhum código adicional necessário: é a invariante de ordenação do laço `expect_violation`.

---

## 4. Resíduo declarado

**R1 — Toolchain ausente com GO_BIN externo.** Quando `GO_BIN` é fornecido externamente e `go`
não está em PATH, `go version -m` falha. O script usa `go` em :44 (GOPATH/GOMODCACHE), mas não
falha com `set -e` nessa linha; `go` pode estar ausente mesmo assim. A guarda `[[ -n "$BIN_GOOS" ]]`
exposta na seção 3 torna a falha ruidosa. Não há cobertura garantida; depende do ambiente CI.

**R2 — Cross-compilation.** Se o gate rodar com um binário Windows em host POSIX (ou vice-versa),
`go version -m` retorna o GOOS correto, mas o binário não executa (`exec format error`). A
falha ocorre antes do discriminante; não é novo risco e não precisa de cobertura adicional.

**R3 — Pins 8–20 em Windows (ocultos pelo SystemExit do pin7).** A análise das fixtures e a
medição de pin14 mostram que nenhum dos pins 8–15 depende do bit de execução. Após a correção
do pin7, espera-se que todos passem. A AC2 exige medição na VM; se algum falhar, entra nesta
REQ (Regra Dura). O resíduo é: **não foram medidos individualmente neste Wave 0** porque o
SystemExit impede sua execução.

**R4 — `chmod +x` sem efeito em Windows.** As fixtures `cg-claude-present`, `cg-cursor-present`,
`cg-copilot-relativo-present` (pins 16, 19, 20) usam `chmod +x`. Em Windows/NTFS/noacl, o bit
de execução não é representável — `os.Stat().Mode()&0111 == 0` sempre. A Windows guard em :493
pula a verificação de bit de execução, então esses fixtures permanecem silenciosos por design.
Correto por construção; sem residual de segurança.
