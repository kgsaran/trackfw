---
status: done
date: 2026-08-12
author: "Hades (Segurança)"
---

# Parecer de segurança — estado do credential-guard após a Barreira B1 (ML-4A)

> ML-4A do roadmap
> `docs/roadmaps/wip/ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard-wave-1-controle-positivo-e-failclosed.md`.
> Insumos: `docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md`,
> `docs/req/REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-consentimento-explicito-verificacao-da-premissa-de-sandbox-e-a-via-do-credential-guard-mode.md`,
> `internal/validator/validator_credential_guard.go`, `scripts/check-gates-falsify.sh` (Cenário 47),
> `docs/agents-working-context.md` (sessões ML-3A/3B/3C).
> Este parecer é puramente de leitura: nenhum arquivo de código foi tocado. Achados vão para Zeus.

## Resumo do veredito

O estado pós-barreira é **defensável para PR**, com uma condição: a lacuna 2 abaixo (falso senso de
segurança) precisa de uma frase de aviso em local que o usuário efetivamente leia — hoje não existe
nenhum texto voltado ao usuário final que documente o que a regra do ML-1A **não** cobre. Sem essa
frase, o PR está tecnicamente correto mas comunica menos do que sabe.

A premissa central do ADR (agente não alcança `~/.trackfw/`) **não foi medida** por este roadmap —
confirmado por leitura do próprio ADR e da REQ nova, que já a trata como bloqueante. Não é uma lacuna
que este ML deveria fechar; é a que decide a prioridade do próximo trabalho (pergunta 5).

---

## Pergunta 1 — O que efetivamente mudou no risco?

**Medido, não hipótese:** antes deste roadmap, o incidente registrado em
`docs/roadmaps/wip/ROADMAP-2026-08-12-semantica-de-falha-de-hook-...md` era: hook de guard aponta
para script que não resolve → CLI prossegue (fail-open, 4 de 6) → nenhuma detecção em nenhum momento,
nem em `validate`, nem em runtime. Depois deste roadmap: existe uma regra (`credential_guard_hook_resolvable`,
`internal/validator/validator_credential_guard.go:113`) que, quando `trackfw validate` roda, varre os
6 arquivos de hook de projeto, extrai comandos que referenciam `trackfw-credential-guard.sh`, resolve
o caminho pelas 3 formas de prefixo que o trackfw emite, e reporta violação se o script resolvido
**não existe** (`os.Stat` falha) ou **não é executável** (`info.Mode()&0111 == 0`, linha 167). A
prova de não-vacuidade é o Cenário 47 do falsify (`scripts/check-gates-falsify.sh:3820`): fixture com
`.claude/settings.json` real apontando para script ausente, executado contra o binário Go real, regra
desligada faz o cenário falhar. Não é alegação — é comportamento observado do binário.

**O delta, honestamente:** o incidente original — o exato modo de falha que motivou o roadmap
(`ROADMAP-2026-08-11`, caminho de hook que não resolvia por causa de `$CWD`) — passa a ser **detectável
em qualquer `trackfw validate`**, incluindo o que roda em CI/gate. Isso é uma melhora real e
verificável, não cosmética: fecha a lacuna "ninguém percebe que o guard morreu" para a classe
específica de "caminho errado ou permissão errada", que é exatamente o que já aconteceu uma vez em
produção deste projeto.

**O que o delta não é:** não é uma redução de risco contra um **adversário ativo**. A regra roda sob
demanda (quando alguém invoca `validate`), não a cada invocação de ferramenta — um agente induzido que
sobrescreve o script e continua trabalhando não dispara nada até a próxima corrida de `validate`, e
mesmo aí só dispara se a sobrescrita alterar existência/bit de execução, o que uma sobrescrita
adversarial minimamente cuidadosa não faz (pergunta 2). O `failClosed` do Cursor — a única mitigação
que atacava o momento da invocação, não o momento do `validate` — foi revertido e confirmado
byte-idêntico ao estado pré-ML-3A (`git grep failClosed` em `internal/generators`, `npm/src`,
`pypi/trackfw` → 0 ocorrências, conforme sessão de auditoria de Zeus em
`docs/agents-working-context.md`).

