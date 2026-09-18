---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md"
squad: ""
---

# Roadmap: Contrato canônico do roadmap e estado analyzing

> Created: 2026-07-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`

O produto já reconhece `analyzing` no scaffold e no validator, mas os três comandos de movimentação
o rejeitam. Em paralelo, `/trackfw:roadmap` gera um terceiro formato de roadmap sem frontmatter,
divergente do artefato produzido por `roadmap new`. O objetivo é tornar criação e transição um único
contrato verificável nos três runtimes.

### Ordem das waves

```
Wave 1 (1A) ─ barrier ─> Wave 2 (2A ‖ 2B) ─ barrier ─> Wave 3 (3A)
  provas negativas          slash template | estado        paridade e ciclo E2E
```

---

## Wave 1 — Provas negativas do contrato quebrado (1 ML)

> Dependencies: none.

### ML-1A — Expor divergência do slash-command e rejeição de analyzing

**Status:** done

**Files affected:**
- `internal/generators/scaffold_test.go`
- `internal/generators/roadmap_test.go`
- `npm/tests/init.test.js`
- `npm/tests/roadmap_move.test.js`
- `pypi/tests/test_generators_init.py`
- `pypi/tests/test_generators_roadmap.py`

**Actions:**
1. Adicionar um teste por runtime que gere/inspecione `.claude/commands/trackfw/roadmap.md` e exija:
   - bloco `---` no início do template de artefato;
   - chaves `status: backlog`, `date:`, `req:` e `squad:`;
   - `req:` preenchido com o caminho da REQ selecionada.
2. Adicionar testes por runtime que criem um roadmap canônico em `backlog/` e executem
   `move ... analyzing`, esperando sucesso, arquivo em `analyzing/`, frontmatter/header sincronizados
   e log de transição.
3. Cobrir layout flat e `by_agent`; o caso `by_agent` deve preservar o agente na resolução do path.
4. Marcar os seis grupos como falha esperada strict:
   - Go: helper que acusa XPASS, sem `t.Skip`;
   - Node.js: `testSkip`/helper equivalente que acusa XPASS;
   - Python: `pytest.mark.xfail(strict=True)`.
5. Registrar a saída que prova os dois defeitos antes de qualquer correção.

**Acceptance criteria:**
- [x] Dois defeitos reproduzidos nos três runtimes.
- [x] XPASS reprova em todos os runtimes.
- [x] Nenhum arquivo de produção alterado neste ML.
- [x] `make quality` verde com falhas esperadas registradas.

**Validation commands:**
```bash
go test ./internal/generators -run 'SlashRoadmap|Analyzing' -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_init.py pypi/tests/test_generators_roadmap.py -q -rxX
make quality
```

**Execution evidence (Artemis, 2026-07-27):**
- Go: `go test ./internal/generators -run 'SlashRoadmap|Analyzing' -v` passou; xfail registrou
  `invalid state "analyzing"` em flat/by_agent e ausência do início canônico
  ` ```markdown` seguido de `---` no slash-command.
- Node.js: `(cd npm && npm test)` passou com `264 pass`, `0 fail`; xfails registraram o slash-command
  sem frontmatter canônico e `invalid state "analyzing"` em flat/by_agent.
- Python: `python3 -m pytest pypi/tests/test_generators_init.py pypi/tests/test_generators_roadmap.py -q -rxX`
  passou com `58 passed, 3 xfailed`; xfails registraram o slash-command sem frontmatter canônico e
  rejeição de `analyzing` em flat/by_agent.
- Gate final: `make quality` passou; suíte Python completa reportou `612 passed, 3 xfailed` e
  `scripts/check-gates-falsify.sh` reportou `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`.
- Escopo preservado: somente testes e artefatos de governança foram alterados; nenhum arquivo de produção foi editado.

---

## Wave 2 — Convergir criação e estados (2 MLs em paralelo)

> Dependencies: Wave 1 complete. Os MLs têm ownership distinto: templates de init × roadmap runtime.

### ML-2A — Slash-command gera roadmap canônico

**Status:** done

**Files affected:**
- `internal/generators/scaffold.go`
- `npm/src/generators/init.js`
- `pypi/trackfw/generators/init_gen.py`
- `.claude/commands/trackfw/roadmap.md`
- testes do ML-1A referentes ao slash-command

