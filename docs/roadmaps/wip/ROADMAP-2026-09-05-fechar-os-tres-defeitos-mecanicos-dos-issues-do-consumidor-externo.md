---
status: wip
date: 2026-09-05
squad: artemis-tf
req: "docs/req/REQ-2026-09-05-tres-defeitos-mecanicos-medidos-por-consumidor-externo-skips-residuais-gate-req-has-adr-vacuo-e-enotdir-classificado-como-ausente.md"
---

# Roadmap: Fechar os três defeitos mecânicos dos issues do consumidor externo

> Criado em: 2026-09-05 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-05-tres-defeitos-mecanicos-medidos-por-consumidor-externo-skips-residuais-gate-req-has-adr-vacuo-e-enotdir-classificado-como-ausente.md`
Triagem: `docs/portabilidade/2026-09-05-triagem-dos-sete-issues-do-lourival.md`

## Diagnóstico

Três dos sete issues do contribuidor externo, todos **confirmados por medição**, todos com mecanismo
provado e **nenhuma decisão de arquitetura pendente**.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Os três, em paralelo (arquivos disjuntos)
> Dependências: PR #280 mergeado (313fe9a) ✅.
> **Disjunção conferida arquivo a arquivo pelo arquiteto, 2026-09-05:**
> ML-1A escreve em 5 arquivos `*_test.go` (`internal/generators/scaffold_doctor_test.go`,
> `internal/generators/update_test.go`, `internal/integrations/manager_test.go`,
> `internal/integrations/manager_persistence_order_test.go`, `internal/thirdparty/provenance_test.go`).
> ML-1C escreve em `internal/integrations/manager.go` + pares Node/Python — **arquivo diferente** do
> `manager_test.go` do ML-1A, mesmo pacote. ML-1B vive em `internal/validator/` + `npm/src/validator/`
> + `pypi/trackfw/`. **Nenhum arquivo aparece em dois MLs.**
> 🔴 **A varredura do acervo do ML-1A é RELATÓRIO, não licença de escrita** — se ela achar `skip` em
> arquivo de outro ML, reporta; não edita.

### ML-1A — `#279`: os 9 `skip` residuais, e a varredura do ACERVO
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
Tratar os 9 pelo **mesmo padrão do ML-4A**, sem inventar padrão novo.

🔴 **O entregável principal NÃO são os 9 — é a varredura do acervo inteiro** (AC2), com cada
`skip`/guard de plataforma classificado em **legítimo** (o teste não faz sentido naquele SO) ou
**apaga asserção**. Fechar 9 sem varrer garante que o próximo resíduo apareça igual: foi exatamente
assim que estes 9 chegaram até aqui — o arquiteto auditou **o diff de cada wave**, nunca o acervo.

### ML-1B — `#278`: `req_has_adr` deixa de detectar vazio por literal
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (3 CLIs — regra dura de paridade)
5 de 7 grafias de vazio escapam. Falsificação nas duas direções: REQ com ADR real **não** acusada;
cada uma das 7 grafias **é**.

🔴 **A contagem do acervo salta de 11 para dezenas, e isso é ACERTO, não regressão** — declare antes
e depois. Um gate que passa a acusar mais depois de deixar de ser vácuo está funcionando.
🔴 **Não corrigir as REQs do acervo aqui** (AC5): consertar o gate e 60 artefatos no mesmo diff
torna impossível atribuir qual mudança produziu qual número.

### ML-1C — `#276`: `ENOTDIR` deixa de ser classificado como ausência
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
`internal/integrations/manager.go:477` — sexto sítio de predicado de plataforma, **fora do
validator**, que é por isso que escapou das varreduras anteriores. Trocar `os.IsNotExist` por
`errors.Is(err, fs.ErrNotExist)`. Verificar os pares Node/Python.
**Falsificação:** com o remendo revertido, o diagnóstico volta a sumir.

