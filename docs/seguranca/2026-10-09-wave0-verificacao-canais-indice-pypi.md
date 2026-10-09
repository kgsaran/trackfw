# Wave 0 — Threat Model: verificação de canais reprova por atraso do índice simples do PyPI

> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md`
> REQ: `docs/req/REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md`
> Autor: hades-tf | Data: 2026-10-09

---

## 1. Evidência dos dois runs reprovados

### Runs identificados

```
gh run list --workflow release.yml --limit 10
```

Saída relevante (os dois runs têm `run_attempt=2`; tentativa 1 falhou, tentativa 2 passou):

| Run ID       | Versão | Conclusão final | Attempt 1 conclusão |
|--------------|--------|-----------------|---------------------|
| 37769496640  | 9.3.3  | success         | failure (rerun)     |
| 37944892382  | 9.4.1  | success         | failure (rerun)     |

Confirmado via `gh api repos/kgsaran/trackfw/actions/runs/<id>` → campo `run_attempt: 2`.

### Cronologia da falha — v9.3.3 (2026-10-08, tentativa 1)

| Evento | Timestamp (UTC) |
|--------|----------------|
| `publish-pypi` job iniciou | 11:25:02Z |
| `publish-pypi` job concluiu (upload para PyPI) | 11:25:54Z |
| `verify-channels` job iniciou | 11:25:57Z |
| `Verify PyPI channel` (JSON API, `--retry`) passou | 11:28:37Z (0 s de retry — encontrou de imediato) |
| `Verify channel content (D7)` iniciou | 11:28:37Z |
| `pip download trackfw==9.3.3` falhou | 11:28:39Z |
| **Delta upload→falha do pip download** | **165 s (~2 min 45 s)** |

Log exato do passo falhado (run 37769496640, attempt 1):

```
=== check-channels-content: published mode (v9.3.3) ===

--- npm shim trackfw@9.3.3 ---
trackfw-9.3.3.tgz
ok: npm-shim-published: contains bin/trackfw.js
ok: npm-shim-published: no src/ directory — correct for v8 shim
ok: npm-shim-published: no bin/trackfw v7 entry — correct for v8 shim
ok: npm-shim-published: contains package.json
ok: published npm shim content assertions passed

--- PyPI wheels trackfw==9.3.3 ---
FAIL: pip download trackfw==9.3.3 failed — wheels may not be published yet

check-channels-content --published 9.3.3: 1 passed, 1 failed, 0 skipped
##[error]Process completed with exit code 1.
```

### Cronologia da falha — v9.4.1 (2026-10-09, tentativa 1)

| Evento | Timestamp (UTC) |
|--------|----------------|
| `publish-pypi` job iniciou | 14:36:00Z |
| `publish-pypi` job concluiu (upload para PyPI) | 14:37:00Z |
| `verify-channels` job iniciou | 14:37:03Z |
| `Verify npm channels` passou | 14:39:43Z (~2:40 de propagação npm) |
| `Verify PyPI channel` (JSON API, `--retry`) iniciou | 14:39:43Z |
| `Verify PyPI channel` passou (JSON API confirmou) | 14:40:13Z (+30 s de retry, ~3:13 desde upload) |
| `Verify channel content (D7)` iniciou | 14:40:13Z |
| `pip download trackfw==9.4.1` falhou | 14:40:15Z |
| **Delta upload→falha do pip download** | **~193 s (~3 min 13 s)** |

Log exato do passo falhado (run 37944892382, attempt 1):

```
=== check-channels-content: published mode (v9.4.1) ===

--- npm shim trackfw@9.4.1 ---
trackfw-9.4.1.tgz
ok: npm-shim-published: contains bin/trackfw.js
ok: npm-shim-published: no src/ directory — correct for v8 shim
ok: npm-shim-published: no bin/trackfw v7 entry — correct for v8 shim
ok: npm-shim-published: contains package.json
ok: published npm shim content assertions passed

--- PyPI wheels trackfw==9.4.1 ---
FAIL: pip download trackfw==9.4.1 failed — wheels may not be published yet

