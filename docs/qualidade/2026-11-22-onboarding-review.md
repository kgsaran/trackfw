# Parecer ML-4A — Revisão de Qualidade: Onboarding Verificável

> Data: 2026-11-22 | Revisor: hefesto-tf | Branch: `fix/onboarding-verificavel` | HEAD: `e6694c06`
> REQ: REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario

---

## Veredito

**APROVA com ressalvas**

- 0 achados bloqueantes
- 1 achado não bloqueante (R1)
- 1 observação de pré-existência (O1)

---

## Cobertura da Revisão

Escopo do `git diff origin/main...HEAD`:

| Categoria | Arquivos verificados |
|---|---|
| Código de produto | `internal/commands/init.go`, `internal/commands/guard.go`, `internal/generators/scaffold.go`, `internal/i18n/locales/en-US.json` |
| Testes novos | `internal/commands/init_flags_both_paths_test.go`, `internal/commands/quickstart_e2e_test.go` (extensão), `internal/generators/trackfw_config_merge_test.go` (extensão T9–T15) |
| Contrato | `docs/cli-parity.md` |
| Documentação | `README.md`, `site/guide/getting-started.md`, `site/guide/commands.md`, `site/guide/ai-agents.md`, `site/index.md`, `site/.vitepress/config.mts` |

---

## Resultado das Medições

```
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test ./internal/commands/ ./internal/generators/ ./internal/guardforms/ \
  -run "TestInitD1|TestInitD2|TestQuickStart|TestWriteTrackfwConfig|TestPresentTopLevelKeys|TestGenerateLefthookHook|TestLefthook" \
  -timeout 5m -count=1
```

| Pacote | rc |
|---|---|
| `internal/commands/` | 0 |
| `internal/generators/` | 0 |
| `internal/guardforms/` | 0 |

```
go build ./...
```

rc=0, sem warnings de compilação.

---

## Ponto 1 — Risco de Vazamento dos Pontos de Injeção

**Arquivos:** `internal/commands/init.go` (declaração de `initStdinIsTTY` e `initFormRunner`),
`internal/commands/init_flags_both_paths_test.go` (uso e restauração).

**Verificado:**

- `initStdinIsTTY` e `initFormRunner` são declarados como `var` em nível de pacote.
- Cada teste que os sobrescreve usa `t.Cleanup` para restaurar o valor original antes de sair.
- Nenhum teste em `init_flags_both_paths_test.go` chama `t.Parallel()`.
- A execução sequencial dentro de um mesmo pacote garante que a restauração ocorre antes do próximo teste começar.

**Conclusão:** sem risco de vazamento detectável. O padrão está consistente com os demais pontos de
injeção do projeto (`identityWizardRunner`, `promptInstallScopeRunner`).

---

## Ponto 2 — Coerência entre os Dois Detectores de Chave YAML

**Arquivos:**
- `internal/generators/scaffold.go:938` — `presentTopLevelKeys` (caminho yaml.Node)
- `internal/generators/scaffold.go:972` — `presentTopLevelKeysFallback` (caminho texto)
- `internal/generators/scaffold.go:2347` — `normalizeYAMLTopLevelKey`

**Comportamento observado:**

`presentTopLevelKeysFallback` extrai a chave na linha 988:
```go
keys[line[:idx]] = true   // scaffold.go:988
```

Não há remoção de aspas: `"pre-commit":` é armazenado como `"pre-commit"` (com aspas). Quando o
chamador depois testa `existingKeys["pre-commit"]` (sem aspas), o resultado é `false` e o bloco é
inserido em duplicata.

`normalizeYAMLTopLevelKey` (linha 2347–2362), usado por `lefthookValidatePresent` e
`generateLefthookHook`, remove as aspas corretamente.

O yaml.Node path (`presentTopLevelKeys`) não tem essa limitação: `yaml.Unmarshal` retorna a chave
sem aspas por construção.

**Combinação não testada:** YAML inválido + chave de seção entre aspas (ex.: arquivo
`lefthook.yml` corrompido com `"pre-commit":`). T14 cobre YAML inválido + chave sem aspas; T13
cobre yaml.Node + chave com aspas. A interseção fica descoberta.

**Ver R1 abaixo.**

---

## Ponto 3 — Fragilidade do Teste de Pinagem do Quick Start

**Arquivo:** `internal/commands/quickstart_e2e_test.go`

**Verificado:**

- `quickStartLines` é a lista única de autoridade (5 elementos) usada tanto na execução quanto na
  conferência dos documentos.
- `findQuickStartRun` exige bloco contíguo: 5 linhas consecutivas normalizadas que caiam dentro dos
  documentos. Não há correspondência parcial aceita.
- `normalizeDocLine` remove espaços e sufixo de comentário inline, tornando a conferência robusta a
  variações cosméticas no markdown.
