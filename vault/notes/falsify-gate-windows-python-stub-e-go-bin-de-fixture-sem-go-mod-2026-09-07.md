# check-gates-falsify.sh no Windows: stub da Store mascarado de "python3 ausente" e go build a partir de fixture sem go.mod

> ML-1A/ML-1B, ROADMAP-2026-09-07-gates-rodam-no-windows-resolucao-de-interpretador-e-binario.md

## Causa raiz 1 — `python3` no PATH do Windows é o stub da Microsoft Store, não o Python real

Medido na VM Windows 10 Pro ARM64 (Go 1.27.1, Python 3.12.10 instalado):

```
command -v python3  -> .../Microsoft/WindowsApps/python3   (stub)
command -v python    -> .../Programs/Python/Python312-arm64/python  (real)
command -v py        -> .../Programs/Python/Launcher/py    (real)

python3 -c 'import sys; print(sys.version_info[0])'  -> rc=49, stderr "Python was not found; ..."
python  -c 'import sys; print(sys.version_info[0])'  -> rc=0,  stdout "3"
py      -c 'import sys; print(sys.version_info[0])'  -> rc=0,  stdout "3"
```

O stub reprova em DOIS pontos simultaneamente: exit code (49, não 0) e stdout (mensagem de erro, não
"3"). Esse par é o critério de rejeição usado pela função `resolve_py_bin()` — não basta checar
`command -v`, porque o stub *existe* no PATH e responde a `-c`, só que com uma mensagem de erro em vez
de executar o Python real.

## Causa raiz 2 — `GO_BIN` ausente dispara fallback que builda a partir da fixture, sem go.mod

`check-gates-falsify.sh` copia vários `scripts/check-*-parity.sh` para dentro de uma fixture temporária
(`$Tn/scripts/...`) e os invoca via `bash "$Tn/scripts/check-xxx.sh"` com `env GO_BIN="$ROOT_DIR/bin/trackfw"`
(caminho absoluto real). Cada um desses sub-scripts, quando `[[ ! -x "$GO_BIN" ]]`, tenta reconstruir o
binário — mas com `cd "$ROOT_DIR"`, onde o `$ROOT_DIR` **daquele sub-script** é resolvido localmente via
`${BASH_SOURCE[0]}` — ou seja, a fixture `$Tn`, que não tem `go.mod`.

Reproduzido isolado na VM (fora de qualquer cenário real, só para confirmar o mecanismo):

```
env GO_BIN="$ROOT/bin/trackfw-does-not-exist" bash "$FIXTURE/scripts/check-identity-parity.sh"
-> go: go.mod file not found in current directory or any parent directory; see 'go help modules'
```

Esse é exatamente o sintoma relatado, e não tem relação com a causa (binário ausente) — levou a duas
hipóteses do arquiteto já falsificadas (sufixo `.exe` em massa; `cp -r` travando no Cenário 8). Sob
`make quality`/`make parity` completo o binário já existe antes de chegar aqui (o alvo `build` roda
primeiro), então o bug é latente ali; ele aparece quando o gate é invocado sem esse pré-requisito — o
caso realista no Windows, onde `make` nem está instalado (`where make` -> not found na VM) e qualquer
execução manual do gate parte de `bash scripts/check-gates-falsify.sh` direto.

Confirmado também: `go build -o bin/trackfw ./cmd/trackfw` no Windows produz um arquivo **sem** sufixo
`.exe` (nome literal `trackfw`), e esse binário extensionless roda normalmente via git-bash
(`./bin/trackfw --version` -> rc=0). A hipótese do sufixo `.exe` estava morta duas vezes: não é preciso
para rodar, e não é o que faltava.

## As 3 direções de `FALSIFY_GO_BIN`, medidas na VM

- só `bin/trackfw.exe` presente -> resolvido pelo branch `GOEXE`, gate avança até o mesmo obstáculo de
  sempre (não fica preso no setup).