check-channels-content --published 9.4.1: 1 passed, 1 failed, 0 skipped
##[error]Process completed with exit code 1.
```

### Observação sobre o npm

Em ambas as falhas, o passo `npm pack trackfw@<v>` (D7) passou em ~1 s, sem nenhum retry. O
`verify-npm-channels.sh` já esperou a propagação do registry npm antes de D7 ser invocado —
portanto D7 herdou a confirmação do passo anterior para o npm. Não há evidência de falha do `npm
pack` nesses dois runs; a superfície npm em D7 não teve retry porque ainda não precisou, não porque
o problema não exista.

---

## 2. Qual endpoint cada passo lê

### PyPI: dois endpoints distintos

**`verify-pypi-channel.py --retry`** (passo "Verify PyPI channel"):
- Endpoint: `https://pypi.org/pypi/trackfw/json`
- Tipo: JSON API (Warehouse REST API, não normalizada por CDN da mesma forma)
- Confirmado no código: `scripts/verify-pypi-channel.py`, linha 52:
  `url = "https://pypi.org/pypi/trackfw/json"`
- Possui retry com backoff exponencial (30→60 s) e deadline de 900 s.

**`pip download trackfw==<v>` em `check-channels-content.sh --published`** (passo "Verify channel content (D7)"):
- Endpoint: `https://pypi.org/simple/trackfw/` — o "índice simples" (PEP 503)
- Tipo: Simple API, servida por CDN (Fastly)
- Confirmado indiretamente: `pip download` não usa a JSON API; usa exclusivamente o índice simples
  para localizar os arquivos. Documentação da Warehouse API
  (`https://warehouse.pypa.io/api-reference/`) separa os dois: JSON API para consulta de metadados,
  Simple API para o `pip` localizar arquivos. PEP 503 define o índice simples como a interface
  primária do pip.
- Cache-Control do índice simples: `max-age=600`; CDN (Fastly) pode responder com dados stale.
  Ausência de retry: o script faz **uma única chamada** `pip download … --quiet` (linha 467–468 do
  `check-channels-content.sh`); se falhar, emite `FAIL` imediatamente.

**Diferença observada:** Na falha do v9.3.3, a JSON API retornou a versão imediatamente (0 s de
retry), enquanto o índice simples estava indisponível 165 s depois do upload. Na falha do v9.4.1,
a JSON API precisou ~30 s de retry (total ~193 s pós-upload) e o índice simples ainda estava
indisponível no mesmo instante. Isso confirma que são dois sistemas de propagação independentes.

**Nota sobre documentação oficial:** A documentação do pip e do PyPI não especifica o SLA de
propagação do índice simples após upload. A observação de CDN com `max-age=600` (10 minutos) está
documentada na infraestrutura do Warehouse e corrobora o atraso medido. Não inferimos — os dados
dos dois runs são evidência primária.

### npm: dois mecanismos distintos também

**`verify-npm-channels.sh`** (passo "Verify npm channels"):
- Endpoint: `https://registry.npmjs.org/<pkg>/<version>` (registry JSON API)
- Tipo: API REST direta, não `npm view` (comentário no script: "to avoid npm's local HTTP cache")
- Possui retry com backoff (30→60 s) e deadline de 900 s.

**`npm pack trackfw@<v>` em `check-channels-content.sh --published`** (passo D7):
- O `npm pack <pkg>@<version>` resolve o pacote pelo registry configurado (padrão:
  `https://registry.npmjs.org/`). O endereço exato da tarball é lido do campo `dist.tarball` na
  resposta do registry.
- São o mesmo registry (mesma URL base), mas o `npm pack` faz resolução completa + download, enquanto
  o check de presença usa `curl --fail` no endpoint de metadados. Não há evidência de divergência
  entre os dois para npm até agora (os dois runs falharam apenas na PyPI).
- Sem retry em D7 para `npm pack`.

---

## 3. Enumeração fechada de passos pós-publicação sem retry

A sequência em `release.yml` (job `verify-channels`, linhas 394–452) após ambos os `publish-*` jobs:

| # | Passo | Script/comando | Retry? | Endpoint lido |
|---|-------|----------------|--------|---------------|
| 1 | Verify GitHub release | `gh release view "v$VERSION"` | Não | GitHub API |
| 2 | Verify npm channels | `scripts/verify-npm-channels.sh` | **Sim** (900 s, backoff 30→60 s) | `https://registry.npmjs.org/<pkg>/<v>` |
| 3 | Verify PyPI channel | `scripts/verify-pypi-channel.py --retry` | **Sim** (900 s, backoff 30→60 s) | `https://pypi.org/pypi/trackfw/json` |
| 4 | Verify channel content (D7) — npm pack | `npm pack trackfw@$VERSION` dentro de `check-channels-content.sh --published` | **Não** | `https://registry.npmjs.org/trackfw/<v>` + tarball |
| 5 | Verify channel content (D7) — pip download | `pip download "trackfw==$PY_VERSION"` dentro de `check-channels-content.sh --published` | **Não** | `https://pypi.org/simple/trackfw/` (CDN) |

**Passos sem retry que leem endpoint diferente do confirmado: 4 e 5.**

- Passo 4 (npm pack): o passo 2 confirmou a presença via registry JSON API; D7 usa `npm pack`
  (resolução + download). Mesma URL base, mas operação diferente. Não falhou ainda; superfície
  existe.
- Passo 5 (pip download): **o defeito observado**. Passo 3 confirmou a presença via JSON API
  (`/pypi/trackfw/json`); D7 usa `pip download` que lê o índice simples (`/simple/trackfw/`).
  Endpoints distintos, propagação independente, sem retry.

**Passo 1 (GitHub release):** único passo; não há passo anterior que confirme o mesmo. Se falhar, é
publicação ausente — comportamento correto (falha imediata, sem necessidade de retry).

---

## 4. Especificação para o ML-1A

### Onde pôr o retry

O retry deve ser adicionado em `scripts/check-channels-content.sh`, no modo `--published`, envolvendo
os dois comandos de download:
- `npm pack trackfw@$VERSION` (linhas 438–451 do script)
- `pip download "trackfw==$PY_VERSION"` (linhas 467–483 do script)

**Não** criar um novo helper separado; reaproveitar o padrão dos helpers existentes
(`verify-npm-channels.sh` e `verify-pypi-channel.py`) como referência de estrutura, mas implementar
dentro de `check-channels-content.sh` para manter a lógica D7 coesa.

Razão para não mover para os helpers existentes: o ML-4H já separou deliberadamente a verificação
de presença (helpers com retry) da verificação de conteúdo (D7). D7 tem uma responsabilidade
distinta (inspecionar o conteúdo do artefato, não apenas confirmar presença). O retry em D7 é
especificamente para o atraso de propagação do índice simples, não para falhas de publicação.

### Backoff e prazo total recomendados

**Base empírica medida:**
- v9.3.3: `pip download` falhou a 165 s pós-upload. A reexecução (tentativa 2) passou em ~4 min
  pós-upload (o job `verify-channels` da tentativa 2 completou às 11:29:22, e o upload foi às
  11:25:54 = ~208 s).
- v9.4.1: `pip download` falhou a ~193 s pós-upload. A tentativa 2 completou às 14:42:29, upload
  às 14:37:00 = ~329 s.
- v9.4.1 foi mais lento. O CDN pode ter condições variáveis.

**Prazo total recomendado:** 300 s (5 minutos) para cada comando de download em D7.
- Justificativa: cobre o pior caso observado (~193 s) com margem de ~55% sem atingir o prazo dos
  passos anteriores (900 s). O job inteiro não deve ultrapassar ~20 min (900 s dos checks de
  presença + 300 s D7 + overhead).
- Variável de ambiente: `VERIFY_CONTENT_DEADLINE` (padrão: 300), análoga ao `VERIFY_DEADLINE` dos
  helpers existentes.

**Backoff recomendado:** inicial 10 s, dobrar até máximo de 30 s.
- Justificativa: o atraso do índice simples tende a ser de 1–5 minutos; polling agressivo a cada
  10 s nas primeiras tentativas localiza a disponibilidade mais rápido que o backoff de 30 s dos
  helpers de presença (que têm 717 s de propagação medida como baseline).
- Total de tentativas dentro de 300 s com esse backoff: tentativas a ~0, 10, 30, 60, 90, 120,
  150, 180, 210, 240, 270 s = 11 tentativas antes do deadline.

**Comportamento ao esgotar o prazo:** idêntico ao atual — `FAIL` com `exit 1`. O retry não altera
o veredito final; apenas postergará a emissão do `FAIL` até o prazo. Nenhum aviso em vez de falha.

