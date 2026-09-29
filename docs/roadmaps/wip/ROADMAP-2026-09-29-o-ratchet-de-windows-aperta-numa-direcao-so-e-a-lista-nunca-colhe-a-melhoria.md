---
status: wip
date: 2026-09-29
req: "docs/req/REQ-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-de-falhas-conhecidas-nunca-colhe-a-melhoria.md"
squad: ""
---

# Roadmap: o ratchet de Windows aperta numa direção só

> Created: 2026-09-29 | Status: wip

## Context
<!-- Derived from REQ -->
REQ: docs/req/REQ-2026-09-29-o-ratchet-de-windows-aperta-numa-direcao-so-e-a-lista-de-falhas-conhecidas-nunca-colhe-a-melhoria.md
ADR: docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md (**Emenda 1**, D6 e D7)
Origem: **#364**. Medido: 1677 PASS · 14 FAIL · **0 obsoletas** — a lista está exata, e é isso que
torna a adoção do D6 gratuita agora.

## Acceptance Criteria
<!-- Consolidados; detalhe por ML nas waves. -->
- [ ] D6: falha declarada que resolve **reprova**, nomeando a entrada e o protocolo do D4
- [ ] Falsificação nas duas direções + contra-braço: regressão nova **continua** reprovando
- [ ] D7: as 14 entradas ganham a razão, e o checker **valida** o campo
- [ ] Triagem das 14 por causa-raiz, **medida**
- [ ] `make quality` e CI verdes, **incluindo o `windows-full-suites`**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model: por que cada uma das 14 falha, e o que o checker já garante
> Dependências: nenhuma. 🔴 **Bloqueia a implementação.**

**Gates da wave:**

```bash
n=$(jq -r '.entries[]|.name' .github/windows-known-failures.json 2>/dev/null | wc -l | tr -d ' '); test "$n" = "14" && echo "Gate W0: $n entradas na lista — a triagem parte deste universo" || { echo "GATE FALHOU: esperava 14 entradas, contou $n — a lista mudou, remeça a medicao antes de triar" >&2; exit 1; }
```

### ML-0A — triar as 14 por causa-raiz e mapear o checker
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-29 · 🔴 **BLOQUEOU o D6 como eu o escrevi**
**Arquivos:** leitura de `.github/`, `scripts/`; escrita em `docs/seguranca/2026-09-29-wave0-ratchet.md`

**Tarefa:**
1. 🔴 **Para CADA uma das 14, a razão MEDIDA da falha** — não presumida. O log do
   `windows-full-suites` da `main` tem a mensagem de cada uma; extraia e classifique por mecanismo
   (permissão POSIX · CRLF · `bash` ausente · outro). **Agrupe:** é provável que 14 nomes tenham
   poucas causas.
2. **Mapeie o checker** que implementa o ratchet: onde compara declarado × observado, onde emite
   `[-1 resolvido]`, e o que já valida do esquema (`_meta.d4_note` descreve o protocolo de remoção).
3. 🔴 **Refute ou confirme a premissa do D6:** que dá para reprovar por "entrada resolveu" **sem**
   falso positivo. Ataque: teste que **não executa** (skip) conta como resolvido? E teste cujo
   **nome mudou**? O **D3** trata suíte que não executa como falha de classe própria e o **D5**
   declara nomes instáveis — verifique se esses dois já cobrem os buracos, ou se o D6 abre um novo.
4. **Onde o campo de razão cabe** no esquema sem quebrar o checker nem o `removed[]`.

**Critérios de aceite:**
- [x] 14 com razão medida; **11 MEDIDAS** na mensagem de falha, **3 inferidas** e marcadas como tal
- [x] **4 grupos:** permissões POSIX (4) · CRLF no renderer (4) · comando externo (2) · representação de caminho (3)
- [x] `scripts/check-windows-known-failures.py`: passo 6 = `obs − known` (D1, não muda) · passo 7 = `known − obs`, **hoje `::warning::`, nunca exit 1** · passo 10 emite `[-N resolvido]`, puramente informativo · `entries[]` **não tem validação de esquema** hoje
- [x] 🔴 **SIM, produzem — D6 de uma linha BLOQUEADO.** Fixture sintética: pacote com `panic` (sem
      marcador `[setup failed]`) faz o teste sumir das observadas, e o D6 ingênuo diria *"resolveu"*.
      Passos 3 e 5b **não cobrem**. Recorte de **três baldes** escrito na ADR **antes** do código
- [x] `reason` obrigatório em `entries[]`, opcional em `removed[]` (as 24 já lá não o têm).
      🔴 **ASCII puro** — o self-test roda sob `PYTHONIOENCODING=cp1252` (`quality.yml:977`), e um
      acento causaria `UnicodeEncodeError` **exatamente quando o checker precisasse falar**

## Wave 1 — o gatilho e a razão
> Dependências: **Wave 0 auditada.** O desenho sai dela — se o D6 precisar de recorte, este bloco muda.

### ML-1A — D6 em três baldes, e o campo `reason` validado
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — 2026-09-29 · self-test 53/53 PASS (UTF-8 e cp1252) · make quality RC=0
**Arquivos:** `scripts/check-windows-known-failures.py`, `.github/windows-known-failures.json`

**O desenho saiu da Wave 0, não do plano.** Três coisas que ela fixou e que são AC:

**1 — D6 em TRÊS baldes** (dois branches no passo 7, **não** um `_warn` → `_err`):

