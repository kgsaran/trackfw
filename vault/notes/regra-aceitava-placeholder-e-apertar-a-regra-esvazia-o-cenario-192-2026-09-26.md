# `req_has_roadmap` aceitava placeholder — e apertá-la ESVAZIA o Cenário 192

> Criado em: 2026-09-26 · Autor: apolo-tf · REQ-2026-09-09 / ML-1D

## O defeito de origem (comentário × código)

`internal/validator/validator.go` declarava, em comentário, que `req_has_roadmap` detectava o vínculo
por `extractRefPath` (frontmatter-first, case-insensitive, **exige `.md`**). O código usava
`extractFrontmatterField`, que aceita **qualquer valor não-vazio** e é **case-sensitive**
(`HasPrefix(field+":")`). Duas claims falsas no mesmo comentário — fonte de verdade e caixa.

Consequência medida: `roadmap: none` satisfazia a regra. E o corpus tinha o caso-bandeira:
`REQ-2026-08-16` passava com `Roadmap: (a criar quando esta REQ sair do backlog…)` — prosa que diz
literalmente que o roadmap **não existe**.

🔴 **O argumento decisivo não foi a medição do validador, foi o contrato do gerador.**
`docs/cli-parity.md` (§`roadmap new` writes the REQ backlink) já declarava que o campo `roadmap:` é
sobrescrito quando o valor é *"anything that is **not** a `.md` reference (`""`, `none`, `-`,
`<!-- … -->`)"*. Ou seja: o **gerador** já classificava `none` como placeholder a preencher enquanto o
**validador** classificava a mesma REQ como vinculada. Não era escolha de política — era divergência
interna do mesmo contrato.

## Medição antes/depois (231 REQs, binário de HEAD × binário corrigido)

| | |
|---|---|
| censo do frontmatter | 212 com `.md` · 17 com `roadmap: ""` · 2 sem o campo · **0 com `none`/não-`.md`** |
| acusadas antes | 12 |
| acusadas depois | **13** |
| perdidas (vínculo legítimo que passou a disparar) | **0** |

Por que o aperto tem raio ~zero no frontmatter: `extractFrontmatterField` faz `Trim(val, "\"'")`, então
`roadmap: ""` já contava como vazio antes. **A população em risco eram as 19 REQs de frontmatter vazio
menos as 12 já acusadas = 7 que passavam só pelo corpo**; dessas, 4 têm caminho simples, 2 têm caminho
entre backticks (que `extractRefPath` remove) e **1** era a prosa. A conta de subtração é o teste, não
a contagem total.

## 🔴 A armadilha de instrumento: apertar a regra torna o Cenário 192-B VÁCUO

`check-gates-falsify.sh`, Cenário 192, provava o achado A2 ("prosa no meio da linha não é vínculo")
sabotando a ancoragem de `contentHasMarkerValue` (`HasPrefix(leading, marker)` → `Contains`) e exigindo
que a violação **`has no linked Roadmap` desaparecesse**. Depois do ML-1D, `req_has_roadmap` não usa
mais `contentHasMarkerValue`: a sabotagem deixa de mudar o veredito, a violação **sobrevive**, e
`assert_lacks_pattern` reprova — *sem que nada esteja quebrado*.

E não há composição que salve: com o `.md` exigido, a prosa é recusada por **duas razões
independentes** (não está ancorada **e** não termina em `.md`), então neutralizar só a ancoragem nunca
vira verde. As duas saídas:

1. **Direção B migrou para o campo ADR** (`req_has_adr` é consumidor **vivo** de
   `contentHasMarkerValue`). A prosa tem de citar o **caminho do ADR**, não só a palavra `ADR:` —
   `adr_orphan` casa por `strings.Contains` do basename em todas as REQs, e sem o caminho no texto o
   ADR do projeto fica órfão e reprova **os dois braços**.
2. **Direção C, nova**, mede a mesma propriedade no leitor novo: prosa **carregando um `.md`** +
   sabotagem da ancoragem da chave em `extractRefPath`.

Regra geral: **migrar uma regra de leitor A para leitor B esvazia todo cenário de falsificação que
sabotava A por conta daquela regra.** Procure por mensagem (`S192_MSG_*`), não pelo nome da função.

## Três detalhes que custam tempo

1. **A sabotagem de `EqualFold` não compila como substituição.** Trocar
   `if ok && strings.EqualFold(strings.TrimSpace(key), field) {` por uma condição sem `key` dá
   `declared and not used: key`. A forma que compila é a **disjunção** (`EqualFold(...) || Contains(...)`).
2. 🔴 **O alvo de vínculo das fixtures NÃO pode morar em `docs/roadmaps/backlog/`.** Os scripts de
   ciclo dos Cenários 24/25/26 fazem `basename "$(find docs/roadmaps/backlog -name "*.md")"` — um
   segundo arquivo ali faz o `find` devolver duas linhas e o `roadmap move` recebe nome corrompido.
   Medido (8 células, REQ Open × REQ Done): `wip/` dispara `wip_wave0` + `wip_has_req` +
   `wip_acceptance`; `done/` dispara `req_roadmap_lifecycle` para REQ Open; **`abandoned/` é limpo nos
   dois** e não é destino de comando nenhum do harness. Daí `ensure_roadmap_link_target`.
3. **`ref_targets_exist` está em `lenientCarveoutRules`** — é violação mesmo em `lenient`. Então trocar
   um placeholder por um caminho `.md` **inexistente** só troca um defeito por outro; o alvo tem de
   existir no disco.

## Sítio de mesma CAUSA fechado na mesma REQ

O lado **frontmatter** de `req_roadmap_sync` também ia por `extractFrontmatterField`. Deixá-lo faria as
duas regras discordarem sobre o que é valor: `req_has_roadmap` diria *"sem vínculo"* e
`req_roadmap_sync` diria *"vínculos conflitantes"* para a MESMA REQ (`roadmap: "none"` + caminho real no
corpo → `frontmatter="none" body="x.md"`, uma divergência que não existe). Corrigido no mesmo ML, com
**0 ocorrências nos 231 REQs** — a saída de `validate` ficou byte-idêntica.

🔴 **O predicado de valor NÃO foi extraído para um helper.** O Cenário 28 fixa por literal a linha
`v := strings.Trim(fields[0], "\"'`")` **dentro** de `extractRefPath`; refatorar o corpo da função
mataria a sabotagem. A delegação é feita remontando a linha do campo
(`extractRefPath("roadmap: "+fmVal, "roadmap")`) — feio, e a razão está escrita no sítio.

## Sítio de mesma classe deixado de fora, com razão escrita

`req_has_adr`, `wip_has_req` e `blocked_has_req` leem **só o corpo** (`contentHasMarkerValue`), então
`adr:`/`req:` de frontmatter são invisíveis para eles — assimetria real, mas **nenhum deles carrega
comentário afirmando outra fonte de verdade**, logo não é a classe do ML-1D. `adr:` é escopo do
**ML-1E** da mesma REQ. `note_orphan`, `req_roadmap_sync` e `resolveAdrStatus` foram conferidos: os
comentários descrevem o código.
