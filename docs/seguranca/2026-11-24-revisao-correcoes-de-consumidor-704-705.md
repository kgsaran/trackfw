# Revisão de Qualidade — Correções de Consumidor #704 e #705

> Hefesto, 2026-11-24. ML-2A do ROADMAP-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md.

**VEREDITO: APROVA COM RESSALVAS**

Nenhum achado bloqueante. Um achado médio (AC do roadmap inconsistente com Wave 0) e três achados baixos.

---

## Escopo e Medições

Delta inspecionado: `git diff origin/main..HEAD -- internal/ docs/cli-parity.md` (commit d0839c69, branch `fix/teste-de-orcamento-de-alias-afirma-trackfw-yaml-deste-repositorio`).

Arquivos alterados pela fix:

- `internal/config/config_merge_key_test.go`
- `internal/validator/validator_gemini_cli_legacy.go`
- `internal/validator/validator_guard_hook_concordance_external_test.go`
- `docs/cli-parity.md`

Comandos de medição:

```
$ go test ./internal/config/ -count=1 -run 'TestAliasBomb_Budget_RealTrackfwYaml|TestAliasBomb_Budget_ConsumerFixture|TestAliasBomb_Budget_CounterArm' -v
=== RUN   TestAliasBomb_Budget_RealTrackfwYaml  --- PASS (0.00s)
=== RUN   TestAliasBomb_Budget_ConsumerFixture  --- PASS (0.00s)
=== RUN   TestAliasBomb_Budget_CounterArm       --- PASS (0.00s)
ok  github.com/kgsaran/trackfw/internal/config  0.273s

$ go test ./internal/validator/ -count=1 -run 'TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns' -v
=== RUN   TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns  --- PASS (0.04s)
ok  github.com/kgsaran/trackfw/internal/validator  0.176s

$ git status --porcelain
?? docs/seguranca/2026-11-24-revisao-correcoes-de-consumidor-704-705.md
```

(Único arquivo untracked é este relatório.)

---

## Regra de Reconciliação

### TestAliasBomb_Budget_RealTrackfwYaml (`config_merge_key_test.go:505`)

**Comentário de reconciliação** (L497-504): "this test afirma somente que parse() não retorna erro sobre o arquivo real — a propriedade que o nome do teste descreve (orçamento suficiente)."

**O que o teste faz:** inicializa `cfg := defaults()`, lê `../../trackfw.yaml`, chama `parse()`, afirma `parseErr == nil`.

**Bate com a conclusão do ML?** SIM. ML-1A ação 1 diz "não acrescente campos não vazios: defaults() já os preenche". O teste não afirma nenhum valor de campo.

**Passaria num consumidor com trackfw.yaml diferente?** SIM. `parse()` retorna erro somente quando o orçamento de nós é excedido (`config.go:500-502`); falhas de unmarshal retornam nil (`config.go:487-488`). Um YAML diferente mas válido e abaixo do orçamento passa. O `Load()` real captura YAMLs malformados via probe separado (L298).

**Falsificação efetiva:** o teste falharia se o real `trackfw.yaml` tivesse alias bomb ou YAML inválido, e falharia se o orçamento fosse configurado abaixo do número de nós do arquivo real. Essa é a propriedade declarada pelo nome do teste.

**Passaria com parse() completamente inerte?** SIM. Essa cobertura é delegada para ConsumerFixture.

**Código morto?** Nenhum. O comentário original "these values are known from the repo's trackfw.yaml" foi removido; o novo comentário é preciso.

---

### TestAliasBomb_Budget_ConsumerFixture (`config_merge_key_test.go:529`)

**Comentário de reconciliação** (L521-527): "este teste afirma que parse() lê e sobrescreve campos com os valores da fixture do consumidor."

**O que o teste faz:** cria fixture em `t.TempDir()` com `req_dir: docs/requisições` / `governance_mode: strict` / `roadmap_namespacing: by_agent` / `agents: [a, b, c]`; inicializa `var cfg ProjectConfig` (zero-value, não `defaults()`); afirma campos iguais aos valores da fixture.

**Bate com a conclusão do ML?** SIM. Com parse() inerte, `cfg.REQDir` fica `""` e a asserção `cfg.REQDir != "docs/requisições"` dispara. Este é o único dos três testes que falsifica parse() completamente.

**Decorativo?** NÃO.

---

### TestAliasBomb_Budget_CounterArm (`config_merge_key_test.go:586`)

**Comentário de reconciliação** (L580-585): "este contra-braço afirma que parse() NÃO preenche campos ausentes do yaml — sem a chave no arquivo, o campo fica vazio (zero-value de string/slice)."

**O que o teste faz:** inicializa `var cfg ProjectConfig` (zero-value); yaml mínimo (`wip_limit: 2`, sem as chaves testadas); chama `parse()`; afirma campos permanecem em zero-value.