## Acceptance Criteria da wave
- [ ] Falsificação nas duas direções em cada ML, com números
- [ ] 🔴 Varredura do **acervo** entregue no ML-1A, não só os 9
- [ ] 🔴 Nenhuma correção esconde defeito — se o produto é que está errado, **parar e reportar**
- [ ] `make quality` verde e `trackfw validate` exit 0, rodados **pelo arquiteto**, uma vez, ao fim

## Fora deste roadmap
`#275` (ratchet por nome) → **ADR de CI**, com as armadilhas nomeadas.
`#273` → REQ própria já aberta. `#274` → passo de CI. `#277` → dívida de portabilidade.


### ML-1D — Cinco fixtures que passavam POR CAUSA do defeito do ML-1B
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
O `make quality` reprovou em **um** cenário: o fixture escrevia `Roadmap:` sem espaço — a grafia 1,
uma das cinco que escapavam. **O cenário passava por causa do defeito que o ML-1B corrigiu.**

🔴 **Meu diagnóstico estava incompleto, e o agente corrigiu.** Eu escrevi no handoff que
`assert_lacks_pattern` "tolera violação extra". **Não tolera** — ela exige exit 0 do processo
inteiro, e por isso os braços `-detects-regression` dos cenários 27/28/32 também quebravam. Eram
**5** fixtures, não 1. Ele achou rodando o script até o fim: **o `make quality` parava no primeiro
FAIL**, antes de alcançar as outras asserções. *Um FAIL não é a contagem de defeitos — é o primeiro
que o gate conseguiu alcançar.*

Não-vacuidade provada por mutação: corrompeu `resolveAdrStatus` e o cenário 27, **já com link real**,
voltou a acusar. Dar valor ao fixture não o vacuizou.

### ML-1E — Item 4 do harness invocava o stub do WSL
**Status:** ✅ Concluído · **Agente:** `ares-tf`
O primeiro run em Windows real após o PR #280 deu `INCONCLUSIVE` no item 4, com saída **UTF-16** —
assinatura do `C:\Windows\System32\bash.exe`. O retarget chamava `bash` por nome cru.

🔴 **É o retarget FUNCIONANDO.** O ML-2C trocou um substituto por invocação real, e a primeira coisa
que a invocação real mediu foi que o `bash` do runner não é o que supúnhamos. O substituto escondia
isso por construção.

`Resolve-ProvenBash` enumera candidatos por caminho absoluto e só aceita quem prova `GNU bash` —
**com o stub do WSL na lista de propósito**, para ser testado e reprovado, não apenas evitado. Filtro
correto por construção, não por acidente de `PATH`.

## Auditoria da wave — arquiteto, 2026-09-05

```
make quality QUALITY_EXIT=0, zero FAIL
check-gates-falsify isolado, rodado pelo arquiteto: exit 0, 359 OK, 0 FAIL, 181 cenarios
trackfw validate exit 0 — 67 avisos de ADR ausente (eram 11), sao AVISOS, o repo nao quebrou
```

🔴 **As 67 são dívida real do acervo e NÃO entram neste diff** (AC5). Corrigir gate e ~60 artefatos
junto tornaria impossível atribuir qual mudança produziu qual número. Vira baseline em trabalho
próprio.

🔴 **O `#276` teve a premissa REFUTADA pela medição**, e a nossa triagem errou antes: marcou
CONFIRMADO porque as **linhas citadas existem**, e eu li isso como se confirmasse a **causa**. São
coisas diferentes. `os.IsNotExist` e `errors.Is(err, fs.ErrNotExist)` chamam o mesmo
`syscall.Errno.Is` para `*PathError` de um nível; no Windows `ENOTDIR` **é** `ERROR_PATH_NOT_FOUND`.
A troca fica como **modernização**, e a issue **não** é fechada por este diff. Comentada com a
medição aberta e com o programa mínimo que derrubaria nossa conclusão na máquina dele.
