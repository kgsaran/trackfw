---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: Regra `thirdparty_artifact_has_provenance` existe em Go e Python, mas não no validator do Node

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do ML-1C da `REQ-2026-08-30-caminho-portavel-...`, e é resultado de **uma hipótese minha que
estava errada**: eu supus que a regra existia **só no Go**. Medido:

| arquivo | ocorrências |
|---|---|
| `internal/validator/validator_thirdparty_provenance.go` | 7 |
| `pypi/trackfw/validator.py` | 10 |
| `npm/src/validator/index.js` | **0** |

**Falta no Node**, não no Go. E o erro da hipótese foi útil: ao checar o Python, descobri que ele
tinha **o mesmo bug de separador** do Go (`os.path.relpath` devolve nativo igual ao `filepath.Rel`) —
corrigido no PR #231. Se eu tivesse acertado a hipótese, teria corrigido só o Go e **introduzido
quebra de paridade dentro da REQ que existe para corrigir divergência**.

## Por que importa

`thirdparty_artifact_has_provenance` é a regra que garante que **artefato de terceiro instalado tem
aprovação de proveniência registrada**. É controle de cadeia de suprimentos.

**Um usuário que roda `trackfw validate` pelo CLI Node não tem esse controle** — e não recebe aviso
de que não tem. O `validate` reporta limpo, porque a regra não existe ali para reprovar.

Mesma família que já nos custou sete ocorrências: **o mecanismo dá sinal verde enquanto o controle
está ausente.** Aqui a ausência é por runtime, o que é pior — o mesmo comando, no mesmo repositório,
dá respostas diferentes conforme quem o executa.

## Acceptance Criteria

- [ ] **AC1** — A regra existe no validator do Node, com **mesma mensagem e mesma severidade** dos
      outros dois.
- [ ] **AC2** — A chave de busca é montada com `/`, não com separador nativo — o mesmo defeito
      corrigido em Go e Python no PR #231. **Nascer já correta**, não repetir o ciclo.
- [ ] **AC3** — 🔴 **Falsificação nos 3 runtimes**: artefato de terceiro **sem** proveniência é
      recusado pelos três, com saída equivalente; **e o controle**, artefato **com** proveniência
      passa nos três. Sem o controle, trocaríamos ausência de regra por regra que recusa tudo.
- [ ] **AC4** — Gate de paridade que reprove se a regra existir em N runtimes e faltar em outro.
      **Esta lacuna sobreviveu porque nenhum gate compara o conjunto de regras entre os 3
      validators** — é a causa raiz, e sem a AC4 ela reaparece na próxima regra nova.

## Negative Scope

- **Não** alterar a semântica da regra em Go ou Python; o Node se alinha aos existentes.
- **Não** tratar o achado de separador — já corrigido no PR #231.

## Linked ADR

ADR: <!-- nenhum. Fechamento de lacuna de paridade sobre regra já decidida. -->

## Linked Roadmap

Roadmap:

---

## Triagem medida — 2026-09-12 (ML-1B)

**Veredito: PARCIAL — não fechar.**

### 🔴 A evidência original desta REQ era falsa

A evidência original (Zeus, 2026-09-01) usou `grep -rn` do ambiente e obteve "0 ocorrências no
Node". Era um **falso negativo**: o `grep` do ambiente é `ugrep 7.8.4` com o flag `-I` ativo por
padrão. `ugrep -I` trata arquivos com byte NUL como binários e os **pula em silêncio** (RC=1, sem
output). `npm/src/validator/index.js` contém um byte NUL literal — mesma causa que a REQ
`note_orphan` da mesma data.

Medição que prova:

```bash
# grep do shell (ugrep 7.8.4 com -I)
grep -c thirdparty_artifact_has_provenance npm/src/validator/index.js   →  (vazio), RC=1

# /usr/bin/grep (grep nativo do sistema)
/usr/bin/grep -c thirdparty_artifact_has_provenance npm/src/validator/index.js   →  8, RC=0
```

Referência completa: `vault/notes/grep-do-ambiente-pula-arquivo-com-nul-2026-09-12.md`

### Medição de presença (/usr/bin/grep, ML-1A)

```
/usr/bin/grep -rn "thirdparty_artifact_has_provenance" internal/validator/validator_thirdparty_provenance.go
  →  9 ocorrências (linhas 24,113,135,166,176,187,196+)

/usr/bin/grep -rn "thirdparty_artifact_has_provenance" npm/src/validator/index.js
  →  8 ocorrências (linhas 3614,3654,3669,3700,3712,3728,3770)
  npm/src/commands/thirdparty.js
  →  1 ocorrência (linha 39: THIRD_PARTY_PROVENANCE_RULE)

/usr/bin/grep -rn "thirdparty_artifact_has_provenance" pypi/trackfw/validator.py
  →  12+ ocorrências (linhas 127,4112,4118,4119,4160,4174,4204,4219,4229,4310,4311)
```

**A regra existe nos 3 runtimes.** A premissa da abertura desta REQ estava errada.

### Status por AC

| AC | Status | Evidência |
|---|---|---|
| AC1 — regra no Node com mesma mensagem e severidade | **Entregue** | `npm/src/validator/index.js:3614`; `check-thirdparty-parity.sh` parte C (`Makefile:86`) verifica byte-parity da mensagem nos 3 CLIs |
| AC2 — chave montada com `/`, não separador nativo | **Entregue** | `npm/src/lib/pathfmt.js:39-43`: `normalizeRefSeparator` substitui `\` por `/`; usada na linha 3696 para `provenanceKey` |
| AC3 — Falsificação nos 3 runtimes | **Entregue** | `check-thirdparty-parity.sh` parte C (linha 421-422): `diff -u go-branch-i.msg node-branch-i.msg`; D2-bis (linha 384): install legítimo → 0 violações em 3 CLIs |
| AC4 — Gate reprova se regra existe em N e falta em outro | **Pendente** | Gate de conjunto de regras não existe; nenhum gate compara o conjunto implementado entre os 3 validators |

**AC pendente: AC4.**

O AC4 será fechado via ML-2A do roadmap
`ROADMAP-2026-09-12-triagem-medida-das-reqs-de-paridade-e-gate-de-conjunto-de-regras.md`.
É a Regra Dura de Causa Raiz: a lacuna sobreviveu porque nenhum gate compara o conjunto de regras
entre os 3 validators — é a causa raiz declarada no texto da própria REQ, e o ML-2A é a correção.
Mesma causa, mesmo roadmap. Não abrir REQ nova.
