---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Os 3 CLIs executam gate de wave com `sh -c`

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md`
ADR: `docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md`

**Item 7 do issue #216** — o mais grave da lista, e o único que quebra a correção da **própria
ferramenta de governança**: `trackfw barrier` pode aprovar uma wave para quem usa um CLI e reprová-la
para quem usa outro, no mesmo repositório e no mesmo commit.

Medição que decidiu o ADR: **83 comandos** de gate em todos os roadmaps, e **nenhum idioma existe no
`cmd.exe`** (35 `grep`/`sed`/`awk`, 14 `test`/`[`, 8 negações com `!`, 3 `&&`/`||`, 3 `$( )`).
No Windows, Node e Python **não avaliam diferente — falham em avaliar**.

## Acceptance Criteria

- [ ] Os 3 CLIs executam gate com `sh -c`
- [ ] Mesmo gate, mesmo veredito nos 3 — **e o controle**: gate que deve reprovar continua reprovando
- [ ] `sh` ausente falha nomeando o remédio, com mensagem byte-idêntica nos 3
- [ ] **"Não pôde ser avaliado" é distinto de "reprovou"**
- [ ] Gate contra regressão para `shell: true`/`shell=True`
- [ ] Item 7 sai de `REPRODUCED` (camada 2 de 4 → 3), com a transição explicada
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Modelo de ameaça da troca de shell
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — Superfície de execução e a semântica de "não pôde medir"
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que esta Wave 0 importa mais que a média:** estamos mudando **como conteúdo de artefato
versionado vira processo**. O gate já executava comando arbitrário — não é superfície nova — mas o
**shell muda**, e com ele o parsing, o quoting e o tratamento de erro.
**Actions:**
1. **A troca amplia superfície?** `sh -c` versus shell do SO: um gate malicioso num roadmap de PR de
   terceiro ganha capacidade nova? (Lembrar: o `barrier` já executa esses comandos hoje no Go.)
   Considere o `roadmapTrustForGates`, que **já tem REQ aberta por fail-open**.
2. 🔴 **A semântica de "não pôde ser avaliado".** A AC4 exige distinguir isso de "reprovou". **Julgue
   qual é o lado seguro** e por quê: um `sh` ausente que resulte em *"wave não passou"* é falso
   negativo que bloqueia trabalho legítimo; que resulte em *"passou"* é falso positivo que libera
   trabalho não verificado. **Nenhum dos dois é obviamente certo — quero o argumento.**
3. **Falsificação nas duas direções**, com atenção ao simétrico: um `barrier` que passe a recusar
   ambiente legítimo (contêiner mínimo sem `sh`? CI de terceiro?) troca um defeito por outro.
4. **Enumeração:** só os dois pontos (`barrier.js:561`, `barrier.py:582`)? Varra por outros lugares
   nos 3 CLIs onde conteúdo de artefato vira processo — `shell: true`, `shell=True`, `exec`, `spawn`,
   `system`. **Assuma que minha lista de dois está incompleta**; nesta sessão isso se confirmou
   repetidamente.
5. **Residual declarado.**
**Critérios de aceite:**
- [ ] Veredito sobre ampliação de superfície, com vetor concreto se houver
- [ ] Argumento explícito sobre o lado seguro de "não pôde medir"
- [ ] Enumeração de pontos onde artefato vira processo, nos 3 CLIs
- [ ] Nenhuma linha de implementação escrita
- [ ] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
```

## Wave 1 — A correção
> Dependências: Wave 0. Particionamento definido após o veredito sobre a semântica de erro — ele muda
> a forma da mensagem e do código de saída.

## Wave 2 — Gate e contrato
> Dependências: Wave 1. 🔴 Nasce ligado, com guarda de vacuidade ancorada no mesmo cwd, `python3`
> nunca `python`. E **prefira `assert_count` a `assert_has`** onde a assinatura puder repetir — o
> gate precisa reprovar se **um** dos dois CLIs regredir, não só se ambos.

## Verificação que só o CI fecha

Item 7 saindo de `REPRODUCED`: camada 2 de **4 para 3**. O check compara o veredito do mesmo gate nos
3 runtimes — **comportamento de produto**, então deve genuinamente virar. Verificado o que ele mede
**antes** de fixar o número.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