**Veredito:** delta real, mas de **detecção de degradação acidental/operacional**, não de **contenção
de adversário**. Os dois são valores diferentes e não devem ser confundidos na comunicação ao usuário.

---

## Pergunta 2 — 🔴 A regra do ML-1A cria falso senso de segurança?

**Sim, existe risco real, e o mecanismo é exatamente o apontado no enunciado.** Dois defeitos
estruturais, ambos confirmados na leitura do código:

1. **Tempo de checagem, não tempo de uso.** A regra só roda quando `validate` é invocado
   (manualmente ou via gate/CI). Entre duas corridas de `validate`, o guard pode ser apagado,
   sobrescrito ou ter o modo rebaixado sem que nada aponte isso — a próxima invocação de ferramenta do
   agente já roda sem guard, e só a próxima corrida de `validate` (que pode nunca acontecer numa
   sessão curta) revelaria o problema.

2. **Detecta ausência, não integridade.** `os.Stat` + bit `0111` são as duas únicas checagens
   (linhas 160–172). Um script substituído por `#!/bin/sh\nexit 0` mantém o mesmo caminho, o mesmo
   bit de execução (se o agente preservar `chmod +x`, que é trivial), e o mesmo `os.Stat` bem-sucedido
   — a regra não lê **nenhum byte** do conteúdo do script. Isso não é uma lacuna teórica: é
   textualmente o vetor 2 listado no próprio ADR (`ADR-2026-08-12`, linha 25) como "pior que apagar:
   produz o mesmo rótulo de sucesso normal, sem sinal observável" — e a regra do ML-1A não muda esse
   rótulo em nada.

Depois da reversão das outras três mitigações (`failClosed`, wrapper, integridade), a regra do ML-1A
é **a única coisa que sobra no escopo de projeto**. Um leitor apressado — "existe uma regra de
validação para o credential-guard, `validate` passa, logo estou protegido" — comete exatamente o erro
que o enunciado antecipa. O nome da regra (`credential_guard_hook_resolvable` — "resolvable", não
"secure" ou "intact") já é honesto no código, mas **não existe hoje nenhum texto voltado ao usuário
final** (README, `--help`, mensagem de sucesso do `validate`) que declare o que ela não cobre. A
mensagem de violação (linhas 163–171) é sobre o caso positivo (script ausente/não executável) e não
tem por que mencionar o caso negativo — não é o lugar certo.

**Onde precisa estar escrito, concretamente:**
- **`docs/cli-parity.md`** (ML-4B, Hefesto, em paralelo) — já é o escopo designado no próprio roadmap
  ("registrar a regra nova... e explicitamente o que continua descoberto"). Correto e suficiente
  como registro técnico para quem lê a documentação do projeto.
- **Recomendação adicional, fora do escopo de arquivo permitido deste ML:** a superfície onde o
  usuário realmente decide se confia no controle é o texto de ajuda de `trackfw validate` ou do
  próprio README de segurança do projeto (se existir) — não só `docs/cli-parity.md`, que é
  referência técnica de paridade entre CLIs, um documento que um usuário final dificilmente abre
  para avaliar postura de segurança. Reporto isso a Zeus como item a considerar para a REQ nova ou
  para um ML de documentação de usuário separado — **não é bloqueante para este PR**, porque
  `docs/cli-parity.md` já cumpre o critério de aceite literal do roadmap.

---

## Pergunta 3 — O estado atual é melhor ou pior que antes do roadmap?

**Melhor, e o argumento não depende de autoridade do ADR.** Comparando estado inicial (antes de
`ROADMAP-2026-08-12-mitigacao-do-fail-open`) contra o estado atual:

