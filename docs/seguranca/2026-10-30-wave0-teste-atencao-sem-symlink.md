# Wave 0 — Modelo de Ameaça: Teste do Invólucro de Atenção sem Symlink

> Roadmap: ROADMAP-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md
> REQ: REQ-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md
> Data: 2026-10-30 | Revisor: hades-tf

Mapa ação do roadmap → seção deste documento:

| ação | seção |
|---|---|
| 1. Conteúdo do diretório de `compiledGuardBinary` | Seção 1 |
| 2. Comparação das opções de correção (dir no PATH × cópia) | Seção 2 — V-3 e Prescrições P-1 |
| 3. Enumeração dos testes que materializam binário por symlink | Seção 1 |
| 4. Alvos de falsificação AC1–AC3; o que medir na VM para AC4 | Seção 3 |
| 5. Prescrições para ML-1A e residual | Seções 4–5 |

---

## 1. Completude da Enumeração

A REQ declara dois sítios que materializam binário por symlink e um universo de ~20 outros.
Verificado por:

```
grep -rn "os.Symlink" internal/ --include="*_test.go" | grep -v "//.*os.Symlink" | wc -l
40
```

Dos 40 resultados, os que materializam executável para PATH de processo filho:

| arquivo | linha | mecanismo | status |
|---|---|---|---|
| `internal/generators/scaffold_test.go` | 408 | `os.Symlink(binPath, fakeBinDir/trackfw)` | BUG — sem fallback |
| `internal/generators/git_branch_guard_test.go` | 932 | `os.Symlink` + `copyExecutableFile` fallback | já correto |
| `internal/commands/ship_test.go` | 1966 (`placeExecutableInPath`) | symlink → hardlink → copy | já correto |

Os demais 37 testam symlinks de arquivos de configuração, ADR, path-guard ou comportamento do
próprio symlink — com `t.Skip` quando sem privilégio. Fora do escopo.

**Enumeração fechada:** a lista da REQ é correta e completa para o escopo desta correção.

### Conteúdo do diretório de `compiledGuardBinary`

`buildGuardBinary()` em `guard_parity_helper_test.go:71`:

```
dir = os.MkdirTemp("", "trackfw-guard-bin-*")
go build -o guardBinaryPath ./cmd/trackfw
```

Um único arquivo é criado no diretório. `jq` está em `/opt/homebrew/bin/jq` (macOS) ou
equivalente — nunca nesse diretório.

**AC2 é garantido por construção se o fix usar `filepath.Dir(compiledGuardBinary(t))` como PATH
do filho.** Não é necessário tripwire ativa; a declaração no relatório do ML-1A é suficiente.

### O invólucro gerado não chama `jq`

`attentionSignalScript` em `scaffold.go:1086`:

```bash
#!/usr/bin/env bash
# trackfw attention signal — compatibility wrapper; attention never blocks tools
command -v trackfw >/dev/null 2>&1 || exit 0
trackfw attention signal "$@" || true
exit 0
```

O invólucro chama apenas `trackfw attention signal` — Go puro. Nenhum `jq`, `python3` ou
interpretador. Confirmado pelo comentário em `scaffold.go:1085`: "No shell parser, Python or jq
participates in payload handling." AC2 não depende de isolamento ativo; depende deste invariante.

---

## 2. Modelo de Ameaça

### V-1: Falha silenciosa de setup → diagnóstico enganoso

**Mecanismo atual (`scaffold_test.go:398–416`):**

1. `exec.LookPath("trackfw")` procura `trackfw` no PATH do processo de teste.
2. Se encontrado, `os.Symlink(binPath, fakeBinDir/trackfw)` — sem privilégio (Windows sem
   Developer Mode), falha com `EPERM`. `isSymlinkPrivilegeError` devolve `true` → continua
   sem o binário (best-effort).
3. Script roda com `PATH=fakeBinDir` (vazio). `command -v trackfw` falha. Script faz `exit 0`.
4. Nenhum arquivo de atenção escrito. `os.ReadFile(attentionFile)` falha.
5. `t.Fatalf("Arquivo de atenção não foi encontrado")` — mensagem que não nomeia a causa.

O diagnóstico correto está 2 asserções depois do ponto de falha real.

