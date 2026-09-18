---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade.md"
squad: ""
---

# Roadmap: robustez dos gates de governanca e paridade

> Created: 2026-07-27 | Status: done

## Context

REQ: docs/req/REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade.md
ADR: docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md

Quatro defeitos de gate apareceram em três REQs consecutivas e **nenhum foi pego pelo CI**. O trackfw
vende governança verificável: seus gates são o produto.

| # | Gate | Defeito | Situação |
|:-:|---|---|---|
| 1 | `check-integration-cli-parity.sh` | número mágico de itens do catálogo | corrigido |
| 2 | `check-cli-parity.sh` | ajuda colorida do `argparse` (Python 3.13+); validava por coincidência de texto | corrigido |
| 3 | `ship` npm/PyPI | `roadmap_dir` divergente; testes com injeção não exercitavam o caminho real | corrigido |
| 4 | `branch_has_wip_roadmap` | pune a Definition of Done que o produto prega | **aberto — ML-1A** |

### Princípios do ADR (aplicar em todo ML)

- **P1** Nenhum número mágico — derivar da fonte de verdade.
- **P2** Falha explícita, nunca degradação silenciosa.
- **P3** Independência de ambiente — runtime, cor, locale, `PATH`.
- **P4** Falsificabilidade obrigatória — provar que o gate **reprova**, não só que passa.

### Regra de paralelismo (calibrada nas REQs anteriores)

MLs só correm juntos se não compartilharem arquivo **nem saída de build**. Quem roda `make quality`
não corre em paralelo com ninguém — o gate escreve `bin/trackfw`. O orquestrador marca o status no
roadmap e **commita antes do spawn**.

### Mapa de dependências

```
Wave 1 (1A) ─ barrier ─> Wave 2 (2A ‖ 2B) ─ barrier ─> Wave 3 (3A)
```

---

## Wave 1 — Corrigir o defeito aberto (agente único)
> Dependências: nenhuma.

### ML-1A — `branch_has_wip_roadmap` aceita roadmap em `done/`
**Status:** done
**Files affected:** `internal/validator/validator.go` (~linha 1506), equivalentes em
`npm/src/validator/` e `pypi/trackfw/validator.py`, mais testes

**Actions:**
1. A regra hoje só percorre `wip/`. Passar a procurar o slug da branch também em `done/`,
   reaproveitando `normalizeBranchSlug` — **não** escrever outra normalização.
2. Reprovar apenas quando não houver roadmap correspondente em `wip/` **nem** em `done/`.
3. **Mitigação do risco de afrouxamento** (registrado no ADR): a aceitação em `done/` exige
   **casamento de slug**. Um roadmap qualquer em `done/` não serve — só o que corresponde à branch.
   Branch de feature sem nenhum roadmap correspondente continua reprovando.
4. Documentar o comportamento em `docs/cli-parity.md`.

**Acceptance criteria:**
- [ ] Roadmap em `done/` com slug da branch → **sem** violação, nos 3 CLIs
- [ ] Roadmap em `wip/` com slug da branch → sem violação (comportamento atual preservado)
- [ ] Branch de feature sem roadmap em lugar nenhum → **continua** reprovando
- [ ] Roadmap em `done/` com slug **diferente** da branch → **continua** reprovando
- [ ] Encerrar um roadmap na própria branch deixa `trackfw validate` verde — provado em repo temporário
- [ ] `make quality` verde

---

## Wave 2 — Auditoria P1–P3 (2 MLs em paralelo)
> Dependências: **barrier** — ML-1A concluído. Diretórios disjuntos: `internal/validator/` × `scripts/`.
> ⚠️ Nenhum dos dois roda `make quality`; o orquestrador roda na barrier.

### ML-2A — Auditoria das regras do validator
**Status:** done
**Files affected:** `internal/validator/` e equivalentes; correções onde houver defeito

**Actions:**
Auditar as 17 regras (`adr_dir_exists`, `adr_orphan`, `blocked_by_draft_adr`, `blocked_has_req`,
`branch_has_wip_roadmap`, `filename_uniqueness`, `folder_status`, `note_orphan`, `ref_targets_exist`,
`req_has_adr`, `req_has_roadmap`, `stale_wip`, `wip_acceptance`, `wip_has_req`, `wip_limit`, e as de
`traceid_*`) contra P1–P3:
- **P1**: alguma regra hardcoda contagem, lista de estados ou caminho que deveria vir da config?
- **P2**: alguma regra **silencia** quando não consegue ler o que precisa (arquivo ilegível, frontmatter
  inválido, diretório ausente) em vez de reportar? Este é o padrão mais perigoso — foi o do
  `analyzing`, que era ponto cego.
- **P3**: alguma depende de locale, ordenação de sistema de arquivos, fim de linha ou fuso?

Corrigir o que encontrar. **Registrar no relatório a lista completa das 17 com o veredito de cada
uma** — inclusive as conformes. Auditoria sem inventário não é auditoria.

