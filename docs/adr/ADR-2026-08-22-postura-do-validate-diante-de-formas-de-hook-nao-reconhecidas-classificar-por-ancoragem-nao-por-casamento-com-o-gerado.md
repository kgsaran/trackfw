---
status: Accepted
date: 2026-08-22
author: "Zeus (Arquiteto)"
---

# ADR: Postura do `validate` diante de formas de hook não reconhecidas — classificar por ancoragem, não por casamento com o gerado

> Date: 2026-08-22 | Status: Accepted

## Context

`resolveCredentialGuardHookPath` (`internal/validator/validator_credential_guard.go:88`) reconhece
exatamente **3 formas** — as que o trackfw emite:

```go
"$CLAUDE_PROJECT_DIR/…"  /  "$GEMINI_PROJECT_DIR/…"
"\"$(git rev-parse --show-toplevel)/…\""
caminho relativo puro            (Cursor/Copilot/Kiro)
```

Qualquer outra coisa cai no `default: return "", false`, e o chamador faz `if !ok { continue }` —
**silêncio**. A intenção original está escrita no comentário e é legítima: *"não é função desta regra
adivinhar wiring próprio de um usuário"*.

A barreira do ML-3A da REQ anterior nomeou 3 formas que atravessam esse silêncio e **falham fora da
raiz**: `$PWD/…`, `$UNDEFINED/…` e o valor entre aspas. A pior é `$PWD`, e a REQ-2026-08-21 explica
por quê: **é o erro que alguém comete tentando consertar**. Quem recebe *"references … with a bare
relative path"* e edita à mão pode escrever `$PWD/` acreditando que ancorou — e o `validate` passa a
ficar em silêncio, confirmando o engano.

A REQ enunciou a escolha como binária: **acusar tudo que não casa** (fecha a classe, arrisca
falso-positivo) × **lista estreita de formas sabidamente quebradas** (não incomoda, mas é a
"condição estreita demais" que esta série já nomeou nove vezes).

## O dado que quebra o empate

**Caminho absoluto cai no conjunto não reconhecido — e é uma forma correta.**

```go
case !strings.HasPrefix(raw, "$") && !strings.HasPrefix(raw, `"`) && !filepath.IsAbs(raw):
```

`IsAbs` true ⇒ a cláusula falha ⇒ `default` ⇒ `ok=false` ⇒ silêncio. Ou seja: hoje
`/opt/scripts/trackfw-credential-guard.sh` é silencioso, **e deve mesmo ser** — um caminho absoluto
ancora, funciona de qualquer cwd, e é wiring customizado legítimo.

Isso **falsifica a opção "acusar tudo que não casa"**: ela acusaria uma forma que funciona. Não é
hipótese de falso-positivo — é falso-positivo demonstrável a partir do código, sem precisar de campo.

E também mostra por que a opção binária estava mal formulada: o conjunto "não reconhecido" mistura
formas **corretas** (absoluto), formas **quebradas** (`$PWD/`) e formas **indecidíveis** (`$FOO/`,
com `FOO` definido no ambiente do usuário). Tratar o conjunto inteiro por um único veredito erra por
construção, em qualquer das duas direções.

## Decision

**O discriminante é a semântica de ancoragem do comando, não o casamento com o que o gerador emite.**

Três classes, avaliadas apenas para os CLIs com `requiresVarOrShellPrefix=true`
(Claude Code, Codex CLI, Gemini CLI — os que invocam hooks a partir do cwd do agente):

| Classe | Formas | Veredito |
|---|---|---|
| **1. Ancorado** | `$CLAUDE_PROJECT_DIR/…`, `$GEMINI_PROJECT_DIR/…`, `"$(git rev-parse --show-toplevel)/…"`, **caminho absoluto** | silêncio |
| **2. Comprovadamente dependente do cwd** | `$PWD/…`, `./…`, `../…`, relativo puro (já coberto) | **acusar** |
| **3. Indecidível** | `$OUTRA_VAR/…`, comandos com pipe/subshell não reconhecidos, `$UNDEFINED/…` | silêncio, **residual declarado** |

**Classe 2 é uma regra, não uma lista de literais.** O critério é *"expande a partir do diretório
corrente"* — propriedade verificável de cada forma, e é assim que o teste deve ser escrito: exercendo
a função de classificação, não os literais. É o que separa esta decisão do padrão "condição estreita
demais": a lista de nove vezes anteriores era um conjunto de strings sem predicado; esta tem
predicado, e formas novas entram por ele.

**Classe 3 permanece em silêncio, e isso é escolha, não omissão.** `$FOO/scripts/…` pode estar
perfeitamente correto no ambiente do usuário — o validador não lê o ambiente em que o hook vai rodar,
e inventar um veredito ali é exatamente "adivinhar wiring próprio", que a regra se proíbe de fazer
desde a origem. O residual fica registrado: **se o usuário usar uma variável própria mal definida, o
guard falha em silêncio e o `validate` não avisa.**

**A mensagem da classe 2 precisa explicar por que a forma não ancora.** Dizer "forma inválida" repete
o engano que levou o usuário até ali. Para `$PWD` especificamente: *`$PWD` expande para o diretório
corrente, não para a raiz do projeto*.

## Consequences

**Positivas**
- Fecha o caminho de indução criado pela correção anterior: quem tentar consertar com `$PWD/` é
  avisado, em vez de confirmado no erro.
- Caminho absoluto — wiring legítimo — continua silencioso, por decisão explícita e não por acaso.
- O predicado dá lugar para `./` e `../` sem nova rodada de decisão.

**Negativas e riscos aceitos**
- **A classe 3 continua sendo um ponto cego real.** Declarado aqui para não ser redescoberto como
  novidade: variável própria mal definida ⇒ guard inerte ⇒ `validate` silencioso.
- **Sem severidade nova.** As mensagens da classe 2 entram nas regras existentes
  (`credential_guard_hook_resolvable` / `git_branch_guard_hook_resolvable`), seguindo o precedente de
  não criar entrada nova em `rules:` — o mesmo critério aplicado quando as 4 checagens de escopo
  global foram dobradas nas regras de projeto. Quem quiser silenciar, silencia a regra inteira.
- **Risco residual de falso-positivo na classe 2:** um usuário poderia, deliberadamente, apontar o
  hook para `./scripts/...` num CLI que ele sabe rodar da raiz. Aceito: para Claude/Gemini/Codex essa
  premissa é falsa por decisão do `ADR-2026-08-11`, e é justamente o bug da REQ-2026-08-17.

## Alternatives Considered

**Acusar tudo que não casa com as 3 formas geradas.** Rejeitada com evidência: acusaria caminho
absoluto, que ancora corretamente. Fecharia a classe ao custo de quebrar o `validate` de quem está
certo — e, pelo `ADR-2026-08-17`, guard que atrapalha é guard que o usuário desliga.

**Lista estreita de literais quebrados (`$PWD`, `.`, `..`).** Rejeitada na forma, aceita no conteúdo:
o efeito prático inicial é o mesmo, mas uma lista sem predicado não diz o que fazer quando aparecer a
quarta forma. A decisão acima entrega o mesmo alcance imediato **com** critério de extensão.

**Severidade nova (`warning`) para a classe 3.** Rejeitada: exigiria regra nova em `rules:`, contra o
precedente estabelecido, e transformaria wiring customizado legítimo em ruído recorrente — o caminho
mais curto para o usuário desligar a regra inteira e perder também a classe 2.

**Executar o hook para descobrir se resolve.** Rejeitada de saída: o validador não executa comandos
de hook. Seria arbitrary code execution disparada por `trackfw validate`.
