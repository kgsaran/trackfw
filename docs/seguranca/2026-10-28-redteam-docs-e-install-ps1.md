# Red Team — docs e install.ps1 (ML-4B)

> Barreira: hades-tf | REQ: REQ-2026-10-28 | Data: 2026-10-29

---

## Escopo e Método

Artefatos revisados: `scripts/install.ps1`, `scripts/check-install-ps1.ps1`,
`internal/guardforms/readme_line_test.go`, `README.md`, `site/guide/getting-started.md`,
`site/en/guide/getting-started.md`, `docs/cli-parity.md`.

Contratos de referência lidos antes dos artefatos:
- `docs/adr/ADR-2026-10-28-instalador-powershell-publicado-na-release-com-o-mesmo-contrato-do-install-sh.md`
- `docs/req/REQ-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell.md`
- `docs/seguranca/2026-10-28-wave0-docs-e-install-ps1.md`

**Nota de método:** os vetores de versão, checksum e tokens banidos foram derivados da ADR e da
Wave 0 antes de ler o código. Os vetores de `INSTALL_DIR` (F-1) e PATH com `;` final (F-2) foram
encontrados lendo o código nas linhas 130 e 260 de `install.ps1`; as expectativas correspondentes
foram registradas depois da leitura. Os vetores de cerca do README (F-3 e F-4) foram derivados de
leitura do código do gate. Os achados F-1 e F-2 têm, portanto, menor peso de barreira independente
— são achados de leitura de código, não de derivação de contrato.

**Ambiente de execução:** macOS (pwsh 7 via Homebrew). Comportamento Windows inferido para os casos
em que o macOS não pode simular (drive C:, registry, NTFS com `;` em pathname). Confirmações
Windows provenientes do CI run 40846363727 (`windows-install-ps1`, 30 PASS).

---

## Matriz de ACs (REQ-2026-10-28)

| AC | Atribuição | Verificação | Resultado |
|---|---|---|---|
| AC1: namespace único `kgsaran` em toda a documentação | ML-4B | `git grep -nE 'github\.com/trackfw/\|trackfw/tap\|kgsaran/tap' -- README.md CONTRIBUTING.md docs/visao-projeto docs/cli-parity.md docs/gate-design-principles.md site` → rc=1 (vazio). `VISION.md:203` usa `kgsaran/trackfw` (tap correto). | PASS |
| AC2: linha de hook do README idêntica ao byte ao constante em `guard_generations.go` | ML-3A + ML-4B | Gate `TestREADMEHookLinesMatchCurrentConstants` passa; 4 constantes requeridas encontradas em blocos cercados no README | PASS |
| AC3: PyPI wheel METADATA e READMEs duplicados removidos | ML-3C (fora do ML-4B) | — | fora do escopo ML-4B |
| AC4: `install.ps1` existe e gate roda em PS 5.1 e pwsh 7 | ML-1A + ML-4B | Gate macOS: 14 PASS / 17 SKIP; CI run 40846363727 (`windows-install-ps1`): 30 PASS. Ver F-1 para gap de cobertura. | PASS com ressalva F-1 |
| AC5: linha de hook PS/POSIX medida no CI Windows x64 | ML-2A (fora do ML-4B) | — | fora do escopo ML-4B |
| AC6: README com matriz Windows rota × arquitetura × shell | ML-3A (fora do ML-4B) | — | fora do escopo ML-4B |
| AC7: `cli-parity.md` sem afirmação de paridade entre runtimes; leitores mecânicos verdes | ML-3A (fora do ML-4B) | — | fora do escopo ML-4B |
| AC8: VISION e site descrevem a 11.x | ML-3B (fora do ML-4B) | — | fora do escopo ML-4B |
| AC9: instalação do `install.ps1` medida em VM Windows 11 ARM64 | ML-4A (paralelo) | — | fora do escopo ML-4B |
| AC10: `make quality` verde; `trackfw validate` verde | arquiteto (fechamento) | — | fora do escopo ML-4B |

---

## Achados Ordenados por Severidade

### F-1 — MEDIUM — TRACKFW_INSTALL_DIR com ponto-e-vírgula: validação não rejeita, idempotência quebra

**Arquivo e linha:** `scripts/install.ps1:130` (validação) e `scripts/install.ps1:245–261`
(idempotência + concatenação).

**Evidência:**

A validação na linha 130 aceita qualquer caminho que comece com `[A-Za-z]:[/\\]` ou `\\`:

```powershell
if (-not ($INSTALL_DIR -cmatch '\A[A-Za-z]:[/\\]' -or $INSTALL_DIR.StartsWith('\\'))) {
    throw "Erro: TRACKFW_INSTALL_DIR deve ser um caminho absoluto totalmente qualificado"
}
```

O valor `C:\tf;%TEMP%` satisfaz `-cmatch '\A[A-Za-z]:[/\\]'` e passa a validação sem erro.

**Quebra de idempotência (confirmada em pwsh 7, isolado):**

