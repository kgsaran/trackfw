# O GitHub ignora palavra-chave de fechamento em zona de CÓDIGO e a honra em zona NÃO-CÓDIGO

> **Data:** 2026-09-24 · **Autor:** Hades (revisor de segurança) · **ML:** `ML-0C` da Wave 0 do
> `ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`
> **Registro completo, com JSON bruto:** `docs/seguranca/2026-09-25-discriminante-do-gate-de-palavra-chave.md` §10

## O que custa >10 min para quem não ler isto

A pergunta *"o GitHub fecha uma issue se a palavra-chave inglesa estiver dentro de bloco de código?"*
**não tem resposta uniforme.** Ela depende da zona, e as zonas se dividem em **dois baldes opostos**. Quem
presumir uniformidade — em qualquer das duas direções — erra metade das zonas. Foi exatamente o que aconteceu
no `ML-0B` deste mesmo roadmap, por duas vezes e em direções contrárias.

## Medição

Instrumento: `gh pr view <n> --json closingIssuesReferences`. **7 PRs sonda** rascunho, base `main`, corpo com
a keyword inglesa **exclusivamente** na zona sob teste (nunca no título, nunca em prosa, sem segundo `#N`),
contra 2 issues descartáveis. Criados e destruídos **pela API** (`gh api` refs + Contents, `gh pr create`,
`gh pr close --delete-branch`) — sem `git` de escrita, porque a autoridade Git é do arquiteto e artefato de
medição descartável não entra no histórico da REQ.

| zona | sonda | `closingIssuesReferences` | o GitHub fecha? | balde |
|---|---|---|---|---|
| prosa (**controle**) | #427 | `[426]` | **sim** | — |
| cerca ```` ``` ```` | #428 | `[]` | **não** | **CÓDIGO** |
| code span `` ` `` | #429 | `[]` | **não** | **CÓDIGO** |
| bloco indentado por 4 espaços | #431 | `[]` | **não** | **CÓDIGO** |
| blockquote `>` | #432 | `[430]` | **sim** | **NÃO-CÓDIGO** |
| célula de tabela | #433 | `[430]` | **sim** | **NÃO-CÓDIGO** |
| aspas retas `"…"` | #434 | `[430]` | **sim** | **NÃO-CÓDIGO** |

Artefatos: issues **#426**/**#430**, PRs **#427**–**#429** e **#431**–**#434** — todos `closed`, branches
`probe/*` deletadas, **nenhuma sonda mergeada**, 13 runs de CI cancelados. Cada valor lido **duas vezes**
(exclusão de consistência eventual): idênticos.

🔴 **O braço de controle é o que torna a medição interpretável.** Sem ele, `[]` nos braços é indistinguível de
instrumento quebrado, e um resultado nulo com controle nulo seria lido como a resposta. Esta é a mesma armadilha
das medições de negativo desta casa: **nunca publique um `[]` sem o braço que devolve não-vazio.**

## Consequência para `scripts/check-pr-closing-keyword.sh`

O gate mascara cerca e code span (`blank_code`) **uma vez** e alimenta com o resultado **os dois** matchers —
o da acusação portuguesa e o da isenção inglesa. **Isso está correto**, e o cabeçalho do gate está justificado
para essas zonas: `Fecha #246.` em prosa com `Closes #246` só dentro de cerca **não fecha a issue**, logo
acusar é verdade.

🔴 **O desenho do fix é POR BALDE, não por zona:**

- zona de **CÓDIGO** (cerca, span, indentado) → máscara **única**, subtraída dos **dois** matchers;
- zona **NÃO-CÓDIGO** (blockquote, tabela, aspas) → máscara em **passe próprio**, subtraída **só** do matcher
  português (senão o gate acusa um corpo que **fecha de verdade** — falso positivo).

Aplicar o passe duplo às zonas de código instala **falso negativo**: silêncio sobre um corpo que declara
fechamento e não fecha nada. É a direção oposta do defeito, e é pior que ele — o incômodo de hoje é um aviso
verdadeiro.

## Duas presunções refutadas, em direções contrárias, no MESMO parecer anterior

| presunção do `ML-0B` | medido |
|---|---|
| *"a zona de código apaga a isenção inglesa"* = **falso positivo do gate** (forma 8, `esp`=0) | **não é defeito**: `esp`=1, o gate acerta. 8 famílias → **7 defeitos + 1 acerto** |
| *"o GitHub fecha a #246 nos cinco casos"* (§3.5-iv, "5 de 5") | **3 de 5**. Indentado e cerca **não** fecham; aspas/blockquote/tabela fecham |

A lição transferível: **uma premissa sobre comportamento de sistema externo que decide a DIREÇÃO de um fix não
pode ficar como premissa.** Aqui ela invertia o veredito, e o corpus de 357 PRs reais **não a decidia** (zero
PRs cuja única keyword inglesa vivesse em zona de código). Uma sonda descartável de 20 minutos resolveu o que
nenhuma quantidade de corpus resolveria.

## Limite declarado — não extrapole o balde por analogia

**Não medidas:** `<!-- comentário HTML -->`, `<pre>`/`<code>` em HTML bruto, aspas **curvas** (`“…”` — medi só
as retas), cerca com `~~~`, cerca com atributo de linguagem (```` ```bash ````), `<details>`, item de lista.
🔴 **A analogia é exatamente o mecanismo que produziu as duas presunções refutadas acima.** Zona nova na
gramática do gate exige **sonda própria**.

Condições da medição: PR **rascunho**, base `main`, mesmo repositório, autor = dono, leitura com o PR
**aberto**. Não medi fork, nem base não-default, nem o merge real.
