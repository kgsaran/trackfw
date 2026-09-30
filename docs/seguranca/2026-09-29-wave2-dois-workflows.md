---
date: 2026-09-29
author: hades-tf
roadmap: ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md
wave: 2
method: independent-reimplementation (black-box, scratchpad fixtures, no diff read)
---

# Wave 2 — Auditoria independente: um workflow de governança por projeto

Referências: `ADR-2026-09-29`, `REQ-2026-09-02` (ACs 7–10), `docs/cli-parity.md`
(seção "As quatro classes de finding" e tabela de artefatos condicionais).
Binário medido: `bin/trackfw` compilado de `fix/dois-workflows-rodam-a-mesma-validacao`.

---

## 1. O comportamento do gerador (D2)

**O que a ADR espera (D2):** `generateGitHubActionsWorkflow` não escreve `trackfw-gate.yml` quando
`.github/workflows/trackfw-validate.yml` já existe como arquivo regular. Detecção via `os.Lstat`
(symlink NÃO conta como presente). O contrário (validate.yml ausente) produz gate.yml.

### Contra-braço — projeto novo sem validate.yml

```
Comando: trackfw update (trackfw.yaml ci: github-actions, sem validate.yml)
Saída:   ✓ .github/workflows/trackfw-gate.yml
         ✓ CI workflow atualizado
gate.yml: SIM | validate.yml: NAO
```

Veredito: **CONFORME.**

### D2 — validate.yml presente como arquivo regular

```
Comando: trackfw update (validate.yml regular pré-existente)
Saída:   ℹ .github/workflows/trackfw-validate.yml já existe —
           .github/workflows/trackfw-gate.yml não será escrito (ADR-2026-09-29 D2)
         ✓ CI workflow atualizado
gate.yml: NAO | validate.yml: SIM (inalterado)
```

Veredito: **CONFORME.** A razão é dita ao usuário.

### Caso do symlink — validate.yml como symlink para arquivo real

```
Fixture: ln -s "../../validate-real.yml" .github/workflows/trackfw-validate.yml
Saída:   ✓ .github/workflows/trackfw-gate.yml
         trackfw: refusing write to .../trackfw-validate.yml:
           refusing reparse-point path "..." (mode Lrwxr-xr-x)
gate.yml: SIM | validate.yml: continua sendo symlink
```

`discoverWorkflowPresent` usa `os.Lstat` e verifica `info.Mode()&os.ModeSymlink == 0`. Um symlink
retorna `false` → gate.yml é escrito. A tentativa de `refreshDiscoverGitHubActionsWorkflowIfPresent`
é recusada pelo pathguard com mensagem explícita.

**A escolha é defensável?** Sim. A direção alternativa (tratar symlink como "presente") permitiria
que um symlink dangling ou malicioso em validate.yml impedisse a instalação de gate.yml sem nenhum
artefato CI ativo. Gate.yml é escrito no path canônico (arquivo regular, guarded por
`rejectScaffoldPath`), não através do symlink de validate.yml — os dois paths são independentes.

Veredito: **DEFENSÁVEL** — derivado da saída das fixtures, não da leitura do comentário de código.

### Caso do symlink — `.github/workflows/` diretório como symlink

```
Fixture: ln -s /tmp/real-dir .github/workflows
Saída:   trackfw: refusing write to .../scratchpad/fix-.../  .github/workflows:
           refusing reparse-point path "..." (mode Lrwxr-xr-x)
         ⚠ CI workflow: refusing write [mesmo erro]
gate.yml em real-dir: NAO
gate.yml escrito em qualquer lugar: NAO
```

`rejectScaffoldPath(ghRoot, absGHDir)` detecta o symlink no diretório antes de `MkdirAll`.
Nenhuma escrita ocorre.

Veredito: **CONFORME.** A proteção cobre o nível de diretório (PoC 1 documentado em discover.go).

### Caso do symlink — validate.yml dangling (aponta para nada)

```
Fixture: ln -s /nonexistent .github/workflows/trackfw-validate.yml
update:  ✓ .github/workflows/trackfw-gate.yml
doctor:  trackfw doctor: no mismatches found
```

`discoverWorkflowPresent` via Lstat: symlink tem `ModeSymlink` set → retorna false → gate.yml escrito.
`scaffold_doctor.go:379` usa `os.Stat` (que falha em symlink dangling) → bloco skipped → sem advisory.

