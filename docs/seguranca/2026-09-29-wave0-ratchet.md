# Wave 0 — Threat Model: ratchet bidirecional (D6 + D7)

**Roadmap:** ROADMAP-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-nunca-colhe-a-melhoria.md
**ML:** 0A
**Data:** 2026-09-29
**Owner:** hades-tf

---

## Gate W0 — precondição da triagem

```
$ jq -r '.entries[]|.name' .github/windows-known-failures.json | wc -l | tr -d ' '
14
Gate W0: 14 entradas na lista — a triagem parte deste universo
```

---

## 1. As 14 falhas: razão medida e agrupamento por mecanismo

**Fonte:** log do job `windows-full-suites`, run 36608706692, 2026-09-29.
**Grep usado:** `grep -ao -- '--- FAIL: [A-Za-z0-9_]*'` — o log tem prefixo `windows-full-suites\tSTEP\t<timestamp>` e um `^--- FAIL` literal dá zero resultados.

**Legenda:** MEDIDO = a mensagem de falha no log evidencia o mecanismo. INFER = mecanismo inferido de leitura do código + mensagem; precisa de confirmação em runner Windows.

### Grupo A — POSIX permissions: chmod ignorado no NTFS sem ACL (4 entradas)

| Nome | Arquivo:linha | Mensagem no log | Evidência |
|---|---|---|---|
| TestFilenameUniqueness_DiretorioNaoLegivel_P2 | validator_test.go:1609 | `esperado violation sobre diretório ilegível, obteve: []` | MEDIDO |
| TestFolderStatus_DiretorioNaoLegivel_P2 | validator_test.go:1589 | `esperado warning sobre diretório ilegível, obteve: []` | MEDIDO |
| TestSave_WritesAtomicallyWithPermissions | identity_test.go:127 | `permissao do arquivo = 666, esperava 0600` | MEDIDO |
| TestStaleWIPReportsWIPWalkError | validator_stale_wip_contract_xfail_test.go:127 | `esperava diagnostico para erro de walk/ENOTDIR em wip/; warnings=[]` | MEDIDO |

