# Detecção de hook relativo não cobre `$PWD`, aspas e variável indefinida

> 2026-08-22 · domínio: validate / credential-guard · REQ-2026-08-17

## Fato

A regra adicionada em `validateGuardHookResolvable` (flag `requiresVarOrShellPrefix` em
`credentialGuardHookFile`) acusa o hook de guard escrito na **forma relativa pura** para Claude
Code, Gemini CLI e Codex CLI. Ela **não** cobre toda a classe de comandos que falham fora da raiz.

Medido pelo `hades-tf` (8 variantes) e reconfirmado por medição própria:

```
CAPTURADAS:      scripts/...   ./scripts/...   sh scripts/...   ../scripts/...
SILÊNCIO CORRETO: $CLAUDE_PROJECT_DIR/...
NÃO CAPTURADAS:  $PWD/...      $UNDEFINED/...  "scripts/..." (entre aspas)
```

Motivo: `isRelativePure(raw)` é `!HasPrefix("$") && !HasPrefix("\"") && !IsAbs(...)`. Qualquer coisa
que comece com `$` ou `"` é tratada como "forma correta" sem olhar o conteúdo.

## Por que importa

`$PWD/scripts/trackfw-credential-guard.sh` **falha fora da raiz igual à forma relativa pura**, e o
`validate` fica em silêncio — confirmando o engano de quem editou o hook à mão. É o erro que alguém
comete tentando consertar.

Mitigação atual: a mensagem da regra diz `run \`trackfw update\` to fix it`, não "adicione um
prefixo" — o caminho sugerido não leva ao `$PWD`.

## Falso-positivo é por construção ausente

A condição é `hf.requiresVarOrShellPrefix && isRelativePure(...)`, com curto-circuito no primeiro
operando: para Cursor, Copilot e Kiro o valor do comando **nem é avaliado**. Relativo é a forma
correta desses três (ADR-2026-08-11). Kiro é `false` por conservadorismo, **não** por medição.

## Onde continuar

- REQ aberta: `docs/req/REQ-2026-08-21-validate-nao-detecta-hook-com-pwd-que-falha-fora-da-raiz.md`
  — a decisão de postura (acusar tudo que não casa × lista de formas sabidamente quebradas) vai no ADR.
- Parecer completo: `docs/seguranca/2026-08-21-revisao-da-deteccao-de-hook-relativo.md`
- Gate: `scripts/check-validate-parity.sh` (blocos CG e GBG) · falsificação: cenários 159 e 160.
