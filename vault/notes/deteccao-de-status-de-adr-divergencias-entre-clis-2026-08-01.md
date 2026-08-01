# Detecção de status de ADR: três divergências entre CLIs que nenhum gate pega

> Data: 2026-08-01 | Autor: Zeus (auditoria) | Domínio: validator — paridade dos 3 CLIs

Registrado durante a implementação de `adr_accepted_when_req_done`
(`ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida`). Três agentes
implementaram a mesma regra em paralelo, um por CLI, e divergiram em três dimensões. **Nenhuma
das três seria detectada pelos gates existentes.**

## Por que os gates não pegam

`scripts/check-validate-parity.sh` compara apenas `(rule, file)` das violações — **não o texto da
mensagem**. E, pior: neste repositório ele passa **vacuamente**, porque não existe nenhum artefato
que viole a regra nova. Um gate que não tem fixture violadora não discrimina nada.

Isso é o padrão a lembrar: **gate verde sobre corpus sem caso positivo não é evidência.**

## Divergência 1 — fonte do status (frontmatter vs cabeçalho)

Go e Python leram o frontmatter `status:` primeiro, com fallback para a linha
`> Date: ... | Status: X`. Node leu **apenas** o cabeçalho.

Para um ADR bem formado os dois concordam, então quase nenhum teste distingue. O caso
discriminante é **ADR com frontmatter e sem linha de cabeçalho** — aí Node resolvia string vazia.

**Canônico decidido:** frontmatter-first, fallback de cabeçalho, comparação case-insensitive.
O frontmatter é o campo estruturado; o cabeçalho é prosa formatada. O fallback cobre ADRs legados
(ex.: `docs/adr/ADR-001-*.md`).

## Divergência 2 — falso-positivo por substring livre na prosa

O mecanismo herdado era `content.includes("Status: Draft")` **sobre o documento inteiro**.
Qualquer ADR que **cite** esse literal na prosa é classificado como não aceito.

Não é hipotético: o próprio
`docs/adr/ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito-...md` — status real `Accepted` — cita
`"Status: Draft"` e `"Status: Proposed"` no `## Context` ao descrever o bug que corrige. Sem a
correção, o ADR que documenta a regra seria flagrado pela própria regra.

**Correção:** extração **ancorada por linha**, nunca `Contains` sobre o corpo.
Detalhe em `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`.

## Divergência 3 — fallback de cabeçalho sem truncar no próximo pipe

Go e Node truncavam o valor do cabeçalho no próximo `" |"`. Python não.

Para um ADR legado com `| Status: Draft | Owner: kg`, Go/Node resolviam `Draft`; Python resolvia
`"Draft | Owner: kg"`, que não casa `draft`/`proposed` após `lower()` — **falso-negativo
silencioso**. Nenhuma fixture do repositório usava cabeçalho com pipe extra, então nenhum teste
pegava.

## Regra prática

Ao implementar a mesma lógica em paralelo nos 3 CLIs, **as divergências aparecem nos casos que
nenhuma fixture cobre**: campo ausente, campo vazio, formato legado, valor com separador extra.
Um ML de reconciliação após a wave paralela não é burocracia — foi ele que pegou as três.

E ao unificar mensagens: prove a igualdade **rodando os CLIs e diferenciando a saída**, não
comparando os literais no código. Templates diferentes (`%q` do Go vs interpolação com aspas do
Node) podem produzir a mesma saída, e literais iguais podem produzir saídas diferentes.

Relacionado: `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`.
