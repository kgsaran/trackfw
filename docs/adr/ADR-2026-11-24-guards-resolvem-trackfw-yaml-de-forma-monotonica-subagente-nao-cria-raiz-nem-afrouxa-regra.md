---
status: Accepted
date: 2026-11-24
author: "zeus-tf (threat model da hades-tf, ML-0A da REQ-2026-11-23 write-scope)"
---

# ADR: Guards resolvem trackfw.yaml de forma monotônica: subagente não cria raiz nem afrouxa regra

> Date: 2026-11-24 | Status: Accepted

## Context

O guard write-scope (`internal/guard/writescope.go`) resolve a raiz pelo primeiro ancestral do alvo que tem
`trackfw.yaml` (`findProjectRoot`). O contrato vem do ADR-2026-10-23 (adendo "D3, contrato do ML-3A"): a raiz é a do
alvo, para que um agente num worktree seja julgado pelas regras do checkout principal.

O threat model da Wave 0 (`docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md`) mediu o efeito
que esse contrato não declarava:
- um subagente escreve `.claude/trackfw.yaml` (rc=0);
- a raiz passa a ser `.claude/`, e a lista de congelados é resolvida contra ela;
- a escrita seguinte em `.claude/settings.json`, onde ficam os hooks, passa (rc=0; antes da planta, rc=2).

O arquiteto reproduziu no binário: baseline 2, planta 0, braço 0, contra-braço sem planta 2.

Variantes do mesmo mecanismo:
- **Diretório:** um diretório chamado `trackfw.yaml` também re-enraíza, porque `os.Lstat` não confere o tipo.
- **Caixa:** `TRACKFW.YAML` também re-enraíza num sistema de arquivos que não diferencia maiúsculas.
- **FIFO:** um FIFO com esse nome trava a leitura.

O guard de credencial tem a mesma causa, porque o `trackfw.yaml` lido decide a regra:
- `RunCredentialGlobal` lê o modo de `<cwd>/trackfw.yaml`, e um `mode: warn` plantado rebaixa `block` para `warn`
  (pre=2, braço=0).
- `RunCredential` também lê só `<cwd>/trackfw.yaml`, sem subir. Pela leitura do código, com o cwd num subdiretório
  sem yaml o guard se desliga, e um `mode: warn` plantado no subdiretório se sobrepõe ao `block` da raiz.

Os comandos (`config`, `validate`, `serve`, `barrier`, `ship`, `push`, `commit`) leem o yaml do cwd, sem subir, e não
decidem permissão de subagente. Ficam fora.

## Decision

**D1: o subagente não escreve `trackfw.yaml` em lugar nenhum.**
- **Regra:** quando o payload tem `agent_id`, o write-scope nega qualquer escrita cujo caminho tenha um componente igual
  a `trackfw.yaml`. A comparação usa `strings.EqualFold` sempre, independente do sistema operacional.
- **Ordem:** a checagem roda depois de normalizar o caminho e antes de resolver a raiz. Assim vale também fora de
  qualquer projeto.
- **Windows:** a normalização remove ponto e espaço finais de cada componente, porque o Windows cria `trackfw.yaml` a
  partir de `trackfw.yaml.`.
- **Fluxo NTFS (emenda da Wave 2, R-8b):** antes de remover ponto e espaço, a normalização corta o componente no
  primeiro `:`. No Windows, `trackfw.yaml::$DATA` é o próprio arquivo. No POSIX, `:` é legal em nome, então negar
  `trackfw.yaml:x` ao subagente é falso positivo aceito.
- **Nome curto 8dot3 (emenda da Wave 2):** nega todo componente na forma de nome curto com extensão `.YAM`
  (`<1 a 6 caracteres>~<dígitos>.YAM`, EqualFold). Isso cobre `TRACKF~1.YAM` e também a forma com hash que o Windows
  gera depois de várias colisões no mesmo diretório (ex.: `TR1A2B~1.YAM`). Um nome curto só aponta para arquivo que já
  existe; sem a regra, ele reescreveria um yaml aninhado existente. Falso positivo aceito: o subagente não grava nenhum
  arquivo com nome nessa forma.
- **Rota legítima:** `trackfw init` num subdiretório é coisa da sessão principal. Teste Go que precisa de um
  `trackfw.yaml` o cria em `t.TempDir()` pelo próprio código do teste.

**D2: a lista de congelados é a união das listas de todos os ancestrais (monotônica).**
- **Cálculo:** a raiz continua sendo a do alvo, e o contrato do worktree no ADR-2026-10-23 não muda. Para julgar, o
  guard sobe do alvo até a raiz do sistema de arquivos e junta a lista completa de cada `trackfw.yaml` encontrado: os
  congelados fixos de subagente mais o `write_scope` do yaml. Cada entrada é resolvida contra a própria raiz.
- **Efeito:** plantar um yaml só pode acrescentar regra, nunca tirar.
- **Limite:** um yaml acima do projeto também entra na união. Isso só restringe, e um yaml inválido nega, como já
  acontece com o yaml da raiz (decisão 6 do ML-4C).

**D3: só "não existe" conta como ausente.**
- **Leitura:** todo `trackfw.yaml` lido pelos guards passa por `fileutil.ReadRegularFile`. Ele segue symlink para
  arquivo regular e não trava em FIFO.
