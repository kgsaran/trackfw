---
status: Open
date: 2026-08-20
author: ""
adr: ""
roadmap: ""
---

# REQ: `note_orphan` existe em Go e Python e está ausente do CLI Node

> Date: 2026-08-20 | Status: Open (backlog, sem roadmap)

## Motivação

Achado **lateral** do ML-1A do contrato pinado, ao anotar a seção `## Vault de conhecimento` como
contrato sem gate. Confirmado por mim, por medição:

```
grep -rc note_orphan internal/validator/validator.go   -> 3
grep -rc note_orphan pypi/trackfw/validator.py         -> 4
grep -rn note_orphan npm/src/                          -> NENHUMA ocorrência
```

E `docs/cli-parity.md:147` documenta a regra como contrato, com tabela de severidade padrão,
escalação via `rules: { note_orphan: error }` e comportamento na ausência de `vault/`.

**Violação viva da regra dura de paridade dos 3 CLIs.** Um projeto governado pelo CLI Node não tem
detecção de nota órfã, e `trackfw validate` passa verde onde os outros dois acusariam.

### O que este achado prova sobre a REQ que o gerou

A `REQ-2026-08-18-contrato-pinado-sem-gate-nomeado` sustenta que contrato sem gate nomeado é
contrato não-aplicado. Aqui está a demonstração, e ela não foi construída: **a violação apareceu no
primeiro contato com o mecanismo, antes mesmo de o mecanismo existir** — bastou alguém perguntar
"qual gate protege esta seção?" para descobrir que não há gate, e que por isso a seção descreve algo
que um dos três CLIs não faz.

Vale registrar como evidência a favor de priorizar o mecanismo, não só as instâncias.

## Escopo

1. Implementar `note_orphan` em `npm/src/validator/`, com paridade de comportamento: severidade
   padrão, escalação por `rules:`, comportamento sem `vault/`, e os formatos de detecção de link.
2. **Gate comparando as três saídas reais** — teste por stack não fecha. Esta série já provou cinco
   vezes que cada runtime concorda consigo mesmo.
3. Cenário P4 com braço de baseline e de detecção.

## O que **não** é escopo

- Mudar a semântica da regra em Go ou Python. A referência é o comportamento existente; se houver
  divergência entre os dois, **isso é achado**, e vira decisão antes de codificar.
- Varrer as demais regras em busca de ausências parecidas. É trabalho legítimo, mas é o ML-2A do
  contrato pinado que vai revelá-las de forma sistemática — fazer manualmente aqui duplicaria o
  esforço e ainda ficaria incompleto.

## Acceptance Criteria

- [ ] AC1 — `note_orphan` implementada no CLI Node, com paridade de comportamento.
- [ ] AC2 — **Antes de codificar**, verificado se Go e Python concordam entre si; divergência é decisão.
- [ ] AC3 — Gate comparando as **três saídas reais** — não por leitura de fonte.
- [ ] AC4 — Cenário P4, baseline + detecção.
- [ ] AC5 — Seção do `docs/cli-parity.md` atualizada, **nomeando o gate**.
- [ ] AC6 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **Falso-positivo em nota legítima** treina o usuário a ignorar o `validate`. Os formatos de
  detecção de link precisam ser exatamente os mesmos dos outros dois CLIs.
- **Não presumir que Go e Python concordam** — a REQ existe justamente porque uma suposição de
  paridade estava errada.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- nenhum; é implementação de regra já decidida -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->

---

## Triagem medida — 2026-09-12 (ML-1B)

**Veredito: PARCIAL — não fechar.**

### 🔴 A evidência original desta REQ era falsa

A evidência original usou `grep -rn note_orphan npm/src/` e obteve "NENHUMA ocorrência". Isso era
um **falso negativo**: o `grep` do ambiente é `ugrep 7.8.4` com o flag `-I` ativo por padrão.
`ugrep -I` trata arquivos com byte NUL como binários e os **pula em silêncio** (RC=1, sem output).

`npm/src/validator/index.js` contém um byte NUL literal (offset 83123). Por isso o arquivo foi
omitido, e a busca concluiu "ausente" quando o código estava lá desde a origem.

Medição que prova:

```bash
# grep do shell (ugrep 7.8.4 com -I)
grep -c note_orphan npm/src/validator/index.js   →  (vazio), RC=1

# /usr/bin/grep (grep nativo do sistema)
/usr/bin/grep -c note_orphan npm/src/validator/index.js   →  3, RC=0
```

Referência completa: `vault/notes/grep-do-ambiente-pula-arquivo-com-nul-2026-09-12.md`

### Medição de presença (/usr/bin/grep, ML-1A)

```
/usr/bin/grep -rn "note_orphan" internal/validator/validator.go   →  4 ocorrências (linhas 170,495,795,2995)
/usr/bin/grep -rn "note_orphan" npm/src/validator/index.js        →  3 ocorrências (linhas 1621,3469,3758)
/usr/bin/grep -rn "note_orphan" pypi/trackfw/validator.py         →  5 ocorrências (linhas 309,2100,2123,2144,4272)
```

**A regra existe nos 3 runtimes.** A premissa da abertura desta REQ estava errada.

### Medição de comportamento (3 binários 7.6.0, ML-1A)

Fixture: `vault/notes/index.md` vazio + `vault/notes/nota-orfa-2026-09-12.md` não linkada.

```
./bin/trackfw validate --json        →  rule: "note_orphan" - note "nota-orfa..." not referenced
node npm/bin/trackfw validate --json →  rule: "note_orphan" - note "nota-orfa..." not referenced
python3 -m trackfw validate --json   →  rule: "note_orphan" - note "nota-orfa..." not referenced
```

Os 3 runtimes detectam e nomeam corretamente.

### Status por AC

| AC | Status | Evidência |
|---|---|---|
| AC1 — `note_orphan` no Node | **Entregue** | `npm/src/validator/index.js:1621,3469,3758`; binário detecta nota órfã |
| AC2 — Go e Python concordam antes de codificar | N/A post-facto | os 3 concordam; processo não é mais verificável |
| AC3 — Gate comparando **3 saídas reais** | **Pendente** | `check-artifact-closed-cycle.sh` testa por runtime independentemente; `cli-parity.md:148` documenta `partial=check-artifact-closed-cycle.sh` explicitamente |
| AC4 — Cenário P4, baseline + detecção | **Entregue** | `check-artifact-closed-cycle.sh`: `note_orphan-silent-for-indexed` + `note_orphan-fires-for-unindexed` para os 3 runtimes |
| AC5 — `cli-parity.md` atualizado, gate nomeado | **Parcial** | gate nomeado (`check-artifact-closed-cycle.sh`) mas documentado como `partial=`; satisfação plena depende do AC3 |
| AC6 — `make quality` verde e CI verde | **Não medido neste ML** | AC3 e AC5 pendentes; gate não pode ser verde enquanto o critério de comparação cross-CLI não existir |

**AC pendentes: AC3, AC5 (depende de AC3), AC6 (não medido).**

O AC3 e o AC5 serão fechados via ML-2A do roadmap
`ROADMAP-2026-09-12-triagem-medida-das-reqs-de-paridade-e-gate-de-conjunto-de-regras.md`.
É a Regra Dura de Causa Raiz: mesma causa (ausência de gate cross-CLI), mesma REQ, mesmo roadmap.
Não abrir REQ nova.
