---
name: handler-utf8-reintroduz-byte-invalido-e-serve-diverge
description: check-output-encoding-declared.sh aceita PYTHONIOENCODING=utf-8:surrogatepass/surrogateescape, que escrevem byte invalido no .trackfw-attention.json; ao le-lo Go/Node dao active:true com U+FFFD e Python da active:false
metadata:
  type: project
---

Achado 2026-09-02 (barreira `fix/saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`),
**APROVA COM RESSALVAS**, não corrigido — vira REQ de acompanhamento.

O gate novo `scripts/check-output-encoding-declared.sh` aceita o sufixo de handler
(`(?::[A-Za-z0-9_]+)?` em `DECL_RE` e `PREFIX_RE`) justificando por escrito que "com encoding utf-8
nenhum str do Python é inencodável". **Falso**: `json.load` preserva surrogate isolado de `\udXXX`.
`utf-8:surrogatepass` → `ED A0 80`; `utf-8:surrogateescape` → `0xFF`. Gate devolve exit 0.

**Why:** o controle anti-reintrodução greenlighta a própria regressão que existe para pegar, e o
consumidor diverge nos 3 CLIs: Go (`api_attention.go:33-38`) e Node (`api_attention.js:9-11`)
devolvem `active:true` com U+FFFD; Python (`api_attention.py:13-16`) devolve **`active:false`**
porque `UnicodeDecodeError` ⊂ `ValueError` — o banner some sem crash e sem log.

**How to apply:** ao revisar qualquer gate que aceite "formas equivalentes" por regex, exigir que a
justificativa escrita seja testada como universal — foi o comentário, não o código, que estava
errado. E ao investigar "banner de atenção não aparece só no CLI Python", suspeitar de byte inválido
no arquivo antes de olhar `app.js`. Detalhe completo em
`vault/notes/handler-de-erro-em-pythonioencoding-reintroduz-byte-invalido-e-os-3-serve-divergem-2026-09-02.md`
e `docs/seguranca/2026-09-02-parecer-codificacao-declarada.md`.
Relacionado: [[feedback-medir-decode-e-encode-separadamente]].
