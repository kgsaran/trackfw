# Triagem das REQs abertas — 2026-09-05

> Autor: Ártemis (QA) · Investigação pura, sem alteração de código de produto/teste, sem operação
> de git. Confirmado por `grep -l "^status: Open" docs/req/*.md`: **36 REQs Open**, não uma
> contagem herdada.

## Premissas do handoff que a medição derrubou

1. **"147 Done" não é a contagem completa de "não Open".** O histograma real de `status:` em
   `docs/req/*.md` é: `147 Done` + `15 done` (minúsculo) + `2 Closed` + `36 Open` = 200. Fecha
   aritmeticamente, mas há **17 arquivos com grafia de status não-canônica** (`done` em vez de
   `Done`, `Closed` em vez de `Done`) que o handoff não sinalizou — consumidor que filtra por
   `status: Done` exato perde 17 REQs entregues.
2. **"34 das 36 sem roadmap" — medido: são 32, não 34.** Cruzando o campo `req:` do frontmatter de
   **todos** os roadmaps (backlog/analyzing/wip/blocked/done/abandoned) contra os 36 slugs abertos,
   4 REQs têm roadmap referenciado em algum lugar:
   - `REQ-2026-08-31-guarda-de-folha-...` → roadmap em `analyzing/`
   - `REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-...` → roadmap em **`done/`**, mas a
     REQ continua `Open` (a maior parte do trabalho da REQ, no entanto, **não** está no roadmap —
     ver linha da tabela)
   - `REQ-2026-09-05-auditoria-externa-...` → roadmap em `wip/`
   - `REQ-2026-09-03-as-217-falhas-...` → roadmap em `wip/`
   A direção do diagnóstico do KG (fila majoritariamente sem triagem) **se sustenta** — só o número
   exato estava errado por dois.
3. **Roadmap "em `done/`" não implica REQ resolvida.** O caso da reconciliação PRs #238/#240 é o
   exemplo vivo: o roadmap correspondente fechou só 2 dos 7 ACs da REQ (AC5-AC7 — a arrumação de
   roadmaps órfãos e o `.gitattributes`); os 4 ACs restantes (AC1-AC4, sobre `errors="replace"` e a
   allowlist de codificação) seguem intocados no código. Um agente que confiasse em "tem roadmap em
   `done/`" teria fechado a REQ inteira por engano — falsificado por leitura direta de
   `scripts/check-roadmap-barrier-contract.sh:531` e `scripts/check-output-encoding-declared.sh:139`.
