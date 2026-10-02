---
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md"
ml: "ML-0A"
author: "hades-tf"
date: "2026-10-02"
---

# Wave 0 — Modelo de Ameaça: critério de identificação de ADR por prefixo `ADR-`

> ML-0A do ROADMAP-2026-10-02 | Hades | Nenhuma linha de implementação

---

## 1. Completude dos sítios

### Grep de varredura

```
$ grep -rn "ADRDirs\|walkADR\|ResolveADRFiles\|findADRFile\|countMDFiles\|WalkADRFilePaths\
\|walkADRFiles\|scanChainDir" internal/ --include="*.go" | grep -v "_test.go" \
| grep -v "^.*://"
```

Linhas relevantes para ADR (parcial — omitindo config/commands que apenas carregam a estrutura):

```
internal/discover/discover.go:478:   r.ADRCount = len(validator.ResolveADRFiles(...))   ← caminho declarado
internal/discover/discover.go:486:   r.ADRCount += countMDFiles(...)                    ← fallback subdir
internal/discover/discover.go:491:   r.ADRCount = countMDFiles(adrRoot)                 ← fallback plano
internal/generators/context.go:42:  for _, full := range validator.ResolveADRFiles(cfg) ← herda
internal/generators/adr.go:193:     paths := validator.WalkADRFilePaths(dir)            ← herda
internal/generators/adr.go:319:     for _, p := range validator.WalkADRFilePaths(adrDir)← herda
internal/generators/update.go:305:  if len(validator.WalkADRFilePaths(globalDir)) == 0  ← herda
internal/serve/api_chain.go:44:     ns, es := scanChainDir(cfg, adrDir, "adr")          ← fora do primitivo
internal/validator/validator.go:342: diskUnionRoots = append(..., diskCfg.ADRDirs...)   ← scope-redirect (ver nota)
internal/validator/validator.go:1449: adrCount := len(ResolveADRFiles(cfg))             ← herda
internal/validator/validator.go:2486: for _, p := range ResolveADRFiles(cfg)            ← herda
internal/validator/validator.go:2843: p := findADRFile(adrBasename, cfg.ADRDirs)        ← fora do primitivo
internal/validator/validator.go:3107: func walkADRFilePathsForRule(...)                 ← PRIMITIVO
internal/validator/validator.go:3137: func findADRFile(...)                             ← WalkDir próprio
```

### Tabela de sítios

| sítio | arquivo:linha | critério atual | muda? |
|---|---|---|---|
| `walkADRFilePathsForRule` (primitivo) | `validator.go:3107` | `HasSuffix(path, ".md")` | **SIM** (D1/D2) |
| `ResolveADRFiles` (consumidor) | `validator.go:3088` | herda do primitivo | via primitivo |
| `WalkADRFilePaths` (consumidor) | `validator.go:3072` | herda | via primitivo |
| `walkADRFiles` (consumidor) | `validator.go:3126` | herda | via primitivo |
| regras do `validate` | `validator.go:1449,2486,2900` | consomem `ResolveADRFiles` | via primitivo |
| `generators/context.go` | `context.go:42` | `ResolveADRFiles(cfg)` | via primitivo |
| `generators/adr.go` (listagem/numeração) | `adr.go:193,319` | `WalkADRFilePaths(dir)` | via primitivo |
| `generators/update.go` | `update.go:305` | `WalkADRFilePaths(globalDir)` | via primitivo |
| **`findADRFile`** | `validator.go:3137` | `WalkDir` próprio, `Base(path)==adrBasename` | **NÃO** (ver decisão abaixo — diverge do D2) |
| **`scanChainDir`** (`serve/api_chain.go:88`) | `api_chain.go:88` | `HasSuffix(d.Name(), ".md")` | **SIM** (D3) |
| **`discover.go` fallback — subdir** | `discover.go:486` | `countMDFiles(...)` | **SIM** (D3) |
| **`discover.go` fallback — plano** | `discover.go:491` | `countMDFiles(adrRoot)` | **SIM** (D3) |
| `discover.go` caminho declarado | `discover.go:478` | `ResolveADRFiles(...)` | via primitivo (já correto) |
| **`scope-redirect` (`mdBasenamesOnDisk`)** | `validator.go:342,454` | `filepath.Ext == ".md"` (todos os `.md`) | **NÃO** (intencionalmente mais amplo — ver nota abaixo) |
| `api_metrics.go countMDFiles` | `api_metrics.go:227` | `HasSuffix(.md)` | NÃO — conta roadmaps, não ADRs |
| `api_board.go HasSuffix` | `api_board.go:95` | `HasSuffix(.md)` | NÃO — processa roadmaps |
| `generators/context.go HasSuffix` (~L80,100) | `context.go:80,100` | `HasSuffix(.md)` | NÃO — enumera roadmaps |