**Actions:**
1. Substituir o formato instruído no slash-command pelo contrato canônico:
   ```yaml
   ---
   status: backlog
   date: <YYYY-MM-DD>
   req: "docs/req/<arquivo-selecionado>.md"
   squad: ""
   ---
   ```
2. Manter header `> Created: <YYYY-MM-DD> | Status: backlog` e as waves/microlotes existentes.
3. Exigir caminho relativo completo no campo `req:`; não aceitar basename ou link Markdown.
4. Atualizar o comando versionado em `.claude/commands/trackfw/roadmap.md` com o mesmo conteúdo
   produzido pelo scaffold.
5. Reativar os testes correspondentes do ML-1A.

**Acceptance criteria:**
- [x] Templates Go, Node e Python produzem a mesma instrução canônica.
- [x] Arquivo versionado e arquivos gerados não divergem.
- [x] Testes de frontmatter reativados e verdes.
- [x] Nenhuma alteração no runtime de movimentação neste ML.

**Validation commands:**
```bash
go test ./internal/generators -run SlashRoadmap -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_init.py -q
```

**Execution evidence (Apolo, 2026-07-27):**
- Go/Node/Python: slash-command atualizado para instruir frontmatter canônico com
  `status: backlog`, `date: <YYYY-MM-DD>`, `req: "docs/req/<arquivo-selecionado>.md"` e
  `squad: ""`, mantendo header `> Created: <YYYY-MM-DD> | Status: backlog`, waves e microlotes.
- Decisão de interpolação: como `/trackfw:roadmap` é um template de instrução, `req:` permanece como
  placeholder explícito `docs/req/<arquivo-selecionado>.md`; o próprio comando instrui preencher esse
  campo com o caminho relativo completo da REQ selecionada, nunca basename nem link Markdown.
- Arquivo versionado `.claude/commands/trackfw/roadmap.md` alinhado ao conteúdo gerado. Os testes
  reativados comparam byte a byte o `roadmap.md` gerado com o arquivo versionado nos três runtimes.
- Testes focados: `go test ./internal/generators -run SlashRoadmap -v` passou; `npm test -- --test-name-pattern=SlashRoadmap`
  passou com `264 pass`; `python3 -m pytest pypi/tests/test_generators_init.py -q` passou com
  `39 passed`.
- Gates amplos: `go build ./...` passou com aviso não bloqueante de cache Go fora do sandbox;
  `go test ./...` passou; `(cd npm && npm test)` passou com `264 pass`; `bin/trackfw validate --json`
  retornou `0 violations` e `0 warnings`.
- Gate composto: `make quality` passou; Python completo reportou `613 passed, 2 xfailed` e
  `scripts/check-gates-falsify.sh` reportou `Falsification checks passed (all 9 scenarios, 8 gates proved non-vacuous)`.
- Correção pós-auditoria: restaurado o bloco estrutural `ML-1B` e `Wave 2` no template materializado
  e nos geradores Go/Node/Python; os testes focados agora afirmam explicitamente esses trechos para
  impedir nova remoção.
- Escopo preservado: nenhum runtime de movimentação foi alterado; xfails de `analyzing`/`move` foram mantidos.

### ML-2B — Estado analyzing completo nos três CLIs

**Status:** done

**Files affected:**
- `internal/generators/roadmap.go`
- `internal/commands/roadmap.go`
- `npm/src/generators/roadmap.js`
- `npm/src/commands/roadmap.js`
- `pypi/trackfw/generators/roadmap.py`
- `pypi/trackfw/commands/roadmap.py`
- testes do ML-1A referentes a `analyzing`

**Actions:**
1. Adicionar `analyzing` às listas canônicas de estados válidos, ordem de listagem e resolvers dos
   três runtimes.
2. Garantir suporte flat e `by_agent` em `move`, `list`, `show` e busca por nome.
3. Reutilizar a reescrita existente para produzir `status: analyzing` no frontmatter e
   `| Status: analyzing` no header, sem tocar ocorrências no corpo.
4. Gravar a transição `backlog → analyzing` no mesmo `.trackfw-log` e preservar o agente no layout
   `by_agent` conforme o contrato Go/Node.
5. Reativar os testes correspondentes do ML-1A.

**Acceptance criteria:**
- [x] `roadmap move <nome> analyzing` passa nos três CLIs.
- [x] Flat e `by_agent` cobertos.
- [x] `list`/`show` encontram o roadmap em analyzing.
- [x] Frontmatter, header e log sincronizados.
- [x] `trackfw validate` não gera `folder_status`.