| balde | condição | ação | mensagem |
|---|---|---|---|
| resolvido | na lista **e** passou | reprova | mover para `removed[]` com `removal_note` (**D4**) |
| **não executou** | na lista, **nem** passou **nem** falhou | reprova | *skip, `panic` de pacote, ou deletado — **não é resolução*** |
| ainda falha | na lista **e** falhou | passa | — |

**2 — `reason` obrigatório em `entries[]`**, ASCII puro, validado por função análoga a
`validate_removed()`.

**3 — 🔴 ~20 fixtures do self-test** (`write_list(...)`, linhas ~1216-1699) usam `{name, runtime,
class}` **sem** `reason`. Se a validação entrar no fluxo normal sem atualizá-los, **`make quality`
fica vermelho na hora**. Atualizá-los é **parte deste ML**, não consequência dele.

**Critérios de aceite:**
- [x] Os **três** baldes implementados, cada um com mensagem própria — a atribuição errada é o
      defeito que o **D3** existe para evitar, e que esta base já pagou 3× no #274
- [x] 🔴 **Falsificação por balde:** entrada que passou → reprova dizendo *resolveu* · entrada que
      sumiu por `panic` → reprova dizendo *não executou* · entrada que falhou → passa
- [x] 🔴 **Contra-braço do D1:** **regressão nova continua reprovando** — o D6 não pode ter
      desligado o que já funcionava
- [x] `reason` obrigatório em `entries[]` e **validado**; ausência → reprova
- [x] 🔴 **ASCII puro**, verificado sob `PYTHONIOENCODING=cp1252 PYTHONUTF8=0`
- [x] **As 14 entradas recebem a razão** da triagem da Wave 0 — as 3 inferidas marcadas como tal
- [x] **Os ~20 fixtures do self-test atualizados**, e o `--self-test` verde
- [x] `make quality` **RC=0**

## Wave 2 — auditoria independente
> Dependências: Wave 1 completa.

### ML-2A — revisão por reimplementação
**Owner:** `hades-tf`
**Status:** ✅ Concluído — 2026-09-29 · 34/34 vetores PASS · self-test 53/53 · APROVA
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o comportamento.
**Critérios de aceite:**
- [x] Entrada que resolve → reprova · lista fiel → passa · **regressão nova continua reprovando**
- [x] Skip e rename **não** produzem falso positivo
- [x] Veredito: sobrou caminho pelo qual uma melhoria passe despercebida? **NÃO.**
**Achado menor:** `TestStaleWIPReportsWIPWalkError` usa marcador `(MEASURED in ADR-2026-09-05 Adendo)` em vez de `(MEASURED)`. Contagem mecânica do marcador estrito dá 8, não 9. ADR diz "9 MEASURED / 5 INFER" — verdadeiro pelo sentido, inconsistente pelo marcador. Sem impacto funcional.
**Parecer:** `docs/seguranca/2026-09-29-wave2-ratchet.md`

## Wave 3 — o sumário contradiz o D6
> Dependências: Wave 2 completa. Achado da auditoria do arquiteto, 2026-09-29.

### ML-3A — o rótulo `resolvido` no sumário agrega os baldes 1 e 2
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente
**Arquivos:** `scripts/check-windows-known-failures.py` · `.github/windows-known-failures.json` ·
`docs/adr/ADR-2026-09-05-...-nunca-por-contagem.md`

**O defeito, medido:** `_cls_label()` (linha ~1050) calcula
`resolved = known_set - obs_set` — *"está na lista e não foi observada falhando"*. Esse conjunto é a
**união dos baldes 1 e 2** do D6. Resultado literal, com fixture própria:

```
entrada some sem passar (balde 2):
  ::error:: ... "neither failed nor passed ... Not a resolution."   ← correto
  resumo:   Go 1/2 [-1 resolvido]                                   ← afirma RESOLVIDO
```

🔴 É a **atribuição errada que o D6 existe para impedir**, sobrevivendo no agregado. Quem ler só a
linha de resumo — que é o que aparece no topo do log de CI — move para `removed[]` um teste que
apenas deixou de rodar.

**Ações:**
1. `_cls_label` passa a receber o conjunto de **PASS** da classe (já disponível: `go_passes`,
   `node_passes`, `py_passes`) e separa `known - obs` em dois: **`-N resolvido`** (passou) e
   **`-N ausente`** (nem passou nem falhou). Os dois podem coexistir na mesma classe.
2. O termo `ausente` não pode conter a palavra `resolvido` — é o que o T24 contra-arma.
3. Normalizar o marcador de `TestStaleWIPReportsWIPWalkError` para `(MEASURED)`, preservando o
   texto da evidência; registrar na **Emenda 1** que o marcador canônico é `(MEASURED)`/`(INFER)`
   e que a contagem `9/5` é conferível por grep estrito.

**Critérios de aceite:**
- [ ] Balde 2 isolado → resumo diz **`ausente`**, e **não** contém `resolvido`
- [ ] Balde 1 isolado → resumo diz **`resolvido`**
- [ ] 🔴 **Os dois na mesma classe** → resumo mostra os **dois** termos
- [ ] Contra-braço: tudo equilibrado → nenhum dos dois termos (T24 continua válido)
- [ ] Contra-braço do D1: regressão nova continua com `[+N NOVO]`
- [ ] `grep -c '(MEASURED)'` nas `entries` = **9** · `(INFER)` = **5** · soma = 14
- [ ] `--self-test` verde, com fixture nova por critério acima
- [ ] `make quality` **RC=0**
