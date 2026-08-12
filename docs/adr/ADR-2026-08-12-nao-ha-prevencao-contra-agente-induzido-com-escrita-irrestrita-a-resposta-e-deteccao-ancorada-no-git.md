---
status: Accepted
date: 2026-08-12
author: "Zeus (Arquiteto)"
---

# ADR: Nao ha prevencao contra agente induzido com escrita irrestrita — a resposta e deteccao ancorada no git

> Date: 2026-08-12 | Status: Accepted
> **Supersede:** `docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md`

REQ: `docs/req/REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-consentimento-explicito-verificacao-da-premissa-de-sandbox-e-a-via-do-credential-guard-mode.md`
Medição: `docs/pesquisa/2026-08-12-alcance-do-agente-ao-home.md`
Parecer: `docs/seguranca/2026-08-12-alcance-do-agente-ao-home.md`

## Context

O ADR anterior decidiu que **a defesa do credential-guard é o escopo global** (`~/.trackfw/`, fora
do repositório), e com base nisso reverteu o `failClosed` e rejeitou wrapper e integridade de
conteúdo. Sua premissa — *"o guard global está **fora do alcance** de um agente restrito ao
workspace"* — **nunca havia sido medida**.

Foi medida. **Não se sustenta**, e de três formas diferentes.

### 1. O alcance ao `$HOME` não é uniforme entre CLIs

| CLI | Alcança `$HOME`? | Mecanismo |
|---|---|---|
| Codex | **NÃO** | sandbox `-s workspace-write` bloqueia com `Operation not permitted` |
| **Cursor** | **SIM** | 3 braços escrevem e apagam sem aprovação nenhuma |
| **Claude Code** | **SIM, tecnicamente** | `permission_denials: []` — nenhuma camada técnica; a recusa observada veio de **heurística de alinhamento do modelo** |
| Gemini · Copilot | INDETERMINADO | bloqueados por conta/política antes de qualquer tool call |
| Kiro | INDETERMINADO | não instalado |

**O caso do Claude é o mais importante e não pode ser lido como proteção.** O mesmo caminho foi
escrito **sem hesitação** quando o nome do arquivo era neutro. A recusa é endereçada por
**conteúdo/nome** — exatamente o que um adversário induzido controla. Alinhamento de modelo é a
primeira camada que a indução ataca; contá-la como defesa contra indução é circular.

### 2. O escopo global é PIOR em visibilidade e sinal — não neutro

Achado do parecer, e ele inverte parte do raciocínio do ADR anterior:

- um artefato **dentro** do repositório é auditável **por construção** — aparece em `git status`, no
  diff, no PR, no code review;
- um artefato em `~/.trackfw/` **não aparece em lugar nenhum** disso.

Ou seja: mover o controle para fora do repositório **compra custo** (para os CLIs com sandbox real)
**pagando com visibilidade**. O ADR anterior tratou o trade-off como se fosse ganho puro.

### 3. 🔴 O escopo global não fecha a via do `credential_guard.mode` — verificado no código

`credentialGuardModeResolution` (`internal/generators/scaffold.go:1005`) faz `grep` de
`credential_guard.mode` no `trackfw.yaml` **do cwd**, e é **compartilhada** entre as variantes de
projeto e global (documentado na Emenda 6 do `ADR-2026-08-06`). Verificado por leitura direta.

**Consequência:** instalar o guard em `~/.trackfw/` **não** protege contra o downgrade de modo. Um
agente induzido não precisa alcançar `$HOME` — basta editar `trackfw.yaml` **dentro do próprio
workspace**. Isso vale inclusive para o **Codex**, o único CLI cujo sandbox bloqueou os alvos fora do
workspace.

A pergunta "o agente alcança `$HOME`?" era a certa para as vias de **deleção** e **sobrescrita**, e
**irrelevante** para a via de `mode`. O ADR anterior otimizou para duas de três vias sem perceber que
a terceira ignorava a defesa escolhida.

## Decision

**Não existe prevenção técnica, no escopo do trackfw, contra um agente induzido com escrita
irrestrita ao workspace. Isso é afirmado, não contornado. O esforço vai para detecção ancorada no
`HEAD` do git — não para escopo de instalação.**

1. **Parar de construir mecanismo de prevenção.** `failClosed`, wrapper e integridade de conteúdo
   permanecem rejeitados — pelos motivos próprios de cada um, registrados no ADR superseded, **e
   agora também porque nenhum deles resolve a via do `mode`**.
2. **O escopo global deixa de ser "a defesa".** Continua sendo **mitigação de custo** para CLIs com
   sandbox técnico real (hoje: Codex). **Não** vira caminho padrão, e **não** justifica escrever no
   `$HOME` do usuário sem consentimento.
3. **A direção de investimento é detecção ancorada no git.** O `HEAD` é âncora de confiança que
   **já existe** e **não depende do escopo global** — diferente da integridade de conteúdo rejeitada,
   que precisava de um valor de referência guardado fora do arquivo gerado e, portanto, do próprio
   escopo global como pré-requisito. Detectar que `scripts/trackfw-credential-guard.sh` ou
   `trackfw.yaml` divergem do `HEAD` cobre **as três vias** — deleção, sobrescrita e downgrade de
   `mode` — e cobre a mais provável delas com o instrumento mais barato.
