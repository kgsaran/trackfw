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

**Resolução original do ML-3A, SUBSTITUÍDA no ML-3B (ver "Achado 2-bis" abaixo) — mantida aqui só
como contexto histórico do erro de desenho:** usar o registro de quarentena
(`.trackfw/thirdparty-quarantine/<checksum>.json`, que guarda `content_base64` do bruto) como
ponte entre os dois domínios (recalcular SHA-256 do `content_base64` e confirmar que bate com
`checksum_sha256`, depois normalizar e comparar byte-a-byte contra o instalado). Correta, mas
**não usar mais** — tornava um artefato de estágio dependência obrigatória de um gate permanente
(ver Achado 2-bis).

## Achado 2-bis (ML-3B, ADR-2026-08-15 D2-bis) — a ponte via quarentena era o desenho errado

A resolução do ML-3A acima estava tecnicamente correta (por isso passava em `make quality`), mas a
auditoria do arquiteto recusou o DESENHO: fazia `.trackfw/thirdparty-quarantine/` — que tem nome,
forma e propósito de diretório de ESTÁGIO, destinado a ser podado — virar dependência
**obrigatória** de um gate **permanente** (`trackfw validate`). Quem apagasse ou colocasse esse
diretório no `.gitignore` faria a ramificação (ii) falhar para sempre, sem caminho de recuperação
(`validate` nunca faz fetch de rede, D6, e a normalização não é reversível para reconstruir o
bruto a partir do arquivo instalado).

**Fix (D2-bis):** segundo campo na entrada de provenance, `installed_sha256` = SHA-256 dos bytes
**NORMALIZADOS**, calculado por `third-party install` no momento em que grava o arquivo de
destino (não pelo aprovador — ele só conhece o bruto). `checksum_sha256` continua intocado, é a
âncora de aprovação D8c. A ramificação (ii) passa a comparar `sha256(arquivo instalado)` direto
contra `entry.installed_sha256` — dois domínios já normalizados, sem ponte via terceiro arquivo. A
quarentena continua sendo escrita/commitada (valor de auditoria), mas sua ausência **deixou de
ser erro** desta regra. `schema_version` da provenance foi de `1` para `2` (sem migração — a
feature nunca tinha um arquivo real no mundo no momento do bump).

**Armadilha de paridade encontrada ao implementar:** o Go grava `installed_sha256` na ordem certa
de graça, porque é um campo de struct (`ProvenanceEntry`) — a ordem de serialização segue a ordem
de declaração do campo. O Python também, porque `provenance.py` já canonicaliza a ordem dos campos
na escrita (`_ENTRY_FIELD_ORDER`) por ser uma linguagem dinamicamente tipada com escritor externo.
O **Node não tem nenhum dos dois mecanismos**: `writeProvenance` só serializa o objeto JS como
está, e um `{ ...existing, installed_sha256: x }` (spread) preserva a ordem de inserção original e
**apenda** a chave nova no FIM do objeto — divergindo da posição "logo após `checksum_sha256`" que
Go/Python produzem. `scripts/check-thirdparty-parity.sh` compara `thirdparty-provenance.json`
byte a byte entre os 3 CLIs depois de um install real, e pegou isso na primeira rodada (Node
sozinho, checksum idêntico, ordem de campo diferente). Fix: construir o objeto Node com a ordem de
chaves explícita (não spread), removendo chaves ausentes depois.

**Como confirmar rápido amanhã:** se um campo novo entrar em qualquer schema `.trackfw/thirdparty-*`
no Node e `check-thirdparty-parity.sh` reclamar de "differs across CLIs" com conteúdo
semanticamente igual, o primeiro suspeito é ordem de campo via spread — Node é o único dos 3 CLIs
sem canonicalização automática de ordem na escrita de provenance.

**Teste de regressão load-bearing, presente nos 3 CLIs (achado 2 e 2-bis):**
`branch_ii_legitimate_install_does_not_false_positive` — fixture cujo bruto é
`"\n# hello\n\nsome content\n\n\n"` e cujo instalado é `strip + "\n"`, com `checksum_sha256` e
`installed_sha256` deliberadamente DIFERENTES na fixture (para provar que a ramificação (ii) usa o
campo certo). `branch_ii_quarantine_deletion_does_not_break_clean_install` /
`branch_ii_quarantine_deletion_still_detects_tamper` — apagam
`.trackfw/thirdparty-quarantine/` inteiro antes de validar (o critério de aceite do ML-3B, ao pé
da letra). Não enfraquecer nenhuma dessas fixtures.

## Por que isso importa para quem tocar esta regra depois

Qualquer alteração em `NormalizeThirdPartyContent`/equivalentes, em como o destino é resolvido
antes de virar chave de manifest, ou em qualquer campo novo de `ProvenanceEntry`, precisa
revisitar os achados acima — são acoplamentos implícitos entre domínios (bruto/normalizado,
absoluto/relativo, campo-novo-vs-ordem-de-serialização) que o ADR não tornou explícitos e que um
teste unitário isolado por linguagem não pega sozinho.

## Referências

- `internal/thirdparty/provenance.go` (`ProvenanceEntry`, doc comment com a explicação completa
  dos dois hashes/domínios)
- `internal/validator/validator_thirdparty_provenance.go` (implementação canônica da ramificação
  (ii), comentários inline)
- `internal/commands/integrations_thirdparty.go` (quem escreve `installed_sha256`, e só ali)
- `npm/src/commands/thirdparty.js` (construção explícita de ordem de campo, NÃO spread)
- `pypi/trackfw/thirdparty/provenance.py` (`_ENTRY_FIELD_ORDER`)
- `internal/commands/integrations_thirdparty_validate_test.go` (teste end-to-end via comando real
  que expôs o achado 1)
- `internal/validator/validator_thirdparty_provenance_test.go` /
  `npm/tests/validator.test.js` / `pypi/tests/test_validator_thirdparty_provenance.py`
  (testes de regressão dos achados 2 e 2-bis)
- `scripts/check-thirdparty-parity.sh` (compara `thirdparty-provenance.json` byte a byte após um
  install real — pegou a armadilha de ordem de campo do Node)
- `docs/cli-parity.md`, seção "`trackfw third-party` — instalação de skills de terceiro via URL"
  (subseção "D2-bis — dois hashes, dois domínios")
- `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md`,
  seção D2-bis
