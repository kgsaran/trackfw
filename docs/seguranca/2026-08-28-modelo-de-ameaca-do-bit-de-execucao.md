# Modelo de Ameaça — bit de execução dos artefatos de scaffold

> Produzido por: `hades-tf` | Data: 2026-08-28
> REQ: `docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold.md`
> Wave: 0 — sem implementação neste ML

---

## Completude de enumeração

**Pergunta:** a lista de 17 artefatos de scaffold do roadmap anterior está completa? Quais são executáveis?

### Artefatos que o gerador escreve com modo 0755 (bit de execução obrigatório)

Enumerados por busca direta no código de escrita — não pela aparência no repositório.

| # | Artefato | Go (scaffold.go) | Node (init.js / hooks.js) | Python (init_gen.py) |
|---|----------|-----------------|--------------------------|----------------------|
| 10 | `scripts/trackfw-validate.sh` | L739: `WriteFile(path, content, 0755)` | init.js:L125: `writeFileSync(path, s, {mode:0o755})` | L528: `open(dest,'w')` + `chmod(0o755)` |
| 11 | `scripts/trackfw-attention-signal.sh` | L822: `WriteFile(signalPath, …, 0755)` | hooks.js:L1491: `writeFileSync(signalPath, …, {mode:0o755})` | L1738: `open()` + `chmod(0o755)` |
| 12 | `scripts/trackfw-attention-cleanup.sh` | L832: `WriteFile(cleanupPath, …, 0755)` | hooks.js:L1494: `writeFileSync(cleanupPath, …, {mode:0o755})` | L1743: `open()` + `chmod(0o755)` |
| 13 | `scripts/trackfw-credential-guard.sh` | L864: `WriteFile(path, …, 0755)` | hooks.js:L1086: `writeFileSync(scriptPath, …, {mode:0o755})` | L1759: `open()` + `chmod(0o755)` |
| 14 | `scripts/trackfw-git-branch-guard.sh` | L1184/L1219: `WriteFile(path, …, 0755)` | hooks.js:L1049/L1069: `writeFileSync(scriptPath, …, {mode:0o755})` | L1776/L1796: `open()` + `chmod(0o755)` |

Sem divergência de intenção entre runtimes: todos os três escrevem os mesmos 5 scripts com 0755.

### Artefatos que o gerador escreve sem bit de execução (AC4 — não acusar)

| # | Artefato | Go | Node | Python |
|---|----------|----|------|--------|
| 1–9 | `.claude/commands/trackfw/*.md` (9 slash commands) | scaffold.go:L659 `WriteFile(…, 0644)` | init.js:L1209 `writeFileSync(…, 'utf8')` sem mode | `open(dest,'w')` sem chmod |
| 15 | `.github/workflows/trackfw-gate.yml` | scaffold.go:L1929 `WriteFile(…, 0644)` | init.js:L249 `writeFileSync(…, 'utf8')` sem mode | `open(dest,'w')` sem chmod |
| 16 | `.gitlab-ci-trackfw.yml` | scaffold.go:L1937 `WriteFile(…, 0644)` | init.js:L254 `writeFileSync(…, 'utf8')` sem mode | `open(dest,'w')` sem chmod |
| 17 | Arquivos de hook (`.husky/*`, `.lefthook/*`, `lefthook.yml`) | scaffold.go:L1968/1986/2010 `WriteFile(…, 0755)` para os scripts de hook | hooks.js:L277/L312/L322 `writeFileSync(…, {mode:0o755})` | `inject_hooks_detected()` em hooks.py — não escreve `.husky/*` diretamente |

Nota sobre #17: os arquivos de hook são escritos com 0755 pelo gerador de init, mas estão **fora do escopo do scaffold doctor** por decisão já documentada em `docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md §Residual-3`. Não alterar este escopo.

### Terceiro escritor — `InstallGates` (discover.go:83)

`InstallGates` (brownfield) escreve `scripts/trackfw-validate.sh` com o conteúdo fixo
`#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n` via `os.WriteFile(dest, content, 0755)`.
Este conteúdo é idêntico a `PYTHON_VALIDATE_SCRIPT_FORM` do doctor (scaffold_doctor.js:L48,
scaffold_doctor.go equivalente). A verificação de pertencimento a conjunto (`set-membership`) já
o aceita. Sem nova superfície.

