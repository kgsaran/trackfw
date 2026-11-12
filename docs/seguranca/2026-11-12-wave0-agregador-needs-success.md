# Wave 0 — Agregador de CI exige `success` dos `needs`

> Data: 2026-11-12 | ML-0A | Autor: hades-tf
> REQ: REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md
> ADR: ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md

---

## 1. Inventário (AC1)

### 1.1 Metodologia e contagem dupla

Dois instrumentos independentes varreram os 9 arquivos `.yml` em `.github/workflows/`
(resultado de `find .github -name '*.yml' -o -name '*.yaml'`; não há `.yaml` no diretório).

**Caminho A — grep por padrão** (captura comentários; sobrestima código executável):

```bash
grep -n "needs\..*\.result\|contains(needs\|success()\|failure()\|cancelled()\|always()\|_RESULT"
```

77 ocorrências totais. Reduzidas a **6 linhas de código executável** após excluir comentários
(`# ...`) e strings de `echo`: quality.yml:975, 977, 1010, 1356; release.yml:433, 443. A linha
release.yml:389 começa com `  #` — é comentário; não entra na contagem.

**Caminho B — walker YAML com `python3 yaml.safe_load`** (captura estrutura; resolve `if:` de job
e step; não detecta `if:` de nível de job via varredura de steps):
- 3 jobs identificados com consumo de `needs.<job>.result` em `steps[].env` ou `steps[].run`:
  `shim-byte-identity-gate` (quality.yml:975, 977), `parity` (quality.yml:1010),
  `verify-channels` (release.yml:433, 443).
- 1 job com `needs.classify.result` em `job.if` (quality.yml:1354–1357: `shim-byte-identity`) —
  capturado na saída `if=` do walker mas não em `needs_result_in_steps`; gap documentado.
- 7 jobs com `needs:` e sem `if:` ou com `if: github.event_name == 'push'` (ver §1.3): todos
  têm `success()` implícito do GitHub Actions (quality.yml:480–487 documenta o comportamento).

**Reconciliação:**

| Linha | O que contém | Grep conta? | Walker conta? |
|---|---|---|---|
| quality.yml:975 | `CLASSIFY_RESULT: ${{ needs.classify.result }}` em `steps[].env` | Sim | Sim (step env) |
| quality.yml:977 | `SHIM_RESULT: ${{ needs.shim-byte-identity.result }}` em `steps[].env` | Sim | Sim (step env) |
| quality.yml:1010 | `if: contains(needs.*.result, ...)` em `steps[].if` | Sim | Sim (step env/if) |
| quality.yml:1356 | `needs.classify.result == 'success'` em `job.if` | Sim | Não (gap de walker) |
| release.yml:389 | comentário `# The needs.*.result context...` | Sim | Não |
| release.yml:433 | `PUBLISH_NPM_RESULT: ${{ needs.publish-npm-shim.result }}` em `steps[].env` | Sim | Sim |
| release.yml:443 | `PUBLISH_PYPI_RESULT: ${{ needs.publish-pypi.result }}` em `steps[].env` | Sim | Sim |

Ambos os caminhos identificam o mesmo único sítio de lista de proibidos (quality.yml:1010). O grep
captura 1 comentário a mais (release.yml:389) e ambos capturam a linha 1356 mas por mecanismos
diferentes.

---

### 1.2 Tabela de consumidores

