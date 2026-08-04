---
title: trackfw branch new — Go vazava "exit status N" e Node hardcodava exit 1 em vez de propagar o código real do git
date: 2026-08-04
tags: [go, nodejs, commands, branch, exit-code, paridade]
---

## Contexto

REQ `docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`,
Wave 2 (ML-2A Node.js). O contrato do comando (ML-1B) diz: "com match, executa `git checkout -b`,
propagando stdout/stderr/exit code do Git **literalmente**". O cenário "branch já existe" (um dos
cenários de teste explicitamente pedidos) é o único caminho que realmente exercita isso de ponta a
ponta — e nenhum dos dois runtimes fazia o que o contrato pedia, cada um de um jeito diferente.

## Achado 1 — Go vazava uma linha que o git nunca produziu

`internal/commands/branch.go`'s `defaultGitCheckout` retornava o erro cru de `exec.Command.Run()`
(um `*exec.ExitError`, cuja `.Error()` é literalmente a string `"exit status 128"`). Esse erro subia
até `RunE`, que o retorna para o cobra; `internal/commands/root.go:Execute()` **sempre** imprime
qualquer erro retornado (`fmt.Fprintln(os.Stderr, err)`), **independente** de `cmd.SilenceErrors`
(esse flag só desliga o auto-print interno do cobra dentro de `cmd.Execute()`, não o wrapper externo).

Resultado: `git checkout -b feat/x` num branch já existente imprimia a mensagem real do git
(`fatal: a branch named 'feat/x' already exists`) e **depois** uma segunda linha `exit status 128`
— um artefato de string interna do Go, nunca produzido pelo git, quebrando a promessa de "propagar
literalmente".

## Achado 2 — Node.js "propagava" um exit code fixo, não o real

`npm/src/branch/runner.js`'s `defaultGitCheckout` original convertia qualquer falha do
`spawnSync` num `new Error(...)` genérico, e `runBranchNew` traduzia isso pra `return 1` sempre —
perdendo o código real do git (128 para "branch already exists", mas poderia ser outro valor pra
outros erros). Não vazava texto espúrio como o Go, mas violava a mesma cláusula do contrato
("exit code ... literalmente") de um jeito diferente: sempre 1, nunca o código real.

O Python (`pypi/trackfw/commands/branch.py:_default_git_checkout`) já fazia certo desde o ML-2B —
`return result.returncode` direto — e ninguém tinha comparado os três runtimes nesse cenário
específico até agora.

## Como foi descoberto

Comparação empírica real (não leitura de código): criei um fixture `git init` com uma branch
`feat/existing-branch-test` já existente e rodei `trackfw branch new feat/existing-branch-test` nos
três binários reais. Go e Node divergiam entre si E do Python nesse único cenário — os testes
unitários de cada runtime (que injetam `execGitCheckout` como fake) nunca exercitam o
`defaultGitCheckout` de produção, então esse bug era invisível para `go test`/`npm test`/`pytest`
isoladamente. Só apareceu testando o CLI de verdade, subprocesso real.

## Resolução

- **Go**: `defaultGitCheckout` agora usa `os.Exit(exitErr.ExitCode())` diretamente quando o erro é
  um `*exec.ExitError` — sai do processo com o código exato do git, sem devolver erro nenhum pro
  cobra/Execute() imprimir por cima. Falhas sem `ExitError` (ex: binário `git` ausente) continuam
  retornando erro normalmente, já que nesse caso não existe diagnóstico do git pra confiar.
- **Node.js**: `defaultGitCheckout` mudou de contrato — retorna `number` (o exit code real de
  `spawnSync`, ou `1` só para spawn-failure/kill-por-sinal, espelhando a convenção já usada pro
  `trackfw barrier` em `docs/cli-parity.md`) em vez de `Error|null`. `runBranchNew` agora só repassa
  esse número (`return execGitCheckout(branchName)`), sem traduzir pra 1 fixo.
- **Python**: nenhuma mudança — já estava correto, serviu de terceiro ponto de comparação que
  confirmou qual dos outros dois (Go ou Node) precisava mudar em cada achado.

Confirmado empiricamente pós-fix: os três binários reais produzem stdout/stderr idênticos e
`exit=128` idêntico para o cenário "branch já existe".

## Lição para próximos MLs envolvendo subprocessos

Testes unitários com dependência injetada (fakes de `exec`/`spawnSync`/`subprocess.run`) não
provam nada sobre o wrapper de produção real (`defaultXxx`) — só provam a lógica que consome o
resultado. Quando o contrato promete "propagar literalmente" a saída de um subprocesso, o único
jeito de validar isso é rodar o binário real contra um subprocesso real e comparar, cenário a
cenário, entre os runtimes — exatamente como o gap de `discover --init` (REQ irmã desta) só foi
achado rodando os três binários reais, não lendo o código.
