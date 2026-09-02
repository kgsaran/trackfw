---
name: feedback-medir-decode-e-encode-separadamente
description: PYTHONIOENCODING move decode de stdin E encode de stdout; payload de json.dump e ASCII puro e so exercita o encode — usar printf com bytes crus para o decode
metadata:
  type: feedback
---

Ao revisar qualquer mudança de codificação, medir **decode de stdin** e **encode de stdout** como
dois canais separados, com payload próprio para cada um.

**Why:** na barreira de 2026-09-02 eu escrevi §1 inteira com payloads gerados por `json.dump`, que
usa `ensure_ascii=True` — todos ASCII puro no fio. O decode teve sucesso trivial nos dois braços, e
eu declarei "negativo medido" para um canal que na verdade só tinha sido **deduzido**. O advisor
pegou. A nota de vault do próprio projeto já dizia que o discriminante era o decode, não o encode.

**How to apply:** para o decode use `printf` com bytes crus, um payload por direção — `\xc3\x81`
(UTF-8 válido cujo byte é indefinido em cp1252: "antes rejeitava, agora aceita") e `\xe9` (latin-1
inválido em UTF-8: "antes passava, agora rejeita"). Para reproduzir cp1252 no macOS **não use
locale** (PEP 540 sob `LC_ALL=C` não discrimina): use `PYTHONIOENCODING=cp1252` explícito sobre a
forma antiga da linha. E quando uma afirmação estrutural sustentar a conclusão ("nenhum ponto
não-ASCII codifica para 0x22/0x5C"), varrer os 1.114.112 pontos de código em vez de argumentar —
custa 3 segundos e converte a peça mais frágil do parecer em medição.
Relacionado: [[feedback-verify-by-execution]], [[feedback-execute-all-named-vectors-before-verdict]].