- **Detecção de degradação operacional:** 0 → regra + prova de não-vacuidade. Estritamente uma soma.
- **`failClosed` do Cursor:** implementado (ML-3A/3B) e depois revertido (ML-3C), confirmado
  byte-idêntico ao estado pré-ML-3A por dois métodos independentes (Apolo: worktree em `fbbeaf9` +
  diff funcional vazio; Zeus, na auditoria: `git diff fbbeaf9` vazio nos 6 arquivos + `git grep`
  zero ocorrências). **Não existe defesa residual, mas também não existe regressão residual** — o
  código voltou ao estado anterior, não a um estado pior. O argumento "removemos uma defesa" é
  factualmente impreciso: uma defesa foi **construída, avaliada e descartada por análise de
  custo-benefício documentada no ADR** (cobre 1 de 6 CLIs, brica clone fresco pelo mesmo mecanismo
  que o item 3 da REQ original foi adiado por bricar), não silenciosamente removida sem
  substituição. A pergunta certa não é "tínhamos e perdemos", é "a proteção que tínhamos era real" —
  e o ADR argumenta, com evidência de código (`scaffold.go:779-837`, script gerado fora do binário),
  que cobria só deleção em 1 CLI, não sobrescrita em nenhum.
- **Vias abertas antes do roadmap e ainda abertas hoje:** sobrescrita de script e rebaixamento de
  `credential_guard.mode` via YAML. Nenhuma das duas foi fechada nem piorada por este roadmap —
  seguem exatamente como estavam, agora **nomeadas explicitamente** no ADR e endereçadas por REQ
  própria, em vez de implícitas.

**Veredito:** estritamente melhor ou igual em cada eixo, nunca pior. O único jeito de argumentar
"pior" é comparar contra um mundo hipotético em que o `failClosed` teria sido mantido — e esse mundo
tem um custo documentado (bricking de clone fresco no Cursor) que o ADR decide não pagar por uma
cobertura parcial. Concordo com a decisão do ADR nesse ponto específico; não é apelo à autoridade,
é reavaliação independente do mesmo trade-off com o mesmo código à vista.

---

## Pergunta 4 — A REQ nova cobre o que precisa?

Lida na íntegra
(`docs/req/REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-...md`). Os três eixos
(consentimento · medição da premissa de sandbox · via do `credential_guard.mode`) cobrem as três
lacunas que o próprio ADR registrou como riscos aceitos (linhas 96-108 do ADR) — não há
desalinhamento entre o que o ADR prometeu investigar e o que a REQ pede.

**Pontos fortes que já vi:**
- A medição da premissa de sandbox é corretamente marcada como **bloqueante, antes de qualquer
  implementação** — e a REQ já prevê o resultado "premissa falsa → parar e reabrir o ADR" como
  resultado legítimo, não falha. Isso é a postura certa: não implementar consentimento/instalação de
  um controle cuja eficácia ainda não foi verificada.
- Escopo negativo explícito impede reabrir `failClosed`/wrapper/integridade por baixo do radar.

**O que falta, na minha leitura de segurança — um vetor não coberto pelos três eixos:**

- **Rotação/revogação do guard global instalado.** A REQ cobre instalar com consentimento e medir
  a premissa de sandbox, mas não menciona o que acontece se o guard **global** (`~/.trackfw/...`) for
  ele mesmo apagado ou sobrescrito depois de instalado — por exemplo, por um agente que **de fato**
  alcança `$HOME` (o cenário que o eixo 2 está justamente medindo). Se a premissa de sandbox se
  confirmar **parcialmente** verdadeira (alguns CLIs restringem, outros não — plausível, já que os
  próprios CLIs têm posturas de fail-open/fail-closed divergentes conforme
  `docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`), o guard global herda o **mesmo** problema
  estrutural do guard de projeto para os CLIs sem sandbox: nada detecta sobrescrita dele também, e
  hoje não há checagem equivalente ao ML-1A apontando para `~/.trackfw/` (a regra atual — de propósito
  — só varre arquivos de hook de **projeto**, `credentialGuardHookFiles`, linha 28). Não é bloqueante
  para a REQ começar (a medição do eixo 2 vem primeiro e pode tornar isso moot para os CLIs
  sandboxed), mas deveria ser critério de aceite condicional: "se a medição do eixo 2 mostrar que
  algum CLI alcança `~/.trackfw/`, a REQ precisa decidir se a regra `credential_guard_hook_resolvable`
  se estende ao escopo global para esse CLI, ou se essa extensão fica para depois".

Reporto este ponto a Zeus como sugestão de emenda à REQ, não como bloqueio ao ML-4A ou ao PR desta
branch — a REQ ainda não tem roadmap (`roadmap: ""` no frontmatter), há espaço para incorporar isso
antes da execução.