**Nota sobre `scope-redirect` (`mdBasenamesOnDisk`, `validator.go:342/454`):**
Este sítio constrói a **união de todos os `.md`** em `req_dir`, `roadmap_dir` e `adr_dirs` para
detectar artefatos perdidos quando um desses diretórios é movido. Ele varre propositalmente mais
amplo que o critério ADR (inclui qualquer `.md`) para ser **fail-closed**: se um ADR sem prefixo
existia e sumiu ao mover `adr_dirs`, o sítio o detecta como "perdido" e acusa. Restringi-lo ao
prefixo `ADR-` criaria um falso negativo de segurança — o ADR movido passaria invisível. Este sítio
deve permanecer no critério atual.

**Conclusão: tabela fechada.** Os sítios identificados cobrem todas as entradas de enumeração,
contagem e resolução de ADR no código Go. Nenhum sítio está ausente da análise.

**`check-adr-enumeration-single-point.sh`:** exclui `discover.go` com o comentário "pre-config scan;
does NOT consume cfg.ADRDirs". Após D3 essa justificativa ficará obsoleta. O gate não precisa mudar
seus padrões de detecção (Pattern A detecta `os.ReadDir` com `cfg.ADRDirs`; o `countMDFiles` usa
`filepath.WalkDir` e não é pego por esses padrões). O comentário de isenção deve ser atualizado em
ML-1B — ajuste não bloqueante (ajuste #4 na seção de Veredito).

### Decisão sobre `findADRFile` — divergência explícita do ADR D2

O ADR D2 afirma: "Todo consumidor que enumera ADR passa por ele: `ResolveADRFiles`, `WalkADRFilePaths`
e `findADRFile`." Wave 0 recomenda divergir para `findADRFile`, pelas seguintes razões:

`findADRFile` não enumera — resolve uma referência explícita. Os seus chamadores são:
- `blocked_by_draft_adr` (~L2843): verifica se um ADR listado em `blocked_by:` está em rascunho.
- `adr_accepted_when_req_done` (~L3606): verifica se o ADR vinculado foi aceito quando a REQ fecha.

Se `findADRFile` aplicar o critério de prefixo e um ADR legado (sem prefixo, com `| Status: Draft`
no cabeçalho e sem frontmatter `status:`) for listado como bloqueador:
- `findADRFile` retorna `""` (não encontrado por critério)
- `adrStatusForRule` retorna `("", true)` — "não encontrado = sucesso"
- `blocked_by_draft_adr` **não dispara** — o bloqueio desaparece sem sinal
- `resolveAdrStatus` usa `readRegularFile` e cai para a linha de cabeçalho só se abriu o arquivo; sem arquivo, o fallback não alcança
- O D4 só detecta `status:` em **frontmatter** — não alcança o `| Status: Draft` do cabeçalho

Resultado: um ADR legítimo em rascunho, mal nomeado, continua bloqueando uma REQ sem que nenhuma
regra acuse. Isso é **fail-open na direção que importa para governança**.

A escolha fail-closed é manter `findADRFile` resolvendo qualquer basename, independente do prefixo.
`NOTAS.md` com `status: Draft` no frontmatter continuaria bloqueando a REQ — mas agora com D4
emitindo um aviso "este arquivo não é contado como ADR, renomeie para `ADR-…`". O usuário tem sinal.
Sem o aviso, a situação seria silenciosa — mas não pior do que o estado atual.

**Decisão: `findADRFile` NÃO aplica o critério de prefixo.** Razão: a alternativa (aplicar o
critério) cria bypass silencioso para ADRs legados com status via cabeçalho, que é exatamente o caso
do `ADR-001` deste repositório antes de ganhar frontmatter. O D4 precisa de um ajuste adicional para
cobrir o cabeçalho (`resolveAdrStatus`) — ver ajuste #2 na seção de Veredito.

**Regras cujo veredito muda dependendo da escolha:**
- Com critério aplicado (descartada): `blocked_by_draft_adr` e `adr_accepted_when_req_done` mudam de
  "dispara para arquivos sem prefixo" para "não dispara" — silêncio sem sinal.
- Sem critério (recomendado): `blocked_by_draft_adr` e `adr_accepted_when_req_done` mantêm o
  comportamento atual para referências explícitas; o D4 avisa se o arquivo referenciado não tem prefixo.

---

## 2. Modelo de ameaça

O adversário aqui é o implementador apressado e o arquiteto otimista, não um atacante externo.

### 2.1 Symlink para diretório fora de `adr_dirs`, nomeado `ADR-evil.md`

Medido com `scratchpad/symlinktest2.go`:

```
$ go run symlinktest2.go
Symlink ADR-evil.md -> outsideDir: err=<nil>

=== WalkDir result ===
  ADR-001-real.md   isDir=false  mode=-rw-r--r--   isADR=true
  ADR-evil.md       isDir=false  mode=Lrwxr-xr-x   isADR=true   ← CONTADO

=== Stat on symlink ADR-evil.md ===
  Lstat: err=<nil>, mode=Lrwxr-xr-x, IsRegular=false
  Stat (follows link): mode=drwxr-xr-x, IsRegular=false, IsDir=true
```

`filepath.WalkDir` NÃO recursea em symlinks de diretório, mas o symlink em si é reportado como
entrada `isDir=false` com `ModeSymlink`. O filtro de nome `HasPrefix("ADR-") && HasSuffix(".md")`
aceita o symlink. **O symlink de diretório nomeado `ADR-evil.md` é CONTADO como ADR.**

Consequência para contagem: `ADRCount` sobe 1. O Governance Score sobe +20 sem ADR real.
Consequência para leitura de conteúdo: `readRegularFile` (`regularfile.go:42`) usa
`openRegularFileNonblock`, que verifica `Mode().IsRegular()` no arquivo aberto. Um symlink de
diretório (`Stat` → `IsDir=true, IsRegular=false`) retorna `errNotRegularFile`. As regras que leem
frontmatter reportam falha de inspeção, não frontmatter ausente.

**Análise de impacto:** este é comportamento pré-existente antes da mudança D1. O critério de
prefixo não muda a política de symlinks — o symlink continua sendo contado porque o filtro é de
nome. A mudança D1 não piora nem melhora isso. O ajuste seria: após aplicar o critério de nome,
verificar `d.Type().IsRegular()` (sem seguir symlink) antes de incluir. Declarado como ajuste #3
na seção de Veredito (não bloqueante, pré-existente).

### 2.2 `ADR-.md` — prefixo sem corpo de nome

```
$ go run prefixtest.go
ADR-.md (no body)   ADR-.md   true
```

`HasPrefix(ToUpper("ADR-.md"), "ADR-") && HasSuffix("ADR-.md", ".md")` = `true`. O arquivo é
contado como ADR. Comportamento correto: o critério é de nome, não de conteúdo. As regras de
frontmatter cuidam do conteúdo. Não é ameaça.

### 2.3 Maiúsculas mistas (`Adr-001.md`) e sufixo maiúsculo (`ADR-001.MD`)

```
$ go run prefixtest.go
mixed case ADRx   Adr-001-foo.md   true
```

`strings.ToUpper` cobre letras latinas. `Adr-001.md` é aceito — correto, conforme D1.
`HasSuffix(path, ".md")` é case-sensitive: `ADR-001.MD` **não** é aceito. O gerador emite sempre
`.md` em minúsculo; nenhum dos 133 ADRs medidos usa `.MD`. Comportamento aceitável e declarado no
resíduo R3.

### 2.4 `ADR-x.md` que é um **diretório real** (não symlink)

```
$ go run symlinktest.go
  /ADR-x.md     isDir=true   isADR=false
  /ADR-x.md/ADR-child.md    isDir=false  isADR=true
```

Diretório real: `d.IsDir()=true` → excluído do filtro de nome. O critério `!d.IsDir()` cobre este
caso corretamente. Filhos do diretório são percorridos normalmente.

**Distinção importante:** diretório real → excluído; symlink de diretório → contado (ver §2.1).

### 2.5 Homoglifo Unicode — `ΑDR-001.md` (Alpha grego U+0391)

```
$ go run prefixtest.go
unicode alpha ADR   ΑDR-001.md   false

ToUpper on homoglyphs:
  Greek Alpha Α (U+0391) -> ToUpper="Α"  Is ASCII upper A/D/R: false
  Cyrillic А (U+0410) -> ToUpper="А"     Is ASCII upper A/D/R: false
  D with stroke Đ (U+0110) -> ToUpper="Đ" Is ASCII upper A/D/R: false
```

`strings.ToUpper` preserva o codepoint não-ASCII — Alpha grego (`Α`) não se converte para
`A` (`A`). `HasPrefix(ToUpper("ΑDR-001.md"), "ADR-")` = `false`. O homoglifo é **rejeitado**.
Comportamento seguro: não pode inflar ADR count sem ser detectado.

### 2.6 Dois caminhos `countMDFiles` em `discover.go`

O fallback do `discover` tem dois ramais (linha 486 e linha 491). Se D3 cobrir apenas um, o outro
persiste contando todo `.md`. Gate: AC5 deve incluir fixture para o caminho plano (`docs/adr` sem
subdir).

### 2.7 `scanChainDir` com filtro aplicado além do tipo `adr`

Se o implementador aplicar o filtro de prefixo a todos os tipos em `scanChainDir` (não só `adr`),
nós de REQ e roadmap param de aparecer no grafo. A guarda deve ser `if nodeType == "adr"`. Gate:
AC4 mais testes do pacote `serve`.

### 2.8 Gate `check-adr-enumeration-single-point.sh` não detecta `countMDFiles` e `scanChainDir`

O script detecta Pattern A (`os.ReadDir` com `cfg.ADRDirs`), Pattern B/C (`filepath.Glob`). Nem
`countMDFiles` (usa `filepath.WalkDir`) nem `scanChainDir` (idem) são pegos. O gate não detecta
uma regressão nesses dois sítios pós-ML-1B. Declarado como resíduo R4 abaixo.

---

## 3. Alvos de falsificação nas duas direções por sítio

Terminologia: **FP** (falso positivo) = critério frouxo, conta arquivo que não é ADR (ex: `NOTAS.md`).
**FN** (falso negativo) = critério restrito, deixa de contar ADR real (ex: `adr-001.md`).

### 3.1 `walkADRFilePathsForRule` (primitivo)

**FP — critério frouxo** (volta a contar `NOTAS.md`):
- Sabotagem: trocar `HasPrefix(ToUpper(base), "ADR-") && HasSuffix(base, ".md")` por `HasSuffix(path, ".md")`.
- Gate: AC2 (braço `NOTAS.md` deve resultar em ADRs 0) e AC9 (tests reprovam com critério antigo).

**FN — critério restrito** (deixa de contar `adr-001.md`):
- Sabotagem: trocar `strings.ToUpper(base)` por `base` (case-sensitive) → `adr-001.md` começa com `"adr-"`, não `"ADR-"`.
- Gate: AC3 (`adr-001-x.md` minúsculo deve ser contado).

### 3.2 `findADRFile`

**FP — critério frouxo** (situação atual, mantida por decisão):
- `findADRFile` resolve qualquer basename em `adr_dirs`, mesmo sem prefixo.
- Efeito esperado e aceitável: `blocked_by_draft_adr` continua operando sobre arquivos referenciados explicitamente, mesmo sem prefixo. O D4 avisa sobre eles.
- Gate: não há AC direto para o comportamento de `findADRFile`. Ajuste #1 (seção Veredito) recomenda um teste explícito.

**FN — critério restrito** (opção descartada):
- `findADRFile` aplica critério → ADR legado referenciado como bloqueador some silenciosamente.
- O `blocked_by_draft_adr` não dispara → bypass de governança sem sinal.
- Por isso esta opção foi descartada.

### 3.3 `scanChainDir` — tipo `adr` (`serve/api_chain.go:88`)

**FP — critério frouxo** (continua gerando nó para `NOTAS.md`):
- Sabotagem: não aplicar o filtro de prefixo para tipo `adr`.
- Gate: AC4 (`/api/chain` sem nó para `NOTAS.md`).

**FN — critério restrito** (aplica filtro também para `req` e `roadmap`):
- Sabotagem: aplicar filtro de prefixo em todos os tipos, não só `adr`.
- Gate: testes do pacote `serve` param ao verificar o grafo completo. Não há AC explícito para "nós REQ e roadmap continuam presentes" — declarado como ajuste #5 (seção Veredito).
- Ausência de gate permanente: `check-adr-enumeration-single-point.sh` não detecta `scanChainDir`.

### 3.4 `discover.go` fallback — ambos os caminhos

**FP — critério frouxo** (continua contando `NOTAS.md`):
- Sabotagem: cobrir apenas um dos dois `countMDFiles` (linha 486 coberta, linha 491 não, ou vice-versa).
- Gate: AC5. Para pegar os dois caminhos, o teste deve usar fixture **sem subdir** (caminho plano) além da fixture com subdir.

**FN — critério restrito** (usa diretório errado no primitivo):
- Sabotagem: `WalkADRFilePaths(filepath.Join(rootDir, "docs", "adr"))` fixo em vez do diretório configurado.
- Gate: AC5 com `adr_dirs` apontando para diretório não-convencional.

### 3.5 Regra `adr_file_without_prefix` (nova, D4)

**FP — critério frouxo** (não dispara para `.md` sem prefixo com `status:`):
- Sabotagem: condição de guarda invertida (exige ausência de frontmatter).
- Gate: AC6, braço `NOTAS.md` com `status: Draft` → deve disparar warning.

**FN — critério restrito** (dispara para todos os `.md`, inclusive `README.md` sem frontmatter):
- Sabotagem: remover a verificação de presença de `status:` no frontmatter.
- Gate: AC6, braço `README.md` sem frontmatter → NÃO deve disparar.

**Lacuna adicional D4:** como especificado, D4 verifica apenas `status:` em frontmatter YAML.
O `resolveAdrStatus` (`validator.go:2805`) também cai para a linha de cabeçalho `| Status: X`
quando não há frontmatter. Um ADR legado com apenas `| Status: Draft` no cabeçalho e sem frontmatter
não seria detectado por D4 — e pelo mesmo motivo não seria detectado por `findADRFile` com critério
aplicado (ver §1). Por isso este ajuste está ligado à decisão sobre `findADRFile`.
Gate atual para esta lacuna: nenhum.

---

## 4. Resíduo declarado

**R1 — ADR sem prefixo e sem frontmatter `status:` (apenas cabeçalho):** arquivo com
`| Status: Draft` no cabeçalho e sem frontmatter `status:` — não é contado como ADR (após D1) e
D4 não avisa (D4 verifica frontmatter). Este é o caso do `ADR-001` deste repositório antes de
ganhar frontmatter. O custo é zero nos 6 acervos medidos, mas é uma lacuna de detecção. Aceito sem
mitigação neste roadmap; ligado ao ajuste #2.

**R2 — Symlink de arquivo nomeado `ADR-*.md` apontando para fora de `adr_dirs`:** contado e o
conteúdo lido do alvo do symlink. Comportamento pré-existente; D1 não o modifica. Cobrir exigiria
verificação de `ModeSymlink` via `d.Type()` antes de incluir — ajuste #3 abaixo, não bloqueante.
O symlink de **diretório** nomeado `ADR-*.md` é contado mas seu conteúdo não é lido (bloqueado por
`readRegularFile`) — inflação de contagem sem conteúdo falso.

**R3 — `.md` com sufixo maiúsculo (`.MD`):** `ADR-001.MD` não é contado. Não há ocorrência nos
133 ADRs medidos. Declarado sem mitigação.

**R4 — Gate `check-adr-enumeration-single-point.sh` não cobre `scanChainDir` e `countMDFiles`:**
uma regressão em `serve/api_chain.go` ou `discover.go` (voltando ao `.md` sem prefixo) não seria
detectada pelo gate textual existente. A cobertura depende de AC4 e AC5 continuarem em execução no
CI. Declarado como lacuna de detecção permanente.

**R5 — `findADRFile` sem teste de AC direto:** comportamento de `blocked_by_draft_adr` e
`adr_accepted_when_req_done` com referência a arquivo sem prefixo não tem AC explícito. Ligado ao
ajuste #1 abaixo.

---

## Veredito: APROVA COM AJUSTES

A decisão de design (D1–D4) é sólida. O primitivo único é o lugar correto. Homoglyphs rejeitados.
Diretórios reais excluídos. `scope-redirect` corretamente mais amplo. A decisão sobre `findADRFile`
diverge do D2 do ADR por razão de fail-closed — declarado explicitamente.

**Ajustes (não bloqueantes para Wave 1, devem estar nos critérios de aceite do ML-1A/ML-1B):**

1. **`findADRFile` — braço de teste ausente:** ML-1A deve incluir teste que verifique que uma REQ
   com `adr: arquivo-sem-prefixo.md` (onde o arquivo tem `status: Draft`) ainda aciona
   `blocked_by_draft_adr`. Arquivo: `internal/validator/validator_test.go` (ou equivalente).
   Sem esse teste, uma regressão de `findADRFile` que aplique o critério passa invisível.

2. **D4 deve usar `resolveAdrStatus`, não só frontmatter:** o D4 especifica verificar `status:` em
   frontmatter. Mas `resolveAdrStatus` (`validator.go:2805`) já cai para o cabeçalho `| Status: X`.
   Usar o mesmo helper na condição de disparo do D4 evita a lacuna de R1 e alinha com o restante
   do pacote. Arquivo: `internal/validator/validator.go` (na implementação de `adr_file_without_prefix`).

3. **Symlink de arquivo/diretório nomeado `ADR-*.md`:** após o filtro de nome, verificar
   `d.Type().IsRegular()` (sem seguir symlink) antes de incluir o caminho. Exclui symlinks de
   diretório da contagem sem afetar symlinks de arquivo que a enumeração atual já aceita.
   Arquivo: `internal/validator/validator.go:3117` (`walkADRFilePathsForRule`). Pré-existente —
   não é regressão desta mudança; pode ser ML separado se o escopo for alargado.

4. **Comentário de isenção em `check-adr-enumeration-single-point.sh`:** após D3, a linha ~88
   "pre-config scan; does NOT consume cfg.ADRDirs" deve ser atualizada para refletir que
   `discover.go` passa a usar o primitivo. Arquivo:
   `scripts/check-adr-enumeration-single-point.sh`.

5. **AC4 deve verificar que nós REQ e roadmap continuam no grafo:** a verificação de que `NOTAS.md`
   não gera nó para tipo `adr` não valida que o filtro não foi erroneamente aplicado aos outros
   tipos. Adicionar ao teste de ML-1B: verificar que um REQ e um roadmap continuam aparecendo
   no grafo após a mudança. Arquivo: `internal/serve/api_chain_test.go`.
