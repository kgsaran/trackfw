---
status: wip
date: 2026-08-18
req: "docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `branch prune` com dry-run por padrão e heurística de arquivos-tocados

> Created: 2026-08-18 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md`

**Decisões de KG (2026-08-18), já fechadas:**
1. `--dry-run` é o **padrão**; apagar exige `--apply`.
2. Fonte de verdade é **só o git**, pela heurística de arquivos-tocados. Sem forge, sem rede.

O comando resolve um procedimento manual de 6 passos do `CLAUDE.md` §1, executado 5 vezes entre
16 e 18/08. E corrige de passagem o `detectPendingSquashMerges` do `ship`, cujo teste ingênuo
acusou a branch **já mergeada** do #181 como tendo trabalho pendente.

## Acceptance Criteria
- [ ] AC1 — Squash-merge sem ancestralidade é reconhecido como integrado.
- [ ] AC2 — Branch defasada e integrada (main avançada) é reconhecida como integrada.
- [ ] AC3 — Trabalho pendente não é apagado, e o motivo é dito.
- [ ] AC4 — Branch corrente e branch em worktree nunca são apagadas.
- [ ] AC5 — Sem `--apply`, nada é apagado.
- [ ] AC6 — Offline/sem remoto: degrada e **não apaga**. Falha fechada.
- [ ] AC7 — `detectPendingSquashMerges` usa a mesma lógica; falso-positivo do AC2 some.
- [ ] AC8 — Paridade nos 3 CLIs, com gate de saídas reais.
- [ ] AC9 — Cenário P4 com fixture de repositório git **real**.
- [ ] AC10 — `make quality` verde **e CI verde**.

## 🔴 Riscos que valem para todos os MLs

1. **Comando destrutivo.** Caso duvidoso **recusa e explica**, nunca apaga. Falha fechada.
2. **Fixture tem de ser repositório git real** com squash-merge de verdade. Mock de `git` provaria
   só que o mock concorda com o código. Precedente: Cenário 50 já cria repo git em fixture.
3. **`make quality` verde localmente não fecha AC.** Na REQ anterior fechei o AC de gate com
   evidência só de macOS e o CI (Linux) reprovou. Cenário que depende de git real, caminho ou
   limite de SO exige CI verde antes de fechar.
4. **`$HOME` e `cwd` de teste sempre de fixture**, nunca os reais.

---

## Wave 1 — Núcleo da decisão

### ML-1A — Heurística de integração compartilhada + `branch prune` (dry-run por padrão)
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/commands/branch*.go` + espelhos Node/Python, testes dos 3.

**Ações:**
1. Extrair a heurística de arquivos-tocados para **uma** função reutilizável, nos 3 CLIs:
   ```
   mb      = merge-base origin/main <branch>
   touched = diff --name-only mb <branch>
   diverg  = diff --name-only origin/main <branch> -- touched
   ```
   `touched` vazio → sem trabalho próprio · `diverg` vazio → integrada · senão → **recusa**.
2. `trackfw branch prune`: relata a decisão de **cada** branch local, com motivo. **Não apaga** sem
   `--apply`.
3. Recusa sempre: branch corrente, branch em `git worktree list`, e qualquer caso duvidoso.
4. Offline / sem `origin`: degrada com mensagem clara e **não apaga**.

**Critérios de aceite:**
- [ ] Sem `--apply`: nenhuma branch é apagada, nem a claramente integrada. Prove contando branches antes/depois.
- [ ] Com `--apply`: apaga a integrada, mantém a pendente, e diz o motivo de cada uma.
- [ ] Branch corrente e branch em worktree nunca apagadas, mesmo com `--apply`.
- [ ] Offline: não apaga; mensagem clara.
- [ ] Fixture de repo git **real** com squash-merge simulado; sem mock de `git`.
- [ ] Paridade nos 3 CLIs.
- [ ] `make quality` verde.

---

## Wave 2 — Convergência do `ship` (depende da Wave 1)

### ML-2A — `detectPendingSquashMerges` passa a usar a heurística compartilhada
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A — a função só existe depois dele.

**Discriminante medido, use como caso de teste:** a branch do PR #181, já mergeada, era acusada de
"unmerged changes" porque a `main` avançara com o #182. Quatro arquivos apareciam divergentes sem
haver trabalho pendente.

**Critérios de aceite:**
- [ ] O `ship` deixa de avisar sobre branch defasada porém integrada.
- [ ] Continua avisando sobre branch com trabalho genuinamente pendente — não-regressão.
- [ ] Uma só implementação da heurística; sem cópia divergente.
- [ ] Cenário P4 com baseline e detecção.
- [ ] `make quality` verde.

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`: revisão de comando destrutivo
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-18-revisao-do-branch-prune.md`

**Ações:** é o primeiro comando do trackfw que **apaga** trabalho. Verificar se há caminho para
apagar branch não integrada — nome com caracteres especiais, branch com upstream sumido, `origin`
apontando para lugar errado, `main` local defasada em relação a `origin/main`, repositório sem
`origin`, branch cujo nome colide com ref ambígua. Avaliar se `--apply` pode ser disparado sem
intenção. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** apagar branch remota; consultar forge; alterar estratégia de merge.
- Commits e branch são exclusivos do `trackfw_architect`.