- `failRecorder` permite executar os contra-braços sem propagar falha para `*testing.T` real,
  confirmando que os contra-braços não são vacuosos.
- `TestQuickStartDocsPinned` confere tanto `README.md` quanto `site/guide/getting-started.md`.

**Conclusão:** o mecanismo é sólido. A fragilidade legítima — edição de README ou do guia do site
quebra o teste — é intencional por D4 e declarada nas frases de reconciliação.

---

## Ponto 4 — Reconciliação de Frases nos Testes

**Arquivos:** `internal/commands/init_flags_both_paths_test.go` (cabeçalho linhas 7–19),
`internal/generators/trackfw_config_merge_test.go` (T9–T15),
`internal/commands/quickstart_e2e_test.go` (cabeçalho do bloco Quick Start).

**Verificado:** cada teste novo declara, em frase explícita de "Affirms:", a conclusão do ML que
ele afirma. Cruzamento com as seções de medição dos MLs:

| Teste | Frase declarada | Conclusão do ML que afirma |
|---|---|---|
| `TestInitD1_NonTTY_BrownfieldNewProject` | "D1: `--brownfield` grava governance_mode no caminho não-TTY" | ML-1A: `--brownfield` honra a flag nos dois caminhos |
| `TestInitD2_NonTTY_BrownfieldExistingYAML_GovernanceUnchanged` | "D2: com trackfw.yaml existente, governance_mode não é sobrescrita; aviso é emitido" | ML-1A: D2 impede rebaixamento em projeto existente |
| `TestInitD1_TTY_AIToolsFlag_SkipsWizard` | "D1: `--ai-tools` passado explicitamente pula o wizard no caminho TTY" | ML-1A: flag vence o wizard |
| T9 `TestPresentTopLevelKeys_P1P2P3` | "P1/P2/P3 do yaml.Node e do texto batem" | ML-1C: detector YAML correto nos dois caminhos |
| T10–T12 `TestWriteTrackfwConfig_D2a_*` | variações de grafias de chave → sem duplicata | ML-1C: D2a resolve merge de chave com aspa/espaço/BOM |
| T13 `TestGenerateLefthookHook_D2a_QuotedPreCommit` | "seção com aspa → trackfw-validate inserido, sem duplicata" | ML-1C: lefthook reconhece `"pre-commit":` |
| T14 `TestGenerateLefthookHook_D2a_InvalidYAMLFallback` | "YAML inválido + chave sem aspa → fallback insere sem duplicata" | ML-1C: fallback cobre YAML corrompido (chave sem aspa) |
| T15 `TestLefthookValidatePresent_D2a_QuotedKey` | "`lefthookValidatePresent` retorna true para `\"pre-commit\":`" | ML-1C: função de verificação reconhece chave com aspa |
| `TestQuickStartEndToEnd` | "5 comandos do Quick start terminam sem violação" | ML-3A: Quick start executável |
| `TestQuickStartDocsPinned` | "README e guia do site contêm o bloco de Quick start literal" | ML-3A: documentação pinada |

Todas as frases são consistentes com as conclusões dos MLs. Nenhuma contradição interna detectada.

---

## Ponto 5 — Contrato cli-parity.md vs. Comportamento

**Arquivo:** `docs/cli-parity.md` (nova seção `### trackfw init flag behaviour (D1 and D2, ADR-2026-11-22)`)

**Verificado:**

1. **Comportamento D1 (non-TTY + projeto novo):** o contrato descreve que `--brownfield` grava
   `governance_mode: lenient` e `lenient_until`. A implementação em `init.go` chama
   `applyBrownfieldOrWarn` → se `trackfw.yaml` ausente, retorna `false` → `cfg.BrownfieldMode` e
   `cfg.LenientUntil` são setados → `executeScaffoldAndInstall` escreve o arquivo. Consistente.

2. **Comportamento D2 (non-TTY + trackfw.yaml existente):** o contrato afirma que governance_mode
   não é escrito e um aviso é emitido para stderr/stdout. `applyBrownfieldOrWarn` imprime
   `i18n.T("init.brownfieldYamlExists")` em `cmd.OutOrStdout()` e retorna `true` (skipGovernance).
   Consistente.

3. **Texto do aviso:** o contrato cita `"governance keys not modified"`. O valor em
   `en-US.json` é `"⚠ governance keys not modified — edit trackfw.yaml directly to enable lenient mode"`.
   O contrato usa substring — verificável por `strings.Contains`. Consistente.

4. **Anotação de gate:** `<!-- trackfw-contract: gate=internal/commands/init_flags_both_paths_test.go -->`
   aponta para o arquivo que realmente contém os testes. Consistente.

5. **Locale mirror:** o contrato menciona `site/guide/commands.md` (inglês, raiz) e
   `site/pt/guide/commands.md` (pt-BR) como arquivos de espelho do site. Ambos existem no diff.
   Consistente.

