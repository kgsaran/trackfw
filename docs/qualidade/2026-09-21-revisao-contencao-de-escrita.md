# Revisão de Qualidade — Barreira Final da Contenção de Escrita

> Data: 2026-09-21 | Agente: hefesto-tf | Branch: fix/afirma-contencao-antes-de-escrever

## Veredito

**Aprovado com ressalvas.**

Nenhum achado bloqueia o merge. Os dois itens de "corrigir neste PR" abaixo são menores; todos
os demais são issues para ciclos futuros.

---

## Pergunta 1 — Duplicação de helpers de recusa

**Evidência:**

```
$ grep -rn "^func reject" internal/
internal/integrations/manager.go:762:  func rejectSymlinks(root, filename string) error { return pathguard.RejectSymlinks(root, filename) }
internal/commands/discover.go:216:     func rejectDiscoverPath(root, absTarget string) error {
internal/generators/scaffold.go:86:    func rejectScaffoldPath(root, absTarget string) error {
```

Corpos de `rejectDiscoverPath` e `rejectScaffoldPath` lidos e comparados — são byte-idênticos:

```go
if err := pathguard.RejectSymlinks(root, absTarget); err != nil {
    fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absTarget, err)
    return fmt.Errorf("refusing write to %s: %w", absTarget, err)
}
return nil
```

Contagem de helpers de recusa hoje:

| função | pacote | corpo |
|---|---|---|
| `rejectScaffoldPath` | `generators` | stderr + wrap |
| `rejectDiscoverPath` | `commands` | stderr + wrap (idêntico) |
| `rejectSymlinks` | `integrations` | thin wrapper sem stderr |
| `rejectHarnessSymlink` | `generators/update` | lógica diferente, não é um guard simples |

**Sim, o padrão está sendo recriado.** Os dois helpers identicos reproduzem o mesmo anti-padrão
que causou o defeito original (`atomicWrite` copiado 3x, só 1 protegido). A extração correta é
uma função em `pathguard`:

```go
// pathguard.RejectAndReport(root, absTarget string) error
// — calls RejectSymlinks, prints to stderr, wraps error
```

Isso consolidaria os dois helpers idênticos e o `rejectSymlinks` em manager.go num único ponto de
definição, e futuros sítios herdariam o comportamento correto (incluindo o stderr) sem precisar
copiar.

**Por que não foi feita aqui:** nenhuma das mensagens de erro impressas ao stderr usa termos
dependentes do contexto do caller — os dois helpers são funcionalmente indistinguíveis. A omissão
não foi documentada no roadmap.

**Severidade:** baixa para este PR. O gate `check-write-containment.sh` captura qualquer novo sítio
sem marcador, impedindo que a cópia seja silenciosa. Mas a extração deve ser feita antes que um
terceiro sítio seja adicionado.

---

## Pergunta 2 — 152 marcadores com texto idêntico

**Evidência:**

```
$ grep -rn "write-containment-allowed:" internal/ | wc -l
157

$ grep -rn "write-containment-allowed:" internal/ | sed 's/.*write-containment-allowed://' | sort | uniq -c | sort -rn
 148  guarded by pathguard.RejectSymlinks at the enclosing write site
   3  atomicWrite is called only after caller invokes rejectSymlinks — the guard is at the call site, not inside this helper
   2  guarded by pathguard.RejectSymlinks via rejectDiscoverPath above
   1  os.OpenFile to os.DevNull, a fixed system path not derived from user-controlled root
   1  GuardedWrite implements the containment helper itself; Rename is always called after RejectSymlinks guard passes
   1  GuardedWrite implements the containment helper itself; MkdirAll is always called after RejectSymlinks guard passes
   1  GuardedWrite implements the containment helper itself; CreateTemp is always called after RejectSymlinks guard passes
```

Os 148 marcadores com texto genérico são ruído potencial, mas documentação útil dentro da restrição
real: o gate é bash puro e não faz análise de fluxo. Ele não pode verificar que a guarda está no
caminho de execução — só verifica que o marcador está presente. O marcador é uma **declaração de
autoria**, não uma prova formal.

O risco concreto: um desenvolvedor copia um sítio com marcador para uma função que não chama
`rejectXxxPath`, e o gate aceita silenciosamente. Isso é o que o arquiteto chamou de "marcador vira
carimbo" — e o arquiteto verificou empiricamente que os 17 arquivos marcados realmente chamam
pathguard (com 2 exceções legítimas documentadas nas notas do vault).

**Não existe forma melhor dentro das restrições do gate.** Um gate que exija rastrear o caminho de
chamada precisaria de análise estática real (go/analysis, semgrep com taint tracking), que está além
do escopo de um script bash e foge dos princípios de portabilidade da casa (P3).

A alternativa seria exigir texto de razão diferente por sítio, mas isso é operacionalmente custoso
sem reduzir o risco real (um desenvolvedor que copia o marcador também copiaria uma razão
específica).

**Veredito:** o texto genérico é aceitável dado o modelo de ameaça. A documentação do gate e do ADR
devem tornar a limitação explícita (hoje está implícita). Issue para ciclos futuros.

---

