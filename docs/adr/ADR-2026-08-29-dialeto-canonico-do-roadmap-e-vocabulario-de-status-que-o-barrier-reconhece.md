---
status: Accepted
date: 2026-08-29
author: "trackfw_architect (Zeus)"
---

# ADR: Dialeto canônico do roadmap e vocabulário de status que o `barrier` reconhece

> Date: 2026-08-29 | Status: Accepted

## Context

**Um roadmap gerado pelo `trackfw roadmap new` e preenchido exatamente como o próprio template
instrui é reprovado pelo `trackfw barrier` — em dois checks independentes.** Medido em 2026-08-29,
com o binário 7.3.0, num projeto de sonda:

```
$ trackfw roadmap new "Sonda do dialeto do barrier"
$ sed -i '' 's|^\*\*Status:\*\* pending$|**Status:** done|'  <roadmap>   # o template diz "pending"
$ sed -i '' 's|^- \[ \]|- [x]|'                              <roadmap>   # critérios marcados
$ trackfw barrier <roadmap> --wave 1 --trust-local-gates

    - ML-1A: not complete (status: done)
  ✗ acceptance_evidence: blocked
    - ML-1A: no acceptance block
```

São **dois defeitos de natureza diferente**, e tratá-los como um só resolveria metade:

**1. Cabeçalho de aceite — problema de idioma.**

| | escreve / procura |
|---|---|
| `roadmap new` (Go) | `**Acceptance criteria:**` — `internal/generators/roadmap.go:64,176,225` |
| `roadmap new` (Node) | `**Acceptance criteria:**` — `npm/src/generators/roadmap.js:31,495,558` |
| `roadmap new` (Python) | `**Acceptance criteria:**` — `pypi/trackfw/generators/roadmap.py:40` |
| `barrier` (Go) | `^\*\*Crit[eé]rios de aceite:\*\*` — `internal/commands/barrier.go:166` |
| `barrier` (Node) | `/^\*\*Crit[ée]rios de aceite:\*\*/` — `npm/src/commands/barrier.js:144` |
| `barrier` (Python) | `^\*\*Crit[ée]rios de aceite:\*\*` — `pypi/trackfw/commands/barrier.py:105` |

**2. Status do ML — problema de representação, não de idioma.** O gerador escreve
`**Status:** pending`; os 3 barriers exigem que o restante da linha **contenha `✅`**
(`barrier.go:554`, `barrier.js:134`, `barrier.py:207`). Traduzir o template para `Pendente` **não
resolveria** — a marca de conclusão é um glifo, e o template não ensina glifo nenhum: ele oferece
`pending` e nenhuma legenda.

**Por que nenhum gate pega.** A paridade entre os 3 CLIs está **intacta**: os três geradores
escrevem inglês, os três barriers procuram português e exigem o emoji. Os gates de paridade medem se
as implementações concordam **entre si**, e elas concordam perfeitamente. O contrato quebrado é
**gerador ↔ verificador**, e nenhum gate atravessa essa fronteira. *Paridade entre implementações não
é o mesmo que correção do contrato.*

**Corpus existente**, medido em 143 roadmaps: 99 usam o cabeçalho PT, 43 o EN. Nos status:
294 `✅ Concluído`, 73 `⬜ Pendente`, 67 `done`, além de variantes com sufixo
(`✅ Concluído · **Agente:** \`apolo-tf\``, `✅ concluído (auditado 2026-08-02)`) que hoje passam
**apenas porque o casamento é substring**.

## Decision

**1. O dialeto canônico dos cabeçalhos é o INGLÊS.** É o que os 3 geradores já escrevem, e o que o
template usa em todo o resto (`**Status:**`, `**Files affected:**`, `**Actions:**`). O `barrier`
passa a reconhecer `**Acceptance criteria:**` **e** `**Critérios de aceite:**`.

**2. A forma portuguesa continua aceita, sem prazo de remoção.** 99 dos 143 roadmaps a usam,
inclusive em `done/`. Quebrá-los seria trocar um bug por outro.

