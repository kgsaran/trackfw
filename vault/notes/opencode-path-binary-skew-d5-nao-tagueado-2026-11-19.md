# OpenCode: PATH binary skew — D5 (`credViewFileScan`) não tagueado (2026-11-19)

**Contexto:** ML-5A (Wave 5, REQ-2026-11-17). Medições de comportamento do credential guard com payloads OpenCode.

## Achado

O commit `cafa1952` (ML-3A, 2026-11-18) introduziu `credViewFileScan` (D5 — scan de arquivo via Antigravity `view_file`). Este commit está na branch `feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli` mas **não foi mergeado nem tagueado**.

**Versões:**
- `bin/trackfw` (HEAD da branch): 11.7.0 — tem D5
- `/opt/homebrew/bin/trackfw` (PATH, Mac KG): 11.5.0 — NÃO tem D5

## Sintoma que confunde

Medição T-D5 (Antigravity `view_file` + arquivo real com AWS key):
- `bin/trackfw`: **exit 2** (bloqueia — D5 lê o arquivo)
- Homebrew 11.5.0 (PATH): **exit 0** (permite — D5 ausente)

Se ML-5B usar o `trackfw` do PATH (que é o que o plugin usa via `spawnSync("trackfw")`), T-D5 sai 0 — sem ser um defeito.

## Como confirmar

```bash
trackfw --version        # PATH binary
git merge-base --is-ancestor cafa1952 v11.5.0 && echo "sim" || echo "nao"
# esperado: "nao" — D5 não está em 11.5.0
```

## Impacto para o plugin OpenCode

O plugin chama `spawnSync("trackfw")`, que resolve pelo PATH. No Mac KG, o PATH binary é 11.5.0 — sem D5. D5 é Antigravity-específico e NÃO afeta o plugin OpenCode (que usa `execute.before` e não Antigravity hooks). Sem impacto funcional para Wave 5.

Mas: T-WV1/T-WV2 (write com credential content) e T-S (shell com credential) têm resultados idênticos em 11.5.0 e 11.7.0 — confirmado por medição. O guard de write e shell funciona com o PATH binary atual.

## O que ML-5B deve registrar

- `trackfw --version` do PATH binary em cada máquina
- Windows npm shim: `trackfw.cmd` resolve pelo PATH? (M14, M21)
- root-`cwd` D4 fallback em 11.5.0: não medido

> Fonte: `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md` §2.3 tabela de medição + M21.