### V-2: `exec.LookPath` encontra o compiledGuardBinary, não o sistema — mas o ponto é irrelevante

`scaffold_test.go:383`:
```go
t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
```

`TestMain` NÃO muta `PATH` (só muta `HOME`, `USERPROFILE`, `GIT_CONFIG_PARAMETERS`). A mutação
de PATH ocorre no início de `TestAttentionScripts_FallbackWithoutJQ` pela linha acima. Portanto:
- `exec.LookPath("trackfw")` encontra `compiledGuardBinary(t)` (diretório prepended)
- O symlink aponta para o binário compilado da árvore — correto
- O teste exercita a versão da árvore, não uma versão instalada no sistema

V-2 não é uma falha real. A única falha é V-1 (symlink sem privilégio no Windows).

### V-3: AC2 — `jq` no PATH do filho mudaria o que o teste afirma

O teste afirma o modo "sem jq". Se `jq` ficasse no PATH do filho, o teste ainda passaria
(o invólucro não usa `jq`), mas testaria condições diferentes das declaradas.

**Comparação das opções:**

| opção | `jq` no PATH do filho? | `.exe` preservado? | complexidade |
|---|---|---|---|
| Option 1: `PATH = filepath.Dir(compiledGuardBinary(t))` | não (compiledGuardBinary dir só tem trackfw) | sim (go build produz `.exe` automaticamente) | mínima |
| Option 2: cópia com `filepath.Base(compiledGuardBinary(t))` em `fakeBinDir` | não | sim | média |

Option 1 é recomendada: zero linhas de symlink, zero linhas de cópia, padrão já estabelecido
por `injectGuardBinaryPath` em `guard_parity_helper_test.go:107`.

---

## 3. Alvos de Falsificação

### AC1 — Teste não usa symlink para colocar binário no PATH do filho

**Falso negativo:** a correção usa `filepath.Dir(compiledGuardBinary(t))` mas o `cmd.Env` ainda
inclui outros diretórios do sistema que contêm ferramentas não-esperadas.
Gate: `cmd.Env` deve ser `[]string{"PATH=" + filepath.Dir(compiledGuardBinary(t))}` — sem herdar
env do processo de teste, sem outros diretórios além do compiledGuardBinary.

**Falso positivo:** o PATH do filho contém só o dir de `compiledGuardBinary`, e o script precisa
de ferramentas que não estão lá (bash, date, etc.).
Medição: o script é invocado por `exec.Command("bash", signalPath)` — `bash` é o processo pai,
não precisa estar no PATH do filho. O script em si só chama `trackfw`. PATH do filho precisa
apenas de `trackfw`. Confirmado pela leitura de `attentionSignalScript`.

### AC2 — PATH do filho sem `jq`

**Falso negativo:** `jq` entra por herança de env do processo de teste.
Gate: `cmd.Env` explícito (sem herdar) — já garantido pelo padrão atual e pela Option 1.

**Falso positivo:** `compiledGuardBinary` dir contém `jq` inesperadamente.
Não se aplica: `go build -o guardBinaryPath ./cmd/trackfw` produz exatamente um arquivo. Invariante por construção.

### AC3 — Falta do binário reprova no setup com mensagem que nomeia a causa

**Falso negativo:** `compiledGuardBinary(t)` sempre retorna um caminho válido (compilado em
`TestMain`); `os.Stat(compiledGuardBinary(t))` passa mesmo que o binário não esteja no PATH do
filho. A asserção `os.Stat` verifica existência em disco, não acessibilidade no PATH do filho.

A probe correta usa o mesmo PATH que o processo filho receberá:

```
exec.Command("bash", "-c", "command -v trackfw")   com   Env = []string{"PATH=" + trackfwBinDir}
```

Se não encontrado, `t.Fatalf` com o PATH e o nome do binário esperado.

**Contra-braço (falsificação — obrigatório):** passar um `trackfwBinDir` vazio e confirmar que
o setup reprova AQUI, não na asserção de `os.ReadFile`. Sem o contra-braço, a probe é decorativa.

**Falso positivo:** a probe reprova mesmo com `trackfw` presente porque `bash` não está no
PATH do filho. Não se aplica: a probe invoca `exec.Command("bash", ...)` do processo de teste
(PATH herdado do processo), não do filho. Bash está disponível no processo de teste.