- **Ausente:** só `os.IsNotExist` é ausente.
- **Nega:** `ErrNotRegularFile` (diretório, FIFO) e qualquer outro erro negam.
- **Detecção de raiz (emenda da Wave 2, MÉDIO-1 da revisão):** a regra vale também para achar o projeto. Um
  `trackfw.yaml` que existe mas não é arquivo regular, ou não é legível, marca a raiz e nega; nunca é pulado. Isso
  inclui o wrapper de shell do guard de credencial, que testava `-f` e saía com 0 diante de um diretório: ele passa a
  parar em qualquer entrada existente com esse nome e deixa o binário decidir.

**D4: o modo do guard de credencial é o mais estrito da cadeia.**
- **Cálculo:** o guard sobe do cwd até a raiz do sistema de arquivos e lê o `credential_guard.mode` de cada
  `trackfw.yaml` no caminho. O modo efetivo é o mais estrito entre o padrão do escopo e todos os lidos.
- **Padrões:** `warn` no escopo de projeto e `block` no global.
- **Escopo de projeto:** o projeto é detectado pelo ancestral mais próximo com yaml, então o cwd num subdiretório não
  desliga mais o guard.
- **Implementação:** o piso é o padrão de cada chamador, nunca um `"block"` fixo dentro da função compartilhada
  (`credWalkStrictestMode`). Com o padrão `block`, o escopo global não lê mais o yaml.

**D5 (emenda da Wave 2, F-LEASE-MONO do red team): leases seguem a mesma monotonia do D2.**
- **Cálculo:** além da raiz do alvo e da raiz do cwd (ADR-2026-10-23, revisões da Wave 4), o guard consulta o lease em
  todo ancestral do alvo que tem `trackfw.yaml`. A regra existente não muda: todo lease encontrado precisa autorizar o
  alvo, e lease inválido em qualquer raiz nega.
- **Por quê:** sem isso, um projeto aninhado legítimo sem lease deixa o subagente escrever em qualquer caminho dele,
  mesmo com um lease restrito na raiz de fora, quando cwd e alvo estão no aninhado.

**D6 (emenda da Wave 2): a lista de congelados compara também a grafia normalizada.**
- **Regra:** no Step 10, o caminho alvo casa com um congelado se o caminho cru casar ou se o caminho com cada
  componente normalizado pela regra do D1 (corte no `:`, sem ponto e espaço finais) casar. Vale em todos os sistemas.
- **Por quê:** é a mesma causa do R-8b. No Windows, `.claude/settings.json.` e `.claude/settings.json::$DATA` gravam
  `.claude/settings.json`; a comparação só pelo caminho cru deixava passar quando o arquivo ainda não existe (o
  `os.SameFile` só pega arquivo existente).
- **Falso positivo aceito:** no POSIX, o subagente não grava `.claude/settings.json.` nem `settings.json:x`, nem
  escreve abaixo de um diretório cujo nome normalizado coincide com o de um congelado (ex.: `.claude:x/settings.json`).
- **Medição:** no macOS, os vetores de fluxo NTFS e ponto final provam o casamento de strings (rc 0→2), não o sistema
  de arquivos; o efeito real é só no Windows e não foi medido em Windows nesta REQ.

**Mudança de comportamento:** um `credential_guard: mode: warn` no projeto deixa de rebaixar o hook global, que continua
`block`. O `init` não gera esse campo, então só quem o escreveu à mão perde o efeito. Confirmado pelo KG em 2026-11-24.

## Consequences

- **Positivo:**
  - pelas ferramentas de escrita, o subagente não muda mais a raiz nem afrouxa uma regra;
  - no guard de credencial, nem a escrita pelo shell afrouxa, porque esse guard é o próprio hook do shell;
  - o contrato do worktree fica como estava.
- **Negativo:**
  - subagente não cria projeto aninhado;
  - um yaml quebrado num diretório acima do projeto nega toda escrita de subagente abaixo dele;
  - o `mode: warn` do projeto não vale mais para o hook global.
- **Residual mantido:**
  - **Shell no write-scope (ADR-2026-10-23):** escrita pelo shell continua escapando do write-scope. Os 12 agentes do
    catálogo têm Bash. Esta REQ fecha a porta das ferramentas de escrita, não a do shell.
  - **Fora de projeto:** escrita em `~/.claude/settings.json` não tem raiz, e o write-scope é de projeto
    (ML-FA, 2026-11-17).
  - **OpenCode:** o write-scope não é instalado no OpenCode (T13, ROADMAP-2026-11-17), então o nome de ferramenta em
    minúsculas não chega a ele.
  - **`MultiEdit`:** fica fora do matcher. Um teste público de 2026 indica que as versões atuais do Claude Code não a emitem mais; não há confirmação oficial.

  Os dois últimos são outra causa (cobertura de ferramenta, não resolução de raiz) e seguem na
  REQ-2026-11-24 (`docs/req/REQ-2026-11-24-guard-de-write-scope-nao-cobre-multiedit-no-claude-code-nem-o-opencode.md`),
  por decisão do KG.

## Alternatives Considered

- **Raiz no `trackfw.yaml` mais externo:** quebra monorepo com projeto aninhado legítimo. Rejeitada.
- **Raiz no cwd da sessão:** reverte o contrato do worktree do ADR-2026-10-23. Rejeitada.
- **Contar só a raiz commitada ou anterior à sessão:** exige `git` dentro do guard, mais um processo externo no caminho
  de cada escrita. Rejeitada.
- **Só congelar `.claude/trackfw.yaml`:** deixa diretório, caixa e outros subdiretórios. Rejeitada.
- **Só `IsRegular()` em `findProjectRoot`:** não pega a caixa e quebra yaml por symlink. Rejeitada.