4. **Política pelo pior caso medido, não por CLI.** Política por CLI multiplicaria pela regra de
   paridade de 3 implementações e, decisivo: **o trackfw não tem como saber, em produção, se o
   sandbox do Cursor de um usuário real está ligado**. Uma política que depende de um fato não
   observável não é política.
5. **Assimetria a registrar, a favor da detecção:** o fallback do guard **global** é `block`
   (Emenda 6 do `ADR-2026-08-06`). Logo, a via que sobrevive à detecção exige uma edição
   **positiva** no `trackfw.yaml` — e é, das três, **a mais diffável**.

### Emenda 1 (2026-08-12, ML-0A) — a âncora é POR ALVO, e este ADR repetia o erro do superseded

Este ADR disse "detecção ancorada no `HEAD`" como se fosse uma âncora só. **É por alvo**, e o
raciocínio do ADR superseded (*"integridade de conteúdo exige o escopo global"*) **cai — mas
parcialmente**, e a forma importa:

| Alvo | Âncora | Por quê |
|---|---|---|
| `scripts/trackfw-credential-guard.sh` | **template do binário** | o script é concatenação pura de constantes, **sem interpolação por projeto** — o binário **sempre foi** a referência externa. O argumento do superseded é **falso** aqui. |
| `credential_guard.mode` | **`HEAD` do git**, comparação **semântica e direcional** (`block` no `HEAD` → não-`block` no disco), não byte-diff | valor **autoral**, sem forma canônica. O argumento do superseded é **verdadeiro** aqui. |

Sem redundância: o `HEAD` **não** deve cobrir também o script — não agrega cobertura e importa falso
positivo desnecessário.

### Emenda 2 (2026-08-12, ML-0A) — pré-requisito verificado por Zeus: falta gate de paridade do script

A âncora de template só é segura se os **três** templates (Go `credentialGuardScript`, Node
`CREDENTIAL_GUARD_SCRIPT`, Python `_CREDENTIAL_GUARD_SH`) forem byte-idênticos. **Verificado por
Zeus: nenhum gate compara os três.** `check-attention-scripts-parity.sh` cobre **apenas** os dois
scripts de *attention*; `check-agent-hooks-parity.sh` só faz `grep` da string no **JSON do hook**,
nunca no conteúdo do `.sh`.

**Consequência:** sem esse gate, o mesmo repositório **dispararia a regra num CLI e ficaria silencioso
nos outros** — falso positivo e falso negativo simultâneos, dependendo de qual binário o usuário roda.
O gate de paridade do script do credential-guard vira **pré-requisito da implementação**.

### Emenda 3 (2026-08-12, ML-0A) — o script não carrega marcador de versão

Nada no script gerado identifica a versão que o produziu. Logo, a âncora de template **não consegue
discriminar** *drift* legítimo (usuário não rodou `trackfw update` após um bump) de adulteração real.

**Decisão:** severidade **`warn`** para o braço do script, com mensagem **causalmente neutra** — não
afirmar adulteração quando a causa pode ser drift. Embutir versão/hash no template é trabalho futuro,
fora deste roadmap, e é o que permitiria elevar para `error`.

## Consequences

**Positivas**
- Para de gastar esforço em prevenção que a medição mostrou não funcionar.
- A direção nova (detecção via `HEAD`) cobre as **três** vias, inclusive a que o escopo global
  ignorava.
- Preserva a visibilidade do artefato dentro do repositório, em vez de trocá-la por custo.
- Elimina a necessidade de escrever no `$HOME` do usuário — some o problema de consentimento.

**Negativas / riscos aceitos**
- **Detecção não é prevenção.** Um agente induzido consegue neutralizar o guard; o que passamos a
  garantir é que **fica visível**, não que seja impedido. Isto precisa estar escrito para o usuário
  final, não só em documento interno de paridade.
- **Detecção via `HEAD` não cobre o que nunca foi commitado** — projeto antes do primeiro commit, ou
  alterações ainda não versionadas. Limite conhecido.
- **Gemini, Copilot e Kiro seguem `INDETERMINADO`.** Aceito: o pior caso já está estabelecido pelo
  Cursor e a política é pelo pior caso, então medir os três não mudaria a decisão.
- **O ADR anterior não é apagado.** Fica marcado como superseded, com sua premissa e o motivo da
  queda — quem propuser "instalar o guard global por padrão" de novo precisa encontrar o registro.

## Alternatives Considered

**Manter "global por padrão"** (o ADR superseded). Rejeitado por três razões independentes, cada uma
suficiente: não protege no Cursor nem no Claude; **piora** visibilidade e sinal; e **não fecha a via
do `mode`**, que é alcançável de dentro do workspace em todos os CLIs.

**Política por CLI** (endurecer só onde o sandbox não protege). Rejeitado: custo de 6 comportamentos
× 3 implementações, e o trackfw **não observa** o estado de sandbox do usuário em produção.

**"Documentar e parar", sem direção nova.** Rejeitado por ser desistência maior que a evidência
exige: a via do `mode` sobrevivente é a **mais diffável** das três, e existe um instrumento barato e
já disponível — o `HEAD` do git. Parar aqui deixaria valor óbvio na mesa.

**Voltar a `failClosed`/wrapper/integridade.** Rejeitado: continuam com os defeitos próprios
(*bricking*, cobertura de 1 de 6, dependência do escopo global) **e** nenhum resolve a via do `mode`.
