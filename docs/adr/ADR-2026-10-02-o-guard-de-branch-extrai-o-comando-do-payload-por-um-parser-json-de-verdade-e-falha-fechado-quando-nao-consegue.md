---
status: Accepted
date: 2026-10-02
author: "trackfw_architect"
---

# ADR: o guard de branch extrai o comando do payload por um parser JSON de verdade, e falha fechado quando não consegue

> Date: 2026-10-02 | Status: Accepted

REQ: `docs/req/REQ-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md`
Origem: **#507** (medido por consumidor externo em Windows 11 sem `jq`)
Relacionada: a nota do vault `guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24`
(contrato fail-closed do dreno de stdin) e a ADR-2026-08-17 (no-op fora de projeto trackfw).

## Context

O `trackfw-git-branch-guard.sh` extrai o comando do payload JSON do hook com `jq`. Sem `jq`, usa um
fallback por `sed` com a captura `"\([^"]*\)"`. Esse fallback **não interpreta JSON**, e falha aberto de
duas formas:

1. **Não desescapa `\n`.** Todo comando multilinha chega como uma linha só. A segmentação não quebra
   em `\n` literal, de propósito (para não fatiar `-m "a\nb"`), então só o primeiro token decide.
   `echo oi` + `git push origin main` em duas linhas → **rc=0**. Medido na #507, com controles: o
   mesmo comando em uma linha (`;`) ou por `TRACKFW_GIT_COMMAND` com newline real → rc=2.
2. **Para no primeiro `\"`.** `[^"]*` termina na aspa escapada. `echo \"a\"; git push` é capturado
   como `echo \`, e o `git push` some. (Inferido da regex; a Wave 0 mede.)

No CI o braço do fallback é **código morto**: os runners têm `jq`, e nenhum teste do guard roda sem
ele. Na máquina sem `jq` (Git Bash do Windows, em geral) é o único caminho que roda.

O script vive em **4 cópias byte-idênticas**: o gerador (`internal/generators/scaffold.go`), a
referência do validator (`internal/validator/validator_git_branch_guard_reference.go`), o arquivo do
repositório (`scripts/trackfw-git-branch-guard.sh`) e o `corrupt_literal` do cenário de falsificação
que sabota o guard.

## Decision

### D1 — Fallback por um extrator JSON em `awk`, não por `sed` nem por `python3`.

Sem `jq`, o comando é extraído por uma função `awk` que percorre o payload caractere a caractere,
respeitando strings e escapes, localiza a chave (`tool_input.command`, depois `command`,
`tool_info.command_line`, `hook_input.command`, na mesma prioridade do `jq`) e **decodifica** o valor:
`\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t` e `\uXXXX`.

- `awk` existe no macOS, no Linux e no Git Bash; não é dependência nova.
- **`python3` rejeitado:** no Windows ele costuma ser o stub da Microsoft Store (rc=49, medido no
  ML-2A do driver de falsificação), e o guard não pode depender de um binário que finge existir.
- `\uXXXX` fora do ASCII vira um caractere marcador fixo. O guard decide por estrutura de shell
  (`git`, `;`, `&&`, newline), que é toda ASCII; o texto de uma mensagem de commit não muda a decisão.

### D2 — Chave presente mas indecodificável → **falha fechado**.

Se a chave do comando existe e o valor não pode ser decodificado (string não terminada, escape
inválido), o guard nega com `exit 2` e mensagem que nomeia a causa. É o mesmo contrato do dreno de
stdin (payload ilegível → nega). **Chave ausente** mantém o comportamento de hoje: nada a guardar.

### D2-bis — NUL no comando decodificado → **falha fechado, nos dois caminhos**. (Wave 0)

`$()` descarta NUL: `git push\u0000origin main` vira `git pushorigin main` e passa **também com `jq`**
(pré-existente, medido na Wave 0). A causa é a mesma (a extração entrega ao guard um comando diferente
do que o shell executaria), então entra aqui. Nenhum comando legítimo contém NUL: o guard nega quando o
valor decodificado contém `\u0000`, tanto no extrator `awk` quanto no caminho `jq` (por exemplo, um
`jq -e '… | contains("\u0000")'` antes da extração).

### D2-ter — Restrições de implementação medidas na Wave 0

- **Chave duplicada: a última vence**, como no `jq`, para que os dois caminhos deem o mesmo veredito (D3).
- **Prioridade exatamente igual à do `jq`**: `tool_input.command`, depois `command` na raiz,
  `tool_info.command_line`, `hook_input.command`. A ordem do `sed` antigo era outra, e
  `{"command":"git push","tool_info":{…}}` falhava aberto.
- **Nada de `RS=""`** (modo parágrafo: uma linha em branco no JSON parte o payload). Acumular o
  payload inteiro e processar no `END`.
- **Nada de `strtonum()`** (não existe no awk do macOS); conversão hexadecimal por função própria.
- **A chave só conta como chave**: `"command"` dentro de outra string não é chave (caso C08).

### D3 — `jq` continua sendo o primeiro caminho; o fallback passa a ser testado.

Toda a tabela de testes do guard roda **duas vezes**: com `jq` e com um `PATH` curado sem `jq`, no
padrão do `TestAttentionScripts_FallbackWithoutJQ`. Um caso que passa num caminho e reprova no outro
é divergência de extrator, e reprova o teste.

### D4 — As 4 cópias mudam juntas.

O gerador é a fonte; a referência do validator, o script do repositório e o `corrupt_literal` do
cenário de falsificação acompanham no mesmo PR. A regra `git_branch_guard_script_integrity` e o
teste `TestGitBranchGuardScriptReference_MatchesGenerator` provam a igualdade.

## Consequences

- O guard deixa de falhar aberto sem `jq` nas duas formas medidas.
- Consumidores com o guard instalado só recebem a correção quando regenerarem os artefatos
  (`trackfw agents update` / `trackfw update`). A regra de integridade acusa o script antigo.
- **Resíduo declarado:** `\uXXXX` não-ASCII é decodificado para um marcador, não para o caractere real.
- **Custo:** o `awk` é mais lento que o `sed` para payload grande. A Wave 0 mede com o payload de
  200 KB da nota do vault.

## Alternatives Considered

- **Corrigir o `sed` (trocar `\n` por newline).** Rejeitada: como a própria #507 aponta, isso quebra
  `-m "linha 1\nlinha 2"` (onde o `\n` do JSON é uma contrabarra literal escapada) e troca um
  falso negativo por um falso positivo.
- **`python3 -c 'import json'`** (como o `trackfw-attention-signal.sh` faz). Rejeitada pelo stub do
  Windows; para um controle de segurança, depender de interpretador opcional é falhar aberto em outra
  máquina.
- **Exigir `jq` (negar tudo sem ele).** Rejeitada: bloquearia todo `git` em toda máquina sem `jq`,
  inclusive os comandos permitidos.

## Linked REQ

`docs/req/REQ-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md`
