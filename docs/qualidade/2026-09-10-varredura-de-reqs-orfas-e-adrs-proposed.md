# Varredura de REQs órfãs e ADRs `Proposed` — 2026-09-10

> Pedida pelo usuário depois de duas descobertas no mesmo dia: o roadmap do ratchet afirmava
> `Accepted` sobre uma ADR `Proposed`, e a REQ dele estava órfã apesar de o roadmap apontar para ela.
> **Vínculo de mão única.**

## O número

```
REQs no acervo                        205
  com roadmap: ""                      44
  🔴 dessas, Open ou In Progress        31
  as outras 13 estão Done — sem efeito

ADRs                                   65
  Accepted                             61
  Superseded                            2
  🔴 Proposed                            2
```

## As duas ADRs `Proposed`

```
ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-...
ADR-2026-09-05-staging-com-escopo-implicito-e-bloqueado-porque-ninguem-audita-...
```

**Nenhuma das duas é referenciada por roadmap nenhum** (`grep -rl` em `docs/roadmaps/`). Ou seja:
não estão bloqueando trabalho hoje — são **decisões arquiteturais tomadas pela metade**, esperando
aceite ou rejeição.

🔴 A terceira ADR `Proposed` era a do ratchet, e **estava** sendo referenciada — por um roadmap que a
declarava `Accepted`. Foi aceita hoje, depois de lida. É o caso perigoso: não a ADR parada, mas a ADR
parada **com trabalho apoiado nela**.

## O achado que responde à pergunta do usuário

> *"minha preocupação são os issues mais antigos"*

Os issues antigos **não estão parados por falta de análise.** Estão parados por **artefato pela
metade** — a REQ existe, o roadmap não.

| issue | aberto em | REQ | estado |
|---|---|---|---|
| **#258** | 03/09 | `REQ-2026-09-05-gate-de-palavra-chave-de-fechamento-...` | 🔴 órfã |
| **#261** | 03/09 | `REQ-2026-09-05-validate-unfiltered-do-python-...` | 🔴 órfã |
| **#268** | 04/09 | `REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-...` | 🔴 órfã (desde 30/08) |
| **#273** | 05/09 | `REQ-2026-08-20-branch-has-wip-roadmap-casa-por-substring-...` | 🔴 órfã (desde 20/08) |

🔴 **Quatro dos cinco issues mais antigos têm REQ escrita e nenhum roadmap.** A análise foi paga e
está guardada num artefato que nada consome.

**Ressalva de método:** o cruzamento acima foi feito por casamento de termos entre título de issue e
nome de REQ, com corte em 3 termos. O **#273 escapou** do corte automático e foi achado à mão — então
**4 é piso, não total.** Um cruzamento por conteúdo, não por nome, provavelmente acha mais.

## Por que isto acontece, e não é desleixo

`trackfw req new` e `trackfw roadmap new` são **dois comandos**, e o segundo se esquece. Abrir a REQ
dá a sensação de ter encerrado o assunto — o artefato existe, o registro foi feito, o assunto "está
tratado". **Registro não é correção**, e é a mesma frase que a Regra Dura de Causa Raiz usa para
59 ocorrências de "REQ própria" espalhadas por 30 roadmaps.

Já existe roadmap em backlog para isso:
`ROADMAP-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md`

🔴 **E ele mesmo está em `backlog/`** — o roadmap que corrige a criação de roadmaps órfãos não foi
despachado. A ironia é útil: mostra que o problema é estrutural, não de disciplina individual.

## Encaminhamento sugerido

1. **Vincular as 4 REQs acima a roadmaps** — é o passo que transforma issue antigo em trabalho
   despachável. Barato: a análise já existe.
2. **Decidir as 2 ADRs `Proposed`** — aceitar ou rejeitar. Nenhuma bloqueia hoje, mas ADR `Proposed`
   que sobrevive vira precedente de que o estado é aceitável.
3. **Despachar o roadmap do `req-nasce-orfa`** — é a correção na origem. Sem ele, esta varredura vira
   rotina.
4. 🔴 **Considerar regra de `validate`**: REQ `Open` com `roadmap: ""` por mais de N dias é aviso.
   Hoje o `validate` avisa sobre o vínculo ausente, mas não sobre a **idade** — e é a idade que
   distingue "acabei de abrir" de "esqueci em agosto".

## Comandos usados (reprodutível)

```bash
# REQs órfãs e abertas
for f in docs/req/*.md; do
  grep -q '^roadmap: ""' "$f" && grep -qE '^status: (Open|In Progress)' "$f" && echo "$f"
done | wc -l

# ADRs Proposed
grep -rl '^status: Proposed' docs/adr/

# ADR Proposed referenciada por algum roadmap?
for a in $(grep -rl '^status: Proposed' docs/adr/ | sed 's|.*/||'); do
  echo "$(grep -rl "$a" docs/roadmaps/ | wc -l) → $a"
done
```
