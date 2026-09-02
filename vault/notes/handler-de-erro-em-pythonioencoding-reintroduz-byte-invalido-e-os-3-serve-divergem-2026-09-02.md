# `PYTHONIOENCODING=utf-8:<handler>` reintroduz byte inválido, e os 3 `serve` divergem ao lê-lo

> 2026-09-02 · `hades-tf` (Security) · barreira do
> `ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`
> Parecer: `docs/seguranca/2026-09-02-parecer-codificacao-declarada.md` (achado S3)

## O que parece verdade e não é

`scripts/check-output-encoding-declared.sh:45-49` justifica aceitar o sufixo de handler assim:

> "com encoding utf-8, nenhum str do Python e inencodavel: o handler nunca dispara e nao ha dado
> escondido"

**Falso.** `json.load` **preserva** surrogate isolado vindo de escape `\udXXX` — JSON permite o
escape e o módulo `json` não valida pareamento. Surrogate isolado **é** inencodável em UTF-8, então
o handler dispara.

## Medido

Hook `attentionSignalScript`, payloads `{"tool_input":{"question":"x\ud800y"}}` e `"a\udcffb"`:

```
utf-8                  -> UnicodeEncodeError -> fallback "Agent needs attention"   (seguro)
utf-8:backslashreplace -> message="\\ud800"                                        (inofensivo)
utf-8:surrogatepass    -> b'…"message":"x\xed\xa0\x80y"…'   <- ED A0 80 cru
utf-8:surrogateescape  -> b'…"message":"a\xffb"…'           <- 0xFF cru
```

As duas últimas são **aceitas hoje** por `DECL_RE` (`:141-146`) e `PREFIX_RE` (`:283-287`), cujo
sufixo é `(?::[A-Za-z0-9_]+)?`. Em árvore-sandbox com os 3 geradores reescritos para
`utf-8:surrogatepass` o gate devolve **exit 0** — o controle anti-reintrodução greenlighta a
regressão que existe para pegar.

## A parte que custa tempo: os 3 `serve` divergem em arquivo inválido em UTF-8

`.trackfw-attention.json` com byte inválido (`ED A0 80`, `0xFF`, ou o `0x97` que o hook **pré-ML-1A**
produzia sob cp1252 — este é alcançável sem handler nenhum):

| runtime | leitura | resultado |
|---|---|---|
| Go `internal/serve/api_attention.go:33-38` | `json.Unmarshal` | **não erra**, substitui por U+FFFD → `active:true`, banner com `x���y` |
| Node `npm/src/serve/api_attention.js:9-11` | `readFileSync(…,'utf8')` | U+FFFD → `active:true` (igual ao Go) |
| Python `pypi/trackfw/serve/api_attention.py:13-16` | `open(encoding="utf-8")` | `UnicodeDecodeError` **⊂ `ValueError`** → capturado → **`{"active": False}`** |

O CLI Python **apaga o banner** onde Go e Node o exibem. Não há crash, não há 500, não há log — o
sinal de atenção simplesmente não aparece. Quem debugar "o banner não sobe no CLI Python" sem esta
nota vai procurar no `app.js`, no polling de 8 s e no `roadmap_dir` antes de suspeitar de um byte.

## Armadilha de método

Reproduzir cp1252 no macOS **por locale não funciona** (PEP 540 sob `LC_ALL=C` — ver
`gate-em-cp1252-tem-duas-falhas-distintas-crash-de-print-e-mismatch-por-transcodificacao-2026-09-02`).
Use `PYTHONIOENCODING=cp1252` explícito sobre a forma **antiga** da linha.

E: payload gerado por `json.dump` é **ASCII puro** (`ensure_ascii=True`), então só exercita o
**encode**. Para exercitar o **decode** de stdin — que é o discriminante — é preciso `printf` com
bytes crus (`\xc3\x81` para o braço "aceito agora", `\xe9` para o braço "rejeitado agora").

## Remédio

Restringir o sufixo a handlers que não podem emitir byte inválido:
`(?::(?:strict|replace|backslashreplace|xmlcharrefreplace|namereplace))?` — recusando
`surrogatepass` e `surrogateescape` — e corrigir o comentário `:45-49`.
