---
title: thirdparty_artifact_has_provenance — chave absoluta vs. relativa e checksum bruto vs. normalizado
date: 2026-08-15
tags: [go, node, python, validator, thirdparty, paridade, adr-imprecisao]
---

## Contexto

REQ/ADR-2026-08-15 (instalação de skills de terceiro via URL), ML-3A (`apolo-tf`). O ADR
especificava a regra `trackfw validate` `thirdparty_artifact_has_provenance` (D2) e o campo
`Claim.Origin` (D11) em prosa, mas duas premissas implícitas do texto não sobreviveram ao contato
com o comando real (`third-party install`). Ambas foram descobertas empiricamente — via um teste
Go temporário que rodava o comando real e dumpava o estado em disco — não por releitura do ADR.

## Achado 1 — domínio de chave: absoluto (manifest) vs. relativo (`.trackfw/thirdparty-*`)

`integrations-manifest.json` usa como chave o **destino absoluto resolvido**
(`Manager.resolve()` em Go; equivalentes em Node/Python). Os 3 schemas novos —
`.trackfw/thirdparty-quarantine/<checksum>.json`, `.trackfw/thirdparty-provenance.json` e
`.trackfw/thirdparty-references.json` — usam o destino **relativo à raiz do projeto** (o valor
pré-`resolve()`).

Uma implementação inicial ingênua (nos 3 CLIs, feita antes desta descoberta) usava o destino
absoluto do manifest como chave de busca em `thirdparty-provenance.json`. Resultado: a entrada de
provenance NUNCA era encontrada, mesmo em uma instalação legítima e aprovada — falso-positivo
sistemático no ramo (i) da regra ("destino de terceiro sem provenance"), silencioso até um teste
que exercitasse o comando `install` real de ponta a ponta (os testes unitários hand-authored
mascaravam isso porque escreviam a fixture de provenance já pré-alinhada com o bug).

**Fix:** `filepath.Rel(root, destination)` (Go), `path.relative(root, destination)` (Node),
`os.path.relpath(destination, root)` (Python) como chave de busca/escrita em todos os 3 schemas.

**Como confirmar rápido amanhã:** se `thirdparty_artifact_has_provenance` sinalizar ramo (i) em
uma instalação que você sabe que foi aprovada corretamente, o primeiro suspeito é sempre domínio
de chave (absoluto vs. relativo), não ausência real de provenance.

## Achado 2 — checksum de provenance é dos bytes BRUTOS, não do arquivo instalado (D6)

`checksum_sha256` em `thirdparty-provenance.json` é o SHA-256 do conteúdo **bruto** buscado por
`fetch`, ANTES de `NormalizeThirdPartyContent` (`TrimSpace(raw) + "\n"`). O arquivo efetivamente
instalado em disco é sempre o conteúdo **normalizado**. Comparar `checksum_sha256` diretamente
contra o SHA-256 do arquivo instalado teria produzido falso-positivo em qualquer instalação
legítima cujo bruto não fosse já canônico (ex.: espaço/newline extra no início/fim) — a maioria
dos casos reais.

**Resolução:** usar o registro de quarentena (`.trackfw/thirdparty-quarantine/<checksum>.json`,
que guarda `content_base64` do bruto) como ponte entre os dois domínios:
1. Recalcular SHA-256 do `content_base64` da quarentena e confirmar que bate com o
   `checksum_sha256` da provenance (auto-consistência quarentena↔provenance).
2. Normalizar esse conteúdo bruto e comparar byte-a-byte contra o arquivo instalado.

Se o registro de quarentena estiver ausente (foi apagado, ou nunca existiu), a regra falha
FECHADO (D8f) em vez de degradar silenciosamente — não há como reconstruir o bruto a partir do
normalizado (a normalização não é reversível).

**Teste de regressão load-bearing, presente nos 3 CLIs:** fixture cujo bruto é
`"\n# hello\n\nsome content\n\n\n"` e cujo instalado é `strip + "\n"` — deliberadamente NÃO
canônico no bruto, para provar que a comparação direta checksum-bruto vs. hash-do-instalado teria
falhado. Não enfraquecer essa fixture para conteúdo já canônico.

## Por que isso importa para quem tocar esta regra depois

Qualquer alteração em `NormalizeThirdPartyContent`/equivalentes, ou em como o destino é resolvido
antes de virar chave de manifest, precisa revisitar os dois achados acima — são acoplamentos
implícitos entre domínios (bruto/normalizado, absoluto/relativo) que o ADR não tornou explícitos
e que um teste unitário isolado por linguagem não pega sozinho.

## Referências

- `internal/validator/validator_thirdparty_provenance.go` (implementação canônica, comentários
  inline com a mesma explicação)
- `internal/commands/integrations_thirdparty_validate_test.go` (teste end-to-end via comando real
  que expôs o achado 1)
- `internal/validator/validator_thirdparty_provenance_test.go` /
  `npm/tests/validator.test.js` / `pypi/tests/test_validator_thirdparty_provenance.py`
  (`branch_ii_legitimate_install_does_not_false_positive` — teste de regressão do achado 2)
- `docs/cli-parity.md`, seção "`trackfw third-party` — instalação de skills de terceiro via URL"
