---
status: backlog
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md"
squad: ""
---

# Roadmap: Contrato canônico do roadmap e estado analyzing

> Created: 2026-07-27 | Status: backlog

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

**Status:** pending

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
- [ ] Dois defeitos reproduzidos nos três runtimes.
- [ ] XPASS reprova em todos os runtimes.
- [ ] Nenhum arquivo de produção alterado neste ML.
- [ ] `make quality` verde com falhas esperadas registradas.

**Validation commands:**
```bash
go test ./internal/generators -run 'SlashRoadmap|Analyzing' -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_init.py pypi/tests/test_generators_roadmap.py -q -rxX
make quality
```

---

## Wave 2 — Convergir criação e estados (2 MLs em paralelo)

> Dependencies: Wave 1 complete. Os MLs têm ownership distinto: templates de init × roadmap runtime.

### ML-2A — Slash-command gera roadmap canônico

**Status:** pending

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
- [ ] Templates Go, Node e Python produzem a mesma instrução canônica.
- [ ] Arquivo versionado e arquivos gerados não divergem.
- [ ] Testes de frontmatter reativados e verdes.
- [ ] Nenhuma alteração no runtime de movimentação neste ML.

**Validation commands:**
```bash
go test ./internal/generators -run SlashRoadmap -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_init.py -q
```

### ML-2B — Estado analyzing completo nos três CLIs

**Status:** pending

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
- [ ] `roadmap move <nome> analyzing` passa nos três CLIs.
- [ ] Flat e `by_agent` cobertos.
- [ ] `list`/`show` encontram o roadmap em analyzing.
- [ ] Frontmatter, header e log sincronizados.
- [ ] `trackfw validate` não gera `folder_status`.

**Validation commands:**
```bash
go test ./internal/generators ./internal/commands -run Analyzing -v
(cd npm && npm test)
python3 -m pytest pypi/tests/test_generators_roadmap.py pypi/tests/test_commands_roadmap_discover.py -q
```

---

## Wave 3 — Paridade, documentação e ciclo completo (1 ML)

> Dependencies: Wave 2 complete.

### ML-3A — Gate cross-CLI e prova de ciclo completo

**Status:** pending

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
- [ ] Gate detecta drift real entre templates dos três runtimes.
- [ ] Prova negativa P4 falha pelo motivo esperado.
- [ ] Ciclo completo verde nos três runtimes, flat e `by_agent`.
- [ ] Documentação PT-BR/EN atualizada.
- [ ] `make quality` e `trackfw validate` verdes.
- [ ] `git status` limpo após os testes.

**Validation commands:**
```bash
scripts/check-artifact-parity.sh
scripts/check-gates-falsify.sh
make quality
bin/trackfw validate --json
git status --short
```

## Global Acceptance Criteria

- [ ] As três waves concluídas na ordem.
- [ ] Slash-command e CLI compartilham um único contrato canônico de roadmap.
- [ ] `analyzing` funciona como estado real nos três CLIs.
- [ ] Paridade e ciclo completo protegidos contra regressão.
- [ ] Nenhum item fora de escopo implementado.
