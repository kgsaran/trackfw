---
title: "Red Team: ML-R3 — Guard de entrada de travessia do Manager.resolve"
date: 2026-10-15
author: hades-tf
roadmap: ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz.md
ml: ML-R3
req: REQ-2026-09-03
branch: fix/fechar-os-grupos-de-falha-de-windows-ml-r3-travessia
status: Entregue
---

# Red Team — ML-R3: Guard de entrada de travessia do `Manager.resolve`

> REQ-2026-09-03 · ML-R3 · 2026-10-15 · hades-tf
> VM: Windows 11 ARM64 (`Lab@192.168.64.6`, Go 1.27.0 windows/arm64)

---

## Veredito

**LIBERA COM RESSALVA (R-1).**

O fix do ML-R3 entrega o que prometeu: os quatro vetores obrigatórios do roadmap
(`..\outside.md`, `a\..\..x.md`, `a/..\x.md`, `..`) são rejeitados pelo guard
de entrada com `"unsafe destination"` no ramo `default:`, e o complemento `~/`
espelha a mesma lógica corretamente. Zero flips em destinos legítimos do catálogo
(project e global) e em destinos de skill de terceiro.

Uma ressalva: variantes de trailing space/dot (`".. /x"`, `"~/.. /x"`) passam
pelos três layers de contenção (guard entry, beneath, rejectSymlinks) em ambas as
gramáticas e em ambas as plataformas testadas. O ataque não tem consequência
observável no Windows 11 ARM64 (Win32 falha no CreateTemp/Rename) nem no macOS
(OS cria literalmente um diretório de nome `".. "` dentro de root, sem escape);
mas os guards não explicam essa segurança — o OS explica. Isso é exatamente o
padrão que o próprio roadmap (l.2471) declara inaceitável: "guard cedo que não faz
seu trabalho torna o sistema dependente de uma camada". A camada aqui é o OS, não
`beneath()`.

---

## Metodologia

**Camada 1 — Reimplementação isolada (macOS/Linux):**
Réplica de `containsUnsafeSegment`, `guardDefault` e `guardTilde` num binário Go
standalone com `go mod init`, em `mktemp -d` fora da árvore.
Exerceu `path.Clean`, `containsUnsafeSegment` e `filepath.Rel` contra 25+ vetores.

**Camada 2 — Código real (macOS/Linux e Windows ARM64):**
`rsync -a --exclude .git` da árvore para `mktemp -d`. Adição de
`internal/integrations/hades_redteam_r3_test.go` (descartável, nunca comitado)
usando os helpers reais `testManager`/`testPlan`. `go test -run TestHadesR3` na
cópia local e via `GOOS=windows GOARCH=arm64 go test -c` + scp + execução na VM.

**Camada 3 — Comportamento Win32 (Windows VM):**
Três binários de sonda compilados com `GOOS=windows GOARCH=arm64 go build` e
executados na VM: (i) trailing-space em posição intermediária e final,
(ii) NTFS ADS, `C:..\x`, nomes reservados, (iii) `Rename` e `CreateTemp` com
trailing-space.

Binários de sonda deletados da VM ao encerrar (`del C:\Users\Lab\winprobe*.exe`).
A cópia rsync e os binários temporários foram removidos do host (`rm -rf`).

---

## Seção 1 — Vetores obrigatórios do roadmap (default: branch)

Exercidos via código real (`testManager`/`testPlan`), macOS e Windows.

| Vetor | Camada que rejeita | Mensagem | macOS | Windows VM |
|---|---|---|---|---|
| `../outside.md` | guard entry | `unsafe destination` | PASS | PASS |
| `..\outside.md` | guard entry | `unsafe destination` | PASS | PASS |
| `a\..\..\..\x.md` | guard entry | `unsafe destination` | PASS | PASS |
| `a/..\x.md` | guard entry | `unsafe destination` | PASS | PASS |
| `..` (bare) | guard entry | `unsafe destination` | PASS | PASS |
| `./..` | guard entry | `unsafe destination` | PASS | PASS |
| `.\..` | guard entry | `unsafe destination` | PASS | PASS |
| `a/./../..` | guard entry | `unsafe destination` | PASS | PASS |

