---
status: done
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md"
squad: ""
---

# Roadmap: sync enumera REQ por caminho literal ignora req_dir e escreve no provedor de PM

> Created: 2026-09-17 | Status: done


## Wave 0 — Threat model
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — o que um enumerador errado permite quando o consumidor escreve fora
**Status:** ✅ Concluído e auditado pelo arquiteto (2026-09-17)

**Parecer:** `docs/portabilidade/2026-09-17-threat-model-sync-req-dir.md`

🔴 **Achado bloqueante, e é a justificativa de a Wave 0 existir:** `ResolveREQFiles`
(`validator.go:1603`) usa `reqDir := cfg.REQDir` **verbatim** — verifiquei: zero chamadas de
contenção no corpo da função. O `Glob` literal defeituoso é **acidentalmente imune** à travessia
porque ignora `cfg.REQDir`; a correção a **introduziria**, num comando que publica em serviço
externo. Corrigir sem contenção trocaria "lê os arquivos errados" por "lê fora da árvore e publica".

Emendei a REQ: **AC7 novo** (contenção com `EvalSymlinks`, não `Rel` lexical — `isOutsideCWD` existe
mas compara lexicamente e um symlink passa; precedente medido na REQ do `serve --api file`),
**AC6** passa a exigir `cfg.REQDir` verbatim na mensagem, **AC5** ganha allowlist e absorve
`check-referential-integrity.sh:10` pela Regra Dura, **AC4** passa a exigir `config.Reset()` — sem
ele o `once.Do` do `config.Load()` faz a falsificação medir o cache do default e passar sem afirmar
nada.

**Decisão arquitetural minha:** a contenção vai em `ResolveREQFiles`, não em wrapper do `sync` nem no
`config.Load()`. Um ponto único que resolve caminho sem conter é pior que a duplicação — o próximo
consumidor herda o buraco sem saber que existe.

**Fora de escopo, com issue própria:** `jira_base_url` vive no mesmo `trackfw.yaml` e um valor hostil
redireciona POST autenticado com `Authorization: Basic` para host do atacante. Pré-existente, causa
diferente, não se corrige aqui.

- O `sync` leva **conteúdo de REQ** para um provedor externo. Com o caminho errado, que conteúdo
  pode vazar para o projeto de PM errado — e o id injetado de volta marca arquivo alheio como
  sincronizado?
- Credenciais: `SyncToLinear`/`SyncToJira` usam token. Um `req_dir` mal resolvido muda o **alvo** da
  escrita autenticada?
- 🔴 A correção passa a ler `cfg.REQDir`, que vem do `trackfw.yaml` — arquivo do repositório. Um
  `req_dir` hostil (`../..`, caminho absoluto, symlink) pode fazer o `sync` **ler fora da árvore** e
  publicar isso? O resolvedor contém o caminho?
- O AC6 manda falar quando acha 0 REQ. Falar **o quê**? A mensagem pode revelar caminho de sistema?

**Aceite:** parecer com os vetores enumerados e, por vetor, se a correção fecha, mitiga ou não toca.
🔴 Vetor não fechado tem de estar nomeado.

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.**

### ML-1A — resolver pelo ponto único, com falsificação em `req_dir` não-padrão e `by_agent`
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
Cobre AC1 a AC7. Implementado em 2026-09-17.

---

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — sync enumera REQ por caminho literal ignora req_dir e escreve no provedor de PM
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

---

### Auditoria do arquiteto — 2026-09-17

**Aprovado, com duas correções de auditoria.**

**Mérito:** contenção no ponto único com resolução **física** (`EvalSymlinks` + ancestral-walking
para caminho ainda inexistente); assinatura `([]string, error)` propagada nos 11 callers, de modo que
nenhum recebe caminho não contido em silêncio; e uma decisão não pedida que melhora o AC4 —
`syncToProvider` recebe `create` injetável, tornando "nenhum teste toca a rede" verificável **por
construção** em vez de por promessa.

🔴 **Correção 1 (ML-1A-bis) — fail-open num controle de contenção.** `checkREQDirContained` tinha duas
portas que desligavam a própria proteção: `os.Getwd()` falhando devolvia `nil` (declarado como
deliberado), e `EvalSymlinks` falhando caía para o caminho não resolvido — voltando à comparação
**lexical**, que é exatamente a distinção que o AC7 existe para garantir. Num comando que publica em
serviço externo autenticado, o fail-open **é** o exploit que a Wave 0 descreveu. Ambos viraram recusa
nomeada, dizendo qual verificação não pôde ser feita. Varredura por outros fail-open: 1 ocorrência, a
própria.

🔴 **Correção 2 (ML-1A-ter) — o teste do AC7 podia passar por skip silencioso.** `make quality`
reprovou (`sync_test.go:448`, guarda de capacidade ausente). A guarda manual tratava **qualquer** erro
como "sem privilégio" e pulava. É o mesmo modo de falha do dia: passar sem medir. E era o teste do
**AC7** — o braço que distingue contenção física de lexical. Trocado pelo helper canônico, que separa
sem-privilégio (skip) de outro erro (falha).

**Verificado por mim, no artefato:** `TestSyncToProvider_REQDirSymlink_AC7` → `PASS`, **sem SKIP**;
`make quality` → RC=0, `8 chunks, 212 OK, 0 FAIL`; fail-closed confirmado por leitura — o único
`return nil` restante é o do caso efetivamente contido.

**Nota de processo:** o ML-1A marcou o próprio status como ✅ no roadmap, contra instrução explícita.
Status de ML é do arquiteto; agente marcar o próprio trabalho como concluído remove a auditoria do
caminho — e estas duas correções são a prova de que ela pega coisa.

