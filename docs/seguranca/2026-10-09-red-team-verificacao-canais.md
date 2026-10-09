# Red-Team — ML-2A: verificação de canais (retry via JSON API)

> REQ: `docs/req/REQ-2026-10-09-verificacao-de-conteudo-dos-canais-reprova-por-atraso-do-indice-simples-do-pypi.md`
> Commit auditado: `76842329` (HEAD em `fix/verificacao-canais-indice-pypi`)
> Arquivos: `scripts/check-channels-content.sh` · `.github/workflows/release.yml`
> Autor: hades-tf | Data: 2026-10-09

---

## Execuções realizadas

```
bash scripts/check-channels-content.sh --self-test
  → 9 passed, 0 failed, 0 skipped  ✅

bash scripts/check-channels-content.sh --published 9.4.1
  → 10 passed, 0 failed, 0 skipped  ✅ (npm + 8 wheels, todos os tags presentes)
```

---

## Achados

### A1 — Nenhum hash verificado contra `digests.sha256` da JSON API (MÉDIO)

**Superfície:** `_do_fetch` / `curl -fSL` em `_fetch_pypi_wheels_with_retry`

**Observado:**
A JSON API do PyPI retorna `digests.sha256` para cada URL de wheel:
```
domains: {'files.pythonhosted.org'}   # confirmado em 9.4.1
digests keys: ['blake2b_256', 'md5', 'sha256']   # campo presente mas descartado
```
O script extrai apenas `u['url']` e ignora `u['digests']['sha256']` (linha 401).
Após o download, nenhuma verificação de integridade é realizada.

**Consequência:** um CDN comprometido ou um DNS spoof em `files.pythonhosted.org`
poderia servir uma wheel diferente. A wheel passaria em `inspect_wheel` se contiver
`.data/scripts/trackfw*` e nenhum `.py`. O binário em si não é verificado.

**Nota sobre o código anterior:** o `pip download` também não verifica hash por padrão
(requer `--require-hashes`). O novo caminho via `curl` não regride; apenas mantém a
ausência de verificação. O campo `sha256` está disponível no mesmo JSON já baixado
— é uma oportunidade de hardening que não precisa de round-trip adicional.

**Reprodução:**
```bash
curl -sf "https://pypi.org/pypi/trackfw/9.4.1/json" | python3 -c "
import sys,json
data=json.load(sys.stdin)
u=data['urls'][0]
print('sha256:', u['digests']['sha256'])   # presente, nunca usado pelo script
"
```

**Recomendação:** após cada `_do_fetch`, verificar SHA256 do arquivo baixado contra
`u['digests']['sha256']` da mesma resposta JSON. Não requer rede extra; o JSON já
está em memória.

---

### A2 — `urls` parcial na JSON API passa o retry e dispara FAIL sem nova tentativa (BAIXO)

**Superfície:** `_fetch_pypi_wheels_with_retry` + bloco do tag-checker em `--published`

**Observado (medido com stub):**
```
PYPI_JSON_CMD retorna 4 de 8 wheels
→ _fetch_pypi_wheels_with_retry: todos os 4 downloads ok → retorna 0 (sucesso)
→ tag-checker: 4 tags ausentes → FAIL imediato, sem novo retry
EXIT CODE: 1
```

O loop de retry em `_fetch_pypi_wheels_with_retry` só reinicia se um download individual
falha. Se a JSON API retorna N < 8 URLs (todos downloads bem-sucedidos), a função retorna
0 e o tag-checker fora do loop emite FAIL sem tentar novamente.

**Janela real de ocorrência:** `publish-pypi` (twine upload 8 wheels em sequência)
termina antes de `verify-channels` começar — `needs: [publish-npm-shim, publish-pypi]`.
A JSON API pode refletir N < 8 wheels no curto intervalo entre o término do job e o início
dos checks de presença. Se `verify-pypi-channel.py` detectar a versão quando só N wheels
estão indexadas, D7 roda com a lista incompleta e falha sem retry de propagação.

**Consequência:** falso vermelho durante upload parcial — o mesmo padrão do defeito
original, agora na direção oposta (retry correto para download falho, mas cego para
lista incompleta).

**Reprodução:**
```bash
cat > /tmp/partial-pypi.py << 'EOF'
import json, sys
TAGS = ['macosx_10_9_x86_64','macosx_11_0_arm64','manylinux_2_17_aarch64','manylinux_2_17_x86_64']
data = {'urls': [{'url': f'FAKE/trackfw-ST-py3-none-{t}.whl','packagetype':'bdist_wheel'} for t in TAGS]}
print(json.dumps(data))
EOF
PYPI_JSON_CMD="python3 /tmp/partial-pypi.py" \
FETCH_CMD="python3 <pass-fetch-stub>" \
NPM_PACK_CMD="python3 <pass-npm-stub>" \
VERIFY_CONTENT_DEADLINE=5 \
bash scripts/check-channels-content.sh --published 9.4.1
# → FAIL: missing expected wheel tags: musllinux_1_2_aarch64, musllinux_1_2_x86_64, win_amd64, win_arm64
```

**Recomendação (alternativa simples):** dentro de `_fetch_pypi_wheels_with_retry`,
contar os URLs antes de baixar; se `count < EXPECTED_WHEEL_COUNT`, setar `attempt_ok=0`
e continuar o loop. Isso cobre o caso "lista parcial ainda cresce".

