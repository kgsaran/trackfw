---
status: Accepted
date: 2026-08-26
author: "Zeus (Arquiteto)"
---

# ADR: Superfície executável de um checkout de PR é auditada por comando dedicado, não por regra de `validate`

> Date: 2026-08-26 | Status: Accepted

## Context

A barreira do `ML-4A` (2026-08-23) nomeou os **irmãos** do residual AC13: além do slash command,
são superfície executável de um checkout de PR os **scripts de hook versionados** referenciados pelo
`.claude/settings.json` do projeto, o `Makefile` e os passos de CI.

**A superfície de hook é mais ampla que a do gate que o `#208` fechou: ela não exige rodar nenhum
comando do trackfw.** Basta o mantenedor abrir o repositório na ferramenta de agente e usar qualquer
ferramenta — o hook `PreToolUse` roda.

**Medido em 2026-08-26**, com fixture de projeto contendo um hook arbitrário:

```
.claude/settings.json  ->  PreToolUse/Bash -> "$CLAUDE_PROJECT_DIR/scripts/inocente.sh"
scripts/inocente.sh    ->  touch /tmp/EVIL_HOOK

$ trackfw validate
⚠  adr_dir "docs/adr" does not exist
1 warning(s)            <- NADA sobre o hook
```

**O `trackfw validate` valida os hooks do próprio trackfw** — integridade e resolvibilidade dos dois
guards, por marcador de nome de script. **Um hook novo, apontando para um script novo, é invisível
para ele.** Não é lacuna de implementação: as regras foram escritas para detectar adulteração dos
artefatos que o trackfw gera, não para inventariar wiring alheio.

## O que decide a escolha

O trackfw **não pode** se colocar entre o checkout e a execução do hook: quem lê o
`.claude/settings.json` e executa o script é o CLI de agente, não o trackfw. Qualquer solução do
trackfw é **antes** (informar o mantenedor) ou **depois** (constatar), nunca **durante**.

E há um risco que esta série já nomeou nove vezes: acusar wiring legítimo do usuário. Projetos têm
hooks próprios por motivos válidos — o próprio `~/.claude/settings.json` do mantenedor tem um
`pretooluse-guardrails.sh` que não é do trackfw. Uma regra de `validate` que acuse "hook não
gerenciado" transforma configuração legítima em ruído recorrente, e pelo `ADR-2026-08-17` **guard que
atrapalha é guard que o usuário desliga** — perdendo junto as regras que funcionam.

## Decision

**1. A auditoria é um comando dedicado, executado deliberadamente antes de confiar num checkout.**
Ele responde a pergunta que o mantenedor tem no momento em que a tem: *"o que neste PR roda na minha
máquina?"*

Escopo do que ele reporta, comparando o ref contra a base:
- wiring de hook em arquivos de configuração de agente versionados;
- scripts referenciados por esses hooks — **inclusive quando só o script muda e o wiring não**;
- alvos de `Makefile` e passos de CI **quando alterados**;
- artefatos gerenciados pelo trackfw cujo conteúdo diverge do template — reusando o que
  `validate` e `doctor` já sabem responder, sem reimplementar.

**2. Não é regra de `trackfw validate`, e não bloqueia nada.** `validate` roda o tempo todo, em todo
projeto; uma regra de inventário de wiring ali vira ruído. O comando novo roda quando o mantenedor
decide auditar.

**3. Não substitui a revisão de diff — torna a parte perigosa dela difícil de perder.** A mitigação
declarada continua sendo o mantenedor ler o diff; o que muda é que a superfície executável deixa de
estar diluída entre arquivos de documentação e código.

**4. O que o comando NÃO faz:** julgar se um script é hostil. Ele nomeia *o que executa*, não decide
*se é malicioso* — heurística de conteúdo é a fuga conhecida de toda allowlist de shell.

## Consequences

**Positivas**
- Cobre a classe inteira nomeada pela barreira, não só o slash command.
- Sem falso-positivo por construção: o comando **informa**, não acusa.
- Reusa `validate`/`doctor` para a parte de integridade que já existe.

**Negativas e riscos aceitos**
- **Depende de o mantenedor rodar.** É a fraqueza estrutural da decisão, e é consciente: a
  alternativa — bloquear por padrão — não é possível, porque o trackfw não está no caminho de
  execução do hook.
- **Não protege quem já abriu o repositório.** Se o hook já rodou, o comando constata, não previne.
  A janela é entre o checkout e o primeiro uso da ferramenta.
- Mais um comando na superfície do CLI.

## Alternatives Considered

**Regra de `validate` inventariando hooks não gerenciados.** Rejeitada: acusa wiring legítimo, roda o
tempo todo e recria o padrão *"guard que atrapalha é guard que o usuário desliga"*. Medido: o próprio
mantenedor tem hook global que não é do trackfw.

**Estender `doctor`.** Rejeitada como forma primária: `doctor` responde *"o disco bate com o
manifesto?"* — pergunta sobre artefatos **gerenciados**. Wiring de terceiro não é artefato gerenciado,
e sobrecarregar o `doctor` misturaria duas perguntas com remédios diferentes.

**Bloquear na origem — trackfw se recusar a operar em repo com hook não reconhecido.** Rejeitada: o
trackfw não é o executor do hook; recusar-se a rodar não impede a execução, só atrapalha o usuário.

**Só documentar.** É o estado atual desde o `#208`, e é insuficiente: a barreira nomeou a superfície
como **mais ampla que a do gate**, e a única mitigação hoje depende de o mantenedor lembrar de olhar
exatamente os arquivos certos num diff que pode ter centenas.