O verificador de idempotência nas linhas 245–254 divide o PATH existente por `;` e compara cada
entrada com `$installNorm`:

```powershell
$installNorm = $INSTALL_DIR.TrimEnd('\', '/').ToLower()  # = "c:\tf;%temp%"
foreach ($entry in ($existingPath -split ';')) {
    if ($entry -eq '') { continue }
    $entryNorm = $entry.TrimEnd('\', '/').ToLower()       # = "c:\tf" — nunca igual
    ...
}
```

Depois da primeira instalação com `C:\tf;%TEMP%`, o PATH contém a entrada `C:\tf`. Na segunda
execução, `$installNorm = "c:\tf;%temp%"` nunca casa com a entrada individual `"c:\tf"`, portanto
`alreadyPresent = false`. O instalador appenda novamente:

```
# Após run 1: PATH = "C:\Windows\System32;C:\tf;%TEMP%"
# Após run 2: PATH = "C:\Windows\System32;C:\tf;%TEMP%;C:\tf;%TEMP%"
# (medido: 5 entradas com entradas duplicadas de C:\tf e %TEMP%)
```

A ADR item 6 afirma "somente se ainda não estiver lá" e o AC4 exige cenário `path-idempotente` —
a garantia de idempotência fica quebrada para qualquer INSTALL_DIR que contenha `;`.

**Injeção de PATH via expansão de `%TEMP%`:** se o valor de PATH for `REG_EXPAND_SZ` (preservado
pelo instalador conforme a ADR), a entrada `%TEMP%` é expandida pelo sistema para o diretório
temporário do usuário. Qualquer executável depositado em `%TEMP%` fica acessível no PATH. Esse
vetor é teórico no macOS mas plausível no Windows com `REG_EXPAND_SZ`. Marcado como **não medido
end-to-end** por depender de comportamento de `New-Item`/`Copy-Item` em NTFS com `;` no pathname.

**Cobertura do gate:** `check-install-ps1.ps1` não tem nenhum `Assert-ValidationFail` para
`TRACKFW_INSTALL_DIR`. O AC4 exige "cada cenário tem contra-braço" — esse vetor está ausente.

**Cobertura da Wave 0:** §2.4 discute junction/symlink para `INSTALL_DIR` mas não semicolon. Gap
triplo: Wave 0 + código + gate.

**Correção mínima:**

```powershell
# Adicionar após a verificação de caminho absoluto (linha 132):
if ($INSTALL_DIR -match ';') {
    throw "Erro: TRACKFW_INSTALL_DIR nao pode conter ponto-e-virgula: '$INSTALL_DIR'"
}
```

**Vetor de gate obrigatório (com contra-braço):**

```powershell
# Falha esperada (validação rejeita):
Assert-ValidationFail 'install-dir-semicolon' @{ TRACKFW_INSTALL_DIR = 'C:\tf;C:\evil' } 'ponto-e-virgula'

# Contra-braço — false positive: caminho legítimo sem ";" ainda instala (já coberto por c1-intact).
# Citar c1-intact como contra-braço no comentário do gate.
```

---

### F-2 — LOW — Ponto-e-vírgula final no PATH existente: entrada vazia no resultado

**Arquivo e linha:** `scripts/install.ps1:260`.

**Evidência:**

Se `HKCU\Environment\PATH` termina com `;` (mal-formado mas possível em algumas configurações
Windows), a concatenação produz entrada vazia:

```powershell
$existingPath = "C:\Windows\System32;"  # trailing ";"
$newPath = if ($existingPath -ne '') { "$existingPath;$INSTALL_DIR" } else { $INSTALL_DIR }
# resultado: "C:\Windows\System32;;C:\Programs\trackfw"  (entrada vazia entre as duas)
```

Confirmado em pwsh 7. A entry vazia `""` no PATH escrito de volta ao registro é inerte no PS 7 e
no PowerShell 5.1 (nenhum dos dois interpreta string vazia como CWD); pode ter efeito em `cmd.exe`
em versões antigas do Windows (não medido). O risco principal é cosmético: o PATH fica com um `;;`
desnecessário.

A Wave 0 §2.7 prescreve o tratamento do caso `$oldPath` vazio (implementado corretamente na linha
260 com `if ($existingPath -ne '')`), mas não prescreve `TrimEnd(';')` para trailing semicolons.
Gap na Wave 0, não divergência de código.

**Correção mínima:**

```powershell
$cleanPath = $existingPath.TrimEnd(';')
$newPath = if ($cleanPath -ne '') { "$cleanPath;$INSTALL_DIR" } else { $INSTALL_DIR }
```

---

### F-3 — LOW — Gate do README: bypass por toggle de cerca via `~~~` dentro de ` ``` `

**Arquivo e linha:** `internal/guardforms/readme_line_test.go:120–128`
(`extractFencedGuardLines`).

**Evidência:**

