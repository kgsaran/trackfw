# Agente polui o repositório ao rodar o CLI manualmente — e `no-repo-mutation` não pega

> 2026-07-29 · domínio: workflow de agentes / gates · descoberto durante a Wave 2-bis do roadmap
> `install-pula-artefato-desatualizado-em-vez-de-abortar`

## Sintoma

Após dois agentes paralelos morrerem por limite de sessão, `git status` mostrava:

```
 M AGENTS.md
 M CLAUDE.md
?? .cursor/
```

`AGENTS.md` e `CLAUDE.md` com +51 linhas cada — um bloco `<!-- trackfw:rules:start -->` injetado —
e `.cursor/rules/trackfw.mdc` criado. Nada disso pertencia ao escopo dos MLs.

## Causa raiz

Um agente executou `trackfw init --ai-tools <tool>` (ou caminho que chama
`generators.InjectRulesForTool`) **com o cwd na raiz do repositório real**, para validar o
comportamento manualmente. `InjectRulesForTool` é idempotente mas não é hipotético: escreve o bloco
de regras no repositório onde é invocado.

## O que torna isso não óbvio

**`make quality` passa exit 0 com a árvore poluída.** Confirmado empiricamente: rodei as três suítes
(`go test ./...`, `npm test` = 329 passed, `python3 -m pytest` = 694 passed) e `make quality`
completo — todos verdes, todos deixando `git status` limpo. O cenário
`falsify/no-repo-mutation` do `check-gates-falsify.sh` (introduzido no ML-6I) **funciona**: ele
compara `git status --porcelain` antes e depois de rodar **os gates**.

O que ele não cobre — e por construção não pode — é um comando ad-hoc que o agente digita fora do
pipeline. O gate guarda o pipeline, não a sessão do agente.

Então a poluição sobrevive silenciosa: nenhuma suíte falha, nenhum gate reprova, e se o agente
commitar com `git add -A` os arquivos entram no PR como se fossem parte do trabalho.

## Como detectar

`git status --short` antes de qualquer `git add`. Se aparecerem `CLAUDE.md`, `AGENTS.md`,
`GEMINI.md`, `.github/copilot-instructions.md`, `.windsurfrules`, `.amazonq/`, `.cursor/` ou
`.claude/agents/trackfw-*.md` sem que o ML os tenha listado, é poluição de invocação manual.

## Remediação

```bash
git checkout -- AGENTS.md CLAUDE.md   # e demais arquivos de regras tocados
rm -rf .cursor                        # untracked: remover à mão
```

## Prevenção

Ao validar `init`/`install` manualmente, **sempre** com cwd descartável e HOME isolado:

```bash
tmp=$(mktemp -d) && HOME="$tmp/home" bin/trackfw init --ai-tools gemini
```

É a mesma disciplina que o ML-6I impôs ao `install_claude_agents()` do script de gates, que herdava
o `cwd` de quem o invocava. A lição vale para o agente humano-no-loop também, não só para o script.

## Relacionado

- `docs/roadmaps/done/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md` — ML-6I
- `scripts/check-gates-falsify.sh` — cenário `falsify/no-repo-mutation`