**Validation commands:**
```bash
go test ./internal/generators ./internal/commands -run Analyzing -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_roadmap.py pypi/tests/test_commands_roadmap_discover.py -q
```

**Execution evidence (Apolo, 2026-07-27):**
- Go: `roadmapStateOrder` e validação de estado em `internal/generators/roadmap.go` agora incluem
  `analyzing`; `roadmap move` aceita o estado, move para `analyzing/`, sincroniza `status:` e
  `| Status:`, e `findRoadmap`/`show`/`list` resolvem o arquivo em layout flat e `by_agent`.
- Node.js: `VALID_STATES`/`STATE_ORDER` em `npm/src/generators/roadmap.js` agora incluem
  `analyzing`; os testes de move foram reativados como obrigatórios e adicionam cobertura de
  `listRoadmaps`/`showRoadmap` para flat e `by_agent`.
- Python: `VALID_STATES`/`STATE_ORDER` em `pypi/trackfw/generators/roadmap.py` agora incluem
  `analyzing`; `move_roadmap` preserva o prefixo do agente no `.trackfw-log`; argparse herda
  `choices=VALID_STATES` e os helpers de `list`/`show` cobrem `analyzing`.
- Mensagens públicas de `roadmap move` foram alinhadas em Go e nos catálogos i18n npm/PyPI
  (`en-US`, `pt-BR`, `es-ES`) para listar `backlog|analyzing|wip|blocked|done|abandoned`.
- Validação focada: `go test ./internal/generators ./internal/commands -run Analyzing -v` passou;
  `(cd npm && npm test -- --test-name-pattern=roadmap_move)` passou com `264 pass`;
  `python3 -m pytest pypi/tests/test_generators_roadmap.py pypi/tests/test_commands_roadmap_discover.py -q`
  passou com `52 passed`.
- Gates amplos: `go build ./...` passou com aviso não bloqueante de cache Go fora do sandbox;
  `go test ./...` passou; `(cd npm && npm test)` passou com `264 pass`; `python3 -m pytest pypi/tests -q`
  passou com `619 passed`; `git diff --check` passou; `bin/trackfw validate --json` retornou
  `0 violations` e `0 warnings`.

---

## Wave 3 — Paridade, documentação e ciclo completo (1 ML)

> Dependencies: Wave 2 complete.

### ML-3A — Gate cross-CLI e prova de ciclo completo

**Status:** done

**Files affected:**
- `scripts/check-artifact-parity.sh` ou novo gate específico reutilizando seu harness
- `scripts/check-gates-falsify.sh`
- `Makefile`
- `docs/cli-parity.md`
- `site/guide/commands.md`
- `site/en/guide/commands.md`
- testes de integração nos três CLIs

**Actions:**
1. Gerar o slash-command pelos três runtimes em diretórios temporários e comparar o conteúdo
   byte a byte.
2. Adicionar prova negativa P4: corromper o template de um runtime e afirmar que o gate reprova com
   diagnóstico identificando runtime e arquivo.
3. Executar o ciclo completo em cada runtime:
   - gerar o slash-command;
   - materializar um roadmap conforme a instrução;
   - mover `backlog → analyzing`;
   - executar validate;
   - confirmar ausência de `folder_status` e existência do log.
4. Documentar frontmatter obrigatório e `analyzing` como estado válido em PT-BR e inglês.
5. Integrar o gate ao `make quality` sem variável auxiliar e sem resíduos.

**Acceptance criteria:**
- [x] Gate detecta drift real entre templates dos três runtimes.
- [x] Prova negativa P4 falha pelo motivo esperado.
- [x] Ciclo completo verde nos três runtimes, flat e `by_agent`.
- [x] Documentação PT-BR/EN atualizada.
- [x] `make quality` e `trackfw validate` verdes.
- [x] `git status` limpo após os testes.

**Validation commands:**
```bash
scripts/check-artifact-parity.sh
scripts/check-gates-falsify.sh
make quality
bin/trackfw validate --json
git status --short
```

## Acceptance Criteria

- [ ] As três waves concluídas na ordem.
- [ ] Slash-command e CLI compartilham um único contrato canônico de roadmap.
- [ ] `analyzing` funciona como estado real nos três CLIs.
- [ ] Paridade e ciclo completo protegidos contra regressão.
- [ ] Nenhum item fora de escopo implementado.