---

### A3 — Variáveis injetáveis ativas em `--published` (sem guarda) (BAIXO)

**Superfície:** `PYPI_JSON_CMD`, `FETCH_CMD`, `NPM_PACK_CMD` no modo `--published`

**Observado:**
```bash
PYPI_JSON_CMD="python3 <empty-stub>" VERIFY_CONTENT_DEADLINE=2 \
bash scripts/check-channels-content.sh --published 9.4.1
# → usa o stub em vez de curl; confirma que as vars são lidas no modo real
```

O cabeçalho documenta `FETCH_CMD` e companhia como "CI override" — intencionais.
No entanto, não há guarda em `--published` que avise se essas variáveis estiverem
definidas. Se um step anterior no mesmo job escrever em `$GITHUB_ENV`:
```
PYPI_JSON_CMD=echo '{"urls":[]}'
```
o D7 usaria o override em vez do curl real, passando pelo npm (que não é sobrecarregado)
e falhando no PyPI por deadline. O cenário mais preocupante: um stub malicioso que
retorna URLs de wheels controladas pelo atacante — a inspeção de conteúdo seria
sobre artefatos não publicados.

**Condição de ativação:** requer step anterior no mesmo job escrever em `$GITHUB_ENV`.
Nenhum step atual faz isso; o risco é supply-chain de ação de terceiro no job.

**Recomendação:** adicionar aviso explícito em `--published` se qualquer dessas variáveis
estiver definida:
```bash
if [[ -n "${PYPI_JSON_CMD:-}${FETCH_CMD:-}${NPM_PACK_CMD:-}" ]]; then
    echo "WARN: check-channels-content --published: injectable command overrides are set" >&2
fi
```

---

## Perguntas da tarefa — respostas diretas

**1. O retry pode esconder publicação quebrada? (JSON API com `urls` vazio/parcial)**

Dois sub-casos:
- `urls` vazio: `_fetch_pypi_wheels_with_retry` retenta até o deadline e então FAIL.
  Não esconde — falha vermelha após prazo. Correto.
- `urls` parcial (4 de 8): a função retorna 0 (downloads ok), tag-checker fora do loop
  detecta tags ausentes e FAIL sem retry. Ver A2.

O retry não esconde conteúdo errado: `inspect_wheel` corre fora do loop de retry, sobre
o arquivo real baixado. Arm 8 confirma: bad-content → exit 1, sleep never called.

**2. Hash SHA256 validado? URL fora de `files.pythonhosted.org` aceita? Redirecionamento?**

- SHA256: NÃO verificado (ver A1). O campo `digests.sha256` existe no JSON, nunca lido.
- Domínio: NÃO validado. Qualquer URL retornada no JSON seria aceita.
  Na prática, PyPI retorna apenas `files.pythonhosted.org` (confirmado para 9.4.1).
- Redirecionamento: `curl -fSL` segue redirects (`-L`). Sem restrição de host final.

**3. Lista fixa de 8 tags: se uma plataforma for acrescentada no release mas não aqui?**

`extra = actual - expected` → emite `warn:` mas `exit_code` permanece 0. O verificador
**não falha** para tags inesperadas. A tag extra é detectada e registrada, mas a
verificação continua verde. Isso é residual de design (declarado na Wave 0, §5).

**4. Injeção (`PYPI_JSON_CMD`, `FETCH_CMD`, etc.) ativável sem querer no CI?**

Confirmado ativo em `--published` (ver A3). Em `release.yml`, o step D7 não tem bloco
`env:` — as vars não são injetadas explicitamente. Ativação acidental exigiria step
anterior escrevendo em `$GITHUB_ENV`. Risco atual: baixo; superfície: existe.

**5. Prazo de 900 s cabe no timeout do job?**

`timeout-minutes` omitido → padrão GitHub Actions = 360 min (6h). Pior caso:
- `verify-npm-channels.sh`: até 900 s
- `verify-pypi-channel.py`: até 900 s
- D7 npm retry: até 900 s
- D7 PyPI retry: até 900 s

Total worst-case: ~3600 s = 60 min. Cabe com folga dentro de 6h. Sem problema.

**6. Execuções reais:**

```
--self-test:  9 passed, 0 failed, 0 skipped  ✅
--published 9.4.1:  10 passed, 0 failed, 0 skipped  ✅
  npm: trackfw-9.4.1.tgz baixado, 4 checks ok
  PyPI: 8 wheels baixadas via JSON API, todos os 8 tags presentes, sem .py, binário ok
```

---

## Veredito

APROVA COM RESSALVAS.

O defeito original (falso vermelho por atraso do índice simples do PyPI) está corrigido:
o `--published` agora busca wheels pela JSON API e reintenta com backoff até 900 s.
`--self-test` verde em todos os 9 arms. `--published 9.4.1` verde em produção.

Dois achados residuais não bloqueiam o merge mas devem entrar como MLs na REQ vigente:

- **A2** (BAIXO): lista parcial de `urls` na JSON API passa o retry e falha sem nova
  tentativa. Correção simples: checar contagem dentro de `_fetch_pypi_wheels_with_retry`.
- **A1** (MÉDIO): hash SHA256 disponível no JSON descartado. Correção simples: um `sha256sum`
  contra `u['digests']['sha256']` após cada download.

A3 (injetáveis sem guarda) fica como residual documentado — sem impacto direto no
fluxo atual.