### Self-test nas duas direções (AC3)

O self-test de D7 precisa provar:
1. **Direção positiva ("disponível após delay → passa"):** simular que o download falha nas N
   primeiras tentativas e passa na (N+1)-ésima.
2. **Direção negativa ("indisponível até o prazo → reprova"):** simular que o download falha em
   todas as tentativas; verificar que o script sai com `exit 1` ao esgotar o prazo.

**Mecanismo sem rede:** o script já usa `npm pack` e `pip download` como comandos externos. A forma
mais limpa de injetar é tornar esses comandos substituíveis via variável de ambiente, análogo ao
padrão já existente em outros scripts do projeto:

```bash
# No check-channels-content.sh --published:
NPM_PACK_CMD="${NPM_PACK_CMD:-npm pack}"
PIP_DOWNLOAD_CMD="${PIP_DOWNLOAD_CMD:-pip download}"
```

O self-test (modo `--self-test` ou novo modo `--self-test-retry`) injeta stubs que:
- Arm A (positivo): falham nas 2 primeiras chamadas (simulando delay), passam na 3ª.
  - Implementação: stub que lê um contador em arquivo, incrementa, sai 1 se contador < 3, sai 0 e
    cria o arquivo esperado se contador >= 3.
- Arm B (negativo): sempre falham. Injetar `VERIFY_CONTENT_DEADLINE=0` (ou um prazo curto, ex: 1 s)
  para que o loop esgote em tempo de teste.
  - Verificar que o script sai com código != 0.

Esse padrão não requer rede e não exige mock de sistema de arquivos. Os arms A/B são análogos aos
arms 1–4 já existentes no `--self-test` atual.

**Arm existente A3 (wheel local) pode ser reaproveitado para a direção positiva:** se o stub de
`pip download` depositar uma wheel sintética no destino (análoga à arm 3 do self-test atual), o
`inspect_wheel` subsequente já cobre o conteúdo.

### Correção do comentário do workflow (AC4)

Linha 448 do `release.yml`:
```yaml
# By this point npm + PyPI confirmed live (steps above), so read-after-write is resolved.
```

Deve ser substituída por algo como:
```yaml
# By this point npm + PyPI confirmed live via their respective JSON/registry APIs (steps above).
# NOTE: pip download uses the Simple Index (/simple/), which is CDN-cached independently of
# the JSON API. Retry for both npm pack and pip download is in check-channels-content.sh.
```

---

## 5. Threat model do retry

### O retry pode esconder publicação quebrada?

**Pergunta:** um retry que eventualmente passa pode mascarar uma publicação com conteúdo errado
(versão errada, wheel faltando, tarball corrompido)?

**Resposta: não, por construção.**

O retry em D7 só cobre a fase de *download* (`npm pack` / `pip download`). A verificação de
*conteúdo* do artefato (`inspect_npm_tarball`, `inspect_wheel`) ocorre **depois** do download, sobre
o artefato real. O retry não toca nem substitui a inspeção de conteúdo. O fluxo é:

```
[retry] download → [sem retry] inspect_content → pass/fail
```

Se o download finalmente passa mas o conteúdo está errado (ex.: wheel com `.py` files, tarball com
`src/`), `inspect_wheel` / `inspect_npm_tarball` reportam `FAIL` normalmente. O retry não tem
visibilidade sobre o conteúdo — ele só sabe se o download falhou ou não.

**Cenário concreto de publicação quebrada:**
- Se a wheel foi publicada com conteúdo errado, `pip download` vai baixá-la (sucesso), mas
  `inspect_wheel` vai reprovar. O retry não muda isso.
- Se a wheel **não foi publicada** (upload falhou silenciosamente e a versão ficou sem wheel para
  uma plataforma), `pip download --only-binary :all:` vai falhar para sempre até o prazo → `FAIL`.
  O vacuity guard (`WHEEL_COUNT == 0 → fail`) ainda cobre o caso de download bem-sucedido mas
  sem wheels.

### O que garante que, esgotado o prazo, a falha continua vermelha?