Sem write-through, sem false advisory. Veredito: **CONFORME.**

---

## 2. O `doctor` (D3 e D4)

**O que o contrato espera:**
- Só validate.yml presente → silencioso quanto à ausência de gate.yml (D4)
- Ambos ausentes com `ci: github-actions` → `scaffold-missing` para gate.yml
- `gate.yml` presente e defasado → `scaffold-divergent`
- Ambos presentes → `scaffold-workflow-duplicated`, advisory, nomeando os dois arquivos e job ids,
  **sem sugerir remoção incondicional**

### Só validate.yml presente

```
trackfw doctor: 6 finding(s) -- 0 ... 0 scaffold-missing ... 0 scaffold-workflow-duplicated
[scaffold-divergent] .github/workflows/trackfw-validate.yml  ← fixture desatualizada
(sem scaffold-missing para gate.yml)
```

Veredito: **CONFORME.** Supressão de scaffold-missing funciona.

### Ambos ausentes com ci: github-actions

```
trackfw doctor: 6 finding(s) -- 6 scaffold-missing
[scaffold-missing] .github/workflows/trackfw-gate.yml
```

Veredito: **CONFORME.**

### gate.yml defasado (contra-braço)

```
trackfw doctor: 6 finding(s) -- 1 scaffold-divergent, 5 scaffold-missing
[scaffold-divergent] .github/workflows/trackfw-gate.yml
```

Veredito: **CONFORME.** A supressão de D4 não neutraliza scaffold-divergent.

### Ambos presentes

```
trackfw doctor: 2 finding(s) -- 1 scaffold-divergent, 1 scaffold-workflow-duplicated

[scaffold-workflow-duplicated] .github/workflows/trackfw-validate.yml
  remedy: .github/workflows/trackfw-gate.yml (job: governance-install-script) and
    .github/workflows/trackfw-validate.yml (job: governance-go-install) both run
    `trackfw validate` — .github/workflows/trackfw-gate.yml is canonical (ADR-2026-09-29 D1).
    Before removing .github/workflows/trackfw-validate.yml, verify that governance-go-install
    is NOT in your repository's required_status_checks: the product cannot check this for you.
    If it is not a required check, remove .github/workflows/trackfw-validate.yml manually.
```

Advisory emitido: CONFORME. Dois arquivos e job ids nomeados: CONFORME. Remoção condicionada à
verificação de required_status_checks: CONFORME. Nenhum comando de remoção automática: CONFORME.

### 🔴 ACHADO-2 — `os.Stat` em validate.yml com live symlink emite `scaffold-divergent` com remédio inexequível

O bloco de checagem do validate.yml em `scaffold_doctor.go:379` usa `os.Stat` (lê ATRAVÉS do
symlink) enquanto todos os outros predicados do mesmo arquivo usam `os.Lstat`:

```
Fixture: validate.yml = live symlink para arquivo com conteúdo diferente do template
Doctor:  [scaffold-divergent] .github/workflows/trackfw-validate.yml
         remedy: trackfw update   # resync .github/workflows/trackfw-validate.yml
```

O problema: `trackfw update` recusa escrever através do symlink. O finding é emitido; o remédio
prescrito não pode ser executado. Isso é um falso positivo com remédio inoperante.

A mesma discrepância com symlink vivo ocorre para `scaffold-workflow-duplicated`: Lstat vê o
symlink como não-presente → `discoverWorkflowPresent` retorna false → advisory NÃO é emitido. Mas
`os.Stat` no bloco de baixo lê o conteúdo e emite `scaffold-divergent`. Os dois predicados
discordam sobre o que há em validate.yml.

Severidade: baixa (requer symlink intencional em validate.yml, não um fluxo normal de adoção).
Afeta: produto (`internal/generators/scaffold_doctor.go`). Não é uma quebra de segurança.
Fix sugerido ao proprietário do código: usar `os.Lstat` + verificar ausência de `ModeSymlink` na
linha 379, consistente com todos os outros predicados do arquivo.

---

## 3. A pergunta que decide se a entrega vale

Ordens testadas e resultado:

| # | sequência | resultado |
|---|---|---|
| A | `discover --init` (sem trackfw.yaml) → `update` | validate.yml escrito; update vê validate.yml e NÃO escreve gate.yml — **1 workflow** |
| B | `update` → `discover --init` | gate.yml escrito; discover vê trackfw.yaml → early-return — **1 workflow** |
| C | `update` → `rm trackfw.yaml` → `discover --init` | gate.yml + validate.yml — **2 workflows** |
| D | `update` com `ci: none` (validate.yml presente) → muda `ci: github-actions` → `update` | validate.yml presente; update suprime gate.yml — **1 workflow** |
| E | symlink em validate.yml → `update` | gate.yml escrito; symlink intocado e recusado pelo pathguard — **1 workflow ativo** |
| F | `.github/workflows/` como symlink → `update` | recusado pelo pathguard antes de qualquer escrita — **0 workflows escritos** |

### 🔴 ACHADO-1 — sítio irmão `writeCIWorkflow` (`internal/discover/discover.go`) não implementa o predicado recíproco de D2

A ADR nomeia a assimetria como o defeito: "um lado pergunta, o outro não."

O que foi fechado: `generateGitHubActionsWorkflow` (scaffold.go) agora verifica a presença de
validate.yml antes de escrever gate.yml.

O que permanece aberto: `writeCIWorkflow` (discover.go) verifica apenas se `validate.yml` já existe
como arquivo regular, e nunca verifica se `gate.yml` já existe. Quando gate.yml está presente e
trackfw.yaml é removido + discover --init é executado, validate.yml é escrito e o projeto chega ao
estado "dois workflows".

```go
// internal/discover/discover.go:370–376 (medido)
if _, err := os.Lstat(dest); err == nil {
    return nil              // "dest" é validate.yml — gate.yml não é verificado
}
```

Este é o caminho C da tabela. Exige exclusão de trackfw.yaml — operação atípica, mas não exótica:
- trackfw.yaml pode ser gitignored em um clone novo
- consumer que segue documentação "run discover --init to adopt" em repo existente
- este repositório já tem REQ sobre init reexecutado destruindo config (memória: `generate_lefthook_hook_segundo_sitio_a`)

**Reclassificação:** não é "dentro do residual declarado." D3 diz que projetos que JÁ TÊM os dois
mantêm os dois. Não autoriza o produto a criar o segundo. D1 diz um workflow por projeto. D2
fecha a assimetria em um lado só.

Sob a Regra Dura de Causa Raiz deste repositório: mesma causa (assimetria de escrita), mesma ADR
(D2). O sítio irmão é um ML adicional nesta REQ, não REQ nova. Produto não está quebrado (o doctor
detecta), mas D1 está parcialmente incumprido.

**Arquivo, linha:** `internal/discover/discover.go:370` — `writeCIWorkflow`, antes do bloco de idempotência.
**Fix:** verificar `generators.GitHubActionsWorkflowPath` via Lstat antes de escrever validate.yml.
Autoridade do fix: proprietário do código (`trackfw_architect` / Apolo). Hades reporta, não modifica.

---

## 4. Nenhum consumidor perde check que já exigia (AC10)

`.github/required-status-checks.txt:29–30` declara:
```
governance-install-script    ← emitido por .github/workflows/trackfw-gate.yml:7
governance-go-install         ← emitido por .github/workflows/trackfw-validate.yml:10
```

Verificação D\W:

```
Comando: python3 scripts/check-required-status-checks.py --scope dw
Saída:   [OK] [scope=dw] declared=9, workflow_checks=45 — D\W=∅.
         R não verificado (GITHUB_TOKEN sem permissão de administrador em CI):
         D\R e R\W não foram verificados nesta execução.
```

Os dois workflows existem neste repositório e emitem os dois job ids. D\W=∅ confirma que nenhum
job declarado deixou de existir nos workflows. R\W e D\R requerem credencial de mantenedor
(`make check-required-full`) — fora do escopo desta auditoria local.

Veredito: **CONFORME** no que é verificável localmente.

---

## 5. A contagem do relatório do `doctor`

Aritmética para cada fixture (soma dos 11 contadores per-kind vs. total declarado):

| fixture | linha de totais | soma | ≡ total |
|---|---|---|---|
| fix-doctor-only-validate | 0+0+0+**1**+**5**+0+0+0+0+0+0 | 6 | ✓ 6 |
| fix-doctor-ambos | 0+0+0+**1**+0+0+0+0+0+0+**1** | 2 | ✓ 2 |
| fix-doctor-ambos-ausentes | 0+0+0+0+**6**+0+0+0+0+0+0 | 6 | ✓ 6 |
| fix-doctor-gate-defasado | 0+0+0+**1**+**5**+0+0+0+0+0+0 | 6 | ✓ 6 |

