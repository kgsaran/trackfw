# Um byte NUL literal no fonte torna o arquivo invisível à busca — 4 ocorrências, 3 notas, 0 controles

> Consolidação de 2026-09-12, substituindo três notas do mesmo fato:
> `serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29`,
> `index-js-tem-um-byte-nul-no-fonte-file-diz-texto-e-grep-diz-binario-2026-09-06` e
> `grep-do-ambiente-pula-arquivo-com-nul-2026-09-12`.

## 🔴 Leia isto primeiro: esta nota é a prova de que nota não basta

O fato abaixo foi medido e escrito **três vezes**, com precisão de offset de byte. E continuou
produzindo defeito:

| quando | o que aconteceu |
|---|---|
| 2026-08-29 | primeira nota — sweep de `by_agent` fechou em 10 arquivos sem o `validator/index.js` |
| 2026-09-05 | a armadilha reaparece **dentro de uma triagem criada para limpar o backlog** |
| 2026-09-06 | segunda nota — já registrava que **duas REQs nasceram com premissa falsa** |
| 2026-09-12 | o arquiteto bate nela de novo, investiga do zero, escreve a **terceira** nota; a triagem do mesmo dia redescobre **as mesmas duas REQs** |

**Registro não é correção.** Três notas não impediram uma quarta ocorrência, porque uma nota depende
de alguém lembrar de lê-la antes de digitar `grep`. O que faltava era o controle — ver a última
seção.

## A causa, em bytes

```
npm/src/validator/index.js     1 byte NUL literal     offset 93041 de 190181
npm/src/integrations/doctor.js 2 bytes NUL literais
```

```js
const seenKey = `${m.raw}<NUL>${m.typeIsCommand}`          // validator/index.js
const ka = [a.destination, ...].join('<NUL>')              // integrations/doctor.js
```

O uso é **legítimo** — NUL como separador de chave composta, idioma comum para evitar colisão de
delimitador. O problema não é o NUL em runtime; é ele estar **literal no arquivo-fonte**.

> Nota histórica: a nota de 08-29 atribuiu a causa a "bytes não-ASCII, não identificado a fundo"; a
> de 09-06 localizou o byte exato. O offset mudou de 83123 (arquivo com 178225 bytes) para 93041
> (190181) porque o arquivo cresceu — é o mesmo byte.

## 🔴 As ferramentas discordam entre si

```
file npm/src/validator/index.js    →  "Unicode text, UTF-8 text"     ← diz que é texto
grep sem -a                        →  pula, em SILÊNCIO, RC=1        ← trata como binário
grep -a                            →  encontra
/usr/bin/grep                      →  encontra
git grep                           →  encontra
tr -d -c '\000' < arquivo | wc -c  →  1                              ← a única medição confiável
```

**Quem tentar "verificar se o alerta procede" usando `file` conclui que é folclore** — e cai na
armadilha na busca seguinte. Foi o que aconteceu pelo menos uma vez.

E o `grep` deste ambiente é pior que o padrão: é uma **função de shell** que executa
`ugrep -G --ignore-files --hidden -I` (ver `~/.claude/shell-snapshots/snapshot-*.sh`). O `-I` é
*ignore binary files*, e o ugrep **não imprime `Binary file matches`** — o arquivo some da saída:

```
$ grep -rc "note_orphan" npm/src/validator/
npm/src/validator/git-exec.js:0
npm/src/validator/traceid.js:0
        ← index.js nem é listado, e tem 3 ocorrências
```

## 🔴 A ironia que já pegou duas pessoas

Medir o NUL com `grep -c $'\x00' npm/src/validator/index.js` devolve **nada** — porque o `grep` pula
o arquivo **por causa exatamente do byte que se procura**. A ferramenta de medição é derrotada pelo
objeto medido.

Quem parar no `grep` conclui que a premissa é falsa e escreve isso.

## O custo real, medido

**Duas REQs abertas com evidência falsa**, ambas dizendo "regra ausente no CLI Node":

```
REQ-2026-08-20  note_orphan                        → estava lá, 3 sítios
REQ-2026-09-01  thirdparty_artifact_has_provenance → estava lá, 7 sítios
```

Em 2026-09-12 isso quase produziu o despacho de um agente para **reimplementar código existente**.

## Como trabalhar, enquanto o controle não existe

- Conclusão de **ausência** (*"não existe em lugar nenhum"*): `/usr/bin/grep`, `git grep` ou
  `grep -a`. **Busca vazia não é ausência.** Conclusão positiva não é afetada.
- Verificar a premissa: **não use `grep` nem `file`** — use leitura em bytes (`tr -d -c '\000'`).
- Toda REQ cuja evidência seja "ausente no runtime X" precisa ser reverificada assim.

## 🔴 O controle que três notas não são

Ninguém, em três notas, perguntou **por que o byte está literal no fonte**. Ele não precisa estar.

Em JavaScript, a sequência de escape `\0` no código-fonte produz o mesmo NUL em runtime **com o
arquivo permanecendo ASCII puro**. Dois caracteres, em dois arquivos, comportamento idêntico — e a
classe inteira de defeito desaparece, sem depender de ninguém lembrar de nada.

Mais um gate que reprove se qualquer fonte rastreado contiver byte NUL literal, para a classe não
voltar.

Enquanto isso não for feito, esta nota continua sendo o que as três anteriores foram: um registro de
um defeito que segue vivo. Ver também
[[dois-falsos-vermelhos-ambiente-e-heuristica-2026-09-12]] — mesma família de "o ambiente mente e o
sinal parece resposta".
