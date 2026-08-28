---
status: Accepted
date: 2026-08-27
author: "Zeus (Arquiteto)"
---

# ADR: `doctor` cobre artefatos de scaffold por comparação com o template, com propriedade dada pelo caminho

> Date: 2026-08-27 | Status: Accepted

## Context

`trackfw doctor` varre "every catalog-managed agents/skills destination" e compara o disco contra o
**manifesto** (`~/.trackfw/integrations-manifest.json`), reportando três estados com remédios
distintos.

**Medido em 2026-08-27:**

```
manifesto:  290 artefatos
artefatos de scaffold no manifesto:  0
```

Os artefatos escritos pelo `Scaffold` (`internal/generators/scaffold.go`) **não têm entrada de
manifesto** — logo o `doctor` não tem contra o quê comparar e **não os enxerga**:

```
.claude/commands/trackfw/*.md      9 slash commands
scripts/trackfw-attention-*.sh
scripts/trackfw-validate.sh
.github/workflows/trackfw-gate.yml
```

O `trackfw validate` cobre **apenas dois** deles, por regra dedicada
(`credential_guard_script_integrity`, `git_branch_guard_script_integrity`). Todo o resto não tem
verificação de integridade em lugar nenhum.

**O achado veio da barreira do ML-1B da REQ da Wave 0** (2026-08-23): um projeto pode ficar com o
slash command **defasado** — ensinando a estrutura antiga de roadmap, sem Wave 0 — e **nada acusa**.
Só um `trackfw update` revela, e ele já corrige no mesmo passo, então o usuário nunca fica sabendo
que esteve defasado.

## Decision

**1. O `doctor` passa a cobrir os artefatos de scaffold comparando-os com o template que a versão
corrente gera** — o mesmo mecanismo que as duas regras de guard já usam, e que o `update` usa para
decidir `updated`/`skipped`.

**2. A propriedade é dada pelo CAMINHO, não pelo manifesto.** `.claude/commands/trackfw/`,
`scripts/trackfw-*.sh` e `.github/workflows/trackfw-gate.yml` são namespaces do produto: o nome
declara o dono. Manifesto existe para distinguir "o trackfw escreveu isto" de "outro escreveu"; onde
o caminho já carrega o namespace, o manifesto é redundante.

**3. Nada é registrado no manifesto retroativamente.** Registrar exigiria migração e faria **todo
projeto existente** reportar `unregistered-write` de uma vez — ruído em massa que treina o usuário a
ignorar o `doctor`, e o padrão *"guard que atrapalha é guard que o usuário desliga"*
(`ADR-2026-08-17`).

**4. `doctor` continua informando, não corrigindo.** O remédio é `trackfw update`, que já existe e já
sabe reescrever.

## Consequences

**Positivas**
- Fecha a lacuna com o mecanismo que já existe: comparação com template, não infraestrutura nova.
- Sem migração e sem ruído em massa — o estado "defasado" aparece só onde de fato existe.
- O `doctor` passa a responder a pergunta que o usuário faz: *"meu projeto está com os artefatos da
  versão que eu tenho instalada?"*

**Negativas e riscos aceitos**
- **Falso-positivo em customização deliberada.** Quem editou um slash command à mão passa a ver
  `hand-modified`. É informação correta — o `update` já sobrescreve essas edições hoje, então
  customizar não é suportado —, mas é atrito novo para quem não sabia disso. **A Wave 0 dimensiona.**
- **Duas fontes de verdade para "quem é dono"**: manifesto para catálogo, caminho para scaffold. É
  divergência conceitual assumida em troca de não migrar.
- O `doctor` fica dependente do template embutido no binário: rodar um binário velho num projeto novo
  reporta divergência que é do binário, não do projeto. Precisa ficar claro na mensagem.

## Alternatives Considered

**Registrar os artefatos de scaffold no manifesto.** Arquitetonicamente mais uniforme e a primeira
opção que considerei. Rejeitada pelo custo de migração: todo projeto existente reportaria
`unregistered-write` no primeiro `doctor` após a atualização — ruído em massa, exatamente o que faz o
usuário parar de ler a saída.

**Uma regra de `validate` por artefato**, no molde das duas de guard. Rejeitada: `validate` roda o
tempo todo e a lista cresceria a cada artefato novo; e a pergunta *"o disco bate com o template?"* é a
pergunta do `doctor`, não a de consistência de governança.

**Não fazer nada — o `update` já revela.** É o estado atual, e é insuficiente: o `update` **corrige no
mesmo passo**, então o usuário nunca sabe que esteve defasado, nem por quanto tempo. Diagnóstico e
remédio precisam ser separáveis.
