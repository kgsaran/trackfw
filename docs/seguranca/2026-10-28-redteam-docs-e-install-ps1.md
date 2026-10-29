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
