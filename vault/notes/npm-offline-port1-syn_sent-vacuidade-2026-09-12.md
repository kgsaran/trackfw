# npm --offline vs. registry morto: SYN_SENT, vacuidade e como provar ausência de rede

**Data:** 2026-09-12 | **Contexto:** Pergunta 15 do `windows-probe.yml` (AC9, opção D)

## O problema que gerou este registro

O install da Pergunta 15 usava `--registry http://127.0.0.1:1` para "provar" que não precisava de rede.

**Medição real:** no macOS (e provavelmente no Linux), a porta 1 não recusa (`ECONNREFUSED`) — fica em `SYN_SENT` até o timeout de TCP. São dois `npm install` esperando, e é por isso que o passo levava ~11 minutos e parecia pendurado.

**No Windows (runner `windows-latest`):** a porta 1 pode recusar rapidamente (`ECONNREFUSED`), por isso o AC9 fechou no runner mesmo com o defeito no design — o comportamento de TCP diverge por OS.

## Por que o registro errado existiu

O bullet "qualquer tentativa de rede falha com ECONNREFUSED" foi escrito na seção "Decisões técnicas" no momento do **design**, antes de medir o comportamento real no macOS. A medição mostrou SYN_SENT. O agente contornou localmente cabeando o `node_modules` à mão, mas **o artefato de design não foi reescrito** para refletir o que foi medido. A seção "Decisões técnicas" virou uma lista de afirmações pós-medição sem ser atualizada — padrão que a Regra Dura de Reconciliação existe para pegar.

## A solução correta: `--offline`

`npm install --offline` faz o próprio npm declarar que não usou rede: ele falha imediatamente com `ENOTCACHED` se qualquer dep não estiver em cache. Sem ambiguidade de timeout de TCP.

**Substituição:** `--registry http://127.0.0.1:1` → `--offline` nos dois installs.

## Vacuidade — armadilha importante

Com `--offline`, a afirmação "npm não usou rede" só é não-vácua se o install tiver algo que poderia vir de registry. No caso da Pergunta 15:

- **shim** com `--omit=optional`: as 6 `optionalDependencies` de plataforma são excluídas. O único item instalado é o tarball local passado como argumento. **Zero resolução de registry** → `--offline` passa vacuamente.
- **@trackfw-bin/win32-x64**: sem dependências. **Zero resolução de registry** → `--offline` passa vacuamente.

**Consequência:** `--offline` nos installs reais guarda contra *regressão futura* (se alguém adicionar deps de registry), mas **não prova nada hoje**. A prova vive no braço separado.

## O braço correto: cache vazio + ENOTCACHED

```powershell
$emptyCache = Join-Path $env:RUNNER_TEMP "p15-empty-cache"
New-Item -ItemType Directory -Force -Path $emptyCache | Out-Null
# instalar lodash (pacote real, publicado) contra cache vazio
npm install --offline --cache $emptyCache lodash 2>&1 | Tee-Object $offlineOutFile | Write-Host
# deve falhar com exit não-zero E "ENOTCACHED" na saída
```

**Por que lodash com `--cache <vazio>`:**
- Pacote real e publicado → a única razão para falhar é o flag, não "pacote não existe"
- Cache vazio garantido → não está em cache por definição (sem depender do estado do runner)
- Verificar tanto o exit code quanto `ENOTCACHED` na saída: exit-nonzero sozinho pode vir de qualquer causa

**Por que não usar nome de pacote falso:** `@foo/does-not-exist-xyz` falharia com um erro diferente (ERESOLVE, 404 bloqueado pelo flag offline, etc.) — não discrimina "o flag funcionou" de "o pacote não existe no registry".

## Relação com os artefatos

- `docs/agents-working-context.md` linha ~27: corrigida de ECONNREFUSED → SYN_SENT (2026-09-12)
- `.github/workflows/windows-probe.yml` seção 3.5 + seção 4: braço ENOTCACHED adicionado, installs com `--offline`
- `prototype/evidence/trilha1-ml1a-ml1d-2026-09-12.md` linhas 478-487: já documentava corretamente o SYN_SENT