## Pergunta 3 — `discover.go`: duas variáveis para o mesmo conceito

**Evidência — setup da função:**

```go
// internal/commands/discover.go:26-39
cwd, err := os.Getwd()                         // linha 26
resolvedCwd := cwd                             // linha 32
if rc, rcErr := filepath.EvalSymlinks(cwd); rcErr == nil {
    resolvedCwd = rc                           // linha 35
}
```

**Mapa completo de usos na função:**

| variável | uso | linha(s) | motivo correto? |
|---|---|---|---|
| `cwd` | display: `"scanning %s"` | 38 | sim — user-facing |
| `cwd` | `discover.Scan(cwd)` | 40 | sim — só lê |
| `cwd` | `discover.InstallGates(r, cwd, out)` | 135 | sim — `InstallGates` chama `resolveRoot(rootDir)` internamente (EvalSymlinks) |
| `cwd` | `generators.InjectRulesDetected(cwd)` | 138 | ATENÇÃO — usa `filepath.Clean(cwd)`, não EvalSymlinks; pré-existente |
| `cwd` | `generators.InjectHooksDetected(cwd)` | 142 | mesmo padrão |
| `cwd` | `generators.PrintArchitectNextSteps(cwd)` | 145 | só imprime |
| `resolvedCwd` | `yamlPath = filepath.Join(resolvedCwd, "trackfw.yaml")` | 128 | correto — target para pathguard |
| `resolvedCwd` | `rejectDiscoverPath(resolvedCwd, yamlPath)` | 130 | correto — root = resolved |
| `cwd` | `discover.GenerateBootstrapLog(r, cwd)` | 164 | correto — só lê; retorna string |
| `resolvedCwd` | `logPath = filepath.Join(resolvedCwd, r.RoadmapDir, ".trackfw-log")` | 166 | correto |
| `resolvedCwd` | `rejectDiscoverPath(resolvedCwd, logPath)` | 171 | correto |

**Conclusão:** o split é correto e intencional. `resolvedCwd` é usado apenas nos dois sítios onde
a string precisa ser a raiz do pathguard (e onde o target é construído a partir dela). As funções
que recebem `cwd` ou resolvem internamente (`resolveRoot`) ou só leem.

**Risco latente identificado:** `InjectRulesDetected` e `InjectHooksDetected` usam
`filepath.Clean(cwd)` como raiz do pathguard — não `EvalSymlinks`. Isso é pré-existente ao PR e não
foi introduzido por ele. Mas a função é chamada de `discover.go` com `cwd` não resolvido, criando
um padrão inconsistente que pode enganar um futuro mantenedor. No macOS `/tmp → /private/tmp`, um
sítio de escrita nesses geradores passaria `"/tmp/projeto"` como root mas o path real seria
`"/private/tmp/projeto"`, o que causaria falso "escapes root".

**Severidade do risco latente:** moderada — reproduzível no macOS com o padrão de CWD que o CI
não exercita. Deve ser documentado como issue, não bloqueia este PR (a causa é pré-existente).

---

## Pergunta 4 — Legibilidade do gate `check-write-containment.sh`

### Conformidade com os princípios da casa

**P1 (sem números mágicos):** `SITE_FLOOR=157` é uma constante calibrada, não derivada de uma
fonte de verdade. O gate documenta a proveniência inline:

> "the floor was determined on 2026-09-21 by running this gate itself... → 157. Note: a raw grep
> count of 158 was off by 1 because roadmap.go:727 has os.WriteFile inside a // comment"

O molde `check-raw-read-ban.sh` usa threshold de 200 linhas (também constante). Ambos seguem a
mesma convenção. Aceitável para um piso de vacuidade.

**P2 (falha explícita):** passa. Corpus vazio → falha com mensagem. SITE_FLOOR violado → falha com
instrução de como corrigir (`atualize SITE_FLOOR no mesmo commit`).

**P3 (independência de ambiente):** `LC_ALL=C` não está declarado. O molde `check-raw-read-ban.sh`
também não declara. O padrão de busca `os\.(WriteFile|Create|...)` é ASCII puro. Consistente com o
molde — não é regressão.

**P4 (falsificabilidade):** 3 braços no `--self-test` (unguarded reprova, marker-accepted passa,
vacuous-scan reprova). Confirmado pelo relatório do ML-2A: 3/3 braços OK.

### SITE_FLOOR — proveniência documentada?

Sim. O comentário no script (linhas 35-42) documenta:
- data da medição (2026-09-21)
- comando para rederivação (`bash scripts/check-write-containment.sh 2>&1 | grep "sítio(s) examinado(s)"`)
- off-by-one do grep cru explicado

### A mensagem de erro diz o que fazer?

Mensagem atual:

```
FAIL [write-containment] unjustified write at internal/foo/bar.go:42: os.WriteFile(path, data, 0644)
```

Não diz "adicione `// write-containment-allowed: <razão>` acima desta linha". O molde
`check-raw-read-ban.sh` tem a mesma omissão — a linha de FAIL do molde também só mostra arquivo,
linha e conteúdo.