---

## Pergunta 5 — A premissa central do ADR não foi medida. Qual a probabilidade de ser falsa?

**Sem medição direta neste ML — a REQ já classifica isso corretamente como bloqueante, e concordo com
essa classificação.** O que posso oferecer é avaliação qualitativa a partir do que **já foi medido**
por trabalho anterior deste projeto (`docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-*.md`), que
é evidência indireta, não a medição que a REQ pede:

- Os CLIs deste projeto rodam hoje, na prática observada (inclusive esta própria sessão), **sem
  sandbox de filesystem restringindo escrita a `$HOME`** — Claude Code, por exemplo, tem acesso de
  escrita amplo ao sistema de arquivos do usuário por padrão (é o que permite a este próprio agente
  escrever em `docs/seguranca/`, fora do subtree típico de "workspace restrito"). Ambientes com
  sandbox de container/VM restringindo `$HOME` existem (CI isolado, containers dedicados), mas não
  são a configuração default de nenhum dos 6 CLIs que o trackfw suporta, pelo que a varredura
  documental de 5 CLIs já registrou.
- **Minha avaliação: a probabilidade de a premissa ser falsa no ambiente default (sem sandbox
  explícito adicional) é alta — não baixa.** Um agente que já tem execução de shell habilitada
  (pré-requisito para o próprio ataque de sobrescrever o script) tipicamente tem o mesmo nível de
  acesso a `$HOME` que tem ao repositório, salvo configuração explícita de contenção que a maioria
  dos usuários não aplica hoje.

**Isso muda minha recomendação de prioridade: sim, na direção que a própria REQ já tomou.** A REQ já
marca a medição do eixo 2 como bloqueante antes de qualquer implementação de consentimento/instalação
— avaliação correta, e reforço com este parecer: se a hipótese "agente alcança `$HOME`" se confirmar
no ambiente default, o "escopo global por padrão" não é uma defesa nova, é o **mesmo problema
reposicionado** — e a REQ precisa, nesse caso, tratar isso não como uma melhoria incremental de UX de
instalação, mas reabrir explicitamente se o ADR-2026-08-12 continua de pé como decisão de arquitetura
(o próprio texto da REQ já prevê esse caminho, linha 62-64: "parar, registrar, e reabrir o ADR").
**Recomendação de sequência para Zeus: tratar o eixo 2 (medição) como o primeiro trabalho da REQ, sem
paralelizar com os eixos 1/3, mesmo que isso pareça sacrificar velocidade** — porque, se a medição
sair como aqui antecipado, o design dos eixos 1 e 3 muda ou perde sentido, e qualquer trabalho
paralelo neles vira retrabalho.

---

## Conclusão

**O estado atual pode seguir para PR.** Os critérios de aceite do ML-4A/roadmap estão satisfeitos:
nenhuma das quatro mitigações revertidas/adiadas deixou o código em estado pior que o inicial (P3);
o delta de risco entregue é real, mas de classe restrita — detecção de degradação operacional, não
contenção de adversário ativo (P1); a regra do ML-1A não é vácua (Cenário 47) e o gate de paridade e
os testes seguem verdes conforme as sessões de auditoria já registradas por Zeus.

**Uma condição não bloqueante, mas que recomendo fortemente resolver antes ou logo após o merge:** o
risco de falso senso de segurança (P2) é real e tem mecanismo concreto (tempo de checagem vs. tempo
de uso; ausência vs. integridade). `docs/cli-parity.md` (ML-4B, em paralelo) é o lugar certo para o
registro técnico e já está no escopo do roadmap — mas não é onde um usuário final avalia postura de
segurança. Recomendo a Zeus abrir um item de documentação de usuário (README ou texto de `--help`)
fora deste roadmap, além do já previsto na REQ nova.

**Um achado para a REQ nova, não bloqueante para este PR:** falta um critério condicional sobre o que
fazer com o guard **global** se a medição do eixo 2 revelar que ele também é alcançável por agente
sem sandbox (P4).

**Nada neste parecer bloqueia o PR desta branch.**