Scripts globais (`~/.trackfw/scripts/`) emitidos por `scaffold.go:L894/L899/L1219` — fora do escopo
do scaffold doctor, que inspeciona apenas caminhos relativos ao projeto.

### Conclusão de enumeração

A lista está fechada: 5 artefatos requerem bit de execução, 12 artefatos não requerem (incluindo
o #17 fora de escopo). Divergência entre runtimes: nenhuma na intenção de modo — mas existe uma
divergência no **mecanismo** de escrita, com consequência crítica para o caminho de remédio (ver
seção 2).

---

## Modelo de ameaça

O adversário é o implementador apressado e o arquiteto otimista, não um atacante externo.
Cada ameaça nomeada pode ser realizada sem violar regra escrita — a proteção passa a ser o
próprio gate (AC6/AC7) que ML-1A deve implementar.

### Ameaça 1 — Verificação de modo omitida da implementação (falso-negativo estrutural)

**Como entra:** ML-1A implementa o doctor com verificação de conteúdo ampliada mas omite o modo.
Justificativa otimista: "o gerador escreve 0755; se o conteúdo bater, a re-geração restaura o modo."

**Por que essa justificativa falha:**
O `trackfw update` nos runtimes Go e Node.js chama `generateValidateScript(cfg)` /
`generators.generateValidateScript(...)`, que usa `os.WriteFile(path, content, 0755)` (Go,
scaffold.go:L739) e `fs.writeFileSync(scriptPath, script, {mode:0o755})` (Node, init.js:L125).

Medido:
```
Go:   WriteFile(existente_0644, content, 0755) → arquivo permanece 0644
Node: writeFileSync(existente_0644, content, {mode:0o755}) → arquivo permanece 0644
```

`os.WriteFile` abre o arquivo com `O_WRONLY|O_CREATE|O_TRUNC`: o `perm` é passado ao syscall `open()`,
que aplica o modo **somente no evento de criação** (`O_CREATE`). Para arquivo existente, `O_TRUNC`
reescreve o conteúdo e o modo no inode não é tocado. Node.js `writeFileSync` com `{mode}` exibe o
mesmo comportamento (o flag de abertura é `w`, equivalente a `O_WRONLY|O_CREATE|O_TRUNC`).

O Python é exceção: `open(dest,'w')` (reescreve conteúdo) seguido de `os.chmod(dest, 0o755)`
(incondicional, ignora umask) — Python's `trackfw update` **restaura o modo**.

**Cascata sem verificação de modo:**
1. Arquivo existe em 0644 (defect herdado, como em 1fc7610).
2. Doctor roda: conteúdo bate → nenhum achado → silent.
3. Usuário não sabe do problema.
4. Mesmo se a comparação de conteúdo fosse adicionada e detectasse divergência de conteúdo,
   o remédio impresso (`trackfw update`) em Go/Node reescreve conteúdo mas não restaura o modo.
5. Doctor roda novamente: se o conteúdo agora bate, silent. Modo ainda em 0644.

Esta é a mesma classe do defeito original: **controle que parece ativo e não está**.

**Constraint para ML-1A:** além de adicionar comparação de modo, o caminho de update (Go/Node)
deve chamar `os.Chmod(path, 0755)` após o `WriteFile`, e o Node deve chamar `fs.chmodSync(path, 0o755)`
após o `writeFileSync`. Python já está correto. Sem esta correção, o gate pode detectar o problema
mas o remédio fica inerte em dois dos três runtimes.

### Ameaça 2 — AC4 ignorado — verificação de modo aplicada a todos os artefatos (falso-positivo estrutural)

**Como entra:** ML-1A adiciona verificação de modo ao loop de todos os artefatos de scaffold, sem
distinguir os que são executáveis dos que não são.

**Consequência:** os 9 slash commands (escritos em 0644) e os 2 workflows de CI (escritos em 0644)
são acusados por não ter bit de execução. Findings acionáveis-mas-errados. Isto reprova AC4 e AC7.

**Discriminante seguro:** a lista dos 5 scripts executáveis é fechada e enumerável por caminho.
O doctor deve manter uma allow-list (ou um campo no descritor de cada artefato) que marca
explicitamente quais são executáveis. A check de modo só roda para artefatos nessa lista.

### Ameaça 3 — Modo verificado como igualdade exata (0755) em vez de bit de usuário

**Como entra:** ML-1A compara `mode == 0755` em vez de `mode & 0o100 != 0`.

**Consequência:** um arquivo criado pelo gerador Go ou Node sob umask não-padrão (ex: 027) resulta em
`0755 & ~0027 = 0750`. O modo no disco é 0750 — funcionalmente executável pelo dono. A verificação
de igualdade exata acusa. Falso-positivo.

Medido (umask 022, padrão):
```
Go WriteFile(0755) sob umask 0022 → 0755 no disco
Python chmod(0o755) → 0755 no disco (umask ignorado pelo chmod)
```

Sob umask 027: Go/Node produziriam 0750. Python produce 0755 (chmod ignora umask).

**Discriminante correto:** `mode & 0o100 != 0` (bit de execução do dono). Isso aceita 0755, 0750,
0700 e reprova 0644, 0600, 0444. O `./script` requer apenas o bit do dono quando executado pelo
mesmo usuário que detém o arquivo.

### Ameaça 4 — Verificação de modo sem guarda de plataforma (Windows/noexec → loop de falso-positivo)

**Como entra:** ML-1A não guarda a verificação de modo contra plataformas onde o bit não é representável.

**Consequência — Windows:** no Windows nativo (sem WSL), `stat()` em um arquivo `.sh` retorna bits
de execução zerados. O doctor acusa todos os 5 scripts. Não há remédio — `chmod` não é aplicável,
`trackfw update` não resolve. O usuário fica preso num loop de falso-positivo permanente.

**Consequência — filesystem com noexec:** o bit pode estar correto (0755) mas a execução falha com
`exit 126` por causa da flag de montagem `noexec`. O doctor veria 0755 e não acusaria (correto),
mas a causa raiz real não seria visível. Isto é residual aceito (ver seção 4).

**Discriminante correto:** verificar `runtime.GOOS != "windows"` (Go) / `sys.platform != "win32"` (Python) /
`process.platform !== "win32"` (Node.js) antes de qualquer comparação de modo. Em vez de acusar,
emitir uma linha agregada de aviso: "verificação de bit de execução indisponível nesta plataforma —
N artefatos não verificados". Esta linha não altera o exit code do comando.

---

## Alvos de falsificação nas duas direções

Para cada uma das 5 superfícies executáveis, nomeando (a) por onde a sabotagem entra, (b) qual gate
deveria capturá-la, (c) em qual direção.

### Scripts #10–14 — artefatos executáveis do scaffold

#### Direção falso-negativo: bit removido, doctor silencioso

| Superfície | Como o bit é perdido | Gate que deveria detectar | Status atual |
|------------|----------------------|--------------------------|--------------|
| `scripts/trackfw-validate.sh` (#10) | Commit aplica mudança de conteúdo via editor que não preserva modos; `core.fileMode=false` no git impede que a regressão apareça no diff de revisão | AC2: doctor compara bit de execução | **AUSENTE** — doctor compara somente conteúdo (`bytes.Equal`, scaffold_doctor.go:L221; `actual === expected`, scaffold_doctor.js:L157) |
| `scripts/trackfw-attention-signal.sh` (#11) | Idem; ou: `cp source dest` sem `-p` | AC2 | **AUSENTE** |
| `scripts/trackfw-attention-cleanup.sh` (#12) | Idem | AC2 | **AUSENTE** |
| `scripts/trackfw-credential-guard.sh` (#13) | Idem | AC2 | **AUSENTE** |
| `scripts/trackfw-git-branch-guard.sh` (#14) | Idem | AC2 | **AUSENTE** |

**Caminho de entrada específico para `core.fileMode=false`:** quando `git config core.fileMode=false`,
o git não rastreia nem aplica mudanças de modo. Um commit que altera `100755 → 100644` não aparece
como diff de modo em `git show` para quem tiver essa config. A regressão entra silenciosamente no
histórico e o doctor-atual não vê porque `stat()` lê o filesystem, não o git object.

**Nota sobre o remédio (Ameaça 1 acima):** mesmo depois que AC2 for implementado, o remédio impresso
(`trackfw update`) em Go/Node não restaurará o modo a menos que ML-1A corrija o caminho de update.
O gate AC7 (falsificação em duas direções) deve incluir um vetor que: (1) remove o bit, (2) chama
`trackfw doctor`, (3) verifica que o finding aparece, (4) chama `trackfw update`, (5) verifica que
o bit foi restaurado. Sem o passo 5, o gate não cobre a cadeia completa.

#### Direção falso-positivo: artefato não-executável acusado indevidamente

| Superfície | Como a acusação indevida entra | Gate que deveria capturá-la |
|------------|-------------------------------|----------------------------|
| Slash commands #1–9 (`.claude/commands/trackfw/*.md`) | ML-1A aplica verificação de modo a todos os artefatos sem lista de executáveis | AC4; AC7 vetor (b) |
| CI workflows #15–16 (`.github/workflows/trackfw-gate.yml`, `.gitlab-ci-trackfw.yml`) | Idem | AC4; AC7 vetor (b) |
| Qualquer script com modo 0750 ou 0700 (umask não-padrão) | ML-1A compara `mode == 0755` em vez de `mode & 0o100 != 0` | AC4; AC7 vetor (b) |

**Vetor de teste obrigatório para AC7(b):** criar um arquivo de slash command com modo 0644 (seu modo correto),
rodar o doctor, verificar que nenhum finding é emitido para aquele artefato.

---

## Residual declarado

O que este design aceita NÃO cobrir, dito claramente.

### Residual-1 — Executabilidade real vs. bit de execução

O doctor verifica a presença do bit de execução no inode (`stat()`). Isso é condição necessária, não
suficiente: um arquivo com modo 0755 montado em filesystem com flag `noexec` falha com `exit 126`.
Verificar flags de montagem requer privilégio de root e varia por OS. Declarado não-coberto.
Impacto: baixo — `noexec` é configuração de segurança deliberada; a ferramenta não deve tentar
contorná-la.

### Residual-2 — Windows nativo

No Windows nativo (fora de WSL), o bit de execução POSIX não é representável no filesystem NTFS e
`stat()` não retorna bits de execução confiáveis para arquivos `.sh`. A verificação de modo deve ser
suprimida na plataforma e declarada no output como "verificação de bit de execução indisponível".
A supressão deve ser via guarda de plataforma (`runtime.GOOS == "windows"` / `sys.platform == "win32"` /
`process.platform === "win32"`), não por inferência do resultado de `stat()`.

O que NÃO é coberto: o doctor não detecta scripts inexecutáveis no Windows. O usuário no Windows
confia no mecanismo do hook runner (WSL, Git for Windows shim, ou invocação explícita via `sh`).

### Residual-3 — Arquivos de hook (`.husky/*`, `.lefthook/*`) — herdado

Mantido do `docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md §Residual-3`.
Os arquivos de hook estão fora do escopo do scaffold doctor. Não alterar neste ML.

### Residual-4 — `core.fileMode=false` como vetor de entrada no git

`core.fileMode=false` permite que regressões de modo entrem no histórico git sem aparecer como diff
de modo em `git show` ou `git diff`. O doctor corrige o sintoma (bit perdido no filesystem), mas não
força a rastreabilidade do modo no git. Declarado não-coberto: forçar `core.fileMode=true` é decisão
de equipe, não de ferramenta.

### Residual-5 — Remédio inerte em Go/Node sem fix adicional no update

Se ML-1A implementar SOMENTE a detecção de modo no doctor, sem corrigir o caminho de escrita do
`trackfw update` em Go e Node.js, o finding será detectado mas nunca resolvível via o remédio
impresso. **Isto não é um residual aceito** — é constraint de ML-1A. Está aqui como aviso explícito
para o arquiteto de que a AC de ML-1A deve cobrir a cadeia completa:
detecção → finding com remédio → remédio restaura o bit (Go: `os.Chmod`, Node: `fs.chmodSync`,
Python: já correto via `os.chmod()`).

### Resumo de cobertura declarada

| Superfície | Coberto | Residual |
|------------|---------|----------|
| Scripts #10–14 em POSIX (Linux/macOS) — bit removido | ✅ AC2 + AC6 | — |
| Acusação indevida de não-executáveis (#1–9, #15–16) | ✅ AC4 + AC7(b) | — |
| Umask não-padrão (0750, 0700) | ✅ ML-1A deve usar `& 0o100` | — |
| Windows nativo | Detecção suprimida + linha informativa | Residual-2 |
| Filesystem noexec | Não detectável pelo doctor | Residual-1 |
| Hook files (#17) | Fora de escopo | Residual-3 |
| core.fileMode=false como vetor no git | Não coberto | Residual-4 |
| Remédio update Go/Node | Constraint ML-1A (não residual) | Residual-5 (aviso) |