**Evidência da camada:** mensagem `"unsafe destination"` é emitida exclusivamente
pelo guard de entrada (`return "", "", fmt.Errorf("unsafe destination %q", ...)`).
A mensagem de `beneath()` é `"is outside ... root"` e a de `rejectSymlinks` é
`"refusing write to ..."`. Os testes verificam a mensagem exata.

**Mecanismo (ML-R3):** `containsUnsafeSegment` divide em `strings.FieldsFunc` por
`'/'` e `'\\'` e compara cada parte com `".."`. Isso fecha o gap do `path.Clean`
(POSIX-only) que permitia `..\outside.md` passar em qualquer host.

---

## Seção 2 — Ramo `~/` (complemento ML-R3)

| Vetor | Camada que rejeita | Mensagem | macOS | Windows VM |
|---|---|---|---|---|
| `~/../x.md` | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/..\outside.md` | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/..` | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/a\..\..\x.md` | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/` (rest vazio) | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/.` (rest=".") | guard entry tilde | `unsafe destination` | PASS | PASS |
| `~/...` com escopo=project | check de escopo | `home destination requires global scope` | PASS | PASS |

O guard do ramo `~/` espelha corretamente o ramo `default:` pelo novo bloco
`path.Clean(rest) != rest || containsUnsafeSegment(rest)`.

---

## Seção 3 — Codificação e variantes de `..`

### 3.1 Homoglifos e codificações (sem separador real)

| Vetor | Resultado guard | Camada real que contém | Plataformas |
|---|---|---|---|
| `...` (três pontos) | ACEITO | literal filename dentro de root | todas |
| `..%2f` (percent-encoded) | ACEITO | Go/OS não decodifica `%2f` em path | todas |
| `..∕outside.md` (U+2215) | ACEITO | U+2215 não é separador em nenhum OS | todas |
| `..\x00/outside.md` (NUL) | ACEITO pelo guard | Go `syscall.UTF16PtrFromString` rejeita NUL (EINVAL) em todos os OS | todas |

**Medição NUL (macOS):** `os.Create(path_with_nul)` → `open ...: invalid argument`. **NUL bloqueado pela camada Go/syscall, não pelo guard.** Alcançabilidade negligível: o NUL chegaria ao campo `Destination` de um `PlannedArtifact`, que é carregado de manifesto JSON ou catálogo — ambos rejeitam NUL durante parsing.

### 3.2 NTFS ADS: `..:stream`

| Teste | macOS | Windows VM |
|---|---|---|
| Guard entry | PASSA | PASSA |
| beneath() | PASSA | PASSA |
| rejectSymlinks | `Lstat` OK (literal file em macOS) | Win32 rejeita sintaxe — "A sintaxe do nome do arquivo... está incorreta." |

**Evidência Windows (TestHadesR3DefaultBranch output):**
```
trackfw: refusing write to C:\...\025\..:stream: CreateFile C:\...\025\..:stream:
A sintaxe do nome do arquivo, do nome do diretório ou do rótulo do volume está incorreta.
```

`rejectSymlinks` captura acidentalmente: a chamada `Lstat` falha no Win32 ao abrir
`..:stream` (ADS de referência `..` é sintaxe inválida). Contenção real, mas pela
camada errada. Em macOS `..:stream` é nome de arquivo literal — sem escape.

**Classificação: Resíduo declarado.** Não exploitável em nenhuma plataforma medida.
Alcançabilidade: slug de terceiro restrito a `[a-z0-9._-]` → impossível de produzir
`:`. Catálogo recusa `\` em validateInstallPath.

### 3.3 Barra dupla e device paths: `\\?\`, `\\.\`

| Vetor | Branch | Resultado |
|---|---|---|
| `\\?\C:\evil` | anchored (IsAnchored=true, IsAbs=false) | `unsafe destination` |
| `\\.\device\x` | default: | guard passa; filepath.Join produz path literal com `\` em macOS |

`\\.\device\x` em macOS: `filepath.Join(root, "\\.\device\x")` → path literal com
`\\` no nome do arquivo — sem escape. Em Windows: `filepath.IsAbs("\\.\device\x")`
= true (inicia com `\\`) → cai no ramo anchored → `!filepath.IsAbs` = false →
`unsafe destination`. Coberto por testes do ML-R1.

---

## Seção 4 — Mistura de ramos (ramos errados)

| Vetor | Branch efetivo | Resultado |
|---|---|---|
| `~` (sem barra) | default: (não inicia com `~/`) | aceito como filename literal `~` |
| `~\outside.md` | default: (não inicia com `~/`) | aceito como filename com `\` literal em macOS |
| `~user/dir` | default: | aceito como subdir `~user/dir` dentro de root |
| `~/` com scope=project | `~/` → rejeita imediatamente | `home destination requires global scope` |
| `C:x` | default: | aceito; `filepath.Join(root, "C:x")` em macOS = `root/C:x` literal |
| `C:..\x` | default: | guard: `containsUnsafeSegment` split em `\` → `["C:..", "x"]`; `"C:.." ≠ ".."` → PASSA; Win32 rejeita `"root\\C:..\\x"` — "sintaxe incorreta" |

**`C:..\x` medido na VM:**
`os.Create("...\\project\\C:..\\x")` → `A sintaxe do nome do arquivo, do nome do diretório ou do rótulo do volume está incorreta.` Win32 rejeita o componente `C:..` em path absoluto do diretório. **Resíduo:** o guard não rejeita, o Win32 rejeita.

---

## Seção 5 — Nomes reservados Windows (CON, NUL, AUX, PRN)

Testados via `testManager`/`testPlan` na VM. Resultado: **nenhum produz "unsafe destination"** (zero-flip correto). Resolve() retorna `"root\CON"`, etc.

**Por que não é escape:** `filepath.Join(root, "CON")` produz path absoluto. Windows
com NTFS trata `root\CON` como arquivo regular (NTFS suporta nomes reservados em
subdiretórios com path completo). `atomicWrite` criaria um arquivo literal chamado
`CON` dentro de root — contido. Bare `"CON"` (sem subdiretório) não chega ao
catálogo: catálogo usa sempre subdiretórios (`".claude/skills/..."`). Via terceiro:
slug `"con"` → dest `"~/.claude/skills/thirdparty/con.md"` — extensão `.md` impede
interpretação como device.

**Classificação: Resíduo.**

---

## Seção 6 — Nomes curtos 8.3

`fsutil 8dot3name query C:` na VM → criação de nome 8.3 **HABILITADA**.

No ramo `default:`, o destino é relativo. `filepath.Join(filepath.Abs(root), dest)` produz path absoluto em forma longa (root já passou por `filepath.Abs`). O destino relativo não contém referências absolutas → `filepath.Join` produz path em forma longa → `beneath()` compara strings na mesma forma → sem mismatch 8.3.

O risco 8.3 está no ramo anchored (caminho absoluto fornecido pelo usuário pode estar em forma curta), onde `beneath()` pode dar falso positivo. Esse é exatamente o defeito do ML-W1, já corrigido. Não é responsabilidade do ML-R3.

**Classificação: Resíduo** para este ML.

---

## Seção 7 — Zero flips

### 7.1 Catálogo (project + global)

`TestManagerAcceptsCatalogRelativeDestinations` e
`TestManagerAcceptsCatalogGlobalDestinations` (entregues pelo ML-R3) cobrem todos
os paths do catálogo embutido com `{{id}}=backend`. Ambos PASSAM em macOS e Windows VM.

### 7.2 Destinos de terceiro

`TestHadesR3ZeroFlipThirdPartyDestinations` (descartável, cópia rsync): para cada
target do catálogo, em ambos os escopos, produz a dest via
`ResolveThirdPartySkillDestination` e testa 4 slugs representativos. PASSA em macOS
e Windows VM.

**Alcançabilidade reforçada:** `thirdPartySlugPattern = ^[a-z0-9][a-z0-9._-]{0,63}$`
proíbe `..`, `/`, `\`, `:`, espaço e NUL no slug. `validateInstallPath` proíbe `\`
em paths do catálogo. Logo nenhum path de terceiro legítimo chega ao guard com
gramática Windows.

### 7.3 Ramo `~/` com novo `path.Clean(rest) != rest`

O check `path.Clean(rest) != rest` foi adicionado pelo ML-R3. Candidato de flip:
paths com `./`, `//` ou trailing `/`. Verificado: `validateInstallPath` já aplica
`path.Clean` em todos os paths globais do catálogo → esses nunca têm `//` ou `./`.
`TestManagerAcceptsCatalogGlobalDestinations` confirma. **Zero flips.**

