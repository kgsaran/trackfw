# credential-guard: bypass de exemption por estrutura JSON — `> /dev/null` em payload de escrita — 2026-10-08

## Contexto

ML corretivo da Wave 0 da REQ-2026-10-06. Ao medir o comportamento do credential guard com payloads
reais de Amazon Q `preToolUse[fs_write]` e Windsurf `pre_write_code`.

## Achado (Vector EE4 — medido em 2026-10-08)

Payload: Amazon Q `fs_write` `str_replace` com JWT + `> /dev/null` em `new_str`:

```json
{"hook_event_name":"PreToolUse","tool_name":"fs_write","tool_input":{
  "command":"str_replace","path":"/project/file.py",
  "old_str":"old","new_str":"<JWT-SINTETICO> > /dev/null"
}}
```

RC obtido: **0** (esperado: 2). **Falso negativo.**

## Causa raiz

O fluxo de detecção (`internal/guard/credential.go`):

1. **Step 3 (Layer 1)**: `credJWTRe` encontra o JWT inline → `match = "JWT"`.
2. **Step 5**: `credRedirectRe` = `[0-9]?>>?[ \t]*[^ \t\r\n|&;,:]+` varre o raw JSON inteiro.
   No payload acima, encontra `> /dev/null"}}`.
3. **Step 8**: `credIsAllEphemeral` verifica se TODOS os redirect targets são efêmeros.
   `credIsEphemeralTarget` recebe `/dev/null"}}`:
   - `tr -d "\"'"` → `/dev/null}}`
   - `TrimRight(path, "},")` → remove `}}` → resultado: `/dev/null`
   - É `/dev/null`? **SIM** → retorna `true`.
   - `hasRedirect=true, allEphemeral=true` → retorna `true`.
4. Step 8 faz `return 0` — mesmo com JWT detectado pela Layer 1.

## Por que é assimétrico com Windsurf

Windsurf `pre_write_code` tem estrutura com array `edits`:

```json
{"tool_info":{"edits":[{"new_string":"JWT > /dev/null"}]}}
```

A correspondência do `credRedirectRe` é `> /dev/null"}}` → mas com `"}]}}` o resultado é:
- `tr -d "\"'"` → `/dev/null}]}}`
- `TrimRight(path, "},")` → remove `}}` → encontra `]` (NÃO em `},`) → PARA → resultado: `/dev/null}]`
- É `/dev/null}`? **NÃO** → retorna `false` → exemption NÃO dispara → RC=2.

**A diferença é a presença do `]` de fechamento de array**: `"]}"` vs `"}}"`.

## Impacto

Um agente adversarial (ou prompt injection) poderia incluir `> /dev/null` junto ao JWT no campo
`new_str` de um Amazon Q `fs_write` para contornar o credential guard. Não há exigência de arquivo
nem de permissão especial — é injeção de texto puro.

Para Windsurf `pre_write_code`, o mesmo ataque NÃO funciona por acidente da estrutura JSON (array).

## Correção programada (ML-1C da REQ-2026-10-06)

A exemption `credIsAllEphemeral` deve ser aplicada SOMENTE quando Layer 1 NÃO encontrou match
(atualmente é aplicada mesmo quando Layer 1 encontrou). Ou: distinguir payloads de write de
payloads de execute antes de aplicar a exemption.

Candidato mais simples: não chamar `credIsAllEphemeral` quando `match` foi definido pela Layer 1
(linhas 91-93 em `credential.go`). A exemption foi projetada para o caso onde Layer 2a encontrou
o JWT em um arquivo apontado por redirect — mas não para o caso onde Layer 1 encontrou JWT direto
no payload e o redirect é parte do CONTEÚDO escrito.

## Referências

- `internal/guard/credential.go` — `credIsAllEphemeral`, `credRedirectRe`, `credIsEphemeralTarget`
- `docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md` — Vector EE4
- REQ-2026-10-06, ML-1C — correção programada

## Alcance maior que o medido no Amazon Q (arquiteto, 2026-10-08)

Remedido com o binário desta árvore num projeto `credential_guard.mode: block`: o mesmo bypass vale para `Write` e
`Edit` do **Claude Code** (`tool_input.content` / `new_string` terminando em `> /dev/null"}}`) → RC 0. Não é
peculiaridade do Amazon Q: a isenção efêmera é avaliada sobre o JSON bruto inteiro, não sobre o comando de shell.
Corrigido na REQ-2026-10-06, ML-1C.

> Não escreva um JWT de exemplo literal em nota, doc ou comando: o guard global do próprio repositório bloqueia a
> tool call que o contém. Monte o token por concatenação dentro do script de teste.