---

## Achados

### R1 — Não bloqueante

**`presentTopLevelKeysFallback` não desaspa chaves YAML**

Arquivo: `internal/generators/scaffold.go:988`
```go
keys[line[:idx]] = true   // armazena "pre-commit" com aspas, não pre-commit
```

**Evidência:** `line[:idx]` para a linha `"pre-commit":` retorna a string `"pre-commit"` (seis
bytes, incluindo as aspas). A consulta posterior com `existingKeys["pre-commit"]` (sem aspas)
retorna `false`. O bloco `pre-commit:` é então inserido em duplicata no lefthook.yml.

**Escopo do defeito:** afeta apenas a combinação YAML inválido + chave de seção entre aspas no
lefthook.yml. Para `trackfw.yaml`, o yaml.Node path (`presentTopLevelKeys`) gerencia a detecção e
só recorre ao fallback se o parse falhar. Para lefthook.yml com YAML válido (caso normal), o
yaml.Node path também é usado. O fallback com chave entre aspas só é acionado por lefthook.yml
corrompido que ainda assim tem estrutura de chave entre aspas — cenário improvável em produção.

**Por que não é bloqueante:** T14 cobre o fallback com chave sem aspas, que é o caso mais frequente
de YAML inválido (tabs no lugar de espaços, entrada truncada). O risco residual é baixo e
circunscrito. Não há regressão em relação ao comportamento anterior ao D2a: antes do D2a, o fallback
era o único caminho; a nova situação é que o yaml.Node path agora cobre o caso normal.

**Correção sugerida (para ML futuro, mesma REQ):** em `presentTopLevelKeysFallback`, substituir
a linha 988 por:

```go
keys[normalizeYAMLTopLevelKey(line)] = true
```

Isso reutiliza a mesma normalização de `lefthookValidatePresent` e `generateLefthookHook`, tornando
os três caminhos coerentes. Acrescentar um sub-caso em T14 com `"pre-commit":` (aspa dupla) para
cobrir a combinação.

---

### O1 — Observação (pré-existência, fora do escopo desta REQ)

**Teste existente no pacote `./internal/generators/` modifica o `trackfw.yaml` do projeto**

**Evidência:** ao executar `go test ./internal/generators/ -timeout 5m -count=1` sem filtro de
`-run`, o arquivo `trackfw.yaml` na raiz do projeto é modificado (chaves de template como
`frontend:`, `backend:`, `rules:` são anexadas). O problema não aparece ao executar apenas os
novos testes (`-run "TestWriteTrackfwConfig|TestPresentTopLevelKeys|..."`).

A modificação vem de um teste pré-existente (anterior a esta branch) que chama
`writeTrackfwConfig` com `cwd = "."` ou equivalente, sem isolar a execução em diretório temporário.

**Por que não é bloqueante para esta REQ:** os testes novos desta branch usam `t.TempDir()` e não
tocam o `trackfw.yaml` do projeto. O defeito de isolamento é pré-existente.

**Encaminhamento:** reportar ao arquiteto como achado separado. A correção envolve identificar qual
teste usa `cwd = "."` ou `os.Getwd()` sem `t.TempDir()` e substituir por um diretório temporário
isolado.

---

## Resumo de Conformidade com as Decisões do ADR

| Decisão | Implementada | Testada | Contrato |
|---|---|---|---|
| D1 — flag honrada nos dois caminhos | Sim | Sim (TestInitD1_*) | Sim (cli-parity.md) |
| D2 — governance não sobrescrita em projeto existente | Sim | Sim (TestInitD2_*) | Sim (cli-parity.md) |
| D2a — merge reconhece grafias de chave | Sim | Sim (T10–T15) | Sim (cli-parity.md) |
| D3 — inglês na raiz do site | Sim (config.mts + reestruturação) | N/A (D6 define cobertura) | — |
| D4 — documentação pinada pelo teste | Sim (quickStartLines + findQuickStartRun) | Sim (TestQuickStartDocsPinned) | — |
| D5 — README contíguo | Sim (reestruturação README.md) | Sim (TestQuickStartDocsPinned) | — |
| D6 — site verificado por ML, sem job CI | Verificação por ML declarada no roadmap | — | — |

---

## Conclusão

A implementação está conforme com as decisões D1–D6 e D2a do ADR-2026-11-22. Os testes novos são
não vacuosos, têm frases de reconciliação corretas e não vazam estado entre si. O contrato em
`cli-parity.md` reflete o comportamento observado no código.

O único ponto de atenção (R1) é uma lacuna de cobertura no caminho texto do detector de chave YAML,
limitada a um cenário de baixa probabilidade em produção. Não bloqueia a aprovação.

A observação O1 é pré-existente e deve ser tratada em REQ separada (causa distinta: isolamento de
teste com cwd).
