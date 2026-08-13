---
status: done
date: 2026-08-13
req: "docs/req/REQ-2026-08-13-fronteira-de-escrita-dos-agentes-auditores-e-coerente-com-as-ferramentas-concedidas.md"
squad: "Prometeu"
---

# Roadmap: Fronteira de escrita dos agentes auditores

> Created: 2026-08-13 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-13-fronteira-de-escrita-dos-agentes-auditores-e-coerente-com-as-ferramentas-concedidas.md`

Os três agentes **auditores** — `code-quality` (Hefesto), `security` (Hades), `ux` (Atena) — têm
definições **internamente contraditórias**: exigem escritas que o `tools:` não concede, e a frase
*"You do not modify code"* colide com duas outras que pressupõem edição e implementação.

**Consequência observada:** na mesma sessão, sob a mesma redação, **Hades escreveu pareceres sem
reclamar** e **Hefesto recusou** um ML equivalente ao que já executara em 4 PRs. Roteamento
imprevisível.

**Corrigir no gerador**, nunca no `~/.claude/agents/` — o instalado é sobrescrito a cada
`trackfw update`.

## Acceptance Criteria

- [x] Os 3 assets concedem `Write` e `Edit`.
- [x] Proibição delimitada a **código de produto**; o que o papel **pode** escrever está **afirmado**.
- [x] Contradições #2 e #3 reconciliadas.
- [x] **Paridade byte-a-byte** nos 3 stacks (9 arquivos).
- [x] Nenhum outro agente alterado.
- [x] `make quality` verde.

### Escopo negativo

Ver a REQ. Em resumo: **não** tocar no instalado, em agentes implementadores, no preset de
identidade, nem em `model:`/`memory:`.

---

## Wave 1 — Assets dos 3 auditores (1 ML)
> Dependências: nenhuma.

### ML-1A — Reconciliar a fronteira de escrita
**Status:** ✅ Concluído (Prometeu; auditado e aprovado por Zeus em 2026-08-13)
**Agente:** Prometeu (`prometeu-tf`) — é o especialista em **configuração de assistente e contratos de
tool-calling**; o alvo é definição de agente, não código de produto.

**Arquivos (9):** `{internal,npm/src,pypi/trackfw}/integrations/assets/agents/{code-quality,security,ux}.md`

**Ações — texto exato, para não reintroduzir ambiguidade:**

**1. `tools:` — acrescentar `Write, Edit` ao fim da lista existente**, preservando a ordem atual:

```
tools: Read, Grep, Glob, Bash, WebSearch, AskUserQuestion, Write, Edit
```

**2. Substituir a seção `## Reporting boundary`** inteira por:

```markdown
## Reporting boundary
You do not modify **product code** — `internal/`, `npm/src/`, `pypi/trackfw/` and their tests. Report
findings ordered by severity, each with concrete evidence (file, line, and the observed behavior),
and hand off the fix to the role that owns the code. Never weaken a control, a test or a permission
to make something pass.

You **do** write your own artifacts, and refusing to is a scope error in the opposite direction: your
report or assessment, the entry in `docs/agents-working-context.md`, and any documentation the
orchestrator assigns you. Writing these is not "modifying code".
```

**3. Reconciliar a contradição #2** — na seção `## Governance prerequisite`, trocar
*"Do not edit code without a requirement and a roadmap already in the `wip` state"* por
*"Do not produce deliverables without a requirement and a roadmap already in the `wip` state"*.

**4. Reconciliar a contradição #3** — na seção `## Git authority`, trocar
*"refuse to implement anything without one"* por *"refuse to act without one"*.

⚠️ **Ajuste por papel, quando o texto citar domínio:** o parágrafo novo fala em *"any documentation
the orchestrator assigns you"* — genérico de propósito, serve aos três. **Não** especialize por
agente; a generalidade é o que evita a próxima ambiguidade.

⚠️ **As 4 mudanças valem para os 3 agentes.** Confira que o texto de partida é idêntico nos três
antes de aplicar — se **não** for, **pare e reporte**: divergência pré-existente é achado.

**Critérios de aceite:**
- [x] As 4 mudanças aplicadas nos 3 agentes × 3 stacks = **9 arquivos**.
- [x] **Byte-identidade entre os stacks** para cada agente (`md5` igual nos 3).
- [x] `git diff` **não** mostra alteração em nenhum outro asset de agente.
- [x] `model:` e `memory:` inalterados.
- [x] `bash scripts/check-integration-assets.sh` sem falha.
- [x] `go build ./... && go test ./...`, `npm --prefix npm test`,
      `python3 -m pytest pypi/tests -q` verdes — **há testes que asseveram conteúdo de asset**
      (`internal/integrations/*_test.go`); se algum quebrar, **atualize-o**, não contorne.
- [x] `make quality` exit 0.

**Comandos de validação:**
```bash
for a in code-quality security ux; do
  md5 -q internal/integrations/assets/agents/$a.md \
         npm/src/integrations/assets/agents/$a.md \
         pypi/trackfw/integrations/assets/agents/$a.md
done   # 3 hashes iguais por agente
bash scripts/check-integration-assets.sh
make quality
```

---

## Wave 2 — Verificação do artefato gerado (1 ML)
> Dependências: Wave 1.

### ML-2A — O agente instalado reflete a mudança?
**Status:** ✅ Concluído (Prometeu; auditado e aprovado por Zeus em 2026-08-13)
**Agente:** Prometeu (`prometeu-tf`)

Gerar os agentes num `$HOME` **isolado** e confirmar que o arquivo produzido carrega `Write, Edit` e
o texto novo — **incluindo** o caminho de renomeação por identidade (`trackfw-code-quality.md` com
`name: hefesto-tf`), que é como o instalado real existe nesta máquina.

⛔ **Nunca** escrever no `~/.claude/` real do usuário. Sempre `HOME` isolado.

**Critérios de aceite:**
- [x] Arquivo gerado em `$HOME` isolado contém `Write, Edit` e a nova `Reporting boundary`.
- [x] Caminho com identidade aplicada produz o mesmo corpo, só com `name`/`description` renomeados.
- [x] Confirmação explícita de que `~/.claude/` do usuário **não** foi tocado.

---

## Notas de execução

- **Autoridade de Git:** apenas Zeus.
- **Paridade nos 3 stacks** é o núcleo deste roadmap, não detalhe.
- **Por que Prometeu e não Apolo:** o alvo é **definição de agente e contrato de tool-calling**, não
  lógica de produto. Apolo continua sendo o dono de `internal/`, `npm/src/`, `pypi/trackfw/` quando o
  que muda é comportamento do CLI.