O extrator usa toggle simples: qualquer linha cujo `TrimSpace` começa com ` ``` ` ou `~~~` inverte
`inFence`. Uma linha `~~~` dentro de um bloco ` ``` ` aberto reverte o estado para "fora de cerca".

Vetor B8b executado no harness isolado (`readme-gate-harness` em scratchpad):

```markdown
` ``` `sh
<todas as 7 formas correntes>
~~~
<forma legada CredentialLegacyPSPOSIX>
` ``` `
```

Resultado: PASS — o gate vê as formas correntes antes do `~~~`, não vê a legada depois, e passa
sem erro. A forma legada fica invisível.

Este é vetor de sabotagem estilizada, não regressão acidental. Uma revisão de PR detectaria a linha
`~~~` suspeita. A vacuidade guard (linha 48) não ajuda porque formas correntes foram inseridas
antes do toggle.

---

### F-4 — LOW — Linhas de hook em `docs/cli-parity.md` em células de tabela, fora do escopo do gate

**Arquivo:** `docs/cli-parity.md:6360` e `6362`.

**Evidência:**

```
cli-parity.md:6360: | PS/POSIX (...) | `emulate -R sh ... trackfw guard git-branch ...` |
cli-parity.md:6362: | Idem credential | `emulate -R sh ... trackfw guard credential ...` |
```

Verificado por `grep -F` contra as constantes extraídas de `guard_generations.go`:
`GitBranchCurrentPSPOSIX` encontrado na linha 6360 (**forma corrente D12**);
`CredentialCurrentPSPOSIX` encontrado na linha 6362 (**forma corrente D12**). Hoje não há
regressão. Mas o gate `TestREADMEHookLinesMatchCurrentConstants` lê apenas `../../README.md` —
essas linhas em tabela de `cli-parity.md` não são verificadas. Uma regressão futura passaria
despercebida no CI.

`site/guide/getting-started.md` e `site/en/guide/getting-started.md` não contêm linhas de hook;
contêm o comando `irm ... | iex` para instalação. Não são superfície deste achado.

Mesma causa que F-3 (escopo do gate limitado a README.md).

---

## Passes Adicionais

