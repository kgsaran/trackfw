---
status: Done
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: "docs/adr/ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md"
---

# REQ: O CI não exercita Windows, e os sete defeitos da issue #216 são invisíveis para o projeto

> Date: 2026-08-30 | Status: Open

## Motivation

**@lourivalgarciajunior** reportou sete defeitos de Windows na v7.3.0 (issue #216), todos verificados
por mim contra o código — inclusive as contagens. Investigando, achamos mais três; ele reportou mais
três em comentários. **Onze no total.**

Todos existiam porque **o nosso CI não olha**. O job `windows-integrations-resolve`
(`.github/workflows/quality.yml:83`) roda **três invocações dirigidas** — um `go test -run`, um
arquivo Node, um arquivo Python — como guard de um bug específico de path. Nenhuma suíte completa,
nenhum `validate`, nenhum `init`.

O projeto publica três CLIs e não sabe se funcionam em Windows.

**E duas correções de segurança desta semana não são validadas lá:** os testes das guardas de symlink
exigem `os.symlink`, que precisa de Developer Mode. **Doze testes vermelhos**, nenhum sinal sobre a
guarda funcionar — na plataforma onde o comportamento de link é mais ambíguo.

## Acceptance Criteria

- [ ] **AC1** — O job de Windows roda as **três suítes completas** (`go test ./...`,
      `npm test`, `pytest pypi/tests`), não invocações dirigidas.
- [ ] **AC2** — **Suíte de reprodução de defeito**, uma verificação explícita por defeito conhecido,
      mapeada um-para-um aos itens da issue #216, exercitando o **caminho real, sem mock**. É ela que
      **nasce vermelha**. A Wave 0 mediu que as três suítes sozinhas expõem **2 dos 11** — entregar
      só elas produziria a falsa sensação de cobertura que esta REQ existe para eliminar.
- [ ] **AC2b** — A saída da primeira execução é anexada ao roadmap como **linha de base**. Cada
      verificação que **não** falhar precisa de justificativa medida: ou o defeito não se manifesta
      no runner (e isso vira residual declarado), ou a verificação está errada.
- [ ] **AC3** — Mapeamento explícito entre cada falha do job e o item correspondente da issue.
      Falha do job que **não** corresponda a defeito conhecido é achado novo e vira registro.
- [ ] **AC4** — O job entra **não bloqueante** (`continue-on-error`) até a última correção, e só
      então vira obrigatório. Sem isso, paralisa todo trabalho não relacionado.
- [ ] **AC5** — Sonda `workflow_dispatch` separada, respondendo perguntas pontuais com saída bruta:
      modo devolvido por `os.Stat`; `Lstat` diante de junction; `isatty` sobre `NUL`; encoding do
      console; terminador de linha dos arquivos que os geradores escrevem; presença de `sh`/`bash` no
      `PATH`.
- [ ] **AC6** — A sonda **não** substitui o job de regressão, e a distinção está documentada. Sonda
      prova o que alguém lembrou de perguntar; regressão prova que não voltou.
- [ ] **AC7** — Teste que exige privilégio faz **`skip` com mensagem nomeando a garantia não
      exercitada** e o motivo (Developer Mode). Nunca falha silenciosa, nunca sumiço do relatório.
      Cobre os 12 testes de symlink nos 3 runtimes.
- [ ] **AC8** — Falsificação do `skip`: com privilégio disponível, os testes **executam** e passam;
      sem privilégio, **pulam com mensagem**. Um `skip` incondicional é o mesmo defeito com outra
      roupa.
- [ ] **AC9** — A sonda não escreve no repositório, não usa segredo, e é `workflow_dispatch` puro.
- [ ] **AC10** — O tempo do job largo é medido e registrado. Se inviabilizar o pipeline, a decisão de
      recorte é explícita, não silenciosa.
- [ ] **AC11** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` continua exit 0 em Linux, e os
      demais jobs do CI seguem verdes. Esta REQ **não** corrige defeito de produto.
- [ ] **AC12** — **Isolação de `$HOME` da própria suíte.** `pypi/tests/conftest.py` e
      `internal/validator/main_test.go` isolam via a variável `HOME`, que a produção **não lê no
      Windows**. `go test ./...` pode escrever na home real do runner — e o Go paraleliza pacotes,
      então é condição de corrida dentro de uma única execução. Ou a isolação passa a valer em
      Windows, ou o job largo é **inviável** até a correção de `$HOME` entrar. Decidir com medição,
      não com suposição.

## Negative Scope

- **Não** corrigir nenhum dos onze defeitos. Esta REQ entrega o **instrumento**; as correções vêm
  depois, cada uma com sua REQ, usando o job como evidência.
- **Não** formalizar o pré-requisito de shell POSIX na documentação e no `init` — REQ própria, já
  acordada.
- **Não** portar `.ps1`/`.cmd` dos cinco scripts `.sh`. Windows nativo sem shell POSIX é decisão de
  produto separada.
- **Não** alterar o job `windows-integrations-resolve` existente, que é guard honesto de um bug
  específico — o novo job é adicional.

## Observação de método

**A ordem aqui é deliberadamente contraintuitiva.** O instinto é corrigir e depois testar; esta REQ
faz o oposto, porque sem o job não temos como provar que as correções funcionam **nem que continuam
funcionando** — e escreveríamos correção validada no ambiente errado, que é o viés registrado em
`vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md` três vezes num único dia.

Vale registrar o custo do modelo atual: **um usuário fez o trabalho que o nosso pipeline deveria ter
feito**, e o descobriu adotando a ferramenta em produção.

## REQ fechada em 2026-08-31

**Entregue:** instrumento de medição em duas camadas (`windows-full-suites`, `windows-defect-reproduction`)
+ sonda sob demanda (`windows-probe.yml`), nos PRs **#221** e **#226**.

**Linha de base congelada e citável: `8 REPRODUCED / 0 inconclusivos / 11 itens`.**

O critério da barreira — *"o job de Windows reprovando pelos motivos esperados"* — **expira com esta
REQ, de propósito**. Ele era correto enquanto o entregável era o instrumento; a partir da primeira
REQ de **correção** ele se inverteria contra nós, porque cada item corrigido sai de `REPRODUCED` e a
contagem **deve** cair. `hefesto-tf` identificou a contradição ao analisar os PRs #222–#225: manter
esta REQ aberta faria de toda correção uma violação da própria governança.

**Regra que passa a valer:** cada item que sair de `REPRODUCED` é explicado no roadmap da REQ que o
corrigiu, citando o run que mediu. Contador decrescente com histórico, não alvo fixo.

**O instrumento provou seu valor duas vezes antes mesmo de corrigirmos qualquer defeito:** a sonda
respondeu a pergunta da junction, que nenhuma suíte de regressão respondia — e a resposta revelou que
o Node **não** tem o defeito, invertendo a expectativa e evitando que corrigíssemos o que não estava
quebrado. Ver `REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-...`.

## Linked ADR
ADR: docs/adr/ADR-2026-08-30-ci-de-windows-como-instrumento-de-medicao-job-largo-que-nasce-vermelho-mais-sonda-sob-demanda.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-sob-demanda.md