**Acceptance criteria:**
- [ ] As 17 regras auditadas, com veredito registrado individualmente
- [ ] Defeitos encontrados corrigidos, ou registrados com justificativa se fora de escopo
- [ ] Nenhuma regra degrada silenciosamente ao falhar em ler o que precisa
- [ ] `go build`, `go test` e `go vet` verdes; testes dos 3 CLIs verdes

### ML-2B — Auditoria dos scripts de gate
**Status:** done
**Files affected:** `scripts/check-*.sh`, `scripts/smoke-integration-packages.sh`, `Makefile`

**Actions:**
Auditar contra P1–P3: `check-cli-parity.sh`, `check-identity-parity.sh`,
`check-integration-assets.sh`, `check-integration-cli-parity.sh`, `check-static-assets.sh`,
`check-validate-parity.sh`, `smoke-integration-packages.sh`.
- **P1**: números mágicos e listas hardcoded que deveriam derivar do catálogo/config.
- **P2**: `|| true`, `2>/dev/null` e `set +e` que engolem falha; comando ausente tratado como sucesso.
- **P3**: dependência de cor, locale, `PATH`, ordenação de `ls`/`find`, versão de runtime.

**Item extra herdado do ML-1A:** a mensagem de violação do `branch_has_wip_roadmap` lista **todos** os
roadmaps encontrados. Com `done/` agora incluído na busca, num projeto maduro isso vira uma parede de
texto — neste repositório já são 15 arquivos numa linha só. Truncar (ex.: 3 primeiros + contagem) ou
listar apenas os de `wip/`. Defeito de usabilidade, não de lógica, mas degrada uma mensagem que
existe para orientar.

⚠️ Atenção especial ao **P2 em shell**: `set -euo pipefail` no topo não protege comando dentro de
`$( )` nem o lado esquerdo de um pipe. Verificar caso a caso.

Registrar no relatório os 7 scripts com o veredito de cada um.

**Acceptance criteria:**
- [ ] Os 7 scripts auditados, com veredito individual registrado
- [ ] Nenhum engole falha silenciosamente
- [ ] Nenhum depende de ambiente para dar o mesmo resultado
- [ ] Cada script corrigido continua reprovando o que deveria

---

## Wave 3 — Falsificabilidade e documentação (agente único)
> Dependências: **barrier** — Wave 2 concluída.

### ML-3A — Testes de falsificação (P4) e documentação dos princípios
**Status:** done
**Files affected:** testes nos 3 CLIs, `scripts/`, `docs/`

**Actions:**
1. **Teste de falsificação por gate.** Cada script de paridade e cada regra corrigida ganha um teste
   que **monta o cenário negativo e prova que o gate reprova**. Hoje nenhum tem — os quatro defeitos
   existiam com o CI verde.
   Usar o mecanismo de teste já existente em cada CLI. **Não criar framework.**
2. **Sem resíduo:** o cenário negativo é montado e desmontado; nada fica no repositório.
3. **Documentar os princípios P1–P4** em `docs/` (seção no `cli-parity.md` ou documento próprio), com
   os quatro defeitos reais como exemplo. Quem escrever o próximo gate precisa encontrar isso.
4. Referenciar as notas de vault existentes.

**Acceptance criteria:**
- [ ] Cada script de paridade tem teste que prova reprovação do cenário negativo
- [ ] Cada regra corrigida na Wave 2 tem teste equivalente
- [ ] Nenhum resíduo após os testes
- [ ] Princípios documentados com os casos reais
- [ ] `make quality` verde **sem** variável de ambiente auxiliar

---

## Log de execução

**2026-07-27 — ML-1A concluído e auditado.**

`make quality` verde. Reúso confirmado: o agente extraiu `resolveStateDirs(cfg, state)` e derivou
`wip` e `done` dele — uma única resolução de caminho, uma única `normalizeBranchSlug`. Era a
exigência explícita, porque duplicar resolução foi a causa raiz do `roadmap_dir` divergente na REQ
anterior.

**Verificação empírica feita no próprio repositório, nesta branch** — o cenário que falhou nas duas
REQs anteriores:

| Estado do roadmap | `trackfw validate` |
|---|---|
| em `wip/` | ✓ sem violações |
| **movido para `done/`** (DoD cumprida) | **✓ sem violações** ← antes reprovava |
| em `done/` com slug **diferente** da branch | ✗ reprova, como deve |

A terceira linha é o que prova que a regra não afrouxou: aceitar `done/` sem exigir casamento de slug
teria feito o gate nunca mais reprovar.

**Efeito colateral encontrado na prova negativa** → movido para o ML-2B: com `done/` na busca, a
mensagem passa a listar todos os roadmaps encontrados — 15 numa linha só neste repositório. Orienta
menos do que antes.

**2026-07-27 — Wave 2 concluída e auditada na barrier.**

A sessão anterior foi interrompida após os commits `3dbeae5` (ML-2A) e `ea79082` (ML-2B), antes de o
orquestrador rodar a barrier e marcar o status. Barrier executada agora: `make quality` **verde** —
Go build/vet/test ok, Node.js 228 pass, Python 586 pass, e os 6 gates de paridade
(`cli-parity`, `integration-cli-parity`, `validate-parity`, `static-assets`, `integration-assets`,
`identity-parity`) passando.