| Arquivo | Linha | Forma | Job | Decisão de veredito | Classificação |
|---|---|---|---|---|---|
| `quality.yml` | 1010 | `contains(needs.*.result, 'failure') \|\| contains(needs.*.result, 'cancelled') \|\| contains(needs.*.result, 'skipped')` | `parity` (`if: always()`) | Step-if do primeiro passo: se disparar, `exit 1`; se **não** disparar (ex.: `abandoned`), passo é pulado e os seguintes rodam | **PROIBIDOS — DEFEITO** |
| `quality.yml` | 975 | `CLASSIFY_RESULT: ${{ needs.classify.result }}` | `shim-byte-identity-gate` (`if: always()`) | Shell: `if [[ "$CLASSIFY_RESULT" != "success" ]]; then exit 1` | PERMITIDOS (correto) |
| `quality.yml` | 977 | `SHIM_RESULT: ${{ needs.shim-byte-identity.result }}` | `shim-byte-identity-gate` (`if: always()`) | Shell: `if [[ "$SHIM_RESULT" == "success" ]]; then exit 0; fi; exit 1` | PERMITIDOS (correto) |
| `quality.yml` | 1354–1357 | `!cancelled() && !(needs.classify.result == 'success' && ...)` | `shim-byte-identity` (NÃO `if: always()`) | Decide se o job RODA — não é gate de merge diretamente | PERMITIDOS (correto) |
| `release.yml` | 433 | `PUBLISH_NPM_RESULT: ${{ needs.publish-npm-shim.result }}`| `verify-channels` (tag push; fora de R) | `verify-npm-channels.sh:41`: `if [ -n "$PUBLISH_NPM_RESULT" ] && [ "$PUBLISH_NPM_RESULT" != "success" ]` → FAIL | Pós-tag, não é merge gate; `abandoned` reprovaria (ver §1.4) |
| `release.yml` | 443 | `PUBLISH_PYPI_RESULT: ${{ needs.publish-pypi.result }}` | `verify-channels` (tag push; fora de R) | `verify-pypi-channel.py:84`: `if publish_result and publish_result != "success"` → FAIL | Pós-tag, não é merge gate; `abandoned` reprovaria (ver §1.4) |

---

### 1.3 Jobs com `if: always()` no nível de job — o que decide o veredito

| Job | Arquivo | Triggers do workflow | Em R? | `needs` | `continue-on-error` (job) | Veredito decidido por |
|---|---|---|---|---|---|---|
| `parity` | `quality.yml` | push, pull\_request | **sim** | `[parity-falsify-shard, parity-other-gates, check-required-checks]` | Não | Passo 1: `contains(needs.*.result, lista de proibidos)` (lista de proibidos); passos seguintes: cobertura de artefatos de shard |
| `shim-byte-identity-gate` | `quality.yml` | push, pull\_request | **sim** | `[classify, shim-byte-identity]` | Não | `CLASSIFY_RESULT != "success"` e `SHIM_RESULT == "success"` (lista de permitidos) |
| `verify-channels` | `release.yml` | push (tag), workflow\_dispatch | Não — só release | `[publish-npm-shim, publish-pypi]` | Não | `PUBLISH_NPM_RESULT != "success"` e `PUBLISH_PYPI_RESULT != "success"` (lista de permitidos) |
| `censo` | `windows-census.yml` | **workflow\_dispatch apenas** | Não | `falsificacao` | **Sim** (`continue-on-error: true`) | Não consome `needs.*.result`; diagnóstico puro; nunca bloqueia merge |
| `apuracao` | `windows-census.yml` | **workflow\_dispatch apenas** | Não | `censo` | Não | Não consome `needs.*.result`; diagnóstico puro; nunca bloqueia merge |

`windows-probe.yml` tem `if: always()` **somente em steps**, nunca em jobs; triggers `workflow_dispatch`; sem `needs:` entre jobs. Classificado: diagnóstico sem efeito em merge.

**`success()` implícito (7 jobs com `needs:` sem `if:` ou com `if:` que inclui `success()` implícito):**
O GitHub Actions prepende `success()` a qualquer `if:` sem função de status (quality.yml:480–487).
Resultado: um job com `needs: [A]` e sem `if:`, ou com `if: github.event_name == 'push'`, só roda
quando `A` terminou com `success`. Se `A` for pulado (`skipped`), o job dependente também é pulado —
`needs.<job>.result` fica em `skipped`, valor que a guarda proibida da linha 1010 captura.
Os 7 jobs com esse padrão em `.github/workflows/`:

| Job | Arquivo | `needs` |
|---|---|---|
| `parity-falsify-shard` | quality.yml | `[go, package-smoke, windows-integrations-resolve]` |
| `parity-other-gates` | quality.yml | `[go, package-smoke, windows-integrations-resolve]` |
| `deploy` | deploy-docs.yml | `[build]` |
| `release` | release.yml | `[quality]` |
| `publish-npm-platforms` | release.yml | `[release]` |
| `publish-npm-shim` | release.yml | `[release, publish-npm-platforms]` |
| `publish-pypi` | release.yml | `[release]` |

Nenhum dos `needs` de `parity` ou `shim-byte-identity-gate` tem `continue-on-error: true` no nível
de job (medido via YAML walker). Ver §4 (R3) para a questão sem resposta sobre o que
`needs.<job>.result` relata quando o job tem job-level `continue-on-error: true`.

**Step-level `always()` em quality.yml:** step-level `always()` NUNCA altera o campo
`result` de um job visto pelo downstream `needs`. Apenas job-level `continue-on-error: true`
pode fazer isso. Nenhum dos jobs em `needs` de `parity` tem step-level `continue-on-error`
(medido via walker). Os steps com `if: always()` em quality.yml são:
- `windows-full-suites` (linhas 265, 321, 491, 565, 625, 650, 722): artifact/sonda steps; não
  alteram resultado de job.
- `parity-falsify-shard` (linha 785): upload de artefatos de shard; `if: always()` correto aqui
  — sobe o log mesmo se o shard reprovou.

---

### 1.4 Classificação dos scripts de release

`verify-npm-channels.sh:41` e `verify-pypi-channel.py:83–84` usam a verificação
`if [ -n "$VAR" ] && [ "$VAR" != "success" ]` (sh) e `if publish_result and publish_result != "success"` (Python).

Com `abandoned` como valor: `-n "abandoned"` é verdadeiro e `"abandoned" != "success"` é verdadeiro
→ ambos os scripts **reprovam corretamente**.

String vazia (`VAR=""`): `[ -n "" ]` é falso → o bloco de saída antecipada não dispara; o script
continua para a verificação positiva no registro npm/PyPI. Isso só ocorre se o job for removido do
array `needs:` do YAML sem atualizar o script — hipotético, não um defeito atual.

Esses dois scripts não são gates de merge (R-membership: não). Registrados para completar o
inventário; fora do escopo de correção do ML-1A.

---

### 1.5 Lista final de sítios para o ML-1A

**Um único sítio a corrigir:**

```
.github/workflows/quality.yml : 1010
```

Passo `Nenhum job de needs pode ter falhado, sido cancelado ou pulado` do job `parity` —
`if: contains(needs.*.result, 'failure') || contains(needs.*.result, 'cancelled') || contains(needs.*.result, 'skipped')`
deve ser substituído pela chamada a `scripts/check-needs-success.sh` com
`env: NEEDS_JSON: ${{ toJSON(needs) }}`.

Pelo ADR D3 e pela Regra Dura de Causa Raiz: os scripts de release (`verify-npm-channels.sh`,
`verify-pypi-channel.py`) já usam lista de permitidos para valores preenchidos e estão fora do
conjunto R; não entram no ML-1A.

**ADR D4 — gate estático novo:** um único sítio da classe foi encontrado. D4 não é reaberto.

---

## 2. Segunda linha de defesa do `parity`

**Pergunta**: se um `needs` do `parity` chegasse com resultado `abandoned` (ou qualquer valor que
não disparasse o passo de guarda da linha 1010), a segunda linha — a guarda de conjunto por
artefato — reprovaria?

**Resposta depende do `needs`:**

### Para `parity-falsify-shard` (matriz de 4 shards): SIM na primeira tentativa

