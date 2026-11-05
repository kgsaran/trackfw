---
status: Accepted
date: 2026-11-05
author: "Zeus (trackfw_architect), decisão do KG em 2026-11-05"
---

# ADR: Guards cumprem o que o README promete: mode lido com CRLF, padrões de credencial ampliados e invólucro de shell analisado

> Date: 2026-11-05 | Status: Accepted

## Context

O primeiro usuário externo (bruno-castiglioni, 11.3.0, Windows 11) abriu os issues #662 e #663, e o Lourival
reproduziu os dois de forma independente no fork. O arquiteto mediu de novo com a 11.4.0 no macOS em
2026-11-05, alimentando payloads ao guard por stdin.

**#662: guard de credencial.**

- Com `credential_guard.mode: block`, o `trackfw.yaml` em LF dá `rc=2` e em CRLF dá `rc=0` (só aviso).
  O `credReadModeWithDefault` (`internal/guard/credential.go`) não remove o `\r` e cai no padrão `warn`.
  O comentário dele chama isso de "resíduo do `.sh`". Já o `extractCredentialGuardMode` do validator usa
  `TrimSpace`. Então o `validate` conclui `block` e o guard conclui `warn` sobre o mesmo arquivo.
- O guard só detecta dois padrões de conteúdo: JWT (`eyJ…`) e chave AWS (`AKIA…`). Token do GitHub
  (`ghp_…`), chave do Google (`AIza…`) e `sk-proj-…` dão `rc=0` sem aviso.
- O README diz que os hooks "block agents from materializing real credentials", o que promete mais do que
  o guard entrega.
- O script gerado `scripts/trackfw-credential-guard.sh` (`credentialGuardDetectionCore` e
  `credentialGuardModeResolution`, em `internal/generators/scaffold.go`) traz os mesmos dois padrões e o
  mesmo leitor de `mode`.

**#663: guard de git.**

- `MatchSubcommand` (`internal/guard/gitbranch.go`) separa o comando em trechos e exige que o primeiro
  token, depois de `env`, `command` e `sudo`, seja `git`.
- Por isso, um invólucro de shell atravessa o guard. Com `rc=0`: `bash -c "git push"`, `sh -c`,
  `bash -lc "git commit"`, `eval "git push"`, `powershell -c "git push"`. Com `rc=2`: `git push` e
  `cd x && git push`.

## Decision

- **D1. O `mode` é lido igual nos dois lados.** O guard remove o `\r` e espaços finais do valor do `mode`,
  como o validator. O script gerado converge na mesma correção. Um teste amarra o guard e o validator na
  mesma tabela de entradas: LF, CRLF, comentário, aspas e indentação.
- **D2. Padrões de conteúdo com baixo falso positivo**, por decisão do KG. Além de JWT e AWS, o guard
  passa a detectar:
  - GitHub: `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_` e `github_pat_`, com comprimento mínimo;
  - OpenAI e Anthropic: `sk-` seguido de corpo longo, incluindo `sk-proj-` e `sk-ant-`;
  - Google: `AIza` seguido de 35 caracteres;
  - Slack: `xox[abprs]-`;
  - cabeçalho PEM de chave privada: `-----BEGIN … PRIVATE KEY-----`.

  O conjunto exato e os limites de cada expressão saem da Wave 0. A mensagem nomeia o tipo detectado,
  como já faz com JWT e AWS.
- **D3. O README diz exatamente o que é detectado** e o que não é: caminho de arquivo, como `.env`,
  `.aws/credentials` ou `id_rsa`, e atribuições genéricas como `password = "…"`. A frase "block agents
  from materializing real credentials" passa a ter ressalva.
- **D4. Invólucro de shell é analisado de novo.** Quando o primeiro token, pelo basename sem `.exe` e sem
  diferenciar maiúsculas, é um interpretador conhecido com a flag de comando, o guard aplica o
  `MatchSubcommand` de novo ao argumento do comando:
  - `bash`, `sh`, `zsh`, `dash` e `ksh` com `-c` (incluindo flags combinadas, como `-lc` e `-ec`);
  - `pwsh` e `powershell` com `-c`, `-Command` ou `-command`;
  - `cmd` com `/c` ou `/k`;
  - `eval`, cujo argumento é o resto do trecho.

  A profundidade é limitada (valor da Wave 0). Ao passar do limite ou num argumento que não dá para ler,
  o guard nega (fail closed), como já faz com payload truncado.
- **D5. Limite declarado.** Um interpretador de linguagem que chama git por API, como `python -c` com
  `subprocess` ou `node -e`, continua fora. O README e o `docs/cli-parity.md` dizem isso. Análise de texto
  não fecha execução arbitrária.

## Consequences

- O `mode: block` passa a valer no Windows com qualquer editor.
- Mais padrões significam mais chance de falso positivo. O preço é aceito porque os padrões escolhidos têm
  prefixo fixo e comprimento mínimo, e o `mode: warn` continua sendo o padrão do escopo de projeto.
- O guard de git fica um pouco mais caro por trecho, só quando o primeiro token é um interpretador.
- Quem dependia de `bash -c "git push"` para contornar o guard passa a ser bloqueado. É a intenção.

## Alternatives Considered

- **Só corrigir o README (cobertura).** Foi rejeitada pelo KG: deixaria o guard abaixo do que um usuário
  espera de um guard de credencial.
- **Detectar por caminho (`.env`, `id_rsa`).** Fica fora por ser um mecanismo novo, com falso positivo em
  leitura legítima (`.env.example`, configuração local). Pode virar uma REQ própria se houver demanda.
- **Casar o literal `bash -c`.** Rejeitada: o Lourival mediu `sh -c` e `bash -lc`, e qualquer combinação
  de flags escaparia.
- **Bloquear todo comando que contenha a palavra `git`.** Rejeitada: o falso positivo seria enorme
  (`git status` dentro de `bash -c`, mensagens de commit, caminhos).