### AC4 — Medição na conta Windows sem privilégio de symlink

**Falso negativo:** a conta Windows de teste tem `SeCreateSymbolicLinkPrivilege` (Developer Mode
ou conta admin). O `windows-latest` CI TEM o privilégio — não serve como prova de AC4.
Gate: `whoami /priv` deve mostrar `SeCreateSymbolicLinkPrivilege ... Disabled` (ou ausente).
Contra-braço: na mesma conta, em `origin/main` (antes do fix), confirmar que o teste falha;
na branch do fix, confirmar que passa.

**Falso positivo:** o fix quebra em conta que TEM privilégio.
Não se aplica: Option 1 elimina symlink inteiramente — funciona com ou sem privilégio.

---

## 4. Residual Declarado

**R-1: AC4 requer VM Windows sem privilégio de symlink.**
O CI `windows-latest` tem o privilégio. O ML-1B (zeus-tf) realiza essa medição na `Windows-Lab`.
Se a conta tiver o privilégio, declarar como residual de cobertura e documentar o contra-braço
alternativo: (a) teste sem symlink passa no fix; (b) a conta usada é descrita com
`whoami /priv`; (c) o `windows-symlink-unprivileged` job do CI exercita o caminho por injeção.

**R-2: O invariante de AC2 depende de `attentionSignalScript` não chamar `jq`.**
Se uma mudança futura adicionar `jq` ao script, o teste precisará de atualização. O invariante
está documentado em `scaffold.go:1085`.

**R-3: V-2 foi falsificado — a única falha ativa é V-1 (symlink sem privilégio).**
`t.Setenv("PATH", ...)` na linha 383 do teste prepends o diretório compilado antes da execução
de `exec.LookPath`. O teste sempre usa o binário compilado em `TestMain`, em qualquer
plataforma. O fix (Option 1) elimina `LookPath` e `os.Symlink` inteiramente, não porque
resolve V-2 (que não existe), mas porque é o padrão mais limpo e evita o risco de V-1.

---

## Prescrições

As prescrições abaixo são obrigatórias para o ML-1A. Nenhuma linha de implementação foi escrita
neste documento — as prescrições são regras comportamentais.

**P-1: Remover o loop `requiredBins` e `os.Symlink`; usar PATH do binário compilado (Option 1)**

Comportamento exigido:
- Substituir o `fakeBinDir` e o loop por uma variável `trackfwBinDir` que contém apenas o
  diretório de `compiledGuardBinary(t)`.
- Construir `cmd.Env` como `[]string{"PATH=" + trackfwBinDir}` — sem herdar env do processo.
- Não chamar `exec.LookPath`, `os.Symlink` nem nenhuma função de cópia para este teste.
- O `.exe` é preservado automaticamente: `compiledGuardBinary(t)` já diferencia
  `runtime.GOOS == "windows"`.

**P-2: Verificação de acessibilidade antes de rodar o script (AC3)**

Comportamento exigido:
- Antes de `exec.Command("bash", signalPath)`, verificar que `trackfw` está acessível no PATH
  do filho, executando `bash -c 'command -v trackfw'` com `cmd.Env` idêntico ao que o script
  receberá.
- Se não encontrado, `t.Fatalf` com mensagem que inclui o valor de `trackfwBinDir` e o nome
  do binário esperado.
- Contra-braço obrigatório no relatório do ML-1A: com `trackfwBinDir` vazio (ou sem `trackfw`),
  confirmar que a verificação reprova neste ponto.

**P-3: Declarar no relatório do ML-1A o que o teste afirma (Regra Dura de Reconciliação)**

"Este teste afirma que, quando `trackfw` está no PATH do filho e `jq` não está, o invólucro de
atenção escreve corretamente o arquivo de atenção."

**P-4: Declarar AC2 por invariante, não por código**

O relatório do ML-1A deve incluir: "O diretório de `compiledGuardBinary` contém somente o
binário `trackfw`; `jq` não está presente por construção (`go build -o guardBinaryPath
./cmd/trackfw` gera um único arquivo). O invólucro gerado não chama `jq` (scaffold.go:1085)."
Não é necessário código adicional para verificar AC2.