**Esta é uma lacuna compartilhada com o molde, não uma regressão deste PR.** A mensagem orienta
sobre *o que falhou* mas não sobre *como corrigir*. Issue para todos os gates da família, não
específico deste.

---

## Pergunta 5 — Cobertura de testes

### Padrão geral

Todos os `*_guard_test.go` seguem o padrão de dois braços:
- **Braço (a):** cria symlink, chama a função, afirma `err != nil`, verifica que a vítima está
  intacta fora do projeto.
- **Braço (b):** caminho limpo, afirma `err == nil`, verifica que o arquivo foi criado.

Isso é cobertura comportamental, não apenas "não entra em pânico".

### Destaque positivo

`TestSyncWriteGuard_SymlinkREQFileRefused` (`internal/sync/sync_write_guard_test.go`) testa a
integração real — inclui a observação de que `create()` pode disparar antes do guard, e confirma
que mesmo assim o arquivo não é modificado em disco. Esse nível de detalhe é raro e correto.

`TestRejectSymlinks_SymlinkInAncestor` (`internal/pathguard/pathguard_test.go`) reproduz o PoC
específico de 2026-09-18 (mkdir -p /fora/out && ln -s /fora/out scripts) e documenta isso no
comentário. Rastreável.

### Ponto fraco identificado

**`internal/commands/configure_guard_test.go` — `TestConfigureGuard_SymlinkLeafRefused`**

```go
// linha 48-59
absYAML := filepath.Join(dir, "trackfw.yaml")
err := pathguard.RejectSymlinks(dir, absYAML)
if err == nil {
    t.Fatal(...)
}
```

O teste chama `pathguard.RejectSymlinks` **diretamente**, não `configure.go`. Isso afirma que a
biblioteca funciona, mas **não** afirma que `configure.go` a chama antes de escrever. Um
desenvolvedor poderia remover a guarda em `configure.go` (linha 153) e este teste continuaria verde.

Confirmado por leitura de `configure.go:153`:
```go
if guardErr := pathguard.RejectSymlinks(configureRoot, absYAML); guardErr != nil { ...
```

O teste correto invocaria o comando `configure` com um `trackfw.yaml` symlinked e verificaria que
o arquivo não foi reescrito fora do projeto. A correção é um teste de integração adicional, não uma
modificação do existente.

**Severidade:** baixa. O gate `check-write-containment.sh` garante que o marcador existe na linha
158 de `configure.go`, e a leitura do código confirma que a guarda está presente. Mas o teste não é
uma barreira independente para essa garantia. Deve ser corrigido antes que `configure.go` cresça.

### Cópia do symlinkOrSkip helper

Há 5+ cópias de `symlinkOrSkip` / equivalentes em diferentes pacotes de teste. Isso é o idioma
correto em Go (`_test.go` não é importável entre pacotes), mas o comentário que acompanha as cópias
em `pathguard_test.go` e `configure_guard_test.go` está presente e explica o porquê. Aceitável.

---

## Pergunta 6 — O que eu bloquearia

**Nada bloqueia este PR.**

O gate `check-write-containment.sh` está correto e falsificável. As guardas estão nos sítios
certos. O CI está verde em 21 checks incluindo Windows. A `trackfw barrier` passou nas 3 waves.

---

## Separação explícita

### Corrigir neste PR (antes do merge)

Nenhum item é estritamente bloqueante, mas os dois abaixo têm custo de correção baixo e se tornam
mais caros depois do merge:

1. **`TestConfigureGuard_SymlinkLeafRefused` testa a biblioteca, não a integração.** Adicionar um
   segundo teste que invoque o comando `configure` diretamente com `trackfw.yaml` symlinked e
   verifique que o arquivo externo não é modificado. O teste existente pode permanecer como
   cobertura da biblioteca.

2. **Mensagem de erro do gate não orienta correção.** Adicionar à linha de FAIL a instrução:
   `"— add '// write-containment-allowed: <reason>' above this line, or route through pathguard"`.
   Custo: uma linha no script. Benefício: elimina busca na documentação.

### Registrar como issue (ciclos futuros)

3. **Extrair `rejectXxxPath` para `pathguard.RejectAndReport`.** Dois helpers byte-idênticos já
   existem. Antes que um terceiro apareça, a função deve existir em `pathguard` com o comportamento
   completo (guard + stderr + wrap). A extração é mecânica e o gate impede regressão durante ela.

4. **`InjectRulesDetected` e `InjectHooksDetected` usam `filepath.Clean` em vez de `EvalSymlinks`
   como raiz do pathguard.** No macOS com CWD sob `/tmp` (real: `/private/tmp`), um sítio de
   escrita nesses geradores pode receber `"/tmp/proj"` como root enquanto o path derivado seria
   `"/private/tmp/proj/..."`, causando falso "escapes root". Pré-existente ao PR; documentar como
   issue rastreado.

5. **Texto genérico dos 148 marcadores pode ser copiado sem pensar.** Limitação estrutural do
   bash gate (sem análise de fluxo). Documentar explicitamente no design doc e no gate header que
   o marcador é uma declaração de autoria, não uma prova, e que a verificação de integração é
   responsabilidade dos testes de braço (a)+(b).
