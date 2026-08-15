# git-branch-guard: parser não é quote-aware, e pode bloquear o próprio teste manual

> 2026-08-14 — Apolo (fix pós-ML-4A, port Node.js de `GIT_BRANCH_GUARD_SCRIPT`)

## Contexto

O ML-1A/3A/3B/3C (`ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-...`) geram
`scripts/trackfw-git-branch-guard.sh` e o wireiam como hook `PreToolUse`/`Bash` real em
`.claude/settings.json`. Isso significa que, **na própria sessão de desenvolvimento deste
guard**, o script já está ativo e intercepta os comandos Bash do agente que está editando/
testando o próprio guard.

## Causa raiz não óbvia

`match_subcommand()` divide o comando recebido em segmentos por `;`/`&&`/`||`/`|`/quebra de
linha (fix dos 3 bugs reais do ML-4A: comando encadeado escapando, path absoluto escapando,
prosa citando "git commit" sendo lida como comando). Essa divisão **não é ciente de aspas de
shell** — ela corta a string onde quer que o caractere delimitador apareça, mesmo dentro de
uma string entre aspas simples/duplas.

Consequência prática: se um agente testando o guard escrever, no PRÓPRIO comando Bash, algo
como

```bash
echo '{"tool_input":{"command":"git status; git push origin HEAD"}}' | bash script.sh
```

o `;` dentro da string JSON entre aspas simples é, para o parser do guard, um separador de
comando real — o comando Bash inteiro (não o payload de teste) é dividido em dois segmentos,
e o segundo segmento (`git push origin HEAD"}}' | bash script.sh`) começa com o token `git`.
O hook PreToolUse da PRÓPRIA sessão bloqueia esse comando Bash antes que ele chegue a rodar —
sintoma: `bash "$S"` nunca executa, o tool call inteiro retorna um "error" contendo só a
`REASON` do guard, sem nenhum stdout do resto do script (inclusive `echo`s anteriores no mesmo
comando somem, porque a ferramenta descarta o buffer de stdout quando o hook nega a chamada).

## Como diagnosticar de novo

Se um teste manual de guard/hook falhar de forma "muda" (sem stdout esperado, só um bloco de
erro com a mensagem do próprio guard), suspeitar primeiro do guard interceptando o PRÓPRIO
comando de teste, não um bug no script sendo testado.

## Solução usada

Escrever os payloads de teste em arquivos via a ferramenta `Write` (não como texto literal em
um comando Bash) e invocar o script lendo do arquivo (`bash "$S" < arquivo.json`), evitando
qualquer substring `; git push`/`; git commit`/etc. no texto do próprio comando Bash.

## Limitação residual (não resolvida, documentada)

Uma linha dentro de um heredoc multi-linha que **começa** com o token `git` ainda bloqueia,
porque quebra de linha é tratada como fronteira de comando. Isso é aceito como ressalva
conhecida do fix (ver comentário em `npm/src/generators/hooks.js` acima de
`GIT_BRANCH_GUARD_SCRIPT` e em `pypi/trackfw/generators/init_gen.py` acima de
`_GIT_BRANCH_GUARD_SH`); `internal/generators/scaffold.go`'s doc comment do
`gitBranchGuardScript` ainda não documenta essa ressalva explicitamente — sinalizado para
`trackfw_architect` considerar adicionar lá também.

## Achado colateral (fora do escopo deste fix, sinalizado para o arquiteto)

O conteúdo gerado por `pypi/trackfw/generators/init_gen.py::_GIT_BRANCH_GUARD_SH` **diverge**
byte-a-byte do conteúdo gerado por Go (`internal/generators/scaffold.go:gitBranchGuardScript`)
e Node (`npm/src/generators/hooks.js:GIT_BRANCH_GUARD_SCRIPT`, confirmados idênticos entre si
via diff contra um binário Go recém-compilado). A lógica de `match_subcommand` do Python é
equivalente em comportamento (mesma técnica de segmentação + basename), mas usa uma
implementação diferente (`sed -E 's/(&&|\|\||[;|])/\n/g'` + `done <<<"$segments"` via
herestring, em vez de `sed -e 's/&&/\n/g' -e 's/||/\n/g' -e 's/[;|]/\n/g'` + `done <<EOF...EOF`
via heredoc do Go/Node) e comentários diferentes. Não fiz nenhuma mudança em `pypi/` — fora do
escopo desta tarefa (só Node.js) e a Python já foi implementada por outro ML/sessão.

**Atualização 2026-08-14 (sessão seguinte, Apolo/backend, port Python dos mesmos 3 bugs):**
resolvido. `_GIT_BRANCH_GUARD_SH` foi reescrito para usar exatamente o mesmo texto do Go
(`normalized=$(... sed -e 's/&&/\n/g' -e 's/||/\n/g' -e 's/[;|]/\n/g')` + `done <<EOF...EOF`,
mesmos nomes de variável `normalized`/`seg`/`seg_trimmed`/`first`/`base`, mesmo comentário).
Confirmado byte-idêntico entre os 3 stacks via dump direto (script Go gerado por `go test`
temporário, Node via `generateGitBranchGuardScript` chamado por `node -e`, Python via
`_GIT_BRANCH_GUARD_SH` direto) — `diff` vazio nas 3 combinações. Ver
[[git-branch-guard-pipe-into-while-loses-return-status-2026-08-14]] para o motivo pelo qual a
primeira tentativa de correção em Python (antes de copiar o texto exato do Go) usava um pipe em
vez de heredoc e falhava silenciosamente — a mesma armadilha que o heredoc do Go evita.
