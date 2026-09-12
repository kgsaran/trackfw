---
status: Open
date: 2026-09-12
author: ""
adr: ""
roadmap: ""
---

# REQ: byte NUL literal no fonte torna o arquivo invisível à busca, e já produziu duas REQs com evidência falsa

> Date: 2026-09-12 | Status: Open

## Motivação

Dois arquivos-fonte do CLI Node contêm **bytes NUL literais**:

```
npm/src/validator/index.js       1 byte NUL   offset 93041 de 190181
npm/src/integrations/doctor.js   2 bytes NUL
```

O uso é **legítimo** — NUL como separador de chave composta:

```js
const seenKey = `${m.raw}<NUL>${m.typeIsCommand}`              // validator/index.js
const ka = [a.destination, ...].join('<NUL>')                  // integrations/doctor.js
```

O problema não é o NUL em runtime. É ele estar **literal no arquivo-fonte**, o que faz toda
ferramenta de busca com heurística de binário **pular o arquivo em silêncio**.

🔴 **`npm/src/validator/index.js` é o maior fonte do CLI Node** e concentra as regras do `validate`.
É exatamente onde uma busca precisa olhar primeiro.

## O custo — medido, não estimado

| quando | o que aconteceu |
|---|---|
| 2026-08-29 | nota de vault; sweep de `by_agent` fechou em 10 arquivos **sem** o `validator/index.js` |
| 2026-09-05 | a armadilha reaparece **dentro de uma triagem criada para limpar o backlog** |
| 2026-09-06 | segunda nota; já registrava que **duas REQs nasceram com premissa falsa** |
| 2026-09-12 | o arquiteto bate nela de novo, investiga do zero e escreve a **terceira** nota |

**Duas REQs abertas com evidência falsa**, ambas afirmando "regra ausente no CLI Node":

```
REQ-2026-08-20  note_orphan                        → estava lá, 3 sítios
REQ-2026-09-01  thirdparty_artifact_has_provenance → estava lá, 7 sítios
```

Em 2026-09-12 isso quase produziu o **despacho de um agente para reimplementar código existente**.

## 🔴 Por que três notas de vault não resolveram

Porque nota é **registro**, e registro depende de alguém lembrar de ler antes de digitar `grep`.
Quatro ocorrências, três notas, zero controles.

Esta REQ existe para trocar registro por **controle**.

## As ferramentas discordam, e é isso que mantém a armadilha viva

```
file npm/src/validator/index.js    →  "Unicode text, UTF-8 text"   ← diz que é texto
grep sem -a                        →  pula, em SILÊNCIO, RC=1
grep -a / /usr/bin/grep / git grep →  encontra
tr -d -c '\000' < arquivo | wc -c  →  1                            ← única medição confiável
```

Quem verifica o alerta com `file` conclui que é folclore. E `grep -c $'\x00' <arquivo>` devolve
**nada** — o `grep` pula o arquivo por causa exatamente do byte que se procura.

Neste ambiente é pior: o `grep` do shell é `ugrep -G --ignore-files --hidden -I`, e o ugrep **não
imprime `Binary file matches`** — o arquivo some da saída.

## A correção que ninguém buscou em três tentativas

**O byte não precisa ser literal.** Em JavaScript, a sequência de escape `\0` (ou `\u0000`) no
código-fonte produz **o mesmo NUL em runtime** com o arquivo permanecendo ASCII puro.

Dois caracteres, dois arquivos, comportamento idêntico — e a classe inteira desaparece **sem
depender de ninguém lembrar de nada**.

## Por que é REQ própria, e não absorvida

Teste da Regra Dura de Causa Raiz: *"se eu corrigir esta causa, exatamente estas falhas fecham — e
nenhuma outra."*

- Corrigir o **gate de conjunto** da `REQ-2026-09-12-reqs-de-paridade-nao-distinguem-entregue-de-pendente`
  **não** remove o byte NUL nem devolve visibilidade ao arquivo.
- Corrigir **este** byte não cria gate de conjunto nenhum.

Causas distintas, remédios distintos. As duas REQs de paridade com evidência falsa são **efeito**
desta causa, e já estão registradas como parciais na REQ de triagem — não são escopo daqui.

## Acceptance Criteria

- [ ] **AC1** — `npm/src/validator/index.js` e `npm/src/integrations/doctor.js` deixam de conter byte
      NUL literal; a sequência de escape produz o mesmo valor em runtime.
- [ ] **AC2** — 🔴 **prova de equivalência comportamental**: o valor da chave composta é
      **byte-idêntico** antes e depois. Não basta "os testes passam" — a chave é usada para dedup, e
      um separador diferente muda o conjunto silenciosamente.
- [ ] **AC3** — `tr -d -c '\000' < arquivo | wc -c` devolve **0** nos dois arquivos.
- [ ] **AC4** — o `grep` **do ambiente** (`ugrep -I`, sem `-a`) passa a encontrar padrões nos dois
      arquivos. 🔴 Esta é a prova que importa — é a ferramenta que falhou nas quatro ocorrências.
- [ ] **AC5** — **gate** que reprova se qualquer fonte rastreado contiver byte NUL literal.
      Falsificação: plantar um NUL num fonte ⇒ reprova nomeando arquivo e offset. Contra-braço:
      árvore limpa ⇒ passa.
- [ ] **AC6** — 🔴 **guarda de vacuidade do gate**: se a varredura não examinar arquivo nenhum
      (glob vazio, `git ls-files` falhando), **reprova** em vez de declarar limpo.
- [ ] **AC7** — o gate **não** usa `grep` para procurar o NUL. Usaria a ferramenta derrotada pelo
      objeto medido. Leitura em bytes.
- [ ] **AC8** — varredura: **os dois arquivos são os únicos**? Confirmar com leitura em bytes sobre
      todos os fontes rastreados, nos três runtimes — não só em `npm/`.
- [ ] **AC9** — a nota consolidada do vault passa a apontar que a causa foi **corrigida**, com a data.
- [ ] **AC10** — `make quality` verde e CI verde.

## Escopo negativo

- **Não** reabrir as duas REQs de evidência falsa — elas estão corretamente marcadas como parciais na
  REQ de triagem, e o que falta nelas é gate, não implementação.
- **Não** alterar o comportamento de dedup nem o formato da chave — só a **grafia no fonte**.
- **Não** mexer no `grep` do ambiente: é config de shell do usuário, não do produto.
- 🔴 **Não** despachar enquanto o ML-1A da REQ de REQ órfã estiver aberto: ele edita
  `npm/src/validator/index.js` pesadamente e os dois colidiriam.

## Linked ADR
ADR: <!-- não requer: correção de grafia de fonte, sem decisão de arquitetura -->