| Vetor | Resultado |
|---|---|
| Version regex — 14 vetores (newline, semicolon, subexpr, dotdot, uppercase V, arabic-indic, unicode ZWSP, rc/beta válidos, rc1234 rejeitado) | PASS |
| Tokens banidos: `TRACKFW_BASE_URL`, `TRACKFW_MIRROR`, `TRACKFW_INSTALL_DRYRUN`, `Microsoft.PowerShell.Utility\`, `irm`, `WebClient`, `BitsTransfer`, `Invoke-RestMethod`, `curl.exe`, `wget.exe`, `[Net.WebRequest]`, `HttpWebRequest`, `certutil`, `bitsadmin` | PASS |
| Arch validation: apenas `amd64\|arm64` aceitos; `x86` rejeitado com 'invalido' | PASS |
| INSTALL_DIR relativo `relative\path`: rejeitado | PASS |
| Checksum: separador tab/espaço, hash uppercase, BOM em PS7, duplicado, `.sbom` sibling, filename case-sensitive | PASS |
| `$env:SystemRoot\System32\tar.exe`: caminho absoluto, não `tar` simples | PASS |
| PATH registry: `valueKind` preservado; leitura via `[Microsoft.Win32.Registry]::CurrentUser.OpenSubKey` | PASS |
| CI run 40846363727 — `c1-intact`, `c2-tampered`, `c3-absent`, `c4-duplicate`, `c5-wrong-hash`, `no-modify-path`, `path-already-present`, `reg-expand-sz`, `traversal` | PASS |
| README gate B1–B8 (exceto B8b = F-3): trailing space, CRLF, fenced with language hint, outside fence, table-only, 4-space indent, legacy alongside current | PASS |
| Vacuidade do README gate (falha se não encontrar linhas no README) | PASS |
| Namespace AC1: docs sem `github.com/trackfw/` ou `kgsaran/tap` indevidos | PASS |
| Nota de "asset a partir do release após v11.1.0" em README, site e cli-parity | PASS |
| cli-parity.md:6360,6362 contêm formas D12 correntes (não legadas) | PASS |

---

## Residuais Declarados (não bloqueantes)

**R-1 (Wave 0 §residual):** `$env:SystemRoot` para selecionar `tar.exe` — variável controlável.
Em ambiente adversarial com env vars comprometidas, o atacante pode apontar para um `tar.exe`
próprio. Baixa probabilidade (controle de `SystemRoot` implica comprometimento total). Aceito.

**R-2 (Wave 0 §residual):** BOM em `checksums.txt` em PS 5.1 — confirmado indiretamente por CI
run 40846363727 (30 PASS incluindo cenários de checksum em PS 5.1 Desktop). Não medido em macOS.
Aceito com evidência indireta.

**R-3 (ADR §Consequences):** outro `trackfw.exe` mais cedo no PATH. O instalador avisa, não remove.
Explícito na ADR.

---

## Veredito

**REPROVA** — F-1 tem cobertura zero no gate e quebra a garantia de idempotência declarada na ADR
(item 6) e exigida no AC4. A correção é de uma linha no validador e de um vetor no gate.

### Correctives obrigatórios antes de reauditoria

**C1.** `scripts/install.ps1` linha 132 — adicionar rejeição de `;` em `INSTALL_DIR`:

```powershell
if ($INSTALL_DIR -match ';') {
    throw "Erro: TRACKFW_INSTALL_DIR nao pode conter ponto-e-virgula: '$INSTALL_DIR'"
}
```

**C2.** `scripts/check-install-ps1.ps1` — adicionar vetor e contra-braço:

```powershell
Assert-ValidationFail 'install-dir-semicolon' @{ TRACKFW_INSTALL_DIR = 'C:\tf;C:\evil' } 'ponto-e-virgula'
# Contra-braço: c1-intact (caminho legítimo sem ";" ainda instala) — citar no comentário.
```

### Correctives recomendados (não bloqueantes para reauditoria)

**C3.** `scripts/install.ps1` linha 260 — `$existingPath.TrimEnd(';')` antes de concatenar (F-2).

**C4.** `scripts/check-install-ps1.ps1` — vetor de PATH com `;` final + contra-braço (F-2).

**C5.** `internal/guardforms/readme_line_test.go` — ampliar para `docs/cli-parity.md` ou adicionar
teste de não-regressão separado para as linhas de tabela de hook desse arquivo (F-3/F-4).

**Todo residual acima vira ML neste roadmap, não REQ nova** (mesma causa, mesma REQ, per
`CLAUDE.md` §Regra Dura de Causa Raiz).

---

## Reauditoria (2026-10-29)

> Barreira: hades-tf | Commits reauditados: 1257d81b (ML-4C) e ad4a4f4b (ML-4D)

### Veredito da reauditoria

APROVADO COM RESSALVAS

Os quatro achados originais F-1 a F-4 estão corrigidos e verificados contra HEAD com vetores
executados sobre o script e os testes reais. A superfície do aviso de `~/bin` não expõe
execução nem leitura de arquivo. A Limitação 1 do ML-4D (A-3) deixa passar regressão futura em
cmd.exe, medida e aceita como R-6 porque afeta documentação, não hook implantado. Quatro achados
novos (A-1 a A-4) são de baixa severidade e não bloqueiam o merge; correções mínimas declaradas.
A-1 aceito como residual (risco ANSI legacy, mapeamento CP1252 confirmado, impacto não medido
end-to-end). Dois residuais adicionais aceitos com motivo escrito (R-5, R-6).

---

### Método

Os vetores originais F-1 a F-4 foram refeitos contra HEAD usando o script real
`scripts/install.ps1` (não uma reimplementação isolada) e `go test`. A superfície nova do
aviso de `~/bin` foi medida com pwsh macOS em sete vetores. As limitações declaradas do ML-4D
foram analisadas e concluídas. Os cenários de registro e de `~/bin` são marcados como
Windows-only no gate; a verificação deles apoia-se no CI run 40918775429 (`windows-install-ps1`,
39/39 PASS, PS 5.1 + pwsh 7).

Nota de reconciliação de contagem: o gate local (macOS) contabiliza 42 cenários (17 PASS +
25 SKIP); o CI Windows reporta 39 PASS + 0 SKIP por versão (39×2 = 78 linhas `PASS` no log do
run 40918775429, confirmadas por `grep`). A diferença de 3 são cenários presentes no runner macOS
e ausentes do runner Windows: `sab-gitbash-bin-warning-nonvacuous`, `sab-trailing-semicolon-nonvacuous`
e `traversal-counter-arm-macos`. Esses cenários não emitem linha `PASS`/`SKIP` no Windows
(bloqueados por condicional de plataforma). Diferença medida por `comm` entre listas de nomes.

Nota de escopo: o ambiente de medição é macOS com pwsh 7 (Homebrew). Comportamentos que
dependem de driver letter (`C:`), Win32 registry ou NTFS — incluindo os cenários `path-trailing-semicolon`,
`gitbash-bin-warning`, `reg-expand-sz` e afins — são marcados como Windows-only e aferidos
exclusivamente pelo CI Windows. Casos onde o comportamento em Windows difere do macOS são
declarados explicitamente abaixo.

---

### F-1 — TRACKFW_INSTALL_DIR com ponto-e-vírgula

**Código (`scripts/install.ps1` linha 131):**

```powershell
if ($INSTALL_DIR -match ';') {
    throw "Erro: TRACKFW_INSTALL_DIR nao pode conter ponto-e-virgula: '$INSTALL_DIR'"
}
```

O guard fica nas linhas 131–133, antes da validação de caminho absoluto (linha 137) e antes de
qualquer escrita (primeiro `New-Item` na linha 146). Confirmado por leitura.

**Vetores executados contra o script real** (env: `TRACKFW_VERSION=v99.0.0`, `TRACKFW_ARCH=amd64`,
macOS pwsh — script abortou em linha 132 ou 146, nunca chegou a download):

| Vetor | TRACKFW_INSTALL_DIR | rc | Linha abortada |
|---|---|---|---|
| ASCII `;` no meio | `C:\tf;%TEMP%` | 1 | 132 (throw ponto-e-virgula) |
| ASCII `;` no início | `;C:\tf` | 1 | 132 (throw ponto-e-virgula) |
| ASCII `;` no fim | `C:\tf;` | 1 | 132 (throw ponto-e-virgula) |
| U+FF1B `；` | `C:\tf；evil` | 1 | 146 (New-Item, macOS não reconhece `C:`) |

O vetor U+FF1B passa o guard da linha 131 e falha na linha 146 por motivo diferente (macOS
não tem `C:` drive). O comportamento relevante **no Windows** é que a linha 146 criaria o
diretório com U+FF1B no nome (NTFS permite); o instalador tentaria o download, que falharia
para `v99.0.0` inexistente. A questão de segurança para U+FF1B está no separador de PATH —
ver achado A-1.

**Gate (C1/C2):** cenários `install-dir-semicolon` (rejeição com `ponto-e-virgula`) e
`install-dir-semicolon-no-dir` (ausência de escrita anterior) em `scripts/check-install-ps1.ps1`
linhas 420–428. Contra-braço: `c1-intact` (caminho legítimo sem `;` instala). Gate macOS: PASS.
CI Windows run 40918775429: 39/39.

**Veredito F-1:** PASS (C1 e C2 entregues). Achado A-1 para variantes Unicode — ver abaixo.

---

### F-2 — Ponto-e-vírgula final no PATH existente

**Código (`scripts/install.ps1` linha 268):**

```powershell
$cleanPath = $existingPath.TrimEnd(';')
$newPath = if ($cleanPath -ne '') { "$cleanPath;$INSTALL_DIR" } else { $INSTALL_DIR }
```

Gate (C4): cenários `path-trailing-semicolon` e `path-trailing-semicolon-idempotent` nas linhas
1651–1691 do gate — SKIP em macOS (dependem do registro), PASS no CI Windows run 40918775429.

**Veredito F-2:** PASS.

---

### F-3 — Toggle de cerca: `~~~` dentro de bloco ` ``` `

