---
status: done
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-jira-base-url-do-repositorio-vira-destino-de-post-autenticado-e-um-pr-que-edita-so-a-config-exfiltra-a-credencial-do-ci.md"
squad: ""
---

# Roadmap: jira_base_url do repositorio vira destino de post autenticado e um PR que edita so a config exfiltra a credencial do ci

> Created: 2026-09-17 | Status: done


## Wave 0 — Medir o exploit e o raio, antes de escolher o remédio
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — reproduzir em laboratório e enumerar a classe
**Status:** ✅ Concluído e auditado pelo arquiteto (2026-09-17)

**Parecer:** `docs/portabilidade/2026-09-17-threat-model-jira-base-url.md`

**Vetor primário reproduzido** contra listener em `127.0.0.1`, com credencial falsa e token redigido:
o POST chegou em `/rest/api/3/issue` com `Authorization: Basic <REDACTED>` no destino configurado
pelo `trackfw.yaml`.

**Raio medido = 1 chave, 1 comando.** `jira_base_url` é a única chave que influencia **destino** de
requisição autenticada; `jira_email` e `jira_token` são credencial, `jira_project` é conteúdo. Linear
tem endpoint fixo; `doctor --remote`, `release tag` e `ship` passam por `gh` com forge validado
contra allowlist; `thirdparty fetch` já valida. Scripts não leem `trackfw.yaml` para montar URL.
**O remédio é cirúrgico, não de classe** — era esta a pergunta que decidia a forma da REQ.

🔴 **O parecer derrubou o meu AC2.** Eu exigia "nenhum passo extra" para o uso legítimo. A combinação
**vulnerável** (`token` do ambiente + `base_url` da config) é **a mesma de boa prática** — quem
hospeda Jira próprio commita a URL e mantém o token fora do repositório —, e não há sinal que
distinga config confiável de config alterada por PR. AC2 reescrito **por combinação**, com o passo
extra confinado à única linha perigosa.

**Correção de rumo dentro do próprio parecer, e ela aumenta a confiança no resto:** a primeira versão
afirmava exfiltração por redirect cross-domain. Medindo com hostnames diferentes, o `Authorization`
**não** é preservado — o stdlib do Go o remove (`shouldCopyHeaderOnRedirect`). O agente **removeu a
afirmação** em vez de mantê-la. Sobra o caso estreito de mesmo hostname com porta diferente, onde o
header sobrevive.

**Decisão minha sobre AC3:** incluir `CheckRedirect`. O resíduo é estreito, mas o padrão já existe em
`internal/thirdparty/fetch.go:57` — reutilizar custa pouco e não inventar nada é o ponto.

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
**Status:** ✅ Concluído — auditado pelo arquiteto (2026-09-17)

**O ponto que sustenta a correção, verificado por mim:** o opt-in
`TRACKFW_JIRA_ALLOW_MIXED_ORIGIN` é lido **só** por `os.Getenv` — não existe chave de config
correspondente, então um `trackfw.yaml` hostil **não tem como destravar**. Há teste dedicado
(`TestNewJiraClient_AC2_YAMLCannotUnlockOptIn`). É o que separa uma guarda de uma sugestão: a saída
de emergência é estruturalmente inalcançável de dentro do repositório.

**Mensagem de recusa** — auditada porque eu a declarei parte do AC: nomeia as duas origens em
conflito (`trackfw.yaml` e `JIRA_TOKEN`), explica **por que** recusa (um PR que edita só a config
redirecionaria a credencial) e indica a saída dizendo explicitamente *"not in trackfw.yaml"*. Recusa
que não ensina a saída vira issue de suporte e depois um `--force` genérico.

**`CheckRedirect`** foi além do padrão do `fetch.go`: além do esquema, compara o host normalizado —
com porta implícita preenchida, para não acusar `https://h:443` como diferente de `https://h`. Fecha
o caso medido na Wave 0 (mesmo hostname, porta diferente), que era o único vetor real de redirect.

**`http`→`https` no mesmo host:** inalcançável por construção — a URL inicial já exige `https`, então
o cliente nunca emite requisição `http`. Decisão registrada em vez de deixada em aberto.

**Contra-braços presentes:** com o opt-in definido a combinação `(config, env)` volta a funcionar **e
emite a requisição** — provando que o cenário discrimina, e não que o teste apenas observa um erro.
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