4. **A armadilha do arquivo binário-classificado se confirmou dentro desta própria triagem.**
   `grep` sem `-a` sobre `npm/src/validator/index.js` (REQ #21, `thirdparty_artifact_has_provenance`)
   devolve **zero matches em silêncio** — e teria produzido um veredito "AINDA VÁLIDA" falso. Com
   `-a`, a regra está lá, implementada e testada. Ver linha 21 da tabela.

## Tabela completa

| # | REQ (título curto) | Veredito | Evidência |
|---|---|---|---|
| 1 | Conformidade i18n entre os 3 CLIs | **AINDA VÁLIDA** (verificado) | `trackfw help wip_limit`/`roadmap_dir` — os três `Impact:` ainda divergem, confirmado por grep direto em `internal/commands/help.go`, `npm/src/commands/help.js`, `pypi/trackfw/commands/help_cmd.py`. |
| 2 | `branch_has_wip_roadmap` casa por substring | **AINDA VÁLIDA** (verificado) | `internal/validator/validator.go:2606` ainda usa `strings.Contains(normalizeBranchSlug(name), branchSlug)`. |
| 3 | `note_orphan` ausente no CLI Node | **AINDA VÁLIDA** (verificado) | `grep -rln "note_orphan\|noteOrphan\|NoteOrphan"` só acha Go e Python; zero em `npm/src/`. |
| 4 | `validate --json` do Python não rotula `branch_has_wip_roadmap` | **AINDA VÁLIDA** (verificado ao vivo) | Reproduzido: Python devolve `"rule": null`; Go devolve `"rule":"branch_has_wip_roadmap"` no mesmo cenário (`TRACKFW_BRANCH=feat/inexistente`). `validate_branch_has_wip_roadmap` (`pypi/trackfw/validator.py:1749`) ainda devolve `list[str]`. |
| 5 | `update harness` lê `trackfw.yaml` do cwd e escreve em escopo global | **SUPERADA** | A própria REQ se autodeclara reescopada em 2026-08-23: o defeito principal foi resolvido por `REQ-2026-08-23-agents-update-...` (status `done`, PR #207 — 19 call sites, confirmados incluindo os 3 desta REQ). O residual (sanitização de `agent_models`) foi absorvido pela REQ #14. |
| 6 | CLI Python sem `--ci`/`--hooks` no `init`, `git-hooks` fora do `update` | **AINDA VÁLIDA** (verificado, parcial) | `pypi/trackfw/commands/init.py` não tem `add_argument("--ci"...)` nem `--hooks`. O sub-item `ci-workflow` já foi entregue por REQ irmã (`update.py:18-32` documenta a implementação); `git-hooks`/flags seguem ausentes, autoconfessado em `update.py:37-38`. |
| 7 | `agents install` não registra governança; `roadmap new` em `by_agent` sempre no primeiro agente | **AINDA VÁLIDA** (verificado) | `internal/generators/roadmap.go:109` ainda usa `cfg.Agents[0]` sem falhar quando há múltiplos namespaces (AC5 não implementado). |
| 8 | `barrier` executa gate não confiável — `roadmapTrustForGates` fail-open | **AINDA VÁLIDA** (verificado) | `internal/commands/barrier.go:667,708` — comentários "→ fail-open" ainda no código, coincide com vault `barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29` (não corrigido). |
| 9 | `status` do Python conta REQs flat, ignora subpastas de estado | **AINDA VÁLIDA** (verificado) | `_count_reqs_by_status` (`pypi/trackfw/commands/status.py:50`) ainda usa `_list_files` (listdir não-recursivo), não `resolve_req_files` como a AC1 reescrita (issue #268) exige. |
| 10 | Débitos de higiene (4 itens) | **AINDA VÁLIDA** (parcial, verificado) | AC1 (package-lock) **já resolvido** — `npm/package.json`/`package-lock.json` ambos em 7.3.0. AC2 (`pypi/build/` na árvore) ainda existe. AC3 (`InstallGates` sem `os.Chmod`) **reproduzido ao vivo**: `os.WriteFile(path, content, 0755)` não restaura o bit de execução em arquivo pré-existente 0644 (confirmado com programa Go de controle). AC4 não verificado. |
| 11 | Fonte única de vetores de teste | **AINDA VÁLIDA** (verificado) | Nenhum arquivo de vetores compartilhado existe no repositório (`find` vazio). |
| 12 | `roadmap move` segue symlink de arquivo `.md` | **AINDA VÁLIDA** (verificado) | `internal/generators/roadmap.go` (`MoveRoadmap`, linha 413) não tem nenhuma chamada a `Lstat`/checagem de symlink. |
| 13 | Título de roadmap com newline forja seção de wave | **AINDA VÁLIDA** (parcial, verificado) | AC1 (vetor executável em `roadmap new`) **já corrigido** nos 3 CLIs, confirmado por vault `roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23`. AC2 (estender a `req new`/`adr new`/`note new`) **não** implementado — `internal/generators/req.go:NewREQ` interpola título sem nenhuma guarda de caractere de controle. |
| 14 | `agent_models` sem sanitização de valor; ancoragem de `~` imprecisa | **AINDA VÁLIDA** (verificado) | Só existe `containsControlChar` (newline); nenhuma função de validação de **formato** do valor de modelo em nenhum dos 3 CLIs. |
| 15 | Guarda de folha só faz `Lstat` na folha, nunca no ancestral | **AINDA VÁLIDA, com bloqueio declarado** (herdado do handoff) | AC1 e AC4 contraditórias, já anotado no corpo da própria REQ — não implementável até reconciliar. Não redescoberto; roadmap correspondente em `analyzing/`. |
| 16 | `/api/chain` do `serve` não desenha aresta (Node) / perde com separador nativo (Python) | **AINDA VÁLIDA** (verificado, os dois runtimes) | Python: `_find_node_by_ref` compara `ref` **cru** (sem `normalize_ref_separator`) contra `by_id` normalizado — referência com `\` continua sem casar. Node: `resolveRef` só strip a extensão `.md`, nunca o diretório — uma referência de caminho completo (`docs/req/REQ-x.md`) nunca bate contra `fileIndex` (chaveado por basename), o que explica a "aresta nenhuma" mesmo com referência limpa. |
| 17 | CLI Node usa `chmodSync` no caminho, não `fchmodSync` no descritor (TOCTOU) | **AINDA VÁLIDA** (verificado) | `npm/src/integrations/manager.js:96,98` ainda chama `fs.chmodSync(tmp, mode)`/`fs.chmodSync(file, mode)`; `grep -rn "fchmodSync" npm/src/` não acha nada em lugar nenhum do repositório. |
| 18 | Gate anti-divergência não prova completude da lista de cópias | **AINDA VÁLIDA** (verificado) | `scripts/check-atomic-write-anti-divergence.sh:94-97` ainda usa array `FILES=(...)` com 3 caminhos hardcoded, sem descoberta nem asserção de contagem esperada. |
| 19 | Gate de shell detecta só a grafia literal, não a semântica | **AINDA VÁLIDA** (verificado) | `scripts/check-shell-posix-portability.sh` ainda usa `assert_no_code_match` com regex textual (`'shell\s*:\s*true'`), sem instrumentação de runtime — bate com o vault `gate-literal-regex-syntax-equivalent-bypass-2026-09-01`. |
| 20 | `pypi/trackfw/tty.py` sem teste unitário direto | **AINDA VÁLIDA** (verificado) | Nenhum `test_tty*.py` existe em `pypi/tests/`; os únicos hits são módulos que só fazem stub/monkeypatch de `tty`. |
| 21 | `thirdparty_artifact_has_provenance` ausente no validator do Node | **JÁ RESOLVIDA** | `grep` sem `-a` sobre `npm/src/validator/index.js` retorna vazio (o arquivo é classificado binário pelo `file`) — armadilha documentada no vault. Com `-a`: a regra está implementada por completo (`validateThirdPartyArtifactHasProvenance`, linhas 3222-3373), registrada via `applyRule`, e coberta por 12 referências em `npm/tests/`. Introduzida no commit `4c69289` ou anterior. |
| 22 | `serve` interpola `--host` em string de shell (injeção) | **AINDA VÁLIDA** (verificado) | `npm/src/commands/serve.js:205-211` ainda monta `openCmd = \`open "${url}"\`` e chama `exec(openCmd, ...)` — shell verdadeiro, string interpolada, sem sanitização. |
| 23 | Guard global instalado sob `$HOME` ≠ `%USERPROFILE%` (instalação fantasma) | **AINDA VÁLIDA, com bloqueio declarado** (herdado do handoff) | Não redescoberto — tratado conforme instrução: AC1/AC4 contraditórias, não implementável até reconciliar. |
| 24 | Guard emite schema de hook que o Claude Code rejeita | **AINDA VÁLIDA** (verificado) | `grep -rn "hookSpecificOutput\|permissionDecision" internal/generators/*.go` não acha nada — nenhum dos 3 ACs implementado. |
| 25 | `init` e `discover` geram dois workflows de CI distintos | **AINDA VÁLIDA** (verificado) | `internal/discover/discover.go:writeCIWorkflow` escreve `.github/workflows/trackfw-validate.yml`; `internal/generators/scaffold.go:generateCIWorkflow`/`GitHubActionsWorkflowPath` escreve `.github/workflows/trackfw-gate.yml` — dois arquivos, dois geradores, nenhuma decisão de unificação documentada (AC1 pede decisão registrada; não encontrada). |
| 26 | Reconciliação pós-merge PRs #238/#240 — `.trackfw-log` conflitante | **AINDA VÁLIDA** (parcial, verificado) | AC5-AC7 (`.gitattributes` `merge=union`, roadmaps órfãos arrumados) **já entregues** — confirmado no arquivo `.gitattributes` da raiz e nos 3 geradores (`scaffold.go`, `init.js`, `init_gen.py`). AC1-AC4 (encoding `errors="replace"`→`strict` ou justificativa medida; allowlist obsoleta; `sys.stdout.reconfigure` não reconhecido por `DECL_RE`) **não** implementados — código idêntico ao descrito na REQ. |
| 27 | Remover `PreToolUse` do settings não é detectado | **AINDA VÁLIDA** (verificado) | Nenhuma ocorrência de `PreToolUse` em `internal/validator/validator.go` (positivo-controle: o termo existe em outros arquivos do projeto, então a ausência aqui não é falha de grep). |
| 28 | As 217 falhas reais de Windows colapsam em poucas causas | **AINDA VÁLIDA** (verificado, quase fechada) | Roadmap `wip/ROADMAP-2026-09-03-fechar-os-grupos-...`: **22 de 23 MLs concluídos**. Só `ML-5A` (parser tolera CRLF na fronteira de entrada, ~14 testes, `ADR-2026-09-04` já `Accepted`) segue `⬜ Pendente`. |
| 29 | `check-gates-falsify.sh` é 610/780s do `parity` | **AINDA VÁLIDA** (presumido) | Nenhum roadmap referenciando esta REQ; `git log -- scripts/check-gates-falsify.sh` não mostra commit de perfilamento/otimização desde a data da REQ. Medir o tempo real de CI exigiria rodar o pipeline — fora do escopo desta investigação. |
| 30 | `check-referential-integrity.sh` OK e exit 0 sobre árvore vazia | **AINDA VÁLIDA** (verificado) | Lido por inteiro: `for req in docs/req/*.md` sem guarda de vacuidade — árvore sem REQs nunca entra no loop e cai direto em `echo "Referential integrity OK"` / exit 0. |
| 31 | Instalação em Windows promete o que não entrega; instalador recusa o binário publicado | **AINDA VÁLIDA** (parcial, verificado) | AC1-AC4 **já entregues** — README declara a dependência de shell no mesmo parágrafo da promessa, seção "Windows support (partial)" existe, a contradição do `install.sh` está declarada por escrito (não corrigida no código, mas a direção foi escolhida: "install manually"), limite de ARM64 documentado. AC5-AC7 (jornada ponta-a-ponta com hook disparando, falsificação sem Git Bash, caminhos com espaço/acento) **não** encontrados — nenhum teste de jornada real em `scripts/windows-repro/`. |
| 32 | Auditoria externa Astra — declaramos correção onde a medição dizia o contrário | **AINDA VÁLIDA** (verificado) | Roadmap `wip/ROADMAP-2026-09-05-reconciliar-...`: 2 de 5 MLs concluídos (`ML-1A`, `ML-1B`); `ML-2A`, `ML-3A`, `ML-3B` seguem `⬜ Pendente`. |
| 33 | Gate de palavra-chave de fechamento não reavalia em `edited` | **AINDA VÁLIDA** (verificado) | `.github/workflows/quality.yml` ainda declara só `pull_request:` sem `types:` — o default do GitHub Actions é `opened, synchronize, reopened`, nunca `edited`. |
| 34 | Guard do trackfw passa a bloquear staging com escopo implícito | **AINDA VÁLIDA** (verificado) | Só existem o ADR e a REQ (`chore(governance): ADR e REQ para bloquear staging com escopo implicito`); nenhuma lógica de bloqueio de `git add -A`/`--all`/`.`/`-u` em nenhum dos 3 geradores de guard — o único hit de "`git add -A`" no código é texto instrucional do template de roadmap (`scaffold.go:617`), não uma checagem. |
| 35 | Hooks de guard não executam no Windows na maioria dos CLIs de agente; `validate` reporta instalado | **AINDA VÁLIDA** (verificado) | Nenhuma regra de `validate` diferencia por plataforma/CLI de agente — a regra de presença do guard não tem caminho `not_evaluated`/consciência de SO. |
| 36 | `validate_unfiltered` do Python devolve lista de tipo misto | **AINDA VÁLIDA** (verificado — mesma causa raiz da #4) | `_enrich_items` (`pypi/trackfw/validator.py:293`) tem `else: result.append(item)` que deixa string crua passar sem embrulhar; é o mesmo `validate_branch_has_wip_roadmap` que devolve `list[str]` da REQ #4. Corrigir uma fecha a outra — ver ordenação. |

## Contagem por veredito

| Veredito | Quantidade |
|---|---|
| AINDA VÁLIDA (verificado, integral) | 24 |
| AINDA VÁLIDA (verificado, parcial — parte já entregue) | 8 (#6, #10, #13, #16\*, #26, #28, #31, #32) |
| AINDA VÁLIDA, com bloqueio declarado (herdado, não redescoberto) | 2 (#15, #23) |
| AINDA VÁLIDA (presumido, sem verificação de código — custo de medição alto) | 1 (#29) |
| JÁ RESOLVIDA | 1 (#21) |
| SUPERADA | 1 (#5) |
| DUPLICADA | 0 (nenhum par estritamente duplicado — #36 e #4 compartilham causa raiz mas atacam consumidores diferentes, tratado como "mesma correção fecha as duas" na ordenação, não como duplicata) |
| A ABANDONAR | 0 |
| INDETERMINADA | 0 |

\* #16 conta como "parcial" porque o Node tem bug estrutural mais amplo (nenhum AC atendido) e o Python
tem só o AC2 pendente — nenhuma parte de nenhum dos dois runtimes está de fato resolvida, mas o
diagnóstico da própria REQ (AC1) já separa as duas causas, o que reduz o trabalho restante.

**Total: 36.** Nenhuma REQ foi classificada A ABANDONAR — todas as 36 descrevem defeito real, não
obsoleto e não superado (com a única exceção de #5, que já se autodeclarou parcialmente superada).

## Ordenação por retorno das "AINDA VÁLIDA" (insumo para as 2 próximas frentes, WIP-2)

Critério: quantos defeitos reais fecha, risco de regressão, e se compartilha arquivo com a outra
frente escolhida (WIP limitado a duas — as duas primeiras da lista não compartilham nenhum arquivo
entre si).

1. **#28 — Fechar o resíduo de Windows (só falta ML-5A, CRLF no parser de frontmatter).**
   Maior retorno por menor esforço da lista inteira: 22/23 MLs já concluídos, ADR já `Accepted`,
   ~14 testes fecham de uma vez. Arquivos: parser de frontmatter dos 3 CLIs + `.gitattributes`
   (não sobrepõe #4/#36).
2. **#4 + #36 juntas — `validate_branch_has_wip_roadmap` devolve `dict`, não `str`.** Uma correção
   fecha as duas REQs (mesma função, dois consumidores — `validate --json` e `validate_unfiltered`).
   Baixíssimo risco (mensagem e exit code já byte-idênticos, só a forma do retorno muda), alto
   retorno de confiabilidade para quem consome `--json` em CI.
3. **#8 — `roadmapTrustForGates` fail-open em todo caminho de erro.** É o gate que decide se
   `barrier` roda gates confiáveis sobre PR de terceiro — superfície de segurança real, não
   cosmética. Escopo bem definido (AC1-AC8 já redigidos), sem sobreposição de arquivo com #28 ou #4.
4. **#17 — `chmodSync` no caminho em vez de `fchmodSync` no descritor (TOCTOU no Node).** Escopo
   pequeno (`npm/src/integrations/manager.js` + 2-3 sites), fecha uma classe de segurança (escrita
   atômica) que os outros 2 CLIs já não têm.
5. **#22 — Injeção de comando via `--host` no `serve`.** Superfície de segurança concreta e
   demonstrável (shell injection real), correção pequena (trocar `exec()`/string por `execFile()`
   com array de argumentos).
6. **#12 — `roadmap move` segue symlink de arquivo `.md`.** Escreve fora do projeto; escopo
   contido, sem AC de "enumerar tudo" nesta REQ como a REQ irmã (#... symlink de diretório, já
   corrigida) exigiu.
7. **#26 (AC1-AC4 residuais) — decisão sobre `errors="replace"` e a allowlist obsoleta.** Pequeno,
   mas com data de vencimento: quanto mais tempo passa mais scripts crescem em cima do padrão
   "aceito porque tinha PR aberto", que já não é verdade.
8. **#30 — `check-referential-integrity.sh` vácuo sobre árvore vazia.** Gate de 54 linhas, correção
   de poucas linhas (checar `${#req_dirs[@]}` ou contagem de arquivos antes do loop).

As demais "AINDA VÁLIDA" (1, 2, 3, 6, 7, 9, 10, 11, 13 residual, 14, 16, 18, 19, 20, 24, 25, 27, 29,
31 residual, 33, 34, 35) são reais e válidas, mas têm escopo maior (ADR a escrever antes de código),
dependem de medição cara (Windows real, CI), ou têm retorno mais baixo por defeito fechado — não
competitivas para as 2 vagas de WIP nesta rodada.

## Notas de execução

- Nenhuma alteração de código de produto ou teste foi feita nesta investigação.
- Nenhuma operação de git foi executada.
- Ambiente de teste isolado usado para reprodução ao vivo: `/tmp/qa_test_open2` (Python) e
  `/tmp/chmodtest` (Go/Python, comportamento de `os.WriteFile`/`os.chmod`) — fora da árvore do
  projeto, sem escrita em `docs/roadmaps/` nem `docs/req/`.
- `npm/src/integrations/doctor.js` e `npm/src/validator/index.js` são classificados binários pelo
  `file` (bytes NUL como separador de chave) — toda busca neste documento usou `grep -a` ou
  verificação por `sed`/`Read` direto nesses dois arquivos.