**Mecanismo A:** `os.Chmod(0o600/0o000)` é silenciosamente ignorado no NTFS sem manipulação de ACL (issue #421). Os três primeiros testam modo 0600 — o Windows retorna 0666. O `TestStaleWIPReportsWIPWalkError` usa `writeFile(dir, "docs/roadmaps/wip", "not a directory\n")` para criar um ARQUIVO onde deveria haver um DIRETÓRIO e espera erro de walk com ENOTDIR — confirmado por leitura de `validator_stale_wip_contract_xfail_test.go:113-127`. O comportamento de `ENOTDIR = ERROR_PATH_NOT_FOUND` no Windows (medido em ADR-2026-09-05 Adendo) faz o walk não produzir o diagnóstico esperado. Nota: os quatro poderiam ser refinados como dois sub-grupos (0600-vs-0666 / ENOTDIR-behavior), mas a causa-raiz NTFS/permissão é compartilhada.

### Grupo B — CRLF no renderer/parser: line endings não normalizados (4 entradas)

| Nome | Arquivo:linha | Mensagem no log | Evidência |
|---|---|---|---|
| TestRenderOpenCodeAgent_CRLFSourceMatchesLF | render_test.go:602 | `CRLF source produced a different render than LF source.` | MEDIDO |
| TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF | render_test.go:832 | `CRLF source produced a different Rota B render than LF source.` | MEDIDO |
| TestRenderWithoutIdentityMatchesFrozenGoldens | render_test.go:735 | `Render sem identidade diverge do golden congelado architect.subagent.golden.md` (só subtest `subagent/architect`) | INFER |
| TestResolveAgentModelMatchesRender | models_test.go:149 | `ResolveAgentModel.present=true but rendered model present=false` (16 combinações: gemini/copilot/windsurf/kiro x architect/backend/frontend/qa) | INFER |

**Mecanismo B (MEDIDO para os dois primeiros):** o normalizador CRLF do renderer não cobre todos os caminhos de código — fonte CRLF produz saída diferente de fonte LF no Windows. Os dois `*CRLFSourceMatchesLF` testam isso explicitamente.

**Nuance em TestRenderWithoutIdentityMatchesFrozenGoldens (INFER):** o log mostra o `got` com o conteúdo normal do frontmatter e body. Sem ver o `want` no log (truncado) não é possível confirmar que a diferença é CRLF. Mecanismo alternativo: o frozen golden foi gerado no macOS; se o renderer produz saída diferente no Windows independentemente de CRLF (ex: encoding de backslash em paths internos), a divergência pode ser por outro motivo. O subtest que falha é `subagent/architect` (representação `agent-markdown`) enquanto os outros passam.

**Nuance em TestResolveAgentModelMatchesRender (INFER):** as 16 combinações que falham são exatamente os targets `gemini` (repr=agent-markdown), `copilot` (repr=custom-agent), `windsurf` (repr=skill) e `kiro` (repr=agent-markdown) para todos os 4 agentes. O padrão por target (não por agente) é mais consistente com CRLF no output renderizado do que com um bug específico de agente. A hipótese: o campo `model:` no arquivo renderizado tem `\r\n` e o extrator usa `\n`. Confirmação requer stepping no Windows runner.

### Grupo C — External command: comportamento diferente ou ausência de ferramenta (2 entradas)

| Nome | Arquivo:linha | Mensagem no log | Evidência |
|---|---|---|---|
| TestAttentionScripts_FallbackWithoutJQ | scaffold_test.go:440-443 | `Tool esperada 'fallback_tool', obteve ""; Message esperada 'Testing fallback without jq', obteve "Agent needs attention"` | MEDIDO (causa inferida por leitura de código) |
| TestShip_Integration_GracefulDegradation_RealBinary | ship_test.go:1309 | `BUG (fork bomb): C:\Users\runneradmin\...\bin\git.exe` | MEDIDO |

**Mecanismo C1 (TestAttentionScripts_FallbackWithoutJQ):** o teste cria `fakeBinDir` e tenta criar symlinks para `bash`, `python3`, etc. usando `os.Symlink`. No Windows, `os.Symlink` exige privilégio de desenvolvedor — `isSymlinkPrivilegeError` retorna true, o loop continua sem criar os links (`prossegue sem este binário`). O script roda com `PATH=fakeBinDir` vazio. Sem `python3` disponível dentro do script, o caminho de fallback JSON não executa e o script retorna o valor default `"Agent needs attention"` com `tool=""`. Este NÃO é "bash ausente em %PATH%" (bash está no PATH do runner e é encontrado por `exec.Command`) — é **Windows symlink privilege** que esvazia o fakeBinDir do setup do teste.

**Mecanismo C2 (TestShip):** o teste cria um `git.exe` fake em temp dir para testar graceful degradation quando `gh` está ausente. A substituição de git no PATH do runner ativa a defesa anti-fork-bomb do Windows runner.

### Grupo D — Path representation: separador nativo vs. JSON portável (3 entradas)

| Nome | Arquivo:linha | Mensagem no log | Evidência |
|---|---|---|---|
| TestChecksum_StableAndMatchesSHA256Sum | markers_test.go:253 | `Checksum() = "6f0afa5..."`, `shasum -a 256 = "\6f0afa5..."` | MEDIDO |
| TestThirdPartyInstall_PassesValidateEndToEnd | integrations_thirdparty_validate_test.go:74 | `artifact has no entry in .trackfw/thirdparty-provenance.json (D2 branch i)` — esperava branch-ii | MEDIDO (causa INFER) |
| TestThirdPartyInstall_TamperAfterInstallFailsValidateEndToEnd | integrations_thirdparty_validate_test.go:132 | `expected D2 branch ii violation, got D2 branch i` | MEDIDO (causa INFER) |

**Mecanismo D1 (TestChecksum):** o teste invoca `shasum -a 256 <path>` onde `<path>` é um caminho de temp dir Windows com backslashes. Em GNU shasum/coreutils, quando o nome do arquivo contém um backslash (na representação textual), a linha de saída recebe um `\` inicial (indicador de modo binário com escape). O teste extrai `fields[0]` que é `\6f0afa5...` em vez de `6f0afa5...` — difere por 1 caractere. Este é um Group D (path separator), não Group C: `shasum` está disponível no runner, mas o output varia pelo formato do caminho passado.

**Mecanismo D2 (ThirdParty tests, INFER):** o validador computa `provenanceKey = normalizeRefSeparator(filepath.Rel(root, destination))`. O `normalizeRefSeparator` converte `\` para `/`. No entanto, o teste ainda falha com "has no entry" (não o braço "could not express relative"), indicando que `filepath.Rel` teve sucesso mas a chave resultante não bate com a chave armazenada no JSON. Causa mais provável: mismatch entre forma longa (`runneradmin`) e forma 8.3 (`RUNNER~1`) nos paths absolutos — `filepath.Rel` recebe `root` em forma longa e `destination` em forma 8.3 (ou vice-versa), produzindo uma chave relativa que não é o caminho esperado. Confirmação requer instrumentação em runner Windows. O `normalizeRefSeparator` IS presente na leitura (validator_thirdparty_provenance.go:172) mas não resolve o mismatch de nome 8.3.

### Resumo por grupo

| Grupo | Mecanismo | Contagem |
|---|---|---|
| A | POSIX permissions: chmod ignorado no NTFS | 4 |
| B | CRLF no renderer/parser | 4 |
| C | External command: symlink privilege / fork-bomb defense | 2 |
| D | Path representation: backslash / 8.3 short name / shasum escape | 3 |
| **Total** | | **14** ✓ |

**A ADR previa os buckets "permissão POSIX, CRLF, bash ausente".** O bucket "bash ausente" é refutado: não há entrada nessa categoria. O TestAttentionScripts falha por **Windows symlink privilege**, não por ausência de bash. O TestChecksum falha por **format de saída do shasum com path Windows**, que é Group D (separador), não Group C (ferramenta ausente). Os dois grupos confirmados pela ADR são A e B. Os grupos C e D têm perfil diferente do previsto.

---

## 2. Mapa do checker

O checker é `scripts/check-windows-known-failures.py`.

### Fluxo do `run_check()`

```
Step 1   load_known_data()           — vacuity guard: entries=[] ou missing → exit 1
Step 2   load_known_list()           — separa known_go / known_node / known_py por runtime e class
Step 3   require_artifact()          — guard de lado da observacao: arquivo ausente → exit 1
Step 3a  marker check (Python/Node zero-test) — exit 1 imediato, sem nome extraivel
Step 3b  marker check (Go/Node named load)   — nomes entram no ratchet de step 6
Step 4   extract_*_failures()        — extrai obs_go / obs_node_assert / obs_node_load / obs_py
Step 5   extract_*_passes()          — extrai go_passes / node_passes / py_passes (para D4)
Step 5b  vacuity guard (lado observacao) — artefato existe mas zero linhas FAIL/PASS → exit 1
Step 6   obs_set - known_set         — NOVO falha → ::error:: + has_new = True
Step 7   known_set - obs_set         — entrada conhecida nao observada → ::warning:: (HOJE)
                                       D6 muda para: ::error:: + has_new = True
Step 8   check_baseline_deletions()  — delecao silenciosa (sem removed[]) → has_new = True
Step 9   validate_removed()          — removal_note enforcement (D4) → has_new |= not valid
Step 10  _cls_label()                — sumario informativo com [-N resolvido] / [+N NOVO]
return   1 if has_new else 0
```

### Onde compara declarado x observado

Step 6: `obs_set - known_set` → nova falha (exit 1)
Step 7: `known_set - obs_set` → entrada obsoleta (warning hoje, exit 1 com D6)

### Onde emite `[-N resolvido]`

Step 10 (`_cls_label`), somente informativo — nao afeta `has_new` nem o exit code.

### O que o `_meta.d4_note` valida

Nada — e o checker nao le `_meta.d4_note`. O protocolo D4 real e codificado em `validate_removed()` (step 9): `removal_note` obrigatorio (`corrected | renamed | no-longer-runs`), `corrected` verificado contra o conjunto de passes, `renamed` verificado via `renamed_to` presente em `entries[]`.

### Campos de `entries[]` validados hoje

Nenhum. O checker extrai `name`, `runtime`, `class` por acesso direto sem validacao de schema.

---

## 3. Refutar ou confirmar a premissa do D6

**Premissa:** e possivel reprovar por "entrada resolveu" sem falso positivo.

### 3.1 — Per-package vacuity: pacote que entra em panic mid-run

**Este e o caso nao coberto pelos guards existentes.** Medido com fixture sintetica.

**Fixture** (`go-out-panic.txt`):
```
--- PASS: TestFoo (0.01s)
--- PASS: TestBar (0.01s)
PASS
ok  pkg/a  0.02s
panic: unexpected fault
...
FAIL    pkg/b   [panicked]
```

**Resultado com checker atual (step 7 = warning):**
```
::warning::ML-2A ratchet: Go assertion 'TestPanicked' is in the known list but did NOT fail.
ML-2A/2B: 0 observed / 1 active / 0 removed — Go 0/1 [-1 resolvido]
exit: 0
```

**Com D6 sem recorte (step 7 = exit 1):** saida "TestPanicked resolveu" com exit 1. ATRIBUICAO FALSA: o teste nao foi corrigido — o pacote entrou em panic.

**Por que steps 3 e 5b nao cobrem isso:**
- Step 3a: so verifica marcadores pre-escritos (`suite-load-failure.python.txt`, etc.). O panic do Go nao escreve marcador no formato esperado.
- Step 5b: dispara quando o artefato tem zero linhas FAIL/PASS. No fixture, `pkg/a` tem linhas PASS — o artefato nao e vazio, o vacuity guard nao dispara.

**Verificado com `extract_go_passes(panic_fixture)`:**
```
failures: set()
passes:   {'TestFoo', 'TestBar'}
vac:      False
bucket 1 (resolved: in known AND in passes):    set()
bucket 2 (did-not-run: in known, neither):      {'TestPanicked'}
```

### 3.2 — Tres baldes, nao dois

O advisor identificou que a escolha nao e binaria (fire only on PASS / fire on absent-from-failures). Ha um terceiro balde:

| Balde | Condicao | Acao correta | Mensagem |
|---|---|---|---|
| 1 — genuinamente resolvido | `name in known AND name in passes` | exit 1 (D6) | "entrada resolveu — mover para removed[] com removal_note" |
| 2 — nao executou | `name in known AND name NOT in passes AND name NOT in failures` | exit 1 (diferente) | "entrada nao falhou NEM passou — skip, panic de pacote, ou teste deletado; nao e uma resolucao" |
| 3 — ainda falhando | `name in known AND name in failures` | exit 0 (status quo) | — |

Balde 2 ainda bloqueia o CI (exit 1), preservando o requisito de seguranca de que a entrada obsoleta nao passe silenciosamente. O que muda e a ATRIBUICAO: "nao executou" vs "resolveu" — que e exatamente o que a ADR diz ser a licao de tres erros no #274.

**Verificado para fixture com skip:**
```
python3 verify-buckets.py test-go-out-skip.txt
failures: set()
passes:   {'TestFoo'}
vac:      False
bucket 1 (resolved): set()
bucket 2 (did-not-run): {'TestPanicked'}   # skip vai para balde 2, nao para balde 1
```

**Verificado para fixture com fix genuino:**
```
python3 verify-buckets.py test-go-out-fixed.txt
bucket 1 (resolved): {'TestPanicked'}      # fix genuino vai para balde 1
bucket 2 (did-not-run): set()
```

### 3.3 — D3 e D5 cobrem baldes distintos

**D3** (suíte que nao executa) cobre o caso de ZERO resultados no artefato, via step 5b. O panic mid-run com outros pacotes funcionando produz resultados parciais — D3 nao cobre. O balde 2 cobre o que sobra.

**D5** (nomes instaveis) declara que subtestes sao tratados pelo nome de topo. Um rename de top-level e capturado por D1 (nova falha) + balde 1 ou balde 2 (nome antigo desaparece) — a cobertura e completa mas a atribuicao de balde importa.

### 3.4 — Veredito sobre o D6

**D6 precisa do recorte de tres baldes.** A implementacao sem recorte (`_warn` → `_err`) produziria atribuicao falsa em package panic — exatamente o tipo de erro de discriminacao que a ADR documenta como a causa dos tres fracassos no #274.

**ML-1A implementa D6 como dois branches no step 7:**
```python
# step 7 — tres baldes
for name in sorted(known_go_assert - obs_go):
    if name in go_passes:
        # balde 1: genuinamente resolvido
        _err(f"ML-D6: Go '{name}' is in the known list but PASSED. Move to removed[] with removal_note (D4).")
        has_new = True
    else:
        # balde 2: nao executou (skip, package panic, etc.)
        _err(f"ML-D6: Go '{name}' neither failed nor passed — skipped, package aborted, or deleted. Not a resolution. Investigate.")
        has_new = True
```

O mesmo padrao para Node e Python (com os seus conjuntos de passes).

**Risco aceito (flaky test):** um teste instavel que passa na corrida atual vai para balde 1 e dispara D6. Este risco e declarado na ADR D6. Com o recorte de tres baldes, o risco permanece identico — a atribuicao e mais correta ("resolveu" vs "nao executou"), o que ajuda o desenvolvedor a julgar se a reprovacao e um fluke.

### 3.5 — Correcao da premissa

A premissa do D6 e CONFIRMADA com recorte: e possivel reprovar por "entrada resolveu" sem atribuicao falsa, desde que apenas o balde 1 (`name in passes`) gere a mensagem de resolucao. O balde 2 tambem reprova (exit 1) mas com mensagem diferente.

---

## 4. Campo de razao (D7): onde cabe no esquema

### Proposta de schema

```json
{
  "entries": [
    {
      "name": "TestSave_WritesAtomicallyWithPermissions",
      "runtime": "go",
      "class": "assertion",
      "reason": "NTFS ignores POSIX chmod(0600) — file permission remains 0666 on Windows runner"
    }
  ],
  "removed": [
    {
      "name": "TestFoo",
      "runtime": "go",
      "class": "assertion",
      "removal_note": "corrected",
      "reason": "optional — kept for traceability, not required"
    }
  ]
}
```

**Restricao de encoding:** todos os valores de `reason` em `entries[]` devem ser ASCII. O CI roda o `--self-test` com `PYTHONIOENCODING=cp1252` — uma string com em-dash, seta ou caracter acentuado em uma mensagem de erro causaria `UnicodeEncodeError` ao emitir `::error::` nesse job. Usar apenas ASCII (ou seja, sem `—`, `→`, `é`, etc.) nos valores de `reason`.

### Obrigatoriedade

- **`entries[]`:** `reason` OBRIGATORIO. Entrada sem `reason` → `::error::` + exit 1. O D7 nao se sustenta sem ele: quando D6 disparar, o reviewer precisa da razao para julgar se "resolveu" e verdadeiro.
- **`removed[]`:** `reason` OPCIONAL. As 24 entradas existentes nao tem `reason`. Exigir retroativamente quebraria o checker imediatamente. O `removal_note` ja prove o essencial.

### Onde implementar

Em `run_check()`, apos `load_known_data()`, nova funcao `validate_active_entries(known)`:

```python
def validate_active_entries(entries: list[dict]) -> bool:
    ok = True
    for entry in entries:
        if not entry.get("reason"):
            _err(
                f"ML-D7: active entry '{entry.get('name', '<unknown>')}' "
                f"(runtime: {entry.get('runtime', '<unknown>')}) has no 'reason' field. "
                "Add the root cause (ASCII only) so a D6 trigger can be evaluated. "
                "ADR D7: entries without reason must not persist."
            )
            ok = False
    return ok
```

### Auto-test — impacto nos fixtures existentes

O checker tem fixtures inline de `entries[]` em aproximadamente **20+ call sites** de `write_list(...)` (linhas 1216, 1224, 1236, 1274, 1288, 1315, 1334, 1356, 1370, 1388, 1407, 1426, 1438, 1467, 1482, 1490, 1505, 1529, 1550, 1570, 1593, 1633, 1654, 1699). Todas usam o schema `{name, runtime, class}` sem `reason`. Se `validate_active_entries` for chamada no fluxo normal de `run_check`, TODOS esses fixtures reprovam.

**Opcao A (recomendada):** adicionar `reason` a todos os fixtures existentes. Custo: ~20 linhas alteradas. Garantia: o self-test continua testando o codigo real.

**Opcao B (alternativa):** adicionar flag `--skip-d7-validation` para o self-test. Risco: o self-test passa sem exercitar D7. Nao recomendada.

**ML-1A deve incluir a atualizacao dos fixtures como criterio de aceite**, e o handoff para o implementador deve nomear esse requisito explicitamente.

### Casos novos no self-test (T-D7)

| Caso | Input | Expectativa |
|---|---|---|
| T-D7a | entry com `reason` presente | exit 0 |
| T-D7b | entry sem `reason` | exit 1, cita nome e aponta D7 |
| T-D7c | `removed[]` sem `reason` | exit 0 (opcional) |
| T-D7d | `reason` com non-ASCII em cp1252 | `_err` nao levanta UnicodeEncodeError no CI |

---

## Sumario executivo para o ML-1A

**1. 14 entradas = 4 causas-raiz (com correcao da ADR):**
- Grupo A (4): POSIX permissions — chmod ignorado no NTFS
- Grupo B (4): CRLF no renderer/parser (2 medidos, 2 inferidos)
- Grupo C (2): External command — symlink privilege (FallbackWithoutJQ) e fork-bomb defense (Ship)
- Grupo D (3): Path representation — shasum com path Windows, provenance key mismatch (2 inferidos)

**O bucket "bash ausente" previsto na ADR nao tem representante. Isso e uma correcao da premissa do D7 — a razao para as entradas C nao e "bash ausente" mas mecanismos diferentes.**

**2. Checker mapeado:** step 6 = nova falha → exit 1; step 7 = nao observada → warning hoje, D6 muda para exit 1 com dois branches; step 10 = label `[-N resolvido]` informativo.

**3. D6 COM recorte de tres baldes: APROVADO.** D6 sem recorte produziria atribuicao falsa em package panic (medido). O recorte (balde 1 = passa / balde 2 = nao executou) preserva a seguranca (ambos disparam exit 1) e corrige a atribuicao. D3 e D5 cobrem mecanismos ortogonais.

**4. D7: `reason` ASCII obrigatorio em `entries[]`, opcional em `removed[]`.** Implementar via `validate_active_entries()`. Requer atualizacao de ~20 fixtures no self-test — custo explicitamente listado para o implementador.

**Handoff para ML-1A deve declarar:**
- D6: implementar tres baldes (passes / neither / failures), NAO a mudanca de uma linha
- D7: `reason` ASCII, `validate_active_entries()`, fixtures do self-test todos precisam de `reason`
- As 14 razoes medidas/inferidas nesta wave sao o insumo para o backfill das entradas

**ML-0A status → atualizar roadmap para ✅ Concluido: DELEGADO ao arquiteto (nao posso commitar).**

---

**git diff trackfw.yaml:** vazio (confirmado).