O comportamento ao esgotar o prazo é preservado explicitamente:
- O loop de retry verifica `elapsed >= deadline` antes de cada tentativa.
- Ao esgotar: o script emite `FAIL: …` e `exit 1`, idêntico ao comportamento atual sem retry.
- Nenhum caminho no retry converte `FAIL` em `skip` ou `pass`.
- A variável `FAIL` do script é incrementada; o bloco final `if [[ "$FAIL" -gt 0 ]]; then exit 1`
  permanece inalterado.

**Falso-negativo residual:** se o deadline for configurado com `VERIFY_CONTENT_DEADLINE=0` em
produção (por engano), o loop expira imediatamente sem tentar. Isso é detectável: a mensagem de
falha deve incluir `deadline=0s` para ser distinguível de "download genuinamente falhou". Recomenda-
se que o ML-1A adicione uma guarda explícita: `if [[ deadline -le 0 ]]; then deadline=300; fi`.

### Threat model: o adversário aqui é o implementador apressado

| Superfície | Ameaça | Gate que deveria pegar |
|------------|--------|------------------------|
| Loop de retry com `exit 0` ao esgotar | Implementador inverte a condição de deadline (ex.: `if elapsed < deadline; then exit 0`) | AC3 Arm B (negativo): garante que prazo esgotado = exit != 0 |
| Retry que substitui a inspeção de conteúdo | Implementador move `inspect_wheel` para dentro do retry e o descarta se o download falhou | Revisão de código + self-test Arm A positivo deve chamar inspect e verificar que passou |
| `VERIFY_CONTENT_DEADLINE` muito grande | Timeout de 3600 s faz o job durar 1h+ e torna a falha difícil de diagnosticar | Sem gate automático; AC2 declara o prazo (300 s padrão) — o comentário do código deve documentar a origem empírica |
| Comentário antigo do workflow não removido | "read-after-write is resolved" permanece, confundindo futuros implementadores | AC4: gate de comentário (grep negativo no release.yml) ou revisão manual do arquiteto |
| Self-test Arm B com deadline insuficiente | `VERIFY_CONTENT_DEADLINE=1` pode ser rápido o suficiente para uma tentativa real passar antes do deadline | Usar `VERIFY_CONTENT_DEADLINE=0` no self-test (loop não tenta nenhuma vez, sai imediatamente) |

### Residual declarado

O retry cobre exclusivamente o atraso de propagação do índice simples do PyPI. Não cobre:
- Falha de rede transitória do runner de CI (distinção difícil de fazer; o prazo de 300 s serve de
  buffer para ambos).
- Atraso de propagação do `npm pack` no registry npm para D7: não observado até hoje, mas a
  superfície existe (sem retry). O ML-1A pode incluir retry para `npm pack` também; se não incluir,
  a superfície deve ser documentada como residual.
- Versões com wheel para uma única plataforma publicada mas não para outras: o vacuity guard cobre
  parcialmente (se nenhuma wheel baixou) mas não o caso em que apenas um subconjunto foi publicado
  e as outras plataformas ainda não propagaram.

---

## Veredito

**DEFEITO CONFIRMADO.** Causa raiz: `check-channels-content.sh --published` usa `pip download` para
verificar o conteúdo das wheels do PyPI, lendo o índice simples (`/simple/`) que é servido por CDN
com latência de propagação independente da JSON API. O passo anterior (`verify-pypi-channel.py
--retry`) confirma a versão pela JSON API, que propaga mais cedo. O comentário do workflow afirma
"read-after-write is resolved" com base na confirmação via JSON API e aplica essa garantia ao
índice simples — premissa incorreta.

**Evidência primária:** dois runs falhados com exatamente o mesmo erro (`FAIL: pip download
trackfw==<v> failed — wheels may not be published yet`) em dois dias consecutivos (v9.3.3 em
2026-10-08, v9.4.1 em 2026-10-09), ambos com delay de upload→falha entre 165 s e 193 s. Em ambos
os casos, a reexecução manual passou sem qualquer alteração de código, confirmando que a falha é de
timing, não de conteúdo.

**Escopo do ML-1A:** retry com backoff (10→30 s) e deadline de 300 s para `pip download` e `npm
pack` em D7, dentro de `check-channels-content.sh`; self-test nas duas direções (AC3); correção do
comentário do workflow (AC4). O retry não altera o que é verificado; esgotado o prazo, o script
continua reprovando.
