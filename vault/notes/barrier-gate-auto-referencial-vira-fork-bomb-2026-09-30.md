# Gate de wave que invoca `trackfw barrier` vira fork bomb

> Domínio: barrier/governança · Data: 2026-09-30 · Severidade: **alta** (DoS local)

## O que aconteceu

Escrevi, no `**Gates da wave:**` da Wave 0 de um roadmap:

```bash
trackfw barrier <este-mesmo-roadmap> --wave 0 --trust-local-gates
```

O `barrier` **executa** os comandos do bloco de gates da wave. O comando era o próprio `barrier`
sobre a mesma wave. **Não há detecção de auto-referência e não há limite de profundidade.**

Medido:

```
~6 min de execução → 3469 processos `trackfw` vivos
cadeia de PPID estritamente linear: 59931 → 59932 → 59933 → ...
load average 12.91 (normal ~4)
```

## Como conter (a parte que custa tempo se você não souber)

🔴 **`pkill -f 'bin/trackfw barrier'` NÃO funciona.** Os processos filhos têm `comm=trackfw` e a
linha de comando **sem** o prefixo `bin/` — o padrão não casa. Um passe reportou "restam 0" e
**3469 continuavam vivos**.

O que funciona:

```bash
pkill -9 -x trackfw            # -x: nome EXATO do executável
ps -eo comm | grep -c 'trackfw$'   # a contagem correta
```

⚠️ **E não meça com `pgrep -f trackfw`**: ele casa qualquer processo cuja linha de comando contenha
o path do repositório, inclusive o seu próprio shell. Aqui ele deu 3472 quando havia 3469 reais —
por sorte o erro foi pequeno, mas o método é inválido. Meça por `comm`, não por linha de comando.

## A regra operável

🔴 **O gate de uma wave nunca pode ser o executor que roda o gate.** O bloco `**Gates da wave:**`
recebe um comando de **verificação** — algo que mede um fato e sai 0 ou 1. `trackfw barrier`,
`trackfw validate --wave`, ou qualquer coisa que reentre no avaliador, não.

Molde que funciona (do `ROADMAP-2026-09-29` do #364):

```bash
n=$(jq -r '.entries[]|.name' .github/windows-known-failures.json | wc -l | tr -d ' ')
test "$n" = "14" && echo "Gate W0: $n entradas" || { echo "GATE FALHOU: esperava 14, contou $n" >&2; exit 1; }
```

## População no acervo (medida em 2026-09-30)

**1** sítio armado, e era meu: `ROADMAP-2026-09-22-init-e-discover-...` (em `done/`, na `main`).
Desarmado na mesma data. Os outros 5 candidatos que um `grep` de 300 caracteres acusou eram **falso
positivo** — o regex atravessava para o ML seguinte, ou casava `$ trackfw barrier <roadmap>` (prosa
de exemplo, com `$` e placeholder).

**Como medir certo:** extraia o **primeiro bloco cercado após o marcador** `**Gates da wave:**` e
exija que a linha **comece** com o comando (`^(trackfw|\./bin/trackfw)\s+barrier\b`). `grep` de
janela fixa não serve — mede a vizinhança, não o bloco.

## O defeito de produto, que é separado do meu erro

Escrever o gate errado foi meu. **Mas o produto deveria recusar**, e não recursa apenas: ele
*multiplica*. Um roadmap é entrada não confiável em qualquer repositório clonado — isto é DoS local
trivialmente acionável por um arquivo de texto. Issue própria.

Candidatos de defesa: profundidade máxima via variável de ambiente herdada · detectar o próprio
binário no comando do gate · recusar gate que contenha o subcomando `barrier`.

## Relacionado

- `REQ-2026-09-30-cerca-nao-terminada-...` (onde o erro apareceu)
- [[processos-orfaos-de-subagente]] — 4ª classe de órfão; esta é a 5ª, e a pior
