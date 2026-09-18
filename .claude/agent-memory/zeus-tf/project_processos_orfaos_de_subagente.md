---
name: processos-orfaos-de-subagente
description: Subagente que termina deixa processos vivos — loops until, cat em pipe não drenado — que mantêm o agente listado como "em execução" e podem commitar lixo
metadata:
  type: project
---

**Quando um subagente encerra, ele costuma deixar processos vivos cujo consumidor desapareceu.**
Eles seguram o agente na lista do CLI como se estivesse trabalhando.

**Why:** a notificação de tarefa só dispara quando o agente para **sem filhos de background vivos**.
Processo órfão = agente eternamente "em execução". Medido em 2026-09-18: 5 shells e 2 agentes
aparentemente ativos, **todos parados**. Ao matar os órfãos, o agente notificou "finished" na hora.

**Três classes observadas, todas no mesmo dia:**

| classe | forma | por que trava |
|---|---|---|
| polling eterno | `until grep -q "EXIT=" arq; do sleep 20; done` | o dono morreu antes de escrever a condição; o arquivo existe mas nunca recebe o marcador |
| polling por tamanho | `until [ $(wc -l < arq) -ge 2100 ]` | o arquivo parou em 316 linhas; ninguém mais escreve |
| **bloqueio em pipe** | `cat "$OUTPUT"` no fim de script | saída grande, consumidor sumiu, fica em `S` esperando escrever — **1h43min** num caso |

**How to apply — diagnóstico em duas linhas:**

```bash
pgrep -fl "until "                      # loops de polling
for p in $(pgrep -f "shell-snapshots"); do c=$(pgrep -P $p|head -1); \
  [ -n "$c" ] && echo "$p -> $(ps -o etime=,comm= -p $c)"; done
```

🔴 **O critério que distingue travado de trabalhando:** olhe o **filho** do shell, não a carga.
Filho `make`/`go`/`compile` = trabalhando. Filho `sleep` dentro de `until`, ou `cat` com muitos
minutos de `etime` = travado. **Carga alta não distingue** — os loops também queimam CPU fazendo
`wc -l` a cada 5s.

Limpeza: `pkill -f '<padrão>'`, ou `kill <pid>` do filho.

**Consequência que não é só cosmética:** um dos loços esperava
`internal/roadmapdoc/testdata/barrier-recapture.txt`, arquivo temporário de recaptura que um
`git add -A` de subagente **commitou** truncado (316 de ~2180 linhas), sem nenhum teste que o
referenciasse. `testdata/` é onde lixo passa despercebido — ninguém estranha um `.txt` grande ali.

**Prevenção, a pôr no handoff:** arquivo intermediário vai em `/tmp`, nunca sob `internal/` ou
`scripts/`; nada de loops de polling em background; e o relatório lista todo arquivo criado,
marcando entregável vs temporário.

Ver [[o-instrumento-mente]] e [[feedback_agente_nao_roda_em_background]].