Inventários exigidos entregues: as 17 regras do validator com veredito individual (ML-2A) e os 7
scripts de gate com veredito individual (ML-2B), ambos registrados em `docs/agents-working-context.md`.
Itens fora de escopo foram **registrados com justificativa**, não silenciados — em particular
`adr_orphan` (walk errors, exige refator de assinatura), o padrão sistêmico `os.ReadFile → continue`
(~30 sites × 3 CLIs) e `stale_wip_days` ausente de `ProjectConfig`. Esses três são candidatos a REQ
própria, não dívida esquecida.

O item de usabilidade herdado do ML-1A (mensagem do `branch_has_wip_roadmap` listando 15 roadmaps)
**não foi corrigido**: o ML-2A tinha instrução explícita de não alterar essa regra e o ML-2B não
mexe no validator. Fica para o ML-3A, que já toca as regras corrigidas.

**2026-07-27 — ML-3A concluído e auditado. Roadmap encerrado.**

A primeira tentativa do ML-3A caiu por erro de API com o trabalho ainda no working tree; a segunda
completou a documentação e commitou tudo junto (`dc9a18f`), sem refazer o que já estava pronto.

Entregue:
- `scripts/check-gates-falsify.sh` — prova que os **6 gates de paridade reprovam** cenário negativo
  concreto (byte drift em static/integration assets, slug drift de identidade, regra removida do npm,
  comando ausente em duas superfícies). Cada asserção exige exit != 0 **e** o diagnóstico esperado —
  um gate que falhasse pelo motivo errado não passaria. Integrado ao alvo `parity`, roda em
  `make quality` sem variável auxiliar.
  **6 gates, não 7:** a Wave 2 inventariou 7 scripts, mas `smoke-integration-packages.sh` não é gate
  de paridade — é smoke de empacotamento, roda fora do alvo `quality` por decisão de custo, e foi
  auditado como conforme no ML-2B. Falsificá-lo exigiria montar pacotes npm/PyPI quebrados a cada
  `make quality`. Fica de fora conscientemente; os 6 gates que reprovam PR são os que ganharam prova.
- Testes negativos das regras corrigidas na Wave 2 nos 3 CLIs — inclusive o truncamento, com 4
  candidatos e asserção de string exata em ordem alfabética, nos três.
- Mensagem do `branch_has_wip_roadmap` truncada em 3 + `", e mais N"` — mesma formatação nos 3 CLIs.
- `docs/gate-design-principles.md` — P1–P4 ancorados nos 4 defeitos reais desta REQ, com checklist
  reutilizável e `check-gates-falsify.sh` apontado como o lugar canônico da prova negativa. Linkado
  de `docs/cli-parity.md`.

Auditoria do orquestrador: `make quality` verde (Go ok | Node 228 pass | Python 588 pass | 6 gates
positivos + 6 falsificações), `git status` sem resíduo após execução, `scripts/check-gates-falsify.sh`
commitado com modo `100755` — o Makefile o invoca sem `bash`.

**O encerramento deste roadmap é, ele próprio, a prova do ML-1A**: mover o arquivo de `wip/` para
`done/` nesta branch mantém `trackfw validate` verde. Era exatamente o cenário que reprovava antes.

## Acceptance Criteria

- [x] Todas as waves concluídas
- [x] Encerrar roadmap na própria branch deixa `trackfw validate` verde
- [x] Inventário completo das 17 regras e dos 7 scripts, com veredito individual
- [x] Todo gate corrigido tem prova de que ainda reprova
- [x] `make quality` verde nos 3 CLIs, sem variável auxiliar
- [x] Escopo negativo respeitado (sem framework novo, sem dependência nova, sem rebaixar severidade)

## Débito registrado (candidatos a REQ própria, não esquecimento)

Levantados na auditoria da Wave 2 e conscientemente deixados fora de escopo:

1. `adr_orphan` silencia erros de walk — exige refator da assinatura de `walkADRFilePaths`.
2. Padrão sistêmico `os.ReadFile → continue` (~30 sites × 3 CLIs) — viola P2 de forma difusa.
3. `staleWIPDays = 7` hardcoded — viola P1; o campo não existe em `ProjectConfig`.
4. `check-identity-parity.sh` com `TARGETS` hardcoded — derivar do catálogo exige lógica não-trivial
   de superfícies padrão.
5. **`trackfw roadmap move` não atualiza o `status` do frontmatter** — encontrado ao encerrar este
   próprio roadmap. `move ... done` reposiciona o arquivo mas deixa `status: wip`, e o `folder_status`
   imediatamente acusa `folder is "done" but status declares "wip"`. O comando que existe para cumprir
   a DoD produz um estado que o próprio validador reprova, e a correção fica manual. É o mesmo tipo de
   defeito do D4 (`branch_has_wip_roadmap` punindo a DoD) e merece REQ própria: o `move` deve reescrever
   o frontmatter, nos 3 CLIs.