- nenhum dos dois presente -> `FAIL [falsify/setup]: binário ausente -- procurado em
  '.../bin/trackfw.exe' e '.../bin/trackfw', ...` (mensagem do novo resolvedor).
- só `bin/trackfw` (extensionless) presente -> caminho que o run completo (412 OK/0 FAIL, diff de
  399 rótulos vazio) já exercitou.

## Sítios de mesma causa, medidos e NÃO corrigidos (fora do escopo desta REQ)

- `scripts/check-identity-parity.sh:40,164` — `python3` bare; é o obstáculo que o gate agora atinge
  depois das correções (rc=49, mensagem do stub).
- O fallback `[[ ! -x "$GO_BIN" ]]` + `cd "$ROOT_DIR"` (ROOT_DIR local da fixture, sem go.mod) se
  repete nos ~26 `scripts/check-*.sh` que seguem o mesmo padrão de resolução de `GO_BIN` (grep
  `[[ ! -x "\$GO_BIN" \]\]` em `scripts/*.sh`) — latente sob `make`, dispara só quando o gate roda
  standalone.
- `python3` bare nos outros ~40 gate scripts fora de `check-gates-falsify.sh` (contagens por arquivo:
  `check-validate-parity.sh` 39, `check-update-parity.sh` 32, `check-barrier.sh` 30,
  `check-roadmap-barrier-contract.sh` 27, `check-output-encoding-declared.sh` 27,
  `check-doctor-parity.sh` 16, e mais ~30 arquivos com 2-12 ocorrências cada).

## Escopo da correção (ML-1A/ML-1B) — só `check-gates-falsify.sh`

Resolvido num ponto único no preâmbulo do próprio `check-gates-falsify.sh` (copiado byte-a-byte para
todo chunk pelo `gen-falsify-chunks.py`, `prelude_end_line` > linha do bloco):
- `PY_BIN` — resolvido por `resolve_py_bin()`, usado nos ~62 pontos onde o PRÓPRIO
  `check-gates-falsify.sh` executa python3 (nunca dentro de conteúdo de heredoc comparado/hasheado).
- `FALSIFY_GO_BIN` — resolvido com suffix-awareness (`go env GOEXE`) e falha alto, nomeando a causa,
  se o binário não existir em nenhuma forma. Substituído nos 65 pontos onde o script passa
  `GO_BIN="$ROOT_DIR/bin/trackfw"` para um sub-script. Binários isolados construídos pelo próprio
  script (`Txx_BIN` via `build_go_or_fail`) não passam por aqui — já existem no ponto de uso.

**Deliberadamente não corrigido** (Wave 2 do roadmap, fora deste escopo): os ~40 `scripts/check-*.sh`
que o `check-gates-falsify.sh` copia para fixtures continuam chamando `python3` bare internamente (ex.
`check-identity-parity.sh:40,164`). Medido na VM: com as duas correções acima, o gate avança de
"quebra no setup do Cenário 3 com `go.mod not found`" para "Cenário 3 falha em
`identity-parity/catalog-target-missing` com rc=49 e a mensagem do stub" — mesma causa raiz (stub),
sítio diferente (arquivo fora do escopo desta REQ).

## Armadilha do próprio script de substituição

Um `sed` ingênuo trocando `python3` por `"$PY_BIN"` quebra sintaxe quando o `python3` está DENTRO de um
único argumento `bash -c "..."` já entre aspas duplas: `"...python3 -m trackfw"` vira
`"..."$PY_BIN"..."`, que ainda é sintaticamente válido por concatenação de strings adjacentes do bash,
mas é frágil (`$PY_BIN` fica fora das aspas, sujeito a word-splitting) — nesses casos o correto é
`$PY_BIN` SEM aspas extras, mantendo-se dentro da mesma string. E um heredoc-tracker que trata linhas
de COMENTÁRIO como candidatas a abrir heredoc real (`# ... "$(cat <<'EOF' ...` é prosa, não heredoc)
falsifica o estado e some ~690 linhas de substituição real sem erro visível — só detectável comparando
a lista de hits antes/depois.
