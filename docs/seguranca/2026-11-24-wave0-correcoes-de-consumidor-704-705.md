# Wave 0 — Threat Model: Correções de Consumidor #704 e #705

> Hades, 2026-11-24. Nenhuma linha de implementação neste arquivo.

---

## 1. Enumeração de Superfícies

### #704 — Teste afirma valores de trackfw.yaml deste repositório

**Forma do consumidor** (issue #704): fork do trackfw com `trackfw.yaml` próprio
(`req_dir: docs/requisições`, `governance_mode: strict`, `roadmap_namespacing: by_agent`).
Demais arquivos versionados (README.md, cli-parity.md, .gitattributes, .claude/, docs/seguranca/)
existem no fork sem modificação.

**Buscas** (cada grep analisado na íntegra — sem truncamento):
`../../` em `*_test.go` → 17 linhas; `filepath.Join("..", "..")` → 7 linhas;
`repoRoot|findRepoRoot|repositoryRoot(` → 20 linhas; `runtime.Caller` → 32 linhas.

**Categoria A — afirma conteúdo específico deste repositório (mesmo defeito):**

| Sítio | Arquivo:linha | Lê | Comportamento no fork consumidor |
|---|---|---|---|
| A1 | `internal/config/config_merge_key_test.go:501` | `../../trackfw.yaml` | FALHA — 3 campos divergem: REQDir, GovernanceMode, RoadmapNamespacing |

A1 é o único sítio que falha no fork descrito no issue. O teste compara cinco valores
literais (L514–529): `REQDir="docs/req"`, `RoadmapDir="docs/roadmaps"`, `ADRDirs[0]="docs/adr"`,
`GovernanceMode="lenient"`, `RoadmapNamespacing="flat"`. O comentário da L515:
*"these values are known from the repo's trackfw.yaml"*. `t.Skip` cobre ausência do
arquivo (L503, L507), não valores divergentes.

**Categoria A' — mesma causa estrutural, causa raiz diferente:**

Outros sítios leem arquivos versionados por `../../` ou `repoRoot()`/`findRepoRoot()`:

| Sítio | Arquivo:linha | Lê | Causa da eventual falha |
|---|---|---|---|
| A2 | `internal/thirdparty/markers_test.go:273` | `../../docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` | `t.Fatalf` se ausente (fork herda o arquivo) |
| A3 | `internal/guardforms/readme_line_test.go:124` | `../../README.md` | `t.Fatalf` se ausente (fork herda) |
| A4 | `internal/guardforms/readme_line_test.go:450,470` | `../../docs/cli-parity.md` | `t.Fatalf` se ausente (fork herda) |
| A5 | `internal/generators/scaffold_test.go:127` | `../../.claude/commands/trackfw/roadmap.md` | `t.Fatalf` se ausente (fork herda) |
| A6 | `internal/generators/gitattributes_test.go:119` | `../../.gitattributes` | `t.Fatalf` se ausente (fork herda) |
| A7 | `internal/commands/quickstart_e2e_test.go:477,499` | `README.md`, `site/guide/getting-started.md` via `repoRoot()` | `t.Fatalf` se ausente (fork herda) |

A2–A7 afirmam invariantes de doc-sync do codebase do trackfw (marcadores de segurança,
formas de hook, templates, gitattributes, Quick Start). O fork herda esses arquivos
sem alteração; os invariantes se mantêm. Causa raiz diferente de A1: A1 falha porque o
consumidor **customiza** o arquivo; A2–A7 falhariam só em projeto externo que não herda
os arquivos — não é o caso do issue. Declarados residuais (§4).

**Categoria B — protegidos; OK em qualquer consumidor:**

| Sítio | Arquivo:linha | Lê | Proteção |
|---|---|---|---|
| B1 | `internal/validator/validator_test.go:2403` | corpus REQs via `repoRoot` | `t.Skip` individual e global quando `found == 0` |
| B2 | `internal/roadmapdoc/gates_lines_test.go:326` | roadmap específico via `repoRoot(t)` | `t.Skipf` se arquivo ausente |
| B3 | `internal/roadmapdoc/roadmapdoc_test.go:510` | `docs/roadmaps/done/` via `repoRoot(t)` | `ErrNotExist` protegido explicitamente |
| B4 | `internal/generators/scaffold_literal_copy_parity_test.go:54` | `scripts/` via `findRepoRoot(t)` | `t.Skip("scripts/ directory absent")` |

**Vacuidade do contra-braço proposto:** `config.go:398-403` — `defaults()` pré-popula
`REQDir: "docs/req"`, `RoadmapDir: "docs/roadmaps"`, `ADRDirs: ["docs/adr"]`. A asserção
`cfg.REQDir != ""` é vacuosa: passa mesmo com `parse()` quebrado. O AC da REQ
("campos obrigatórios não vazios") é satisfeito por `defaults()` sem `parse()`.
O braço de consumidor (ML-1A ação 2) deve afirmar o **valor da fixture**
(`cfg.REQDir == "docs/requisições"`); o contra-braço (ação 3) deve usar
`ProjectConfig{}` zero-value, não `defaults()`.

**Lista fechada para A1.** A2–A7 e B1–B4 formam o universo completo das leituras de
arquivo versionado em `_test.go`. Nenhum outro sítio encontrado nos 4 greps acima.

---

### #705 — Aviso gemini_cli_legacy sem --scope project

**Buscas:**

```
grep -rn 'agents install\|skills install' --include="*.go" | grep 'validator/'
grep -rn 'trackfw update`' --include="*.go" | grep 'validator/' | wc -l  → 39
grep -rn 'update harness' --include="*.go" | grep 'validator/'
grep -n 'agents install\|skills install\|trackfw init' internal/commands/doctor.go
```

| Sítio | Arquivo:linha | Condição | Comando sugerido | Escopo efetivo não-TTY | Status |
|---|---|---|---|---|---|
| S1 | `validator_gemini_cli_legacy.go:48` | projeto tem `.gemini/settings.json` + guard; falta `.agents/hooks.json` — **condição de projeto** | `trackfw agents install --targets antigravity` | `resolveScope` defaulta para `global` (`integrations_flags.go:565`); `--scope global` desde #697 não escreve em ProjectRoot | **DEFEITO** |
| S2 | `validator_antigravity_ide_global_guard.go:72` | condição global | `trackfw update harness …` | harness = global by design | OK |
| S3 | `validator_opencode_plugin.go:388` | condição global | `trackfw update harness …` | harness = global by design | OK |
| S4 | 39 ocorrências de `` `trackfw update` `` em 7 arquivos do validator | condições de projeto | `trackfw update` | `update.go:24`: "project-scope, never mutates global state" | OK |
| S5 | múltiplas em `validator_git_branch_guard.go` | condições globais (`$HOME`-rooted) | `trackfw update harness` | harness = global by design | OK |
| S6 | `integrations/doctor.go:241` | dinâmico | `` trackfw %s install --force --items %s --targets %s --scope %s `` | `claim.Scope` interpolado — correto | OK |
| S7 | `validator_write_scope_wiring.go:54,74` | condição de projeto (hook ausente) | `trackfw init` **ou** `trackfw update` | `init` defaulta global (`init.go:200`), mas a alternativa `trackfw update` é project-only e resolve a condição | OK — alternativa resgate |

**Lista fechada.** Apenas S1 combina: condição de projeto + único remédio sem escopo.
S7 tem alternativa resgate (`trackfw update`); não é o mesmo defeito.

**O remédio resolve a condição:** `agents install --scope project` chama
`InjectHooksForTools(["antigravity"], root)` (`integrations_flags.go:296`) →
`InjectAntigravityHooks(root)` (`generators/hooks.go:76-80`) → escreve `.agents/hooks.json`
com credential guard. `fileHasCredentialGuardMarker(root, ".agents/hooks.json", ...)` passa
a ser `true`, silenciando o aviso.

**Blast-radius de `--items` omitido** (`integrations_flags.go:328-343`): sem `--items`,
12 agentes são instalados e registrados em `trackfw.yaml`. Comentário L329: "projects with
2+ agents in agents: will get a hard error from bare req new / roadmap new — expected
outcome of ML-3A (not yet landed)". ML-3A não aterrisou; o hard-error ainda não existe.
Ver §4.

---

## 2. Quem Esvazia Esta Wave 0 Sem Quebrar Regra Escrita

**#704:** Implementador que substitui as 5 comparações de valor por `cfg.REQDir != ""`
sem ler a seção sobre `defaults()`. O teste passa em CI deste repositório (defaults
preenche os campos), satisfaz o AC "campos obrigatórios não vazios", e deixa `parse()`
sem fiscalização efetiva. O que fecha o vetor: o braço de consumidor do novo teste
irmão deve afirmar `cfg.REQDir == "docs/requisições"` — falha se `parse()` não
sobreescreve o default.

**#705:** Implementador que edita o literal de string e escreve apenas `strings.Contains(msg, "--scope project")` sem o braço reverso "reprova com mensagem antiga". O braço reverso fecha esse vetor.

---

## 3. Falsificação Nos Dois Sentidos

**#704 — A1:**

- **Falso negativo:** braço de consumidor usa `cfg.REQDir != ""` → passa com defaults()
  mesmo se parse() silencia. Gate: afirmar valor da fixture (`"docs/requisições"`).
- **Falso positivo:** `t.Skip` existente (L503, L507) já cobre arquivo ausente.
- **Contra-braço:** deve usar `ProjectConfig{}` zero-value. `defaults()` como base do
  contra-braço é vacuoso — os campos já são não-vazios antes do parse.

**#705 — S1:**

- **Falso negativo:** mensagem regride para forma sem `--scope project`. Gate:
  `strings.Contains(msg, "--targets antigravity --scope project")` (substring específica
  completa, não apenas `"--scope project"`).
- **Falso positivo:** a flag ser renomeada para `--scope=project` quebraria o test, o
  que seria sinal correto de drift.
- **Blast-radius de ML-3A (futuro):** quando ML-3A aterrissar, o remédio sem `--items`
  registra 12 agentes em `trackfw.yaml`, e bare `req new` passa a requerer `--agent`.
  Este Wave 0 não pode exigir `--items` agora (ML-3A não está no ar). É falsificação
  futura — ver §4.

---

## 4. Residual Declarado

1. **Sítios A2–A7** (causa diferente de A1): falha somente em projetos externos que não
   herdam os arquivos doc-sync do trackfw. O fork consumidor descrito no issue #704 não
   é afetado. Não entram nesta REQ; seriam REQ própria se consumidores externos
   manifestassem o problema.

2. **AC da REQ ("campos obrigatórios não vazios") é vacuoso como escrito:** satisfeito
   por `defaults()` sem `parse()`. O ML-1A precisa reformular: "campos afirmam o valor
   da fixture do consumidor, não apenas não-vazio".

3. **Blast-radius de `--items` omitido (futuro):** quando ML-3A aterrissar, o remédio
   `trackfw agents install --targets antigravity --scope project` sem `--items` registra
   12 agentes em `trackfw.yaml`, quebrando `req new` bare em projetos `flat`. A REQ de
   #705 é escopo estreito (só o texto do remédio); `--items` é uma questão de ML-3A.
   Declarado como residual.

4. **Integração de não-TTY real para #705:** Wave 0 analisa código; não mede execução
   com stdin fechado. A análise de `resolveScope` (`integrations_flags.go:562-565`) é
   determinística; medição de runtime não agrega neste caso.