Cada shard, ao concluir (mesmo em falha), executa o passo `Upload dos rótulos emitidos por este shard`
com `if: always()` (quality.yml:785), que envia `falsify-shard-out/` como artefato. Se o job inteiro
é `abandoned`, nenhum step roda — incluindo o de upload — e nenhum artefato é enviado.

O job `parity` então executa `check-falsify-shard-coverage.sh` (linha 1031), que verifica:

```
# check-falsify-shard-coverage.sh:46
if [[ ! -d "$ARTIFACTS_DIR" ]]; then exit 1; fi    # diretório ausente

# check-falsify-shard-coverage.sh:83
if [[ ! -f "$actual" || ! -f "$rc_file" ]]; then   # arquivos do shard ausentes
  fail "shard-coverage/shard_${idx}" "artefato ausente ..."
```

Se o shard 0 fosse `abandoned`, `shard_0.actual` e `shard_0.rc` não existiriam → `fail` na linha 83
→ script sai com `rc=1` → passo do `parity` falha → job `parity` falha → **check reprovaria na
primeira tentativa**.

**Risco em re-execução (não medido):** se `actions/download-artifact@v4` conseguir baixar artefatos
de uma tentativa anterior do mesmo `run_id` (comportamento dependente da versão da action e da
configuração do runner), os arquivos `shard_0.actual` e `shard_0.rc` estariam presentes com
`rc=0` da tentativa anterior. O `check-falsify-shard-coverage.sh` encontraria esses artefatos e
os aceitaria como evidência válida, aprovando sobre evidência do passado. Esta direção é
não verificada — ver §4 R6.

### Para `parity-other-gates` ou `check-required-checks`: NÃO reprovaria

Esses dois jobs não produzem artefatos. O `parity` não verifica nenhuma evidência positiva deles
além do passo de guarda da linha 1010. Se `parity-other-gates` ou `check-required-checks` fosse
`abandoned` e a guarda não disparasse, o `parity` concluiria verde sem verificação nenhuma desses
jobs.

**Gravidade:** os ~45 gates de `parity-rest` (incluindo `check-required-status-checks.py` e os
gates de paridade de scripts) passariam sem execução se `parity-other-gates` fosse `abandoned`. Este
é o vetor de maior impacto: a segunda linha de defesa não existe para esses dois jobs.

---

## 3. Threat model do script `scripts/check-needs-success.sh` (ADR D2)

### 3.1 Escolha de interpretador: `python3` em vez de `jq`

**Medições que justificam a escolha:**
- `actions/setup-python@v7` está declarado no job `parity` (quality.yml:1015–1017); `python3` via
  setup-python está disponível nos passos que vierem depois.
- `check-falsify-shard-coverage.sh`, já presente no mesmo job `parity`, resolve `python3` via
  `resolve_py_bin` (linha ~55 do script). Mesmo mecanismo pode ser reutilizado.
- `/bin/bash` no macOS local é 3.2.57 (`/bin/bash --version`); `python3` local é 3.15.2 (medido).
- `grep -rl '\bjq\b' scripts/` retorna 7 arquivos onde `jq` aparece, mas como token em tabelas de
  padrões (`check-unguarded-capture-rc.sh:406/413`) ou em scripts de recalibração separados — não
  é uma dependência instalada explicitamente em nenhum step de parity. `python3` é.

**Ordem dos passos no job `parity`:** o script lê `NEEDS_JSON` passado via `env:` no step, cujo
valor `${{ toJSON(needs) }}` é resolvido pelo runtime do GitHub Actions antes de qualquer step rodar.
Portanto checkout e setup-python podem (e devem) vir antes do script sem comprometer a semântica.

### 3.2 Vetores de aprovação sem verificação (false negative)