**3. Conclusão de ML é reconhecida por TOKEN, não por substring.** A regra: o restante da linha de
status é tokenizado, e o ML é considerado concluído quando o **primeiro token** é um marcador de
conclusão — `✅`, `done` ou `Concluído` (insensível a caixa e a acento). Aceitos:

```
✅                                                    → concluído
✅ Concluído                                          → concluído
✅ Concluído · **Agente:** `apolo-tf`                 → concluído   (sufixo preservado)
✅ concluído (auditado 2026-08-02)                    → concluído
done                                                  → concluído
Concluído                                             → concluído
⬜ Pendente · 🔄 Em andamento · ❌ Bloqueado          → NÃO concluído
```

**4. A leitura por primeiro token existe para fechar o buraco do substring.** `Contains(marker, "✅")`
é o mesmo mecanismo que `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`
registrou como falso-positivo em campo de status. Ao **ampliar** o vocabulário para aceitar palavras,
o substring passaria a classificar `**Status:** não done` e `**Status:** pending (era done)` como
concluídos. O primeiro token é o discriminante: `não` e `pending` não são marcadores.

**5. O template do `roadmap new` passa a ensinar o vocabulário.** Hoje ele escreve `pending` e não
diz o que colocar no lugar. Passa a escrever a forma canônica e a trazer a legenda dos quatro
estados. Sem isso, corrigir o parser deixa o usuário adivinhando.

**6. `**Gates da wave:**` fica como está — em português, nos dois lados.** Gerador e `barrier` já
concordam nesse token; não há defeito. Renomear criaria exigência de forma dupla para corrigir nada.

## Consequences

**Positivas**
- O ciclo `roadmap new` → preencher → `barrier` fecha sem edição manual, que é a promessa da
  ferramenta.
- O casamento por token corrige, de quebra, um falso-positivo latente que hoje passa despercebido
  porque o vocabulário é pequeno demais para expô-lo.
- O template deixa de ser a única fonte que o usuário tem e não ensina o que a ferramenta exige.

**Negativas e riscos aceitos**
- **Duas formas de cabeçalho aceitas para sempre.** É superfície de parsing permanente, em 3
  runtimes. Aceito: o custo de migrar 99 arquivos históricos é maior, e migração de artefato
  concluído é reescrita de registro.
- **Vocabulário de status maior é superfície maior.** Cada marcador novo é uma chance de falso
  positivo. Mitigado pela leitura por primeiro token e por gate falsificável nas duas direções — mas
  a lição do ML-2G da REQ anterior vale aqui: *cobertura maior é superfície maior*.
- **Roadmaps históricos não são reclassificados.** Um ML com `**Status:** feito` continua não
  reconhecido, porque `feito` não entra no vocabulário. Preferimos vocabulário fechado e explícito a
  heurística de linguagem natural.
- O `barrier` fica sabendo de duas línguas, o que é dívida conceitual. A alternativa — canonizar uma
  e migrar — foi rejeitada abaixo.

## Alternatives Considered

**Canonizar o português e mudar os 3 geradores.** Alinharia com os 99 roadmaps e com
`**Gates da wave:**`, que já é PT. Rejeitada: o template é inglês em todos os outros cabeçalhos, e
mudar só um deixaria o artefato gerado ainda mais misturado. O `barrier` teria que aceitar as duas
formas de qualquer jeito, pelos 43 roadmaps em inglês — então o custo de parsing é o mesmo e o de
implementação é maior.

**Migrar os roadmaps históricos para uma forma única.** Elimina a dupla aceitação. Rejeitada:
reescreve registro de trabalho concluído, e `done/` é evidência, não código.

**Manter só `✅` e corrigir o template para escrevê-lo.** Menos superfície de parsing. Rejeitada por
decisão de KG: os 67 MLs que dizem `done` seguiriam invisíveis, e ancorar a marca de conclusão num
emoji digitável é frágil para quem edita em terminal ou colabora por patch.

**Fazer o `barrier` aceitar qualquer status não vazio como conclusão.** Transformaria o check em
no-op — exatamente a classe de defeito que a release 7.3.0 inteira combateu.