`DoctorScaffoldWorkflowDuplicated` tem case explícito no switch (`internal/commands/doctor.go:153`)
e linha de totais em posição própria. Nenhuma inflação silenciosa.

Veredito: **CONFORME.**

---

## Sumário

| item | contrato | veredito |
|---|---|---|
| D2 — gate.yml não nasce com validate.yml regular presente | CONFORME | mensagem informativa emitida |
| D2 — gate.yml nasce sem validate.yml (contra-braço) | CONFORME | — |
| D2 — symlink em validate.yml (live) → gate.yml nasce, symlink recusado para escrita | CONFORME, DEFENSÁVEL | — |
| D2 — symlink em validate.yml (dangling) → gate.yml nasce, sem false advisory | CONFORME | — |
| D2 — `.github/workflows/` como symlink → recusado, nada escrito | CONFORME | — |
| D4 — doctor silencioso quanto a gate.yml ausente quando validate.yml presente | CONFORME | — |
| D3 — doctor acusa ambos ausentes (ci: github-actions) | CONFORME | — |
| D3 — doctor acusa gate.yml defasado (contra-braço) | CONFORME | — |
| D3 — doctor emite advisory com dois arquivos e job ids, sem remoção incondicional | CONFORME | — |
| AC10 — nenhum job id perdido nos workflows (D\W=∅) | CONFORME | — |
| Contagem do doctor fecha com total | CONFORME | — |
| Caminho A (discover→update) | 1 workflow | CONFORME |
| Caminho B (update→discover) | 1 workflow | CONFORME |
| Caminho C (update→rm trackfw.yaml→discover) | 2 workflows | **🔴 ACHADO-1** |
| Caminho D (ci:none→ci:github-actions com validate.yml) | 1 workflow | CONFORME |
| Caminho E (symlink em validate.yml) | 1 workflow ativo | CONFORME |
| Caminho F (diretório .github/workflows como symlink) | recusado | CONFORME |
| Lstat vs Stat em scaffold_doctor.go:379 | live symlink → scaffold-divergent com remédio inoperante | **🔴 ACHADO-2** |

---

## Achados para o proprietário do código

### ACHADO-1 (severidade: média — incumpre D1, não é break de segurança)

**Arquivo:** `internal/discover/discover.go`
**Função:** `writeCIWorkflow`
**Linha:** ~370 (bloco de idempotência)
**Descrição:** A função não verifica a presença de `trackfw-gate.yml` antes de escrever
`trackfw-validate.yml`. D2 fechou a assimetria em `generateGitHubActionsWorkflow` mas não no sítio
irmão. Um consumidor que deleta `trackfw.yaml` e roda `discover --init` recebe dois workflows; o
doctor detecta, mas o estado contradiz D1.
**Comportamento observado:** Caminho C (update → rm trackfw.yaml → discover --init) → gate.yml +
validate.yml presentes, doctor emite `scaffold-workflow-duplicated`.
**Fix:** verificar `generators.GitHubActionsWorkflowPath` via `os.Lstat` no início de
`writeCIWorkflow`, antes do bloco de idempotência de validate.yml. Se gate.yml existir como arquivo
regular, skip (análogo ao que `generateGitHubActionsWorkflow` faz para validate.yml).

### ACHADO-2 (severidade: baixa — falso positivo com remédio inoperante em edge case)

**Arquivo:** `internal/generators/scaffold_doctor.go`
**Linha:** ~379
**Descrição:** O bloco de checagem de `trackfw-validate.yml` usa `os.Stat` (lê ATRAVÉS de symlink)
enquanto todos os outros predicados do mesmo arquivo (incluindo o D3 check e `discoverWorkflowPresent`)
usam `os.Lstat`. Com live symlink em validate.yml: `os.Stat` sucede, compara conteúdo do alvo com
o template, emite `scaffold-divergent`; o remédio "trackfw update" é prescrito mas update recusa
escrever através do symlink.
**Comportamento observado:** live symlink com conteúdo divergente → `scaffold-divergent` emitido;
`trackfw update` → recusa com `refusing reparse-point path`.
**Fix:** na linha ~379, usar `os.Lstat` + verificar ausência de `ModeSymlink`, consistente com o
restante do arquivo.
