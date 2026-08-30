---
title: rodar as 3 suítes completas em Windows não basta — 8 dos 11 defeitos da issue #216 não acendem vermelho por motivo próprio
tags: [ci, windows, gotcha, teste, metodo]
date: 2026-08-30
related: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## O padrão

ML-0A (REQ-2026-08-30-ci-nao-exercita-windows-...) partiu da suposição implícita de que "rodar
`go test ./...`, `npm test`, `pytest pypi/tests` inteiros num `windows-latest`" reproduziria os 11
defeitos conhecidos da issue #216. Verificando código **e** teste item a item (não só "existe teste
no pacote certo"), só **2** dos 11 acendem vermelho com confiança direta, e um terceiro de forma
ruidosa. Os outros 8 não vão aparecer na primeira execução do job largo — cada um por um motivo
diferente, e nenhum desses motivos é "o defeito não existe".

## As três formas de "a suíte não vai pegar isto"

1. **O teste faz mock/monkeypatch do próprio ponto que falha.** `pypi/tests/test_init_identity.py` e
   `test_identity_wizard.py` fazem `monkeypatch.setattr("sys.stdin.isatty", lambda: ...)` em todo
   ponto de teste — substituindo exatamente a chamada real que mente para `NUL` no Windows. A suíte
   nunca invoca `isatty()` de verdade.
2. **O teste lê o resultado por uma via que mascara o sintoma.** `test_generators_roadmap.py:774-836`
   já abre os arquivos gerados em `"rb"` (bytes crus) — mas só compara idempotência
   (`bytes_before == bytes_after`), nunca asserta ausência de `\r\n`. O mecanismo certo (leitura
   binária) está lá; falta o oráculo (o assert de conteúdo esperado).
3. **O caminho de produção que dispara o defeito nunca é exercitado pelo teste que existe no mesmo
   arquivo/pacote.** `test_commands_basic.py:287` só testa `run_trackfw("roadmap", "--help")`
   (help de subparser) via subprocess real — nunca `run_trackfw()` sem argumento nenhum, que é o
   único caminho de produção (`cli.py:180`, `parser.print_help()`) que renderiza a `description=` do
   parser raiz com o `→` que quebra em cp1252.

## O achado que generaliza

Confirmar "existe um teste para X" não é o mesmo que confirmar "o teste exercita o código real que
falha em X, pela via real que falha". Para qualquer REQ que dependa de um job de CI "nascer vermelho"
reproduzindo defeitos conhecidos, o critério de aceite não pode ser "rodar a suíte completa" — precisa
ser, item a item, "ler o código do defeito E o teste que deveria cobri-lo, e confirmar que a chamada
real (não mockada) é atingida pela via real (não by-passada)".

## Vetor de ameaça descoberto no caminho

A isolação de `$HOME` da própria suíte de testes é vácua no Windows nos 3 runtimes ao mesmo tempo:
`pypi/tests/conftest.py` (`os.environ["HOME"] = fake_home`) e
`internal/validator/main_test.go` (`os.Setenv("HOME", home)`) isolam via variável de ambiente `HOME`,
que a produção não lê no Windows (`os.path.expanduser`/`os.UserHomeDir` leem `%USERPROFILE%`). Rodar
`go test ./...` no Windows pode escrever de verdade na home real do runner efêmero — e como o Go
paraleliza pacotes por padrão, isso vira condição de corrida dentro de uma única execução do job,
mascarada como "teste Windows instável" em vez do defeito real.

## Onde está o detalhe completo

`docs/roadmaps/wip/ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md`,
seção "Resultado do ML-0A" — tabela item a item dos 11, com o file:line de cada evidência.
