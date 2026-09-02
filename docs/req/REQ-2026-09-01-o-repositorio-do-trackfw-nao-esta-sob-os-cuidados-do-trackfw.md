---
status: Done
date: 2026-09-01
author: "zeus-tf"
adr: "docs/adr/ADR-2026-09-01-o-repositorio-do-trackfw-e-governado-pelo-trackfw-com-o-mesmo-rigor-que-o-produto-vende.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md"
---

# REQ: O repositório do trackfw não está sob os cuidados do trackfw

> Date: 2026-09-01 | Status: Done

## Motivation

Três lacunas medidas, todas do mesmo tipo — **o produto resolve isto em outros projetos e não em si
mesmo**:

| lacuna | medição |
|---|---|
| **Portão de merge** | `required_status_checks` **ausente** na `main`; `required_approving_review_count: 0`; `enforce_admins: false` — **qualquer PR merge com todo o CI vermelho e zero revisão** |
| **Guards** | vivem só em `.claude/settings.json`; `.git/hooks/` vazio; `core.hooksPath = /dev/null` — **protegem agentes, não pessoas** |
| **Cadeia exigida** | sem `CONTRIBUTING.md`, sem template de PR — a regra existe na cabeça dos mantenedores |

Nesta sessão os guards me bloquearam **seis vezes**, em situações reais: `git stash` num worktree
compartilhado com subagentes, `checkout --` destrutivo, `push` bruto. **Um humano com git normal não
tem nenhuma dessas proteções neste repositório.**

## O argumento que decide, e ele é empírico

**Disciplina não é controle.** Nesta única sessão, e sendo eu quem impõe as regras:

- deixei roadmap em `wip` com trabalho mergeado **cinco vezes**
- criei branch paralela violando a regra de branch única que eu mesmo aplico
- commitei resíduo de PoC (`.agents/`) **duas vezes**, a segunda depois de já o ter apagado
- troquei de branch com agente vivo, fazendo arquivos caírem na branch errada

Nenhuma dessas apareceu no CI verde. **O produto existe precisamente porque disciplina não escala** —
e o repositório dele é a prova mais acessível disso.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Enumeração primeiro.** O que o `trackfw` instala em projeto de terceiro e este
      repositório **não** usa? Varra o que `init`, `discover`, `update harness` e `integrations`
      geram, e compare com o que existe aqui. **As três lacunas acima são o ponto de partida
      conhecido-incompleto** — nesta sessão duas enumerações minhas erraram por uma ordem de
      grandeza.
- [ ] **AC2** — `required_status_checks` configurado na `main`.
      🔴 **Quais checks são exigidos é decisão de desenho, não consequência automática.** Os jobs de
      Windows **nascem vermelhos por projeto** e são `continue-on-error`; torná-los exigidos travaria
      todo merge até os onze defeitos fecharem — **o oposto do que o instrumento existe para
      permitir**. A escolha precisa ser justificada por escrito.
- [ ] **AC3** — Guards ativos para humanos: `core.hooksPath` deixa de ser `/dev/null` e os hooks são
      instalados pelo próprio `trackfw`.
      🔴 **Controle obrigatório:** o guard **não pode** quebrar fluxo legítimo de quem não usa agente.
      Um `git commit` normal tem de continuar funcionando.
- [ ] **AC4** — 🔴 **Falsificação de cada controle, nas duas direções.** Para o portão: um PR com CI
      vermelho **não** merge; e o controle — um PR verde **merge**. Para os guards: comando
      destrutivo é bloqueado; e comando legítimo **passa**. **Sem a segunda metade, trocamos ausência
      de controle por controle que trava o trabalho.**
- [ ] **AC5** — `enforce_admins` avaliado explicitamente. Ficar `false` é **decisão registrada**, não
      omissão — há argumento legítimo para manter escotilha de emergência num projeto com um
      mantenedor.
- [ ] **AC6** — 🔴 **O `trackfw doctor` passa a acusar estas lacunas.** É o que transforma o achado em
      **produto**: qualquer projeto que adote o trackfw ganha o mesmo diagnóstico. **Esta AC vale mais
      que as outras** — as demais consertam um repositório; esta conserta todos.
- [ ] **AC7** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** publicar o `CONTRIBUTING.md` aqui — tem REQ própria
  (`REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-...`), que ficou **pausada** e volta
  depois desta. A ordem importa: **publicar regra antes de ter o portão seria repetir o erro** de
  contrato sem gate.
- **Não** aplicar retroativamente ao que já foi mergeado.
- **Não** tornar exigidos os jobs de Windows enquanto nascerem vermelhos por projeto.

## Linked ADR

ADR: `docs/adr/ADR-2026-09-01-o-repositorio-do-trackfw-e-governado-pelo-trackfw-com-o-mesmo-rigor-que-o-produto-vende.md`

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-01-o-repositorio-do-trackfw-sob-os-cuidados-do-trackfw.md`