**Bate com a conclusão do ML?** SIM. A propriedade afirmada ("parse() não chama defaults() internamente") está correta: `config.go:483` mostra que `parse()` não chama `defaults()`. Se `parse()` chamasse `defaults()` internamente, `cfg.REQDir` ficaria `"docs/req"` e a asserção `cfg.REQDir != ""` dispararia. Essa é a falsificação efetiva do CounterArm.

**Passaria com parse() completamente inerte?** SIM, trivialmente (zero-value permanece). Dependência de falsificação: ConsumerFixture captura esse caso; o Counter-Arm cobre especificamente a regressão de parse() chamando defaults() internamente, que é o vetor documentado no Wave 0.

---

### TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns (`validator_guard_hook_concordance_external_test.go:1119`)

**Comentário de reconciliação** (L1117-1118): "o remédio sugerido inclui --scope project para garantir que a instalação seja de projeto, não global."

**O que o teste faz:** monta diretório com `.gemini/settings.json` (com guard) mas sem `.agents/hooks.json`; chama `Validate`; afirma que nenhum violation tem `geminiLegacyMarker`; afirma que o warning encontrado contém `"--targets antigravity --scope project"`.

**Bate com a conclusão do ML?** SIM. Com a mensagem antiga (`--targets antigravity`), `strings.Contains(msg, "--targets antigravity --scope project")` é false e o `t.Errorf` dispara.

**O remédio realmente silencia a regra?** Cadeia verificada por leitura e por teste:

| Hop | Verificação | Evidência |
|---|---|---|
| `--scope project` aceito pelo CLI | `integrations_flags.go:556`: `"project"` é valor válido | leitura |
| `antigravity` tem superfície de projeto no catálogo | `catalog_test.go:77`: `antigravity / project / ".agents/agents/..."` declarado | leitura |
| `agents install --targets antigravity --scope project` executa sem erro | `integrations_scope_project_writes_test.go:467-470`: `cmd1.Execute()` sem fatal | leitura de teste |
| L286: scope project → `InjectHooksForTools(["antigravity"], root)` | `integrations_flags.go:287` (bloco L286) | leitura |
| `InjectHooksForTools` sem gate de scope para `antigravity` | `hooks.go:151-158`: itera `cliDetectors`; `antigravity` em cliDetectors:76 | leitura |
| `InjectAntigravityHooks` → `.agents/hooks.json` com guard | `agentfiles.go:2084`: `antigravityInjectCredential` escreve credential guard | leitura |
| `.agents/hooks.json` escrito pelo gerador silencia a regra | `TestGeminiCliLegacy_BothPresent_Silent` chama `InjectAntigravityHooks(dir)` real (L1173) e regra fica silenciosa | teste passando |

**Residual declarado no Wave 0 §4.3 (blast-radius de --items):** o remédio sem `--items` instala os 12 agentes do catálogo e registra `agents:` no `trackfw.yaml` do consumidor. ML-3A (que introduziria hard-error para projetos com `agents:` em flat mode) ainda não aterrisou. O remédio é funcional; o custo secundário de `--items` ausente é o residual aceito pelo Wave 0.

---

## Achados por Severidade

### M1 — MÉDIO: ACs consolidados do roadmap contradizem o Wave 0

**Arquivo:linhas:** `docs/roadmaps/wip/ROADMAP-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md:18-19`

**Texto atual:**
```
- [ ] O teste afirma só `parse()` sem erro e campos obrigatórios não vazios (issue #704)
- [ ] Braço com yaml de consumidor passa; contra-braço com config vazia reprova
```

**Problemas:**
- L18 "campos obrigatórios não vazios": o Wave 0 mediu que essa asserção é vacuosa — `defaults()` preenche os campos antes de `parse()`. O AC da REQ (L26) já diz explicitamente "Sem 'campos não vazios', que `defaults()` torna vácuo (Wave 0)". O ML-1A entregou assertivas de valor de fixture (ConsumerFixture) e zero-value sem defaults (CounterArm), não "campos não vazios".
- L19 "contra-braço com config vazia reprova": o CounterArm entregue passa (afirma zero-value), não reprova. A REQ L29 diz "deixa os campos vazios", o que é o comportamento correto — o AC do roadmap contradiz a REQ e o código entregue.

**Consequência:** se ML-2B marcar esses ACs como atendidos com o texto atual, o roadmap fica com AC satisfeito por critério que o próprio projeto declarou vacuoso (L18) e por critério que contradiz o comportamento correto (L19). É o padrão "medimos X, declaramos Y" da Regra de Reconciliação.

**Texto correto:**
```
- [ ] O teste afirma só `parse()` sem erro sobre o arquivo real (RealTrackfwYaml);
      ConsumerFixture afirma valores da fixture do consumidor (não valores deste repositório);
      CounterArm afirma zero-value quando as chaves estão ausentes do yaml.
- [ ] Braço: ConsumerFixture passa com yaml de consumidor; CounterArm passa (fields
      permanecem zero-value sem defaults()).
```

**Ação:** o arquiteto atualiza os ACs L18-19 do roadmap antes de mover para done. Este achado **bloqueia** o movimento do roadmap para done; não bloqueia o merge do PR (o código está correto).

---

