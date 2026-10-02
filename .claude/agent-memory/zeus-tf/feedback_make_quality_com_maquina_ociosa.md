---
name: make-quality-com-maquina-ociosa
description: Antes de rodar make quality local, conferir que não há go test/chunk órfão de agente travado; as 3 vezes que "pendurou" foram com atividade concorrente
metadata:
  type: feedback
---

Antes de rodar `make quality` (ou o driver paralelo de falsificação), confira com `ps` que não há `go test`, `chunk_*.sh` ou `make` vivos de agente que travou no watchdog ou de execução morta por limite. Medido em 2026-10-02 (#504): com a máquina ociosa, 5 de 5 execuções fecharam (driver ~3 min; `make quality` 9 min 23 s). As 3 que "penduraram" rodaram com atividade concorrente, e em nenhuma o `ps` foi capturado.

**Why:** gastei cerca de 3h tratando como defeito do driver o que não reproduzia isolado. E "chunk com 3 linhas de log por 6 min" não é travamento: o chunk só escreve quando um cenário fecha. A árvore de processos mostrou que ele avançava.

**How to apply:** se travar, capture **na hora**, antes de matar, os processos do chunk **por grupo e por marcador**, não por parentesco: `ps -axo pid,ppid,pgid,stat,etime,%cpu,command`, filtrando pelo `pgid` do chunk e por um marcador do argv (caminho do `chunk_N.sh`, diretório de trabalho). 🔴 **Não desça pelos `ppid`.** A Wave 0 da #504 mediu que o processo cujo pai intermediário saiu é reparentado ao PID 1 e some da árvore, e esse é justamente o caso da hipótese (agente morto no watchdog). Nenhum método isolado cobre tudo: o grupo não pega `setsid()`, o parentesco não pega o órfão. Para diagnosticar, o marcador é o único que não depende da topologia que o travamento já destruiu. Apontado pelo Lourival no PR #506. Laço de contorno sempre em `bash -c`, nunca em zsh (ver [[o-instrumento-mente]]).
