---
status: wip
date: 2026-09-09
squad: apolo-tf
req: "docs/req/REQ-2026-09-09-update-harness-reescreve-o-script-do-guard-e-nao-o-contabiliza-e-a-instrucao-do-validate-fica-desacreditada.md"
---

# Roadmap: O alvo do manifesto é dono do caminho do script do guard

> Criado em: 2026-09-09 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-09-update-harness-reescreve-o-script-do-guard-e-nao-o-contabiliza-e-a-instrucao-do-validate-fica-desacreditada.md` · Issue [#300](https://github.com/kgsaran/trackfw/issues/300)

🔴 **Alvo da 7.5.1** — o defeito desacredita a instrução em destaque no CHANGELOG da 7.5.0.

## Diagnóstico — medido em 3 máquinas, não rederivar

```
alvos cujo path é o script do guard:  ZERO
```

O script é escrito como **efeito colateral** do alvo de fiação; a contagem só enxerga o arquivo de
fiação. Três estados — *conteúdo novo*, *reescrita idêntica*, *dry-run* — produzem relatórios
indistinguíveis quanto ao script.

| estado | relatório | efeito real |
|---|---|---|
| desatualizado (macOS) | `updated=9 skipped=18` | conteúdo novo, 561 → 580 linhas |
| já em dia (macOS, ao vivo) | `updated=0 skipped=27` | **reescrito**: mesmo sha, mtime +160s |
| `--dry-run` (Windows) | `updated=0 skipped=33` | nada |

🔴 O `updated=9` veio de arquivos de **fiação**. O script mudou nos dois primeiros e **não foi contado
em nenhum**.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1

### ML-1A — O script vira alvo de primeira classe
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`

O manifesto ganha alvo cujo `path` é o script do guard. A contagem passa a refletir o que aconteceu
com **ele**, não com a fiação.

🔴 **E parar de reescrever quando o conteúdo é idêntico** — é a reescrita desnecessária que torna o
estado 2 indistinguível do 1. Comparar conteúdo antes de escrever.

⚠️ **Medir se o `trackfw-credential-guard.sh` tem o mesmo padrão** — não presumir. Se tiver, é
**mesma causa** e entra neste ML.

**Falsificação nas três direções:**
- script desatualizado ⇒ `updated`, e o conteúdo muda;
- script idêntico ⇒ `skipped`, **e o `mtime` NÃO muda** (prova a idempotência);
- `--dry-run` ⇒ nada escrito, `mtime` inalterado.

🔴 **Controle de não-regressão:** o script **continua sendo escrito** quando precisa. Um alvo que
conta certo e para de escrever seria **pior** que o defeito atual — a correção da 7.5.0 deixaria de
chegar às máquinas.

**Critérios de aceite:**
- [ ] os 3 estados distinguíveis pelo relatório
- [ ] `mtime` inalterado quando o conteúdo é idêntico
- [ ] controle: conteúdo novo continua sendo escrito
- [ ] `credential-guard` medido; se mesmo padrão, corrigido junto
- [ ] paridade nos 3 CLIs (`check-cli-parity.sh` rc=0)
- [ ] `make quality QUALITY_EXIT=0`, três medidas: `MAKE_RC`, `grep -c '^FAIL'`, `grep -c '^OK'` ≥ 1033

## Fora deste roadmap

- **Regras de integridade do `validate`** — lacuna **inversa** (cobrem script e modo, não a fiação):
  `REQ-2026-09-02-remover-a-entrada-pretooluse-...`, que está **órfã**.
- **Resolução de escopo do `update harness`**: `REQ-2026-08-21-...`, também **órfã**.
