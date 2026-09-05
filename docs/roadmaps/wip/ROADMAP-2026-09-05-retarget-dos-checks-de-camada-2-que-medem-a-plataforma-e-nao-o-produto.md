---
status: wip
date: 2026-09-05
squad: ares-tf
req: "docs/req/REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-nos-itens-2-e-3-retarget-dos-checks-de-home-e-bit-de-execucao.md"
---

# Roadmap: Retarget dos checks de camada 2 que medem a plataforma e não o produto

> Criado em: 2026-09-05 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-nos-itens-2-e-3-retarget-dos-checks-de-home-e-bit-de-execucao.md`

## Diagnóstico

Quatro checks do harness de reprodução de Windows (`scripts/windows-repro/`) medem um **substituto**
em vez do produto:

| item | o que o check mede hoje | o que deveria medir |
|---|---|---|
| 2 | `os.homedir()` / `expanduser("~")` do runtime | o `trackfw` resolvendo home com `HOME` ≠ `USERPROFILE` |
| 3 | o bit `0111` em NTFS | o validator deixando de alarmar em Windows |
| 4 | o mecanismo cru (helper Python lendo o markdown) | o `.sh` real que a correção de 02/09 mudou |
| 7 | uma **réplica** do gate dentro do próprio harness | o `barrier` |

**Nenhum deles pode ir a `ABSENT`, por construção.** `os.homedir()` vai ignorar `$HOME` no Windows
para sempre; o bit `0111` vai ser 0 em NTFS para sempre. As correções **desviam o trackfw** dessas
propriedades — elas não mudam a plataforma.

🔴 **Com quatro instâncias, não é check mal escrito: é propriedade do instrumento.** Um harness que
replica o mecanismo em vez de invocar o produto **mede a si mesmo**, e envelhece em silêncio a cada
correção que o produto recebe.

**Consequência que dá a prioridade:** o dashboard mostra hoje defeitos **corrigidos** como
`REPRODUCED`. Quem abrir o painel — inclusive o contribuidor externo que abriu o issue #216 — chega
à conclusão errada. **6 dos 7 itens do #216 estão corrigidos no produto e a issue não pode ser
fechada com honestidade por causa disto.** Evidência:
`docs/portabilidade/2026-09-05-parecer-fechamento-issue-216.md`.

## Acceptance Criteria

- [ ] Os checks 2, 3, 4 e 7 invocam **o produto**, não substitutos
- [ ] 🔴 Cada retarget provado por **reversão da correção** — check que passa ao nascer não prova nada
- [ ] Contagem da camada 2 recalculada e **justificada item a item**, nunca herdada de previsão
- [ ] Enumeração completa dos checks do harness, cada um classificado
- [ ] Nenhuma correção de produto alterada por este roadmap

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Enumerar antes de consertar (sozinho, bloqueia o resto)
> Dependências: nenhuma.

### ML-1A — Enumerar e classificar TODOS os checks do harness
**Status:** ✅ Concluído · **Agente:** `ares-tf`
**Arquivos:** leitura de `scripts/windows-repro/**` (`run.ps1`, `go/checks.go`, `js/checks.js`,
`py/checks.py`). **Investigação — não corrigir nada aqui.**

Para **cada** check das camadas, classificar: **invoca o produto** · **replica o mecanismo** ·
**mede a plataforma**. Um check que replica o mecanismo e **não** está declarado como confirmatório
é defeito, e entra na lista.

🔴 **Isto vem antes das correções, e é o entregável que impede a quinta instância.** As quatro
conhecidas apareceram uma a uma, por acidente, ao longo de 5 dias. Corrigir só as quatro conhecidas
deixa o padrão vivo.

**Critérios:** tabela completa, nenhum check sem veredito · para os que invocam o produto,
**demonstrar** que invocam (linha do código), não presumir pelo nome.

**Resultado (2026-09-05):** `docs/portabilidade/2026-09-05-enumeracao-dos-checks-do-harness-de-windows.md`.
12 posições de veredito no `run.ps1` + 18 subcomandos de sonda, cada classificação provada por linha
de código. **Defeituosos: 4 — os quatro já conhecidos. NÃO existe quinta instância.** Isso fecha a
AC7 e, ao contrário do que eu temia, torna a Wave 2 suficiente.

🔴 **Achado que a REQ não tinha:** o item 3 está declarado confirmatório **só em comentário**
(`run.ps1:169`); o veredito continua entrando em `$reproduced.Count` (`run.ps1:664`) e na condição de
saída (`run.ps1:683`). **Comentar não tira da conta** — o ML-2B tem de excluir do contador.

🔴 **Achado fora da taxonomia:** o item 12 é sonda declaradamente **fora** da issue #216 ("NÃO
CORRIGE nada") e mesmo assim o veredito dela contamina o mesmo gate. Tratamento explícito no ML-3A.

🔴 **Premissa minha derrubada:** os caminhos que passei no handoff (`js/checks.js`, `py/checks.py`)
não existem — são `node/checks.js` e `python/checks.py`. Escrevi de memória.

## Wave 2 — Retarget (SEQUENCIAL, um único agente)
> Dependências: ML-1A.
> 🔴 **Correção do plano original, feita com a enumeração na mão:** eu tinha escrito "paralelo por
> item". **Está errado** — os quatro itens vivem nos MESMOS arquivos (`run.ps1`, `go/checks.go`,
> `node/checks.js`, `python/checks.py`). Paralelizar seria a colisão que este roadmap existe para
> evitar, e que eu já criei uma vez nesta campanha. Um agente, os quatro em sequência.

### ML-2A — Item 2: medir o trackfw resolvendo home
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
Rodar o **binário** com `HOME` ≠ `USERPROFILE` e afirmar o caminho que o `trackfw` escolhe.
**Falsificação:** revertendo `homedir.Dir()`, volta a `REPRODUCED`.

### ML-2B — Item 3: medir o validator, ou declarar confirmatório
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
Medir o **validator deixando de alarmar** em Windows. 🔴 Se a conclusão for que o item 3 **deve**
permanecer confirmatório, **declarar explicitamente** e tirá-lo da contagem de `REPRODUCED`
corrigíveis — a REQ autoriza essa saída, desde que escrita.
**Falsificação:** revertendo a guarda de plataforma, volta a `REPRODUCED`.
🔴 **Não relitigar** a decisão do bit NTFS: `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`.

### ML-2C — Item 4: invocar o `.sh` real
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
O check invoca hoje o mecanismo replicado. Passa a invocar `scripts/check-parity-contract-coverage.sh`.
**Falsificação:** revertendo o fix de 2026-09-02, volta a `REPRODUCED`.

### ML-2D — Item 7: invocar o `barrier`, não a réplica
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
`scripts/windows-repro/go/checks.go:135` roda `exec.Command("sh","-c",...)` — réplica. A correção
mudou `barrier.js`/`barrier.py`.
**Falsificação:** revertendo a correção do barrier, volta a `REPRODUCED`.

## Wave 3 — Recontagem
> Dependências: Wave 2 completa.

### ML-3A — Recalcular a contagem esperada, item a item
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **Justificar item a item, nunca herdar de previsão.** O erro que originou esta REQ foi
transformar um número previsto em critério sem verificar o que os checks mediam.
🔴 A `AC3` da `REQ-2026-08-31` **continua marcada como FALSIFICADA**. Este roadmap corrige o
instrumento, **não o histórico**.
