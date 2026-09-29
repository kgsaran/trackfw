---
status: Accepted
date: 2026-09-28
author: "trackfw_architect"
---

# ADR: `trackfw init` reexecutado preserva a configuração autorada pelo consumidor

> Date: 2026-09-28 | Status: Accepted

## Context

`trackfw init` é tratado hoje como **bootstrap de projeto vazio**. Reexecutado num projeto já
onboardado, `writeTrackfwConfig` (`internal/generators/scaffold.go`) faz `os.WriteFile` com o
template fixo — sem ler, sem mesclar, sem avisar. Todo valor customizado é perdido.

**O dano principal não é a perda do arquivo — é o produto passar a mentir sobre o estado de
governança.** Ocorrência real neste repositório em 2026-09-27, com a perda de `governance_mode:
lenient` e `lenient_until`:

| | antes | depois do `init` |
|---|---|---|
| `trackfw validate` | **170 warnings** | **156 violations** |
| exit code | `0` | `1` |

Um repositório conforme passou a ser reprovado. E o efeito de segunda ordem é pior: o agente que
causou isso mediu o `validate` **depois** e reportou as 156 como *"todas pré-existentes"*. Não eram —
eram consequência da própria ação, três comandos antes.

🔴 **Um comando que reescreve config em silêncio não corrompe só o arquivo: corrompe as medições que
vierem depois dele**, inclusive as de quem age de boa-fé e não sabe que o chão mudou.

### O que distingue os sítios de escrita — medido em 2026-09-28

`internal/generators/scaffold.go` tem **22** chamadas reais de `os.WriteFile`/`os.OpenFile`
(o `grep` ingênuo por substring conta 25; **3 são comentários**). A régua correta **não é a forma da
chamada** — é a **natureza do conteúdo destruído**:

| classe | conteúdo | truncar é |
|---|---|---|
| **(a)** | configuração/declaração **autorada pelo consumidor** (`trackfw.yaml`) | 🔴 **defeito** |
| **(b)** | artefato **gerado pelo produto** (scripts de hook, workflows) | ✅ **correto** — é como a correção chega |
| **(c)** | **híbrido**: bloco gerado dentro de arquivo do usuário (`.gitignore`, `.gitattributes`, `lefthook.yml`) | depende — já há ramo de merge; verificar se está completo |

A classe (b) é intencional e **não deve mudar**: foi exatamente por ela que a correção de CRLF do
#353 chegou às máquinas dos consumidores. Confundir (a) com (b) produziria o defeito oposto —
congelar scripts defeituosos no projeto de quem instalou.

## Decision

**`init` reexecutado preserva todo valor já presente no `trackfw.yaml` e acrescenta apenas as chaves
ausentes.** Não recusa, não sobrescreve, não reordena.

E a decisão que decide a implementação:

🔴 **A preservação inclui os comentários.** O bloco `agent_models` perdido na ocorrência real
carregava *a justificativa de cota* em comentário. Preservar as chaves e descartar a razão delas é
preservação aparente.

**Medido em 2026-09-28**, com `gopkg.in/yaml.v3 v3.0.1` (a única lib YAML do projeto):

| estratégia | comentários | ordem das chaves |
|---|---|---|
| `map[string]any` (round-trip ingênuo) | 🔴 **destruídos** | 🔴 reordenada (alfabética) |
| `yaml.Node` | ✅ preservados | ✅ preservada (muda só indentação) |
| merge **textual** por chave ausente | ✅ preservados | ✅ preservada (zero diff nas linhas existentes) |

**Decidido: merge textual por chave ausente.** O `yaml.Node` preserva o que importa, mas reescreve o
arquivo inteiro (a indentação muda de 2 para 4 espaços), produzindo diff em linhas que ninguém pediu
para mudar. O merge textual produz **zero diff** no que já existia — e é a propriedade que torna a
correção auditável por `git diff`.

Fato que reforça a escolha: **o projeto nunca fez `yaml.Marshal`** — a config sempre foi escrita como
template de texto. Introduzir round-trip estrutural seria novidade arquitetural para resolver um
problema que o merge textual já resolve.

## Consequences

**Positivas**
- `init` torna-se **idempotente** sobre config: reexecutar é seguro, que é o que o nome promete.
- Upgrade continua funcionando: chaves novas de uma versão nova **são** acrescentadas.
- O veredito do `validate` deixa de depender de quantas vezes alguém rodou `init`.

**Negativas e aceitas**
- Uma chave cujo **default mudou** entre versões não é atualizada — o valor do consumidor vence.
  É o comportamento correto: o valor está lá porque alguém o escolheu. Comunicar mudança de default
  é problema de release notes, não de sobrescrita silenciosa.
- 🔴 **Emenda 1 (2026-09-28, achado da Wave 2): sub-chave nova sob bloco de nível 0 já presente
  também NÃO é entregue** — e isso é **decisão**, não lacuna.

  Medido: um consumidor com `rules:` contendo `some_other_rule: warning` **não** recebe
  `branch_has_wip_roadmap: error` num `init` posterior. O merge vê `rules:` como chave de nível 0
  presente e pula o bloco inteiro.

  **Por que manter assim:** entregar sub-chave nova sob `rules:` significaria **injetar silenciosamente
  uma regra de severidade `error` num repositório conforme** — que é exatamente o dano
  `170 warnings → 156 violations` que esta ADR existe para eliminar. Corrigir isso com merge
  recursivo reintroduziria a classe de defeito pela porta de trás, com outra roupa.

  Consequência aceita: uma regra nova de uma versão nova só chega a quem **não** tem o bloco `rules:`.
  Para quem tem, a comunicação é por release notes — que é onde mudança de comportamento pertence.
- O merge textual precisa de teste para YAML com aspas, listas e blocos aninhados — a classe (c)
  mostra que o projeto já sabe fazer isso, mas a superfície é maior aqui.

## Alternatives Considered

**Recusar e avisar quando `trackfw.yaml` existe** — rejeitado. Quebraria o uso legítimo de `init`
para adicionar chaves novas após upgrade, que é justamente o caso de uso que o consumidor tem depois
de atualizar. Transformaria um defeito em obstáculo.

**Round-trip estrutural com `map[string]any`** — rejeitado **por medição**: destrói comentários e
reordena chaves. Teria "preservado" a config destruindo a justificativa dela.

**Round-trip estrutural com `yaml.Node`** — rejeitado por custo/benefício. Preserva o essencial, mas
reescreve o arquivo inteiro e introduz dependência estrutural que o projeto nunca teve.

**Fazer backup antes de sobrescrever** — rejeitado. Trata o sintoma; o consumidor continua com a
config errada em vigor e um `.bak` que ninguém lê.
