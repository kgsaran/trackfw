# Título de `roadmap new` com newline forja seção `## Wave N` inteira; `barrier` executa o gate forjado

**Data:** 2026-08-23
**Domínio:** `internal/generators/roadmap.go` (interpolação de título) + `internal/commands/barrier.go`
(`parseWaves`/`parseGates`) — mesma classe nos 3 CLIs
**Descoberto em:** ML-3A (barreira da Wave 0 do modelo de ameaça no harness), ao completar o vetor
"newline no título" que a tarefa de revisão pediu e que não havia sido executado na primeira
passada.

## Status: NÃO CORRIGIDO — achado reportado, requer REQ própria

Diferente da nota irmã
`rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21` (que trata newline no valor de
`agent_models` interpolado no frontmatter de asset, JÁ corrigido com `containsControlChar`), este
achado é em outro par de arquivos (`roadmap.go` + `barrier.go`) e **continua aberto**.

## Causa raiz

`NewRoadmapFromContent` (`internal/generators/roadmap.go:150`) interpola `content.Title` com
`fmt.Sprintf("# Roadmap: %s", ...)` sem remover/escapar newlines. Um título contendo
`"\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n```bash\n<comando>\n```\n"` planta uma **seção
Markdown inteira**, incluindo bloco de gate, ANTES da seção real que o template emite mais abaixo no
mesmo arquivo.

`parseWaves`/`parseGates` (`internal/commands/barrier.go`) resolvem a **primeira** ocorrência
textual do heading `## Wave N` no arquivo — não distinguem seção gerada de seção forjada por
conteúdo de usuário. `barrier --wave N` executa o comando da seção forjada via `sh -c`
(`runGateCommand`), independentemente do status final agregado do barrier (aqui `blocked` por outros
checks falharem, mas o `sh -c` já rodou antes da composição do resultado).

## Medição

```
$ trackfw roadmap new $'forjado\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n```bash\ntouch /tmp/HEADING_PWNED\n```\n'
$ trackfw barrier <arquivo> --wave 0 --json
{"checks":[
  {"name":"gates","status":"passed","commands":["touch /tmp/HEADING_PWNED"],
   "evidence":["touch /tmp/HEADING_PWNED: exit 0"]}
]}
$ ls /tmp/HEADING_PWNED   → existe
```

Reproduzido em Go e Node (arquitetura idêntica; não testado em Python, mas mesmo padrão de parsing
string-level). Reproduzido também contra `--wave 1` com o mesmo padrão — **pré-existente à REQ da
Wave 0** (mecanismo de gates existe desde ADR-2026-07-26-principios-de-design-de-gates-verificaveis).
A REQ da Wave 0 não introduziu o buraco — apenas estendeu a mesma superfície (já alcançável via
`--wave 1+`) para `--wave 0` também, ao aceitar essa wave nova.

## Por que o AC13 daquela REQ não cobre isto

AC13 exigia que o **gate literal do próprio template** (o `exit 1` fail-closed da Wave 0 gerada)
nunca fosse interpolado com título/REQ — e isso se sustenta, testado com `$(...)`, backtick, aspas,
`;` em título e conteúdo de REQ via `--from-req`: nenhum desses vetores vaza para dentro do gate
real. O que nenhum desses vetores testa é se o título pode **plantar uma seção paralela com gate
próprio** — classe de ataque diferente (injeção estrutural de Markdown, não injeção de string dentro
de um campo já delimitado).

## Recomendação (não implementada — fora da fronteira de quem escreveu esta nota)

1. Sanitizar o título antes de interpolar em `# Roadmap: %s` — rejeitar ou colapsar newlines
   (mesmo padrão de `containsControlChar` já usado na correção irmã).
2. `parseWaves`/`mlBlock` deveriam pinar a seção por outro critério que não "primeira ocorrência
   textual do heading" — vulnerável a qualquer campo que aceite string livre e seja interpolado
   antes da geração das seções canônicas.
3. Mitigação mais fraca, mas imediata: `barrier` recusar (ou avisar antes de) executar comandos de
   gate vindos de roadmap com múltiplas ocorrências do mesmo heading de wave.

## Ligação

Ver `docs/seguranca/2026-08-23-barreira-da-wave-0-no-harness.md` §2-bis para o relato completo desta
sessão, incluindo o teste cruzado contra `--wave 1` que provou o caráter pré-existente do achado.