**Código:** `extractFencedGuardLines` reescrita em `internal/guardforms/readme_line_test.go`
aplicando regras CommonMark (mesmo caractere, comprimento ≥ abertura, sem texto após o run de
fechamento).

**Vetor B8b — independente (standalone Go):**

Reimplementação mínima de `fenceInfo`/`closesBlock`/`extractFencedGuardLines` executada fora
do módulo do projeto. Entrada: `` ```sh ``, as formas correntes, linha `~~~`, forma legada de
credential, `` ``` ``. Saída medida: 5 linhas extraídas — todas as formas correntes MAIS a forma
legada. Conclusão: `~~~` não fecha o bloco `` ``` ``; a forma legada é visível e o gate a
rejeitaria.

```
Extracted lines (5):
  "emulate -R sh -c '...'; trackfw guard credential; ..."
  "emulate -R sh -c '...'; trackfw guard credential --global; ..."
  "emulate -R sh -c '...'; trackfw guard git-branch; ..."
  "emulate -R sh -c '...'; trackfw guard write-scope; ..."
  "trackfw guard credential; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }"
```

**Sub-testes do implementador** (todos PASS):
`B8b_tilde_inside_backtick_fence_does_not_close`, `four_backtick_fence_not_closed_by_three_backticks`,
`indented_fence_up_to_3_spaces_is_valid`, `closing_with_trailing_text_does_not_close`.

**Veredito F-3:** PASS.

---

### F-4 — Linhas de hook em `docs/cli-parity.md` fora do escopo do gate

**Código:** `extractCLIParityGuardForms` + `checkCLIParityGuardForms` em linhas 279–490 de
`internal/guardforms/readme_line_test.go`.

**Vetor — forma legada na linha de credential (independente):**

Verificação direta em `docs/cli-parity.md`: 1 forma PS/POSIX de credential na linha 6361
(backtick span em célula de tabela, contém `LASTEXITCODE` e `trackfw guard credential`).
Substituindo o prefixo `emulate -R sh 2>${null-/dev/null}; ` por string vazia, a forma
resultante não consta nos registros `currentForms` nem `legacyPSPOSIX` do gate — o
`checkCLIParityGuardForms` emitiria o erro `"unrecognized PS/POSIX guard form"`. Sabotagem
confirmada como não-vácua: o form original está presente verbatim no arquivo.

**Sub-testes do implementador** (ambos PASS):
`real_file_green` (arquivo atual passa, vacuidade ≥ 3 formas); `sabotage_legacy_without_marker_reproves`
(git-branch legada em linha não-histórica detectada).

**Veredito F-4:** PASS.

---

### Superfície nova — aviso de `%USERPROFILE%\bin\trackfw.exe`

Código relevante em `scripts/install.ps1` (linhas 299–312), executado com `$ErrorActionPreference = 'Stop'`
(definido na linha 20):

```powershell
if ($env:USERPROFILE) {
    $gitBashBin     = Join-Path (Join-Path $env:USERPROFILE 'bin') 'trackfw.exe'
    $gitBashBinNorm = $gitBashBin.TrimEnd('\', '/').ToLower()
    $destBinNorm    = $destBin.TrimEnd('\', '/').ToLower()
    if ($gitBashBinNorm -ne $destBinNorm -and (Test-Path -LiteralPath $gitBashBin -PathType Leaf)) {
        Write-Warning "Atencao: $gitBashBin existe."
        ...
    }
}
```

**`USERPROFILE` vazio ou nulo.**
Medido: `$env:USERPROFILE = ''` → bloco ignorado (`if ($env:USERPROFILE)` é falso). Idem `$null`.
Nenhuma chamada a `Join-Path`. PASS.

**`Test-Path -LiteralPath` com wildcard no nome do arquivo.**
Medido: arquivo `track[fw].exe` criado em diretório temporário. `-LiteralPath` retornou `True`
(correto); sem `-LiteralPath` retornou `False` (glob expandido). Implementação usa `-LiteralPath`.
PASS.

**Symlink ou junction em `bin\trackfw.exe`.**
`Test-Path -LiteralPath $gitBashBin -PathType Leaf` retorna `True` tanto para arquivo regular
quanto para symlink de arquivo. Medido no macOS com `ln -s`. O código exibe `Write-Warning` apenas —
não lê, executa, nem desreferencia o arquivo. Comportamento correto: um symlink que oculte a
instalação merece o aviso. Junctions (NTFS) não medidas no macOS. PASS com ressalva para junctions:
`PathType Leaf` no Windows retorna `True` para file-junctions; comportamento é o mesmo (aviso
emitido), sem impacto de segurança.

**`USERPROFILE` com caminho de traversal (`..`).**
`Join-Path` não resolve `..` — o caminho resultante contém `..` literalmente. Medido com
`/home/user/../other`: `Join-Path` produziu `/home/user/../other/bin/trackfw.exe`. `Test-Path -LiteralPath`
no Windows chamaria `GetFileAttributes` que RESOLVE `..` via filesystem. Impacto: o aviso pode
apontar para o caminho resolvido. Restrito ao componente informativo — não controla download nem
escrita. PASS (aceito como residual R-5).

**Comparação de caixa com o destino.**
Ambos os caminhos passam por `.TrimEnd('\', '/').ToLower()` antes da comparação. Em Windows
(filesystem case-insensitive), `ToLower()` produz comparação correta. Se `INSTALL_DIR` for
exatamente `%USERPROFILE%\bin`, `gitBashBinNorm == destBinNorm` e o aviso é suprimido —
comportamento correto. PASS.

**Arquivo nunca executado nem lido.**
`Test-Path -LiteralPath` usa chamada de sistema de existência (stat equivalente). Nenhum
`Invoke-Expression`, `&`, `Start-Process`, `Get-Content`, `[IO.File]::ReadAll*` ou similar
presente no bloco. Confirmado por leitura. PASS.

**Vetor novo — `USERPROFILE` com valor UNC (`\\server\share\home`).**

Medido sob `$ErrorActionPreference = 'Stop'`:
```
Join-Path (Join-Path '\\server\share\home' 'bin') 'trackfw.exe'
```
Resultado: **erro terminante** — "Cannot process argument because the value of argument 'drive'
is null." O bloco de `~/bin` fica entre a cópia do binário (linha ~219) e a mensagem de sucesso
(linha ~315). Se `Join-Path` lançar um erro terminante, `$ErrorActionPreference = 'Stop'`
propaga o erro pelo `try`/`finally` — a instalação (binário + PATH) já está completa, mas o
instalador **sai com rc≠0 sem imprimir a mensagem de sucesso**. Implantações automatizadas que
verificam `$LASTEXITCODE` concluiriam que a instalação falhou.

Comportamento no Windows com USERPROFILE UNC: não medido diretamente. No Windows, o
FileSystem provider do PowerShell suporta UNC paths, mas o erro "value of argument 'drive' is null"
sugere que o decompositor de qualificador de `Join-Path` falha com UNC tanto no macOS quanto
possivelmente no Windows. Classificado como achado A-2 — ver abaixo.

---

### Limitações declaradas do ML-4D

**Limitação 1 — formas cmd.exe não verificadas.**

O gate exclui formas cmd.exe de `cli-parity.md` por ambiguidade: a forma legada D2
(`trackfw guard git-branch`) é textualmente idêntica a menções de subcomando em prosa (o
discriminador `LASTEXITCODE` não está presente). `cli-parity.md:6361` contém a forma D12
correta `` `trackfw guard git-branch \|\| exit 2` `` — verificado por `grep`. Se um commit
futuro remover `|| exit 2`, o gate não detectaria. **Avaliação:** nenhuma regressão em HEAD;
gap declarado no código (comentário nas linhas 279–298); célula em documentação, não em
template de hook implantado. Não deixa regressão real passar hoje. Residual R-6.

**Limitação 2 — critério de "histórico" por linha.**

`isHistoricalLine` aceita forma legada PS/POSIX somente se a linha contiver `pre-ML-`,
`pre-REQ-`, `pré-v8` ou `legad`. Avaliado em dois sentidos:

- *Falso negativo de segurança (regressão não detectada):* não ocorre — inserir uma forma legada
  sem marcador é detectado como erro pelo gate. Esse é o comportamento correto.
- *Falso positivo de workflow (gate bloqueia referência histórica legítima):* possível se o texto
  não usar nenhum dos quatro marcadores. Risco de workflow, não de segurança. Mitigação: adicionar
  `(legada)` ou `pre-ML-X` ao texto.

`real_file_green` confirma zero falsos positivos no HEAD atual. Não deixa regressão real
passar hoje.

---

### Achados novos

**A-1 — LOW — Variantes Unicode de `;` passam o guard**

`-match ';'` verifica U+003B (ASCII) por comparação ordinal. Três variantes não são capturadas:

| Caractere | Código | Normalização | `-match ';'` | PATH-split Windows (Unicode) | best-fit CP1252 (ANSI) |
|---|---|---|---|---|---|
| FULLWIDTH SEMICOLON | U+FF1B | NFKC → U+003B | False | Não (ordinal) | 0x3B — confirmado (`bestfit1252.txt`) |
| SMALL SEMICOLON | U+FE54 | NFKC → U+003B | False | Não (ordinal) | não consta em `bestfit1252.txt` |
| GREEK QUESTION MARK | U+037E | NFC → U+003B | False | Não (ordinal) | 0x3B — confirmado (`bestfit1252.txt`) |

Medido: PowerShell `-split ';'` não divide em U+037E (comparação ordinal, confirmado com
codepoints na saída). Medido: NFC(U+037E) = U+003B (normalização Unicode confirma canonicidade).
`bestfit1252.txt` ([unicode.org/MICSFT/WindowsBestFit](https://www.unicode.org/Public/MAPPINGS/VENDORS/MICSFT/WindowsBestFit/bestfit1252.txt)) confirma `0xff1b → 0x3b` e `0x037e → 0x3b`.
U+FE54 não figura na tabela CP1252.

Risco residual que não foi medido end-to-end: aplicações ANSI (CP1252) em Windows aplicam
best-fit na conversão de Unicode para ANSI. Para U+FF1B e U+037E, a tabela oficial confirma
a conversão para 0x3B. Se um aplicativo ANSI lê o PATH via `GetEnvironmentVariableA`, um
`INSTALL_DIR` contendo U+FF1B ou U+037E seria tratado como duas entradas separadas. Aplicações
modernas usam Unicode APIs e não são afetadas. Aceito como residual (ver fim desta seção).

**Correção mínima:** estender a validação para incluir as variantes confirmadas e a variante NFKC:

```powershell
if ($INSTALL_DIR -match '[;\u037E\uFE54\uFF1B]') {
    throw "Erro: TRACKFW_INSTALL_DIR nao pode conter ponto-e-virgula ou variante Unicode: '$INSTALL_DIR'"
}
```

**Severidade:** LOW — requer que o atacante controle `TRACKFW_INSTALL_DIR` com caracteres Unicode
incomuns; o impacto (PATH splitting em aplicações ANSI legadas via U+FF1B/U+037E) é indireto.
Aceito como residual: exige INSTALL_DIR com Unicode incomum; risco restrito a aplicações ANSI.
Não bloqueia o merge.

---

**A-2 — LOW — `Join-Path` pode causar rc≠0 após instalação bem-sucedida em dois sítios**

Medido no macOS: `Join-Path` lança erro terminante quando o caminho não tem qualificador de
unidade reconhecido pelo PSProvider (no macOS, `C:` não existe). O mecanismo Windows correspondente
é uma PSDrive mapeada mas desconectada — hipótese, não medida. O provider FileSystem do Windows
suporta caminhos UNC nativamente; `Join-Path '\\server\share' 'bin'` no Windows não foi medido.

**Sítio 1 — bloco de `~/bin` (linhas 304–305).**
Medido (macOS): `Join-Path (Join-Path '\\server\share\home' 'bin') 'trackfw.exe'` lança erro
terminante sob `$ErrorActionPreference = 'Stop'`.
O bloco de `~/bin` é executado após a cópia do binário e a modificação do PATH no registro.
Um erro aqui faz o instalador sair rc≠0 sem imprimir a mensagem de sucesso — embora a
instalação esteja funcionalmente completa.
Gatilho plausível no Windows: USERPROFILE aponta para PSDrive desconectada (não a UNC direta,
que o provider trata; HOMESHARE é o caminho UNC em perfis de roaming, não USERPROFILE).

**Sítio 2 — loop de "outras cópias" (linha 289).**
`$candidate = Join-Path $dir "trackfw.exe"` é executado para cada entrada de `$env:PATH`.
Gatilho plausível: entrada de PATH com letra de unidade não mapeada (e.g., `Z:\tools` após
desconexão de rede). Medido (macOS): `Join-Path 'C:\stale' 'trackfw.exe'` não lança (macOS não
tem `C:`; o PSProvider macOS retorna o caminho sem validação de unidade). Comportamento real no
Windows não medido.

**Correção mínima:** substituir ambos os sítios para usar `[IO.Path]::Combine(...)` —
concatenação de string pura, sem consulta ao PSProvider:
- Sítio 1: `[IO.Path]::Combine($env:USERPROFILE, 'bin', 'trackfw.exe')`
- Sítio 2: `[IO.Path]::Combine($dir, 'trackfw.exe')`

**Severidade:** LOW — instalação funcionalmente completa (Sítio 1) ou interrompida antes da
seção de avisos (Sítio 2); comportamento Windows não medido diretamente; em nenhum caso é
escrita/executada coisa não autorizada. Não bloqueia o merge.

---

**A-3 — LOW — Formas cmd.exe em `docs/cli-parity.md` fora do escopo do gate**

O gate `TestCLIParityHookLines` cobre formas PS/POSIX em `docs/cli-parity.md`. A forma cmd.exe
na linha 6361 (`` `trackfw guard git-branch || exit 2` ``) não é verificada.

Vetor: substituir `|| exit 2` na linha 6361 por string vazia (forma legada D2).
`real_file_green` passa sem detectar a regressão. Medido.

Razão da exclusão: `trackfw guard git-branch` (sem `|| exit 2`) é textualmente idêntico a
menções de subcomando em prosa — não há discriminador confiável sem heurísticas frágeis.
Gap declarado no código (comentário nas linhas 279–298 de `readme_line_test.go`).
Nenhuma regressão em HEAD; célula em documentação, não em template de hook implantado.

**Severidade:** LOW — afeta documentação, não hook implantado. Aceito como residual R-6.
Não bloqueia o merge.

---

**A-4 — LOW — `.ToLower()` sensível à cultura: falso-positivo de aviso em locale tr-TR**

`$gitBashBinNorm = $gitBashBin.TrimEnd('\', '/').ToLower()` aplica comparação sensível à
cultura. Em tr-TR, `I` (U+0049) minusculiza para `ı` (U+0131, dotless i) em vez de `i` (U+0069).

Condição para o falso-positivo: INSTALL_DIR foi fornecido com caminhos em minúsculo e USERPROFILE
usa letra maiúscula (`C:\Users\Ivan\...` vs `C:\Users\ivan\...`). Os dois caminhos apontam
para o mesmo arquivo (Windows é case-insensitive), mas sob tr-TR `.ToLower()` produz `ı` para
um e `i` para o outro — comparação falha, aviso espúrio emitido.

Medido: `'C:\Users\Ivan\bin\trackfw.exe'.ToLower()` e
`'C:\Users\ivan\bin\trackfw.exe'.ToLower()` divergem sob tr-TR (`ı` ≠ `i`).

**Correção mínima:**
```powershell
$same = [string]::Equals(
    $gitBashBin.TrimEnd('\', '/'),
    $destBin.TrimEnd('\', '/'),
    [System.StringComparison]::OrdinalIgnoreCase
)
if (-not $same -and (Test-Path -LiteralPath $gitBashBin -PathType Leaf)) { ... }
```

**Segundo sítio — verificação de idempotência (linhas 251 e 255).**
`$installNorm = $INSTALL_DIR.TrimEnd('\', '/').ToLower()` e
`$entryNorm = $entry.TrimEnd('\', '/').ToLower()` usam o mesmo `.ToLower()`. Sob tr-TR,
um INSTALL_DIR com `I` maiúsculo e uma entrada PATH existente com `i` minúsculo (ou vice-versa)
produzem normas divergentes — o check de idempotência conclui "ausente" e adiciona uma entrada
duplicada. Na execução seguinte, uma das entradas já está no PATH e não há crescimento adicional.
Impacto: uma entrada duplicada por execução em ambiente tr-TR com capitalização mista.

**Severidade:** LOW — aviso espúrio (sítio 1) ou entrada duplicada pontual (sítio 2) em locales
turcos com caminhos de capitalização mista. Sem impacto de segurança. Não bloqueia o merge.

---

### Residuais novos

**R-4 (A-1 — variantes Unicode de `;` em PATH ANSI):** aceito na própria seção A-1.

**R-5 (USERPROFILE com traversal):** impacto restrito ao aviso informativo. Aceito.

**R-6 (A-3 — cmd.exe forms fora do gate):** gap declarado no código; nenhuma regressão em HEAD.
Aceito.

---
