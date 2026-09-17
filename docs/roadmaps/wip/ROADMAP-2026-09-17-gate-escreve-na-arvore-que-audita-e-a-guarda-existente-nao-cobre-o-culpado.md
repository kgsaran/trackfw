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
**Status:** ✅ Concluído — auditado pelo arquiteto (2026-09-17), com uma correção de auditoria
**Papel:** `ares-tf`
Cobre AC1, AC2, AC3 (emendado), AC4 e AC5.

🔴 **Entrada obrigatoria da Wave 0:** a guarda precisa comparar **conteudo**, nao so o codigo de
status; e a falsificacao parte de arvore comprovadamente limpa, falhando nomeada se nao estiver.
🔴 **R6:** se a correcao do AC2 for outra lista — ainda que gerada —, o defeito de fundo continua.

---

---

### Auditoria do arquiteto — 2026-09-17

**Aprovado no mérito.** A cobertura deixou de ser lista: o Cenário 18 usa `find` **na execução** e
decide o escopo **lendo o fonte de cada gate** (`GO_BIN|bin/trackfw`). É classificação, não
enumeração de nomes — um gate novo nasce **dentro** da cobertura sem ninguém editar nada. Fecha o R6,
que era a armadilha nomeada: o ML-6I de julho errou exatamente por codificar "os gates hoje
conhecidos como limpos". As duas exclusões vêm com motivo escrito ao lado.

A guarda passou a comparar **OID de árvore** (`git write-tree` sobre cópia limpa) em vez de
`--porcelain`. O contra-braço mostra a guarda antiga imprimindo `OK` enquanto o `trackfw.yaml` era
sobrescrito — a vacuidade que a Wave 0 mediu.

**Verificado no artefato, não no relatório:** restaurei o `trackfw.yaml`, rodei o gate corrigido, e a
árvore ficou limpa. `make quality` RC=0, e `trackfw.yaml` **não** aparece no `git status` ao final.

🔴 **Uma correção de auditoria (ML-1A-bis), porque o remédio introduziu vacuidade nova.** O
`(cd "$WORK/project" && …)` fez o `GO_BIN` **relativo** deixar de resolver:

```
saiu com exit 127 — nao travou
```

`127` é "comando não encontrado". O gate existe para provar que o binário **não trava** sem TTY — e
um binário que **não existe** também não trava. O relatório original chamou o exit code de
"irrelevante"; não é: é a assinatura de que o objeto medido não foi executado. Pior, a guarda `-x` da
linha 25 já protegia contra isso, mas roda **antes** do `cd` — o `cd` a derrotou.

Impacto medido antes de dimensionar: `FALSIFY_GO_BIN` e o default do gate são absolutos, então CI e
`make quality` **não** quebravam. Era fragilidade latente. Corrigido normalizando `GO_BIN` antes da
guarda, no padrão que **17 dos 18** gates do Cenário 18 já usavam — o `check-tty-detection.sh` era o
único sem, o que é coerente com ele ser o único culpado.

**Falsificação reproduzida por mim:** `GO_BIN` relativo → `exit 0` (binário executa); `GO_BIN`
inexistente → recusa nomeada com `RC=1`, medido **sem pipe**, porque `| tail` devolve o `$?` do
`tail`.

**AC5 — `GEMINI.md`: mantido.** Não há consumidor automatizado, mas o arquivo cumpre para o Gemini CLI
o mesmo papel que o `CLAUDE.md` cumpre para o Claude Code. E, com a guarda nova, qualquer reescrita
acidental futura muda o OID e é pega — o arquivo rastreado virou ativo, não passivo. A divergência de
conteúdo em relação ao `CLAUDE.md` é o mecanismo do #376, com REQ própria.