---

## Achados

### R-1 — Trailing space/dot em componente de caminho (Ajuste)

**Vetores:** `".. /outside.md"`, `".. "`, `".. ."`, `".. ./secrets.md"`,
`"~/.. /secrets.md"` e variantes.

**Evidência de camada (macOS, réplica isolada):**
```
".. /outside.md"  | PASS:guard-entry | joined="/project/.. /outside.md" | beneath=true
".. "             | PASS:guard-entry | joined="/project/.. "             | beneath=true
"~/.. /secrets.md"| PASS:tilde-guard-entry | joined="/project/.. /secrets.md" | beneath=true
```

`path.Clean(".. /x")` → `".. /x"` (inalterado — espaço não é normalizado).
`containsUnsafeSegment(".. /x")` → segmentos `[".. ", "x"]` → `".. " ≠ ".."` → false.
`filepath.Rel(root, join(root, ".. /x"))` → `".. /x"` → não casa com `".."` nem com
`"../"` → `beneath` retorna true.

**Evidência Windows VM (binário de sonda):**
```
MkdirAll("...\\project\\.. ")  → err=nil   (Win32 apara espaço → vê ".." → parent já existe)
CreateTemp("...\\project\\.. ", ...) → err: O sistema não pode encontrar o caminho especificado.
Rename(temp, "...\\project\\.. ") → err: Acesso negado.
```

