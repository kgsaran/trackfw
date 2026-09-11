# Por que o consumidor externo acha e nós não

> 2026-09-11 · pergunta do usuário: *"por que o Lourival está achando tantos erros e nós não? O que
> está errado no nosso ciclo? Precisamos colocar o agente de QA na jogada?"*

Medi antes de opinar. **A resposta não é "falta QA".** É mais específica — e a parte final é
estrutural, não corrigível com mais um agente.

---

## 1. O que a medição mostra

### Nosso CI

```
18 jobs  ubuntu-latest
 6 jobs  windows-latest
```

### Onde os defeitos dele moravam

| issue | ambiente do defeito | nós cobrimos? |
|---|---|---|
| **#314** | console `cp1252` (Windows pt-BR) | 🔴 **não** no caminho do self-test |
| **#315** | Windows **sem privilégio de symlink** | 🔴 **não** — o runner tem Developer Mode |
| **#320** | projeto `by_agent` com **2 agentes** | 🔴 **não** — zero testes com 2+ agentes |
| **#319** | um job **VERDE** com 10 anotações de erro | 🔴 **não olhamos o que está verde** |
| **#309** | gate com mutação aplicada | parcial — falsificamos gate **novo**, não o antigo |

🔴 **Em nenhum dos cinco o defeito estava num caminho que a gente exercita.**

---

## 2. As quatro diferenças de postura — medidas, não supostas

### 2.1 Ele **usa**; nós **testamos**

`#320` saiu de `req new` num projeto descartável com `agents: [alpha, beta]`. **Temos zero testes com
dois agentes** — derivado, não estimado.

Nossos testes exercitam **caminho de código**. Ele exercita **uso**. Um projeto com dois agentes é
configuração trivial e nunca foi testada porque ninguém *usou* assim.

### 2.2 O ambiente dele é hostil por padrão; o nosso é curado

```
ele      Windows 11 pt-BR · console cp1252 · sem WSL · sem Developer Mode · git fora de /usr/bin
runner   Windows en-US · UTF-8 · Developer Mode ligado · git padrão
nós      macOS, e 18 de 24 jobs em ubuntu
```

**Testamos na máquina que funciona.** O runner do GitHub tem Developer Mode — então `os.Symlink`
sempre funciona lá, e o `t.Fatal` do `#315` nunca dispara no nosso CI.

### 2.3 🔴 Ele olha o que está **verde**

O `#319` é um job que **passou**, com 10 anotações de erro.

Nossa disciplina inteira é *"deixar verde"*. Uma vez verde, paramos de olhar. **Ele leu as anotações
de um job aprovado** — e achou fixtures de self-test sendo reportadas como erro real.

Isso não é ferramenta que falta. É **hábito que não temos**.

### 2.4 Ele **muta o verificador**; nós falsificamos o gate **novo**

No `#309` ele trocou `newline="\n"` por `"\r\n"` e checou se o gate acusava. Não acusou — o gate
verificava **presença** do argumento, não o valor.

Nós exigimos falsificação nas duas direções **ao criar** um gate. **Não voltamos a mutar gates
antigos.** Um gate correto no dia 1 pode virar vacuoso no dia 30 por mudança adjacente, e nada
percebe.

---

## 3. E a diferença que nenhum agente resolve

**Ele não sabe o que o código deveria fazer.**

Nós lemos **intenção**; ele lê **texto**. Quando eu audito um handoff que eu mesmo escrevi, carrego a
premissa que gerou o defeito. Foi assim hoje, três vezes:

- rodei `check-serve-browser-security.sh` à mão, vi 21/21, aprovei — **verifiquei que o gate funciona,
  não que alguém o executa**;
- reproduzi o ataque do `barrier` nas duas direções e aprovei — o `hades-tf` achou que **a prova
  cobria um buffer e a execução usava outro**;
- contei 27 REQs órfãs pelo frontmatter — o produto lê o corpo e vê **57**.

🔴 **Nos três, o erro não foi falta de teste. Foi eu verificar a coisa que eu já acreditava.**

Qualquer agente interno herda o meu enquadramento **pelo handoff que eu escrevo**. É limite
estrutural, não de esforço.

**Mas há precedente de que dá para mitigar:** o `hades-tf` funciona quando recebe a instrução de
**reimplementar o raciocínio a partir da leitura**, em vez de conferir o diff contra o relatório. Foi
assim que ele achou o F1 do `barrier` e bloqueou o `serve` com o vetor de zone ID. **A independência
vem da instrução, não do papel.**

---

## 4. O que eu proponho

### 4.1 Três lacunas de CI — permanentes, sem agente

| lacuna | job | pega |
|---|---|---|
| console **cp1252** | passo Windows com codepage 1252 | `#314` e a família inteira |
| Windows **sem Developer Mode** | job com symlink desprivilegiado | `#315`, `#279` |
| **consumidor novo** | `init` em projeto descartável, `by_agent` com 2 agentes, roda os comandos | `#320` |

🔴 **Estes três são gates, não revisões.** Uma vez ligados, pegam para sempre — e não dependem de
ninguém lembrar.

### 4.2 Duas posturas a institucionalizar

**Ler o verde.** Job aprovado com anotação de erro é defeito. Cabe em gate: *job `success` com
anotação `failure` reprova*.

**Re-mutar gate antigo.** Uma rodada periódica de mutação nos gates existentes — o que o `#309` fez
com o `check-python-writes-lf`. Pode ser alvo de `make`, amarrado ao release, como fizemos com o
`check-required-full`.

### 4.3 O agente de QA — **sim, mas não para escrever teste**

🔴 **Um agente que faça o que já fazemos não acrescenta nada.** Mais testes, nos mesmos ambientes,
com as mesmas premissas, produzem mais verde.

O papel útil é **consumidor adversarial**, com carta explícita:

- **roda o artefato**, não lê o diff;
- em **ambiente hostil declarado** (encoding, privilégio, layout de caminho);
- como **projeto novo**, não como o repositório;
- **muta** o que já passa, para ver se o verificador acusa;
- e 🔴 **recebe o mínimo de contexto nosso** — porque o contexto é justamente o que transmite a
  premissa errada.

O modelo é o do `hades-tf`: reimplementar a partir da leitura. **A instrução é o que cria a
independência**, e ela precisa estar na carta do papel, não no meu handoff do dia.

---

## 5. A conclusão desconfortável

**O nosso ciclo não é fraco em teste. É fraco em três coisas:**

```
ambiente     testamos onde funciona
uso          testamos código, não uso
independência quem audita já sabe o que esperava encontrar
```

E há um dado que resume: hoje ele e eu achamos **o mesmo defeito** (`#322`) com minutos de diferença.
Eu cheguei lá **puxando o fio de um aviso lateral** — não por processo. **Sorte não é método.**
