---
status: Accepted
date: 2026-11-22
author: "zeus-tf (decisões do KG em 2026-11-22)"
---

# ADR: Onboarding verificável: init honra as flags sem TTY, site em inglês na raiz e pinado por teste

> Date: 2026-11-22 | Status: Accepted

## Context

A auditoria de onboarding de 2026-11-22 (`docs/qualidade/2026-11-22-auditoria-onboarding.md`) percorreu o README e o
site com o binário 11.8.1, como um usuário novo, e registrou 25 achados: 3 bloqueantes, 14 atritos e 8 cosméticos. O
arquiteto reproduziu os três bloqueantes no binário.

- **Defeito de produto.** Sem TTY, `trackfw init --brownfield` termina com rc=0 e não grava `governance_mode` nem
  `lenient_until` no `trackfw.yaml`. O caminho não-TTY (`internal/commands/init.go:266`) retorna na linha 309, antes da
  única leitura da flag (`:531`). O mesmo desenho sugere o espelho: `--ai-tools` só é lido no caminho não-TTY (`:304`), e
  no caminho TTY a lista começa vazia e só o wizard a preenche.
- **Site sem verificação.** O guia do site manda rodar `adr new` e `req new` sem título, e os dois saem com rc=1. Manda
  também `roadmap move REQ-...`, que não existe. O site não tem teste: o último commit em `site/` é de 2026-10-30, e o
  conteúdo cita a versão 11.1.0.
- **Teste e README divergem.** `quickstart_e2e_test.go` afirma 2 avisos no `validate` do Quick start, e o README promete
  `✓ No violations found.`. O teste guarda uma cópia da sequência; ele não lê o README.
- **Idioma.** O produto é inglês primeiro desde 2026-11-02, mas o site serve pt-BR na raiz e inglês em `/en/`.
- **Carga de leitura.** O README tem 1285 linhas. O Quick start e a continuação dele estão separados por 415 linhas,
  318 delas do bloco de Windows dentro de *Installation*.

## Decision

**D1 — `init` honra cada flag nos dois caminhos.** Toda flag de `trackfw init` tem o mesmo efeito com e sem TTY. No
caminho TTY, uma flag passada explicitamente vence o wizard ou pula a pergunta correspondente. Isso vale para
`--brownfield` e `--ai-tools`, e para qualquer outra flag que a varredura da Wave 0 encontrar lida em só um dos
caminhos (mesma causa, mesma REQ).

**D2 — `--brownfield` só escreve governança em projeto sem `trackfw.yaml`.** Sem `trackfw.yaml`, `init --brownfield`
grava `governance_mode: lenient` e `lenient_until` (hoje + 30 dias), com e sem TTY. Com `trackfw.yaml` existente, o
comando segue normalmente: mescla as chaves ausentes que não são de governança e instala os agentes. Mas não escreve
`governance_mode` nem `lenient_until`, mesmo que faltem. Imprime um aviso dizendo que as chaves de governança não
foram alteradas e que, para ativar o modo lenient, é preciso editar o `trackfw.yaml`. Sai com rc=0. Medição da Wave
0 (`docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md` §2): todo projeto criado pelo `init` padrão não tem
`governance_mode`, e nenhum guard impede um agente de rodar `trackfw init` pelo shell. Sem este limite, a correção do
D1 daria a qualquer agente um jeito de rebaixar um projeto para lenient.

**D2a — O merge do `trackfw.yaml` reconhece as chaves como o leitor as reconhece.** O `init` decide quais chaves já
existem com o mesmo parse YAML que o `config` usa para ler o arquivo, e não por texto. Hoje, uma chave com aspas
(`"adr_dirs":`), com espaço antes do `:` ou com BOM na primeira linha é tratada como ausente. O `init` acrescenta então
o valor padrão, que vence o do usuário, porque o leitor fica com a última ocorrência. Medido no 11.8.1: `"adr_dirs":`
com `custom-adr` termina com um segundo `adr_dirs: docs/adr`. Isso vale para todo `init`, com ou sem `--brownfield`,
e para os dois usos do detector em `internal/generators/scaffold.go` (`trackfw.yaml` e lefthook).

**D3 — Inglês na raiz do site.** O site serve inglês em `/trackfw/` e pt-BR em `/trackfw/pt/`. O GitHub Pages não
redireciona, então os links antigos para páginas internas em português quebram. O trackfw ainda não tem consumidores
conhecidos que dependam deles (decisão do KG em 2026-11-22).

**D4 — A documentação é pinada pelo que o teste executa.** O teste do Quick start mantém a lista dos comandos que
executa e afirma que cada um deles aparece literalmente no README e no guia de início do site em inglês. Afirma também
que a saída prometida no documento é a que o `validate` produz. O teste não interpreta markdown: só confere se a linha
existe no texto. Assim o documento não muda sem que o teste mude junto.

**D5 — O README fica com o caminho do usuário novo contíguo.** O Quick start e a continuação dele ficam juntos, e o
bloco de Windows sai de dentro de *Installation* para uma seção própria logo depois dela, fora do caminho de leitura
de quem instala no macOS ou no Linux. Nenhum conteúdo é removido. Toda âncora
interna afetada é atualizada.

**D6 — O site é verificado na REQ, sem job novo de CI.** Cada ML que mexe no site roda `npm ci && npm run build` em
`site/`, porque o build do VitePress reprova link morto, e faz uma captura em Chrome headless de `/trackfw/` e
`/trackfw/pt/`. Não entra job novo no CI: gates novos seguem congelados (direção de 2026-10-28).

## Consequences

- **Positivo:** cada comando que o usuário novo copia do README ou do site funciona, e um teste reprova quando o
  documento e o binário se afastam. Agentes de IA, que rodam sem TTY, recebem o mesmo `init` que um humano.
- **Negativo:** links profundos antigos para o site em português quebram (D3). O teste do Quick start passa a reprovar
  em edição de README e do guia do site, o que é intencional.
- **Risco aceito:** num projeto ainda sem `trackfw.yaml`, um agente pode rodar `init --brownfield` e criar o projeto em
  modo lenient. Não há governança anterior a rebaixar. O `lenient_until` limita o modo a 30 dias, e o `trackfw.yaml`
  criado é versionado e aparece no diff.

## Alternatives Considered

- **Tirar o snapshot literal de saída do site, sem teste:** reduz manutenção, mas não impede o próximo comando quebrado.
  Rejeitada como solução única; vale para blocos de saída que mudam a cada versão (status).
- **Parser que extrai blocos `bash` do markdown e os executa:** cobre mais, mas o parser vira um produto à parte, e o
  parser de roadmap já mostrou o custo disso. Rejeitada a favor da conferência literal (D4).
- **Job de CI que constrói o site em todo PR:** pegaria link morto antes do merge, mas é gate novo. Rejeitada enquanto
  os gates novos estiverem congelados.
- **Manter pt-BR na raiz ou remover o pt-BR:** o KG escolheu inverter (D3).
- **`init --brownfield` mesclar a governança em `trackfw.yaml` existente (comportamento atual no TTY):** rebaixa
  projeto que nunca declarou `governance_mode`, e o D1 levaria isso também a agentes sem TTY. Rejeitada (D2).
- **Guard que bloqueie `trackfw init` pelo shell:** gate novo, com os gates congelados. Rejeitada; o D2 resolve na
  origem.