Win32 apara espaço/ponto do **componente final** mas **não** de componente intermediário
em path absoluto. `CreateTemp` falha porque Win32 não consegue navegar `".. "` como
diretório intermediário (procura literalmente `".. "`). `Rename` falha porque `".. "`
no final resolve para o diretório pai, que é um diretório existente, não um destino
de arquivo válido.

**`TestHadesR3DefaultBranch` (real code path, macOS + Windows VM):** PASS —
confirmado que não produz `"unsafe destination"` (gap no guard) e não produz escape
(OS/Win32 bloqueia em plataformas testadas).

**Impacto prático atual:** Nulo em Windows 11 ARM64 e macOS/Linux. A contenção vem
do OS, não dos guards.

**Por que é Ajuste (não Resíduo):** É o mesmo padrão que o roadmap (l.2471) declara
inaceitável — "guard early que não faz seu trabalho torna o sistema dependente de
uma camada". Aqui a única camada é o OS, e a dependência é frágil: Windows APIs em
paths UNC ou SMB podem ter comportamento diferente para trailing spaces.

**Alcançabilidade:** Baixa. `validateInstallPath` rejeita paths que não passam em
`path.Clean` → catálogo nunca produz `".. "`. Slug de terceiro limitado a
`[a-z0-9._-]` → impossível produzir espaço. Exposição real: flag `--destination`
passada manualmente (se existir) ou destino derivado de metadados externos não
validados.

**Correção proposta:** em `containsUnsafeSegment`, substituir

```go
if part == ".." {
```

por

```go
if strings.TrimRight(part, " .") == ".." {
```

Isso rejeita `".. "`, `".. ."`, `"..."` e qualquer combinação de trailing spaces e
dots que o Windows normalizaria para `".."`. O teste é revert-proof sem Windows:
`containsUnsafeSegment(".. /x")` retornaria `true` na versão corrigida, e
`TestHadesR3ContainsUnsafeSegment` (ou equivalente) verificaria que `".. "` → true e
`"dir"` → false. Válido em macOS/Linux.

**Nota:** `"..."` (três pontos) passaria a ser rejeitado como unsafe. Três pontos não
são um nome de arquivo legítimo em paths de skill/agente; o catálogo não os usa.
Se houver caso de uso legítimo, a correção pode ser restringida a
`strings.TrimRight(part, " ") == ".."` (só spaces, não pontos).

---

### R-2 — `..:stream` (NTFS ADS) (Resíduo declarado)

Guard entry: PASSA. beneath(): PASSA. rejectSymlinks: captura **acidentalmente** em
Windows — `Lstat("..:stream")` via Win32 retorna "sintaxe incorreta".

