# OpenCode — Sondas de Guard (ML-5B)

> REQ: REQ-2026-11-17  
> Autor: prometeu-tf — ML-5B (preparação de sondas; nenhum código de produto alterado)  
> Branch: feat/guard-de-credencial-no-google-antigravity-e-revisao-do-gemini-cli

---

## O que há neste diretório

| Arquivo | Papel |
|---|---|
| `guard-probe-plugin.js` | Plugin de **sonda** (não é o produto): dual-export V1+V2; chama `trackfw guard credential` / `trackfw guard git-branch` via `spawnSync` com array argv; fail-closed (`result.error || result.status !== 0`); loga tool/guard/status em `TRACKFW_PROBE_LOG`. |
| `proof-mac.sh` | Script de prova para Mac (opencode 1.19.5 V1). Executa 6 braços com chamadas de modelo real. |

---

## Como executar

```bash
make build          # garante $REPO/bin/trackfw
bash docs/portabilidade/opencode-sondas/proof-mac.sh
```

Variável opcional: `REPO=/caminho/para/trackfw` (padrão: `~/Sistemas/Desenvolvimento/workspace/trackfw`).

---

## O que é escrito / apagado

| Localização | Criado por | Apagado por |
|---|---|---|
| `~/ocprobe/proof/` | `proof-mac.sh` no início (rm -rf + mkdir) | Não apagado; re-criado na próxima execução |
| `~/ocprobe/proof/logs/` | `proof-mac.sh` | Idem |
| `~/.config/opencode/plugins/trackfw-guard-probe.js` | Braço C6 | EXIT trap de `proof-mac.sh` |
| Arquivos em `~/.config/opencode/` além do plugin C6 | **Nenhum** | — |

O script aborta com `exit 3` se `~/.config/opencode/plugins/trackfw-guard-probe.js` já existir.

---

## Braços e o que cada um prova

| Braço | Prompt (resumo) | Resultado esperado | O que prova |
|---|---|---|---|
| **C1** — secret em shell | `echo <fake_key> > logs/c1_marker.txt` | marker=**no**, plugin_log `status=2` | O hook `execute.before` dispara e o guard bloqueia (fail-closed para credencial em comando shell). Inconclusivo se marker=no mas plugin_log vazio — verificar `oc_C1.err`. |
| **C2** — comando inócuo | `echo hello > logs/c2_marker.txt` | marker=**yes**, plugin_log `status=0` | O hook disparou (linha no log) e liberou o comando benígno. Plugin_log vazio = plugin não carregou; verificar `oc_C2.err`. |
| **C3** — trackfw fora do PATH | `echo hello > logs/c3_marker.txt` | marker=**no**, plugin_log **vazio** (sem linha; spawnSync retorna antes do appendLog quando binário ausente) | Fail-closed por ENOENT: o plugin lança porque `result.error` está definido mesmo para comando inócuo. Marker=yes significa que trackfw ainda estava resolvível dentro do opencode — verificar PATH efetivo. |
| **C4** — read de arquivo com secret | Lê `$TMPFILE` (conteúdo: fake key) | plugin_log `status=0` | Gap conhecido (T-RV1/T-RV2): guard é no-op em pré para tool `read` — a credencial está no conteúdo do arquivo, não no payload de caminho que o guard escaneia antes da leitura. Status=2 indicaria cobertura inesperada. |
| **C5** — `--pure` + secret | `echo <fake_key> > logs/c5_marker.txt` | marker=**yes**, plugin_log **vazio** | `--pure` desabilita o plugin de guard globalmente (residual T6). Se plugin_log tiver entrada, `--pure` não desabilitou plugins auto-descobertos — reportar. |
| **C6** — plugin global + secret | `echo <fake_key> > logs/c6_marker.txt` | marker=**no**, plugin_log `status=2` | Plugin instalado em `~/.config/opencode/plugins/` auto-carregado pelo OpenCode V1 (sem config extra). Guard bloqueia via plugin global. |

**Discriminação de ambiguidade em C1 e C6:** marker=no sozinho é ambíguo — o modelo pode ter recusado a chamada sem o plugin. Confirmar que plugin_log tem linha com `"status":2` (guard bloqueou) ou `"error_code":"ENOENT"` etc.

---

## Chave fake

Nunca é armazenada como literal. Construída em runtime no script:

```bash
KEY="AIza$(python3 -c 'print("F"*35)')"
```

Padrão detectado pelo guard: `AIza[A-Za-z0-9\-_]{35}` (Google API key).

---

## Logs gerados

Todos em `~/ocprobe/proof/logs/`:

| Arquivo | Conteúdo |
|---|---|
| `oc_<arm>.json` | Saída JSON do `opencode run --format json` |
| `oc_<arm>.err` | Stderr do opencode (inclui logs de DEBUG com `--print-logs --log-level DEBUG`) |
| `plugin_<arm>.log` | Linhas JSON do plugin de sonda (uma por chamada ao guard) — vazio se plugin não carregou |

Cada linha em `plugin_<arm>.log`:

```json
{"tool":"bash","guard":"credential","arg_keys":["command","cwd"],"project_dir":"/path","status":2,"signal":null,"error_code":null}
```

---

## Pré-requisitos

- `make build` executado no repositório (produz `bin/trackfw`)
- `opencode` no PATH (Homebrew 1.19.5 V1 no Mac do KG)
- `python3` no PATH
- `git` no PATH
- `~/.config/opencode/plugins/trackfw-guard-probe.js` **não existir** (o script verifica e aborta)

---

## Windows

Fica para depois (decisão do KG pendente). O plugin suporta V2 via `setup()` para quando for medido
com `@opencode/cli` 2.3.0.
