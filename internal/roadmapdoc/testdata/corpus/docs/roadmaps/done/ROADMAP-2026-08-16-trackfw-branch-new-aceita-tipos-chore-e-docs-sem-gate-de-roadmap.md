---
status: done
date: 2026-08-16
req: "docs/req/REQ-2026-08-16-trackfw-branch-new-aceita-tipos-chore-e-docs-sem-gate-de-roadmap.md"
squad: "apolo-tf"
---

# Roadmap: `trackfw branch new` aceita tipos `chore` e `docs` sem gate de roadmap

> Created: 2026-08-16 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-16-trackfw-branch-new-aceita-tipos-chore-e-docs-sem-gate-de-roadmap.md`

Fix pequeno e bloqueante: sem ele **não é possível criar a branch da release 7.0.0** por nenhum
caminho sancionado (`git checkout -b` bloqueado pelo guard; `branch new` recusa `chore`).

### Estado apurado no código (2026-08-16)

| Comando | Trata `chore`/`docs` como | Onde |
|---|---|---|
| `trackfw ship` | housekeeping permitido, sem roadmap | regra 3 do `--help` |
| `trackfw commit` | housekeeping permitido, sem roadmap | regra 3 do `--help` |
| `trackfw branch new` | **recusa de todo** | `internal/commands/branch.go:16` (`branchValidTypes`), `:157` |

O vocabulário do `branch new` foi espelhado do `ship`, mas os dois já divergem na prática — o `ship`
aceita e só avisa; o `branch new` recusa.

## Acceptance Criteria
- [x] AC1 — `chore/<slug>` e `docs/<slug>` criam a branch, sem exigir roadmap.
- [x] AC2 — `feat`/`fix`/`refactor` continuam exigindo roadmap (gate **não** afrouxado).
- [x] AC3 — Tipo inválido continua recusado, com mensagem listando o vocabulário novo.
- [x] AC4 — Idêntico nos 3 CLIs; `scripts/check-branch-new-parity.sh` cobre os casos novos.
- [x] AC5 — `make quality` verde.

---

## Wave 1 — Fix nos 3 CLIs + paridade
> ML único: a mudança é pequena e o script de paridade é compartilhado, então dividir por stack
> criaria coordenação sem ganho.

### ML-1A — `chore`/`docs` aceitos, sem afrouxar o gate dos demais
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Arquivos afetados:**
- Go: `internal/commands/branch.go` (`branchValidTypes` :16, doc :58 e :157) + testes
  (`internal/commands/branch_test.go`)
- Node: equivalente em `npm/src/commands/branch.js` (ou onde o vocabulário estiver) + `npm/tests/`
- Python: equivalente em `pypi/trackfw/commands/branch.py` + `pypi/tests/`
- `scripts/check-branch-new-parity.sh`
- `docs/cli-parity.md`, `CLAUDE.md` (se documentarem o vocabulário)

**Ações:**
1. Acrescentar `chore` e `docs` ao vocabulário aceito.
2. **Separar vocabulário de gate**: para `chore`/`docs`, **pular** a checagem de roadmap em
   `wip/`/`done/`. Para `feat`/`fix`/`refactor`, **manter exatamente** o comportamento atual.
   ⚠️ Este é o ponto onde o ML pode dar errado em silêncio: afrouxar o gate para todos passaria nos
   testes de criação de branch e destruiria a garantia de governança. O teste de AC2 é o guarda.
3. Atualizar a mensagem de erro de tipo inválido para listar o vocabulário novo, **idêntica nos 3**.
4. Estender `check-branch-new-parity.sh` com os casos novos.

**Critérios de aceite:**
- [ ] `trackfw branch new chore/release-x.y.z` cria a branch com `wip/` **vazio**.
- [ ] `trackfw branch new docs/<slug>` idem.
- [ ] `trackfw branch new feat/<slug>` **sem** roadmap correspondente **continua bloqueando**, com a
      mesma mensagem de hoje — teste explícito de não-regressão do gate.
- [ ] `trackfw branch new banana/<slug>` recusado, mensagem listando `feat, fix, refactor, chore, docs`,
      byte-idêntica nos 3 CLIs.
- [ ] `make quality` verde, incluindo `check-branch-new-parity.sh`.

**Comando de validação:** `make quality`

---

## Notas
- **Dois débitos do `git-branch-guard`, ambos fora de escopo, para uma REQ própria do guard:**
  1. cobre `git checkout -b` mas **não** cobre `git switch -c` — brecha de contorno;
  2. **falso-positivo por prosa**: linha de *mensagem de commit* que **começa** com `git <sub>` é
     lida como comando e bloqueia o commit. Descoberto na prática hoje, ao escrever uma mensagem
     que documentava justamente este problema. Detalhe e contorno em
     `vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-2026-08-16.md`.
- Este fix precisa estar mergeado **antes** do PR de release 7.0.0.