### L1 — BAIXO: comentário "braço" de ConsumerFixture descreve ADRDirs como neutro quando falharia

**Arquivo:linha:** `internal/config/config_merge_key_test.go:576`

**Texto atual:**
```go
// Agents check not present in original, but ADRDirs[0] "docs/adr" check would not apply.
```

**O que realmente aconteceria:** a fixture tem sem `adr_dirs`; `cfg` parte de zero-value (ADRDirs nil). A guarda original era `len(cfg.ADRDirs) == 0 || cfg.ADRDirs[0] != "docs/adr"`. Com `ADRDirs` nil: `len == 0` → condição true → `t.Errorf` dispararia. O check "aplicaria" e falharia, como os outros três campos listados acima.

**Phrasing correto:** "ADRDirs[0] check would also FAIL: fixture has no adr_dirs, so ADRDirs is nil, len == 0, t.Errorf fires."

Este é um comentário de código, não uma asserção executada. A REQ AC L27-28 ("as asserções antigas reprovam nele") é satisfeita por entailment: `cfg.REQDir == "docs/requisições"` implica `cfg.REQDir != "docs/req"`. Nenhuma das asserções dos campos listados é re-executada como verificação explícita — o braço é por entailment das asserções de L556-570.

**Impacto:** só o comentário está errado; o comportamento do teste não é afetado. Não bloqueia.

---

### L2 — BAIXO: TestAliasBomb_Budget_CounterArm não documenta dependência de falsificação

**Arquivo:linha:** `internal/config/config_merge_key_test.go:586`

**Situação:** o Counter-Arm passaria com parse() completamente inerte (zero-value permanece sem ler). Sua falsificação do caso "parse() inerte" depende de ConsumerFixture estar presente. O comentário de reconciliação não documenta essa dependência.

**Risco:** se ConsumerFixture for removido em refatoração futura, o CounterArm passa a ser vacuoso para o caso "parse() inerte" sem nenhum aviso.

**Sugestão:** adicionar ao comentário: "Dependência: se ConsumerFixture for removido, este teste não falsifica mais parse() inerte — only the defaults()-call regression is covered." Não bloqueia.

---

### L3 — BAIXO: doc drift em ADR-2026-11-18

**Arquivo:linha:** `docs/adr/ADR-2026-11-18-guard-de-credencial-e-de-git-branch-no-google-antigravity-e-gemini-cli-como-legado.md:77`

**Texto atual:**
```
aponta `trackfw agents install --targets antigravity`. O `doctor` repete o aviso.
```

Sem `--scope project`. O ADR descreve o comportamento do validator antes do fix do #705. Após a correção, a mensagem emitida contém `--scope project`; a linha do ADR apresenta o comando sem scope como orientação vigente.

**Outros arquivos com o literal sem `--scope project`:** o roadmap de backlog (`ROADMAP-2026-11-17-...md:164`, em `done/`) é AC histórico de roadmap fechado, não orientação. Os documentos de governança do #705 (`REQ-...md:18`, `wave0:86`) usam o literal para descrever o defeito — context correto, não drift. Apenas ADR:77 apresenta o literal sem scope como orientação.

**Ação:** o arquiteto acrescenta uma nota ao ADR ou atualiza a linha após merge do PR. Não bloqueia.

---

## Checklist do Handoff

| Item | Resultado |
|---|---|
| Frase de reconciliação bate com o que cada teste afirma | SIM para todos os quatro |
| RealTrackfwYaml falsifica budget exceeded sobre arquivo real | SIM — falharia se trackfw.yaml tivesse bomb ou se orçamento fosse configurado abaixo do nó-count real |
| CounterArm falsifica defaults() chamado internamente por parse() | SIM — `cfg.REQDir != ""` dispararia se parse() chamasse defaults(); não falsifica parse() inerte |
| ConsumerFixture falsifica parse() completamente inerte | SIM — campos ficam em zero-value ("") se parse() não lê |
| RealTrackfwYaml passa em consumidor com trackfw.yaml diferente | SIM — parse() retorna erro só por budget exceeded; yaml diferente/válido/abaixo do orçamento passa |
| Remédio do #705 resolve a condição | SIM — cadeia de 7 hops verificada (leitura + BothPresent_Silent com generator real) |
| Residual blast-radius --items omitido | Declarado em Wave 0 §4.3; ML-3A não aterrisou; aceito |
| Outra cópia da mensagem antiga sem `--scope project` em internal/ | NÃO encontrada |
| Outra cópia em scripts/ ou site/ | NÃO encontrada |
| ADR com literal antigo como orientação vigente | ADR-2026-11-18:77 — ver L3 |
| defaults() torna "não vazio" vácuo | Mitigado — ConsumerFixture afirma valor da fixture; CounterArm usa zero-value. AC do roadmap tem texto desatualizado — ver M1 |
| Comentário mentindo encontrado | config_merge_key_test.go:576 — ver L1 |
| git status --porcelain | `?? docs/seguranca/2026-11-24-revisao-correcoes-de-consumidor-704-705.md` (somente este arquivo) |
