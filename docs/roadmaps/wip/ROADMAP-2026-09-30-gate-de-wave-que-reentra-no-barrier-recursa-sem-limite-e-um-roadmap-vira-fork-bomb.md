---
status: wip
date: 2026-09-30
req: "docs/req/REQ-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md"
squad: "hades-tf"
---

# Roadmap: gate de wave que reentra no barrier recursa sem limite e um roadmap vira fork bomb

> Created: 2026-09-30 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md
Issue: #485 · Nota de vault: `vault/notes/barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`

Varredura (2026-09-30): nenhuma issue ou REQ aberta com este mecanismo. A REQ-2026-09-11 (resíduos
dos pareceres do barrier) não trata reentrada nem profundidade. O #476 compartilha `barrier.go` e
está **bloqueado atrás deste** (ver o roadmap dele, § BLOQUEADO).

**Trazido da branch do #476 para cá, por ter a mesma causa:** o desarme da mina em
`done/ROADMAP-2026-09-22-...:73` e a nota de vault. Os hunks são idênticos nas duas branches.

**Medição de partida** (extração do 1º bloco cercado após `**Gates da wave:**`, linha que começa com
`^(trackfw|./bin/trackfw|bin/trackfw)\s+barrier\b`): **0** no acervo desta branch · **1** na versão
da `main` do ROADMAP-2026-09-22 (contra-braço: o detector acusa a mina quando ela existe).

## Acceptance Criteria
- [ ] **AC1** — reentrada direta recusada antes de executar, sem multiplicar
- [ ] **AC2** — reentrada por indireção contida
- [ ] **AC3** — 🔴 gate legítimo continua rodando, inclusive o que aninha `barrier` sobre **outro** roadmap
- [ ] **AC4** — duas invocações sequenciais continuam funcionando
- [ ] **AC5** — todo outro executor de gates tem a mesma contenção
- [ ] **AC6** — acervo com 0 gates reentrantes
- [ ] **AC7** — `cli-parity.md` descreve contenção, exit code e mensagem
- [ ] **AC8** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — modelo de ameaça e escolha do discriminante
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Arquivos de leitura:** `internal/commands/barrier.go` (`runGateCommand`, `evalGateCommands`,
`runBarrier`, trust check) · `internal/commands/barrier_contract_test.go` (~:105, ~:258) ·
`scripts/check-barrier.sh` · `scripts/check-roadmap-barrier-contract.sh` · `docs/cli-parity.md`
(§ `trackfw barrier`) · o slash command `/trackfw:barrier` gerado · nota de vault acima · issue #485
**Entregável:** `docs/seguranca/2026-09-30-wave0-gate-reentrante-no-barrier.md`
**Método:** medir. 🔴 **Não escrever implementação.** A Wave 0 tem autoridade para **bloquear**.

**Perguntas:**

1. **Completude da enumeração.** Quais caminhos do produto **executam** conteúdo de roadmap? Não se
   limite ao `barrier`: procure `ship`, `commit`, `roadmap move ... done`, hooks gerados, o slash
   command e os scripts. Se algum outro executar gates, um gate que o chama também recursa, e corrigir
   só o `barrier` não fecha.
2. **Discriminante.** Avalie e **falsifique** cada candidato:
   - (a) contador de profundidade herdado por env (`TRACKFW_BARRIER_DEPTH`) que recusa acima de 1;
   - (b) inspeção estática do gate procurando o subcomando `barrier`;
   - (c) pilha herdada de chaves `(roadmap, wave)` que recusa só a reentrada na **mesma** chave, com
     teto de profundidade como backstop;
   - qualquer outro que a medição sugerir.
3. 🔴 **Contra-braço que derruba o (a) ingênuo.** O `barrier_contract_test.go` builda e **executa**
   `trackfw barrier` como processo filho, herdando o env se `cmd.Env` não for setado, e o
   `scripts/check-barrier.sh` roda `barrier` sobre fixtures. Então um gate legítimo como
   `go test ./internal/commands/...` ou `make quality` **aninha `barrier` sobre outro roadmap**, e com
   um contador cego vira erro de uso. **Meça quantos gates do acervo real caem nisso.**
4. **Exit code e mensagem.** O que é coerente com a convenção do `barrier` (exit 2 = não avaliável ≠
   `blocked`)? Reentrada é usage error ou `not_evaluated`?
5. **Resíduo declarado.** O que o desenho escolhido aceita não cobrir, dito explicitamente.

**Ataque as minhas premissas:**

- Afirmo que **nenhuma** correção de profundidade fecha "DoS por arquivo de texto", porque o gate é
  `sh -c` arbitrário e o trust check não protege clone hostil. Se isso estiver errado, **diga**: muda
  o escopo negativo da REQ.
- Afirmo que o `barrier_contract_test.go` herda o env do pai. **Verifique na linha**, não presuma.
- Afirmo **0** gates reentrantes no acervo desta branch (medição acima). Reimplemente a contagem.

**Critérios de aceite:**
- [x] Enumeração dos executores de gates, com `arquivo:linha`, declarada completa ou com o que falta
- [x] Cada candidato de discriminante com o contra-braço que o derruba ou o sustenta, **medido**
- [x] Contagem de gates do acervo que aninham `barrier` sobre outro roadmap
- [x] Exit code e mensagem propostos, com o motivo
- [x] Resíduo declarado
- [x] 🔴 Se a medição refutar qualquer premissa da REQ, **dizer**
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**

> 🔴 **O gate de uma wave NUNCA pode ser `trackfw barrier` sobre a própria wave.** É o defeito que
> este roadmap corrige.

```bash
test -f docs/seguranca/2026-09-30-wave0-gate-reentrante-no-barrier.md || { echo "GATE FALHOU: parecer da Wave 0 ausente" >&2; exit 1; }
n=$(python3 -c "
import glob,re
B=re.compile(r'^\s*(trackfw|\./bin/trackfw|bin/trackfw)\s+barrier\b')
F=re.compile(r'^\s*(\`{3,}|~{3,})')
n=0
for f in glob.glob('docs/roadmaps/**/*.md',recursive=True):
    L=open(f,encoding='utf-8',errors='replace').read().split(chr(10))
    for i,l in enumerate(L):
        if '**Gates da wave:**' not in l: continue
        j=i+1
        while j<len(L) and not F.match(L[j]): j+=1
        j+=1
        while j<len(L) and not re.match(r'^\s*(\`{3,}|~{3,})\s*$',L[j]):
            if B.match(L[j]): n+=1
            j+=1
print(n)
"); test "$n" = "0" && echo "Gate W0 OK: parecer presente, 0 gates reentrantes no acervo" || { echo "GATE FALHOU: $n gates que invocam barrier no acervo" >&2; exit 1; }
```

## Wave 1 — Implementação
> Dependências: Wave 0 auditada. MLs escritos pelo arquiteto **a partir do parecer**, não antes.
