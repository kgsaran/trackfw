---
status: wip
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md"
squad: ""
---

# Roadmap: gate escreve na arvore que audita e a guarda existente nao cobre o culpado

> Created: 2026-09-17 | Status: wip


## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — superfície de ataque de um gate que escreve na árvore auditada
**Status:** ✅ Concluído e auditado pelo arquiteto (2026-09-17)
**Papel:** `hades-tf`

A pergunta não é "o gate sujou o arquivo": é **o que um gate com escrita na árvore auditada permite**.
Pontos a cobrir:

- Um PR hostil pode **induzir** um gate a escrever conteúdo escolhido na árvore? O `init` grava a
  partir de `trackfw.yaml`, `.claude/`, env — algum desses é influenciável pelo PR sob teste?
- 🔴 O dano concreto já observado foi **rebaixar a política de governança** (`governance_mode`
  removido ⇒ warning vira erro; o inverso também é possível ⇒ erro vira warning). Um PR que
  silenciasse violações via efeito colateral de gate seria detectado?
- A guarda proposta (comparar `git status --porcelain` antes/depois) é **suficiente**? Enumere o que
  ela não vê: escrita fora do repositório (`$HOME`, `/tmp`, `~/.trackfw/`), escrita seguida de
  reversão dentro do mesmo gate, e mudança de **modo** de arquivo.
- O `GEMINI.md` criado por gate e commitado por acidente: que classe de arquivo um gate pode
  introduzir na árvore sem ninguém notar?

**Critérios de aceite:** parecer escrito com os vetores enumerados e, para cada um, se a correção
proposta na REQ o fecha, o mitiga ou não o toca. 🔴 Vetor que a correção **não** fecha tem de estar
nomeado — um parecer que só confirma o plano não mediu nada.

**Gates da wave 0:**
```bash
# O parecer existe e responde as cinco perguntas — nao um arquivo vazio.
P=docs/portabilidade/2026-09-17-threat-model-gate-escreve-na-arvore.md
test -s "$P" || { echo "FAIL: parecer ausente ou vazio: $P"; exit 1; }
for termo in "guarda" "governance_mode" "GEMINI.md" "ML-6I"; do
  grep -qi -- "$termo" "$P" || { echo "FAIL: parecer nao cobre: $termo"; exit 1; }
done
# A exigencia e DECLARAR a incerteza; a grafia varia entre agentes ("Residual
# Declarado" vs "o que nao consegui determinar"). Casar literal aqui mediria a
# palavra, nao a propriedade — foi o que este gate fez na primeira versao.
grep -qiE "residual|nao consegui|não consegui|indetermin" "$P" \
  || { echo "FAIL: parecer nao declara o que ficou em aberto"; exit 1; }
echo "OK [wave0/threat-model]: parecer presente e cobre os cinco eixos"
```

**Resultado auditado:** 🔴 o achado bloqueante foi que a guarda proposta seria satisfeita
**vacuamente**: `git status --porcelain` reporta estado, nao conteudo, e num path ja sujo a saida e
byte-identica antes e depois de nova escrita. Reproduzi no arquivo real. Localmente o `parity-rest`
(que contem o gate culpado) roda **antes** do `parity-falsify`, entao o Cenario 18 tiraria o *before*
de uma arvore ja suja pelo proprio dano. O AC3 foi emendado para exigir comparacao **com conteudo** e
partida de arvore comprovadamente limpa. Residuais R4, R5 e R6 nomeados na REQ.

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.**

### ML-1A — isolar o `cwd` e enumerar a cobertura da guarda
**Status:** ⬜ Pendente
**Papel:** `ares-tf`
Cobre AC1, AC2, AC3 (emendado), AC4 e AC5.

🔴 **Entrada obrigatoria da Wave 0:** a guarda precisa comparar **conteudo**, nao so o codigo de
status; e a falsificacao parte de arvore comprovadamente limpa, falhando nomeada se nao estiver.
🔴 **R6:** se a correcao do AC2 for outra lista — ainda que gerada —, o defeito de fundo continua.

---
