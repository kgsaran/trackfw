---
title: Go GenerateAttentionScripts precisava de parâmetro rootDir, não só exportação
date: 2026-08-04
tags: [go, generators, discover, attention-hooks, paridade]
---

## Contexto

REQ `docs/req/REQ-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`,
ML-1A (Go). O roadmap pedia só "exportar `generateAttentionScripts()`, sem alterar o conteúdo
gerado" e chamá-la em `InstallGates`. Uma exportação ingênua (`func GenerateAttentionScripts() error`
sem parâmetro) teria introduzido um bug silencioso.

## Achado não óbvio

A implementação original em `internal/generators/scaffold.go:682` escrevia em `"scripts"`
(caminho relativo ao **cwd do processo**), não a um `rootDir` passado por parâmetro. Isso é
inofensivo em `trackfw init` (roda sempre com cwd == raiz do projeto) e em `trackfw update`
(idem), mas `internal/discover/discover.go:InstallGates(r, rootDir, w)` é chamado com um
`rootDir` explícito que **não é necessariamente o cwd do processo** — os testes de
`InstallGates` em `discover_test.go` usam `t.TempDir()` como `rootDir` sem `os.Chdir`, e o
código de produção (`internal/commands/discover.go:125`) passa `cwd` só porque nesse call site
`cwd` já é o processo cwd, mas isso é incidental, não garantido pela assinatura da função.

Se a função exportada continuasse sem parâmetro, os scripts teriam sido escritos no cwd real
do processo `trackfw`, não em `rootDir` — silenciosamente errado em qualquer cenário onde
`InstallGates` for chamado com um `rootDir` diferente do cwd (testes, ou uma futura chamada de
`discover --init --path=<dir>`).

## Como foi confirmado

Comparação direta com o Node.js (`npm/src/generators/hooks.js:122`,
`generateAttentionScripts(cfg, cwd)`, com `const root = cwd || process.cwd()`) e o Python
(`pypi/trackfw/generators/init_gen.py:852`, `_generate_attention_scripts(cwd: str)`, com
`scripts_dir = os.path.join(cwd, 'scripts')`) — os dois outros CLIs **já** recebiam um parâmetro
de diretório raiz. Só o Go usava caminho relativo implícito. Confirma que a assinatura correta
para paridade real é `GenerateAttentionScripts(rootDir string) error` (rootDir vazio = cwd, para
não quebrar os call sites de `init`/`update`), não a exportação trivial sugerida
literalmente pelo texto do ML.

## Resolução

`internal/generators/scaffold.go`: `GenerateAttentionScripts(rootDir string) error` — usa
`filepath.Join(rootDir, "scripts")`; `rootDir == ""` cai para `"."` (mesmo comportamento de
antes). Call sites de `init`/`update` passam `""`. `internal/discover/discover.go:InstallGates`
passa o `rootDir` recebido, chamando antes de `InjectHooksDetected(rootDir)` (mesma ordem do
Python).

## Addendum — stdout também precisou de ajuste (não só o caminho de escrita)

Uma segunda pegadinha da mesma raiz: `GenerateAttentionScripts` também fazia
`fmt.Printf("  ✓ %s\n", signalPath)` usando o `signalPath` absoluto (porque
`internal/commands/discover.go` passa `cwd = os.Getwd()`, que é absoluto, como `rootDir` para
`InstallGates`). Isso teria feito `discover --init` no Go imprimir
`  ✓ /Users/.../scripts/trackfw-attention-signal.sh` em vez do texto fixo
`  ✓ scripts/trackfw-attention-signal.sh` que o Node.js sempre imprime (string literal em
`npm/src/generators/hooks.js:generateAttentionScripts` — não deriva de `cwd`). Corrigido para
sempre imprimir o caminho relativo `filepath.Join("scripts", ...)`, independente do valor de
`rootDir` usado para escrita. Confirmado empiricamente rodando `trackfw discover --init` real
(binário compilado) e `node npm/bin/trackfw discover --init` num fixture `git init` vazio: as
duas linhas de saída ficaram byte-idênticas entre Go e Node. Python não imprime nada em sucesso
para os attention scripts em nenhum runtime (`pypi/trackfw/generators/init_gen.py:_generate_attention_scripts`
não tem `print` de sucesso) — divergência pré-existente e fora do escopo desta REQ.

## Lição para próximos MLs de paridade

Ao portar uma função "óbvia" de um CLI para os outros dois, sempre comparar a **assinatura**
nos três runtimes antes de assumir que só falta "expor"/"chamar" — o parâmetro que falta às
vezes é o próprio motivo da lacuna de comportamento, não só de visibilidade.