| # | Vetor | Comportamento esperado sem guarda | Braço de self-test (nome, entrada, rc esperado) |
|---|---|---|---|
| V1 | `NEEDS_JSON` não definida | `KeyError` ou substituição por vazio depende da implementação | `unset`: rodar o script sem a variável → rc≠0 |
| V2 | `NEEDS_JSON=""` (string vazia) | `json.loads("")` lança `ValueError` | `empty-string`: `NEEDS_JSON="" script` → rc≠0 |
| V3 | `NEEDS_JSON="{}"` (objeto vazio) | Script que só percorre chaves sem verificar contagem mínima aprovaria | `empty-object`: `NEEDS_JSON='{}' script` → rc≠0 |
| V4 | `NEEDS_JSON='[]'` (array em vez de objeto) | Iteração por chaves de lista retorna strings de índice | `array-toplevel`: rc≠0 |
| V5 | `NEEDS_JSON='"success"'` (string no top-level) | `for job in json_obj` iteraria sobre chars | `string-toplevel`: rc≠0 |
| V6 | `NEEDS_JSON='{"a":{}}'` (job sem chave `result`) | `.get("result")` → `None`; `None != "success"` é True — reprovaria corretamente. Mas `.get("result", "success")` aprovaria indevidamente | `missing-result-key`: rc≠0 |
| V7 | `{"a":{"result":null}}` | `null` → `None`; `None == "success"` é False — correto; verificar que não há coerção | `null-result`: rc≠0 |
| V8 | `{"a":{"result":"Success"}}` | Comparação com `.lower()` aprovaria indevidamente | `uppercase-success`: rc≠0 |
| V9 | `{"a":{"result":"success "}}` (espaço à direita) | Comparação com `.strip()` aprovaria indevidamente | `trailing-space`: rc≠0 |
| V10 | JSON inválido (`not-json`, `{a:b}`) | `json.JSONDecodeError` → deve reprovar | `invalid-json`: rc≠0 |
| V11 | Um job `success` + um job `abandoned` (misto) | Bug "any success" aprovaria; a lógica correta é ALL `success` | `mixed-success-abandoned`: rc≠0 |
| V12 | `outputs` com `\n::error::forged` | Script nunca deveria ecoar `outputs`; se ecoar, anotação de CI pode ser injetada | `outputs-injection`: rc=0 (aprova), **sem** linha `::error::forged` em stdout+stderr |
| V13 | Chave do job com `\n::warning::forged` | AC2 manda imprimir o nome do job reprovado; se a chave conter `%0A` ou `\n`, a saída ecoaria uma linha de workflow command | `jobname-injection-newline`: rc≠0 (reprova o job), sem linha `::warning::forged` em stdout+stderr |
| V14 | Chave do job com `\r` | CRLF pode fragmentar a linha de output do script | `jobname-injection-cr`: rc≠0, sem `::` extra |
| V15 | PATH sem `python3` disponível | `command not found`, rc=127 — fail-closed | `no-interpreter`: `PATH=/dev/null script` → rc≠0, mensagem que nomeia o interpretador ausente; medido localmente: `/bin/bash -c 'PATH=/dev/null python3 -c "print(1)"' → rc=127` |
| V16 | Formato compacto (JSON em uma linha, sem espaços) | `json.loads` aceita; verificar que o parsing não depende de indentação | `compact-json`: `{"a":{"result":"success"}}` → rc=0 |

**Nota sobre V13/V14:** os IDs de job do GitHub Actions são restritos por sintaxe a
`[a-zA-Z_][a-zA-Z0-9_-]*` (verificado: todos os IDs de quality.yml passam nesse regex). Em
produção, a chave do JSON não contém `\n` nem `\r`. O script deve escapar mesmo assim porque o
`--self-test` alimenta JSON arbitrário. O esquema de escape recomendado para workflow commands do
GitHub é `%`, `\r`, `\n` → `%25`, `%0D`, `%0A` (alinhado com `actions/toolkit` `escapeData`).
Este mapeamento não foi testado na implementação real — declare como requisito para ML-1A.

**V14 (encolhimento de `needs:`):** se o array `needs:` do job `parity` for reduzido sem atualizar
o script, o JSON terá menos jobs do que o esperado e o script aprovará porque todos os presentes
são `success`. Ver §4 R5.

