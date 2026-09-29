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
**Status:** ⬜ Pendente
**Método:** 🔴 **não conferir o diff.** Ler ADR e REQ, derivar o esperado, medir o comportamento.
**Critérios de aceite:**
- [ ] Entrada que resolve → reprova · lista fiel → passa · **regressão nova continua reprovando**
- [ ] Skip e rename **não** produzem falso positivo
- [ ] Veredito: sobrou caminho pelo qual uma melhoria passe despercebida?
