# Triagem — os issues que o nosso próprio ferramental gerou

> 2026-09-11 · pedida pelo usuário depois de constatar que **fechamos 2 issues e chegaram 5**.

## O balanço que motivou a triagem

```
fechados hoje    2   (#274, #275 — o ratchet)
chegaram         5
saldo           +3   a fila CRESCEU
```

🔴 **Quatro dos cinco são sobre código que nós entregamos nas últimas 48 horas.** Não estamos
parados — estamos **gerando defeito quase na velocidade em que corrigimos**, e num lugar pior que a
média: o **ferramental de governança**. Ferramental quebrado não protege o resto, e estamos
construindo em cima dele.

O **#322** já fechou: chegamos ao mesmo achado em paralelo (ele às 18:35, eu puxando o fio de um
aviso lateral do `trackfw push`).

---

## Os quatro restantes, por causa

### Grupo A — `check-windows-known-failures.py`: a saída do self-test vaza para o canal do gate real

**#314** — o `--self-test` morre com `UnicodeEncodeError` em `cp1252`, no T15, ao imprimir `→`.
Entrada tratada em 10 sítios, **saída em nenhum**.

**#319** — os casos sintéticos imprimem `::error::`, o runner do GitHub os converte em anotação, e o
job `parity-other-gates` aparece **verde com 10 anotações de erro**. `badpkg` e `TestFoo` são
**fixtures**.

🔴 **Hipótese de causa comum, a medir antes de agrupar:** o self-test escreve **no mesmo canal** que
o gate de produção. Por isso herda o encoding do console (#314) e é interpretado pelo runner (#319).

Se a causa for essa, corrigir o canal fecha os dois — e é o teste de agrupamento do projeto:
*"se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma outra."*

⚠️ **Se a medição mostrar causas distintas** (encoding vs marcador de anotação), **separam-se** — mas
só com a medição escrita. O erro do grupo do `IsAbs` nesta campanha foi agrupar por sintoma parecido.

**Destino:** REQ própria — é ferramental novo, sem REQ que o governe.

### Grupo B — `#315`: a classe do `t.Fatal` mascarando privilégio, num 5º arquivo

```
internal/discover/discover_test.go   2 sítios
os.Symlink → "A required privilege is not held" → t.Fatal → "teste reprovou"
```

**Dois estados, um observável** — a família do dia inteiro.

🔴 **Não é REQ nova.** O issue **#279** cobria *"9 t.Skip de classe plataforma em 4 arquivos"* e está
**CLOSED**. A REQ que o governa é
`REQ-2026-09-05-tres-defeitos-mecanicos-medidos-por-consumidor-externo-skips-residuais-...`.

**A REQ foi fechada com a classe não varrida** — o 5º arquivo apareceu depois. É o achado **A1** de
novo: fechar afirmando que a classe está resolvida quando sobrou sítio.

**Destino:** **reabrir** aquela REQ e acrescentar ML, pela Regra Dura de Causa Raiz. E registrar
**por que a varredura original não o pegou** — provavelmente derivou por `t.Skip` e não por `t.Fatal`.

### Grupo C — `#320`: `by_agent` sempre cria no primeiro agente

```
                                    Go      Node    Python
req new "x"                         alpha/  alpha/  alpha/
roadmap new --req .../beta/REQ-….md alpha/  alpha/  alpha/   ← ignora o agente da REQ
req new --agent beta                erro    erro    erro
roadmap new --agent beta            erro    erro    OK       ← só o Python tem a flag
```

**São duas coisas**, e o relator já as separou: a **lacuna** (sem flag, cai em `agents[0]`; o
`--req` não herda o agente da REQ) e a **quebra de paridade** (só o Python aceita `--agent`).

**Destino:** REQ própria. Causa distinta de tudo em curso, e toca o produto, não o ferramental.

---

## Encaminhamento recomendado

| # | destino | por quê |
|---|---|---|
| **#322** | ✅ fechado | resolvido pelo PR #323 |
| **#314 + #319** | **1 REQ** (hipótese de causa comum, a medir) | canal do self-test |
| **#315** | **reabrir** a REQ dos três defeitos mecânicos | mesma classe, 5º arquivo |
| **#320** | **REQ própria** | produto, causa distinta |

🔴 **Três issues viram uma REQ, não três.** É o inverso do padrão que produziu 59 ocorrências de "REQ
própria" espalhadas por 30 roadmaps.

## A observação que importa mais que a triagem

**O relator externo está achando os nossos defeitos em horas.** O #322 foi encontrado por ele e por
mim quase ao mesmo tempo; o #319 mediu um job nosso na `main` que **ninguém tinha olhado**.

Isso não é motivo para reduzir engajamento — é evidência de que a taxa de defeito do nosso
ferramental está alta. **A contramedida é diminuir a taxa, não a visibilidade.**

E a prioridade, se houver conflito: **ferramental antes de produto.** Um gate quebrado aprova o que
não devia, e nós estamos construindo governança em cima desses gates.