### 3.3 Vetores de reprovação do legítimo (false positive)

| # | Vetor | Avaliação |
|---|---|---|
| FP1 | GitHub introduce novo valor benigno (ex.: `"neutral"`) | Reprovaria — aceito pela ADR (Consequences), mesmo padrão do `shim-byte-identity-gate` |
| FP2 | `actions/setup-python` não instalado antes do passo | Race de ordem; a ordem declarada (checkout → setup-python → script) evita |
| FP3 | `toJSON(needs)` retorna `{}` ou `null` num contexto sem `needs` | `{}` ou `null`, não medido — os braços V3 e V7 cobrem os dois casos; job sem `needs:` não usaria o script |

### 3.4 Braços de self-test obrigatórios

O `--self-test` deve rodar o script como subprocesso com `NEEDS_JSON` definida via ambiente e
verificar o `rc` retornado. Testar contra o próprio script como subprocesso garante que se testa
o caminho de código real, não um proxy isolado do primitivo compartilhado.

Braços mínimos (nomes a usar no output do self-test):

| # | Nome | Entrada (`NEEDS_JSON`) | rc esperado |
|---|---|---|---|
| 1 | `all-success-1job` | `{"j":{"result":"success","outputs":{}}}` | 0 |
| 2 | `all-success-3jobs` | 3 jobs, formato multi-linha real com indentação, todos `success` | 0 |
| 3 | `compact-json` | `{"a":{"result":"success"}}` | 0 |
| 4 | `abandoned` | `{"a":{"result":"abandoned"}}` | ≠0 |
| 5 | `failure` | `{"a":{"result":"failure"}}` | ≠0 |
| 6 | `cancelled` | `{"a":{"result":"cancelled"}}` | ≠0 |
| 7 | `skipped` | `{"a":{"result":"skipped"}}` | ≠0 |
| 8 | `invented-value` | `{"a":{"result":"pending"}}` | ≠0 |
| 9 | `empty-object` | `{}` | ≠0 |
| 10 | `empty-string` | `""` | ≠0 |
| 11 | `invalid-json` | `not-json` | ≠0 |
| 12 | `null-toplevel` | `null` | ≠0 |
| 13 | `array-toplevel` | `[]` | ≠0 |
| 14 | `string-toplevel` | `"success"` | ≠0 |
| 15 | `missing-result-key` | `{"a":{}}` | ≠0 |
| 16 | `null-result` | `{"a":{"result":null}}` | ≠0 |
| 17 | `uppercase-success` | `{"a":{"result":"Success"}}` | ≠0 |
| 18 | `trailing-space` | `{"a":{"result":"success "}}` | ≠0 |
| 19 | `mixed` | `{"a":{"result":"success"},"b":{"result":"abandoned"}}` | ≠0 |
| 20 | `outputs-injection` | `{"a":{"result":"success","outputs":{"k":"\n::error::forged"}}}` | 0, sem `::error::forged` em stdout+stderr |
| 21 | `jobname-injection-newline` | `{"x\n::warning::forged":{"result":"abandoned"}}` | ≠0, sem `::warning::forged` em stdout+stderr |
| 22 | `unset` | variável ausente no ambiente | ≠0 |
| 23 | `no-interpreter` | qualquer entrada válida, `PATH=/dev/null` | ≠0, mensagem nomeia o interpretador |

**Frase de reconciliação por braço** (Regra Dura de Reconciliação): cada braço afirma uma conclusão
desta análise. O ML-1A deve listar essas frases no relatório — uma por braço novo do `--self-test`.

### 3.5 Contra-braço do `parity-rest` (AC5)

A forma correta do contra-braço é uma mutação por `sed` sobre o script instalado:

