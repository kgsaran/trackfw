---
status: wip
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md"
squad: ""
---

# Roadmap: jira_base_url do repositorio vira destino de post autenticado e um PR que edita so a config exfiltra a credencial do ci

> Created: 2026-09-17 | Status: wip


## Wave 0 — Medir o exploit e o raio, antes de escolher o remédio
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — reproduzir em laboratório e enumerar a classe
**Status:** ⬜ Pendente · **Papel:** `hades-tf`

Esta Wave 0 **não é re-análise** — o mecanismo já está verificado no código. É **medição**, porque a
escolha entre os remédios depende de duas coisas que ainda não sabemos.

1. 🔴 **Reproduzir a exfiltração contra um listener LOCAL** (`127.0.0.1`), nunca contra host externo.
   Provar que a requisição sai com o header `Authorization` para o destino configurado. Sem
   reprodução, estamos corrigindo uma leitura de código.
2. **Enumerar o raio:** quais chaves do `trackfw.yaml` influenciam **destino** de requisição
   autenticada? `jira_base_url` é a conhecida — `jira_email` e `jira_project` mudam conteúdo ou
   destino? Há outra chave com a mesma forma em outro comando (`doctor --remote`, `update`,
   `thirdparty fetch`)? **O número decide se o remédio é pontual ou de classe.**
3. **Precedência:** confirmar por leitura se todas as chaves de sync seguem "config antes de env".
   Se alguma seguir a ordem inversa, isso é precedente interno para o AC1 — e vale mais que
   argumento teórico.
4. **Compatibilidade:** qual das direções do issue (#380) preserva o caso self-hosted sem passo
   extra? Se **nenhuma** preservar, diga — é resultado válido e muda o AC2.

**Aceite:** parecer com a reprodução (comando, listener, header capturado — **com o token
redigido**), a enumeração com número, e recomendação de direção **com o custo de compatibilidade de
cada uma**. 🔴 Parecer que só confirme o issue não mediu nada.

**Gates da wave 0:**
```bash
P=docs/portabilidade/2026-09-17-threat-model-jira-base-url.md
test -s "$P" || { echo "FAIL: parecer ausente ou vazio: $P"; exit 1; }
for termo in "127.0.0.1" "Authorization" "jira_base_url" "compatibilidade"; do
  grep -qi -- "$termo" "$P" || { echo "FAIL: parecer nao cobre: $termo"; exit 1; }
done
grep -qiE "residual|nao consegui|não consegui|indetermin" "$P" \
  || { echo "FAIL: parecer nao declara o que ficou em aberto"; exit 1; }
# 🔴 O parecer NAO pode conter token real capturado.
grep -qE "Basic [A-Za-z0-9+/]{20,}" "$P" \
  && { echo "FAIL: parecer contem credencial nao redigida"; exit 1; }
echo "OK [wave0/threat-model-jira]: parecer presente, reproduz em local e sem credencial vazada"
```

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.** A direção escolhida sai do parecer, não desta linha.

### ML-1A — separar destino de credencial, validar a URL e fechar a classe
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre AC1 a AC7.

---

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md

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

### ML-1A — jira_base_url do repositorio vira destino de post autenticado e um PR que edita so a config exfiltra a credencial do ci
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