**Evidência Windows VM (TestHadesR3DefaultBranch stdout):**
```
trackfw: refusing write to C:\..\..:stream: CreateFile C:\..\..:stream:
A sintaxe do nome do arquivo, do nome do diretório ou do rótulo do volume está incorreta.
```

Em macOS: `..:stream` é nome de arquivo literal. Nenhuma leitura de stream ADS.
Sem escape em nenhuma plataforma. Alcançabilidade: slug restrito a `[a-z0-9._-]`
impossibilita `:`. **Aceito como resíduo.**

---

### R-3 — `C:..\x` (drive-relative Windows) (Resíduo declarado)

`containsUnsafeSegment("C:..\\x")` → split em `\\` → `["C:..", "x"]` → `"C:.." ≠ ".."` →
PASSA o guard. `filepath.Join(root, "C:..\\x")` em macOS = `root/C:..\\x` (literal).
Na VM: `os.Create("root\\C:..\\x")` → "A sintaxe do nome do arquivo... está incorreta."
Win32 rejeita `C:..` em componente de path relativo. **Aceito como resíduo.**

---

### R-4 — Homoglifos, percent-encoding, NUL (Resíduos declarados)

`..%2f`: Go não decodifica percent-encoding em paths — file criado com nome literal.
`..∕` (U+2215): não é separador em nenhum OS — literal.
NUL em path: Go `syscall.UTF16PtrFromString` (Windows) e syscall POSIX rejeitam
NUL com EINVAL — nunca chega ao Win32. Todos os três: aceitos pelo guard como filenames
literais inócuos.

---

## Seção 8 — Declaração de resíduos

| Resíduo | Declaração |
|---|---|
| Trailing space/dot | Bloqueado por Win32 e POSIX. Ajuste proposto (R-1). Aceito como resíduo ativo até o ajuste ser implementado. |
| `..:stream` | Bloqueado por Win32 "sintaxe incorreta". Aceito. |
| `C:..\x` | Bloqueado por Win32 "sintaxe incorreta". Aceito. |
| NUL em path | Bloqueado por Go/syscall EINVAL. Aceito. |
| `...` / `..%2f` / homoglifos | Filenames literais, sem semântica de traversal. Aceito. |
| 8.3 curto no default: | Não aplicável (paths derivados de root via filepath.Abs). Aceito. |
| Nomes reservados (CON/NUL) | Sem escape — NTFS permite em path completo. Aceito. |
| Slug de terceiro com `:` | Impossível — padrão `[a-z0-9._-]` proíbe. Aceito. |

---

## Evidências consolidadas

**macOS (código real, cópia rsync):**
```
=== RUN   TestHadesR3DefaultBranch  --- PASS (0.01s)
=== RUN   TestHadesR3TildeBranch    --- PASS (0.00s)
=== RUN   TestHadesR3TildeWithProjectScope --- PASS (0.00s)
=== RUN   TestHadesR3ContainsUnsafeSegment --- PASS (0.00s)
=== RUN   TestHadesR3ReservedNames  --- PASS (0.00s)
=== RUN   TestHadesR3ZeroFlipThirdPartyDestinations --- PASS (0.03s)
PASS  ok  github.com/kgsaran/trackfw/internal/integrations
```

**Windows ARM64 VM (`Lab@192.168.64.6`):**
```
=== RUN   TestHadesR3DefaultBranch  --- PASS (0.01s)
=== RUN   TestHadesR3TildeBranch    --- PASS (0.00s)
=== RUN   TestHadesR3TildeWithProjectScope --- PASS (0.00s)
=== RUN   TestHadesR3ContainsUnsafeSegment --- PASS (0.00s)
=== RUN   TestHadesR3ReservedNames  --- PASS (0.00s)
=== RUN   TestHadesR3ZeroFlipThirdPartyDestinations --- PASS (0.03s)
PASS
```

Nota: `TestHadesR3DefaultBranch` imprimiu o stderr esperado para `..:stream` no
Windows (`"refusing write to ... A sintaxe do nome do arquivo..."`) — isso confirma
que o path `..:stream` chega até `rejectSymlinks` antes de ser bloqueado pelo Win32.