```bash
# No --self-test ou num step separado de Makefile:
cp scripts/check-needs-success.sh /tmp/sabotado.sh
# Trocar a linha de comparação de permitidos para lista de proibidos
sed -i 's/== .success./!= "success"/' /tmp/sabotado.sh   # exemplo; linha exata definida pelo ML-1A
NEEDS_JSON='{"a":{"result":"abandoned"}}' /bin/bash /tmp/sabotado.sh && {
  echo "FAIL: script sabotado aprovou com 'abandoned' — contra-braço não detectou a regressão"
  exit 1
}
echo "ok: contra-braço confirmado — script sabotado reprova com 'abandoned'"
```

A sabotagem altera o código de produção em cópia isolada. O braço `abandoned` deve reprovar com a
versão sabotada. A atenção ao acoplamento de linha literal (`sed` contra linha exata) se aplica
aqui (ver vault `cenario-de-falsificacao-fixa-linha-literal-do-gate...`): se a linha mudar, o sed
não cassará e o contra-braço ficará inerte — detectável pelo próprio teste (o script sabotado deverá
se comportar como o original).

---

## 4. Residual declarado

**R1 — Novo valor benigno do GitHub:** um valor futuro como `"neutral"` reprovaria por construção
(custo aceito pela ADR, seção Consequences, mesmo padrão do `shim-byte-identity-gate`).

**R2 — Encolhimento de `needs:` (V14 acima):** se o array `needs:` do job `parity` for reduzido
(um job removido do YAML sem atualizar o script), o JSON `toJSON(needs)` terá menos jobs e o script
aprovará porque todos os presentes são `success`. O script não pode verificar cardinalidade sem uma
lista de jobs esperados codificada nele — o que cria acoplamento frágil. A inspeção humana no PR é
o controle residual.

**R3 — Comportamento de `needs.<job>.result` com job-level `continue-on-error: true`:**
não medido. Duas hipóteses:
- Se o GitHub relata `result: failure` para o job que falhou: o script da ADR D2 reprovaria
  corretamente (fail-closed para a nova guarda).
- Se o GitHub relata `result: success` para um job com `continue-on-error: true` que falhou:
  a nova guarda de lista de permitidos falharia aberta, da mesma forma que a guarda atual.

Fato medido: nenhum dos 5 `needs` de `parity` e `shim-byte-identity-gate` tem `continue-on-error`
no nível de job hoje (verificado via YAML walker). O vetor de ataque seria adicionar
`continue-on-error: true` a `parity-other-gates`, o que faria a nova guarda aprovar um job que
falhou — independente de lista de permitidos ou proibidos. Esse vetor é coberto pela revisão de PR,
não pelo script. A pergunta sobre o valor exato relatado é candidata a nota de vault.

**R4 — `verify-channels` em `release.yml`:** os scripts `verify-npm-channels.sh` e
`verify-pypi-channel.py` aceitam string vazia para `*_RESULT` sem reprovar. Baixo risco, fora do
conjunto R, pós-tag.

**R5 — PR controla o script que o valida:** o passo `check-needs-success.sh` roda a partir do
checkout do PR — portanto, a versão do script que valida é a que o PR introduz. Um PR pode
introduzir um script que aprova qualquer entrada. A defesa existente é a mesma que o `shim-byte-identity`
já usa (ADR-2026-09-17, versão BASE para classificação) — não é adotada aqui. Declarado como
residual; não bloqueia o ML-1A.

**R6 — Re-execução de `parity` com artefatos de tentativa anterior (não medido):** se
`actions/download-artifact@v4` conseguir baixar artefatos de uma tentativa anterior do mesmo
`run_id` durante uma re-execução, `check-falsify-shard-coverage.sh` encontraria os arquivos de
shard presentes e aprovaria sobre evidência de outra execução. Comportamento dependente da versão
da action e da configuração do repositório. Não foi testado. Candidato a nota de vault.

**R7 — Fragmentos históricos em `internal/roadmapdoc/testdata/corpus/`:** o corpus contém
roadmaps que descrevem o padrão de lista de proibidos como artefato histórico. Não é código de
produção nem template gerador. Nenhum ML corretivo necessário.
