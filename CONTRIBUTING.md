# Contribuindo com o trackfw

O trackfw é um CLI de governança de entrega. Ele existe para impedir que trabalho de software
avance sem rastreabilidade — e, por consequência, **ele se aplica a si mesmo**. É por isso que
contribuir aqui tem algumas exigências que não são comuns em outros projetos.

Este documento descreve o que se espera, e **por que** — sempre com o caso que originou a regra.

---

## Onde cada coisa vai

| você quer | vá para |
|---|---|
| relatar um **defeito** | [Issue](https://github.com/kgsaran/trackfw/issues/new/choose) |
| **perguntar** algo (uso, planejamento, "por que a decisão foi X?") | [Discussions](https://github.com/kgsaran/trackfw/discussions) |
| reportar **vulnerabilidade** | [Advisory privado](https://github.com/kgsaran/trackfw/security/advisories/new) — **não** abra issue pública |

⚠️ **Esta separação passou a existir em 2026-09-26.** Antes dela, não havia Discussions nem este
arquivo — então perguntas iam para issues **porque não havia outro lugar**, e isso era nosso, não de
quem perguntou.

---

## O que faz um bom relato de defeito

**Medição, não impressão.** Os issues que mudaram decisões de arquitetura deste projeto
(#273, #308, #353) trouxeram números e o método de obtê-los.

🔴 **E declare a sua régua.** Nesta casa o mesmo defeito já foi contado como **9, 12, 13 e 16**
sítios — todas "certas" para quem contou, porque `grep` literal, leitura de código e análise de
provenância medem coisas diferentes. A régua costuma **ser** o achado.

**Leitura de código é evidência legítima** — desde que você diga que foi leitura, e não execução.
O #273 fez exatamente isso (*"relações reimplementadas a partir da leitura, não medidas pelo
binário"*), e a ressalva o tornou mais útil, não menos.

**O contra-braço vale muito.** Diga o que **continua funcionando** e não pode quebrar. Sem ele,
*"nada mais funciona"* satisfaz o relato — e já trocamos defeito por paralisação aqui, mais de uma vez.

**Você não precisa acertar a causa.** Várias vezes o mecanismo proposto no issue estava errado e o
defeito era real. Isso é útil do mesmo jeito.

---

## Se você vai mandar código

### A cadeia de governança vem antes do código

```
ADR → REQ → ROADMAP → backlog / analyzing / wip / blocked / done / abandoned
```

Para mudança não-trivial, o artefato de governança **precede** a implementação:

```bash
trackfw req new "título"
trackfw roadmap new "título"
trackfw roadmap move <nome> wip
trackfw branch new fix/<slug>
```

🔴 **Não é cerimônia.** `trackfw validate` reprova branch `feat/`/`fix/`/`refactor/` sem roadmap em
`wip/` — o produto cobra de si o que cobra de quem o instala.

**Dispensam REQ+roadmap** (lista fechada): typo, renomear variável local, mudança **doc-only**,
ajuste de config sem efeito em runtime, `revert` direto, e responder pergunta.

### Todo teste novo declara o que afirma

Em uma frase: **qual conclusão do seu trabalho este teste sustenta?**

🔴 **Se não for possível escrever a frase, o teste não deveria existir** — ou é decorativo, ou está
afirmando outra coisa. Em 2026-09-05 um trabalho aqui mediu que `ENOTDIR` é indistinguível de
"ausente" no Windows, escreveu isso no relatório, e **na mesma entrega** criou um teste afirmando o
contrário. Passou pela revisão. Quem pegou foi uma auditoria externa, um dia depois.

### Falsificação nas duas direções

Não basta o teste passar. Ele precisa **reprovar quando o defeito volta**:

- **braço positivo** — o defeito reproduzido é acusado;
- **braço de controle** — o caso legítimo **continua funcionando**.

⚠️ **Sem o controle, "nada mais passa" satisfaz o braço positivo.** E cuidado com a fixture: aqui um
mutante já **degenerou no estado pré-fix** porque um diretório da fixture não existia — o teste
passou, dando impressão de cobertura, e não media nada.

### Mesma causa, mesma REQ

Se você descobrir que o defeito que está corrigindo existe em outros sítios — **mesma causa, mesmo
mecanismo** —, eles entram **no mesmo trabalho**, não em issue nova.

🔴 **Motivo, medido:** em 2026-09-06 havia **59 ocorrências** de *"vira REQ própria"* espalhadas por
30 roadmaps. Cada uma era um defeito **localizado, medido e não corrigido**, empurrado para uma fila
onde se perdia — com a aparência tranquilizadora de estar "registrado". **Registro não é correção.**

**Não justificam separar:** *"é superfície diferente"*, *"está fora do escopo declarado"*, *"é
difícil"*. Justifica separar **só** a causa ser outra, e a diferença de mecanismo fica **escrita**.

### Se a medição refutar o plano, diga primeiro

O relatório de quem implementa começa pelas **refutações**. Nesta campanha o executor estava certo e
o arquiteto errado **catorze vezes** — inclusive quando o arquiteto indicou um exemplar de *"forma
correta"* que **não satisfazia o próprio analisador** do projeto.

Divergir com medição é o comportamento esperado, não atrito.

---

## Rodando os gates

```bash
make build      # compila em bin/trackfw
make test       # go test ./...
make quality    # a barreira completa — build, testes, gates e suíte de falsificação
```

⚠️ **`make quality` leva ~13 min e satura 8 núcleos.** Use `TRACKFW_FALSIFY_JOBS=4` se precisar da
máquina.

🔴 **Dois gates enumeram por `git ls-files`** (`check-symlink-privilege-guard`,
`check-write-containment`), e por isso **arquivo de teste não commitado é invisível a eles**. Já
aconteceu **seis vezes** nesta base um teste passar na barreira e reprovar depois do commit. **Rode
a barreira de novo depois de commitar.**

**Leia o rc em linha separada:**

```bash
make quality > log 2>&1
echo $?          # ← e não `make quality; echo $?` numa linha composta
```

---

## Estilo de commit e de PR

Commits seguem [Conventional Commits](https://www.conventionalcommits.org/). O corpo do commit
importa mais que o título: registre **o que foi medido**, não só o que foi mudado.

**No PR:** use a palavra-chave de fechamento **em inglês** — `Closes #123`. 🔴 O GitHub **não** fecha
issue com `Fecha #123`, e este repositório tem um gate que reprova a forma portuguesa. Ele também
entende declaração de escopo negativo (`Não fecha #456`) sem acusá-la — mas se acusar uma frase sua
que **não** afirma fechamento, isso é um defeito e vale um issue.

---

## Uma palavra sobre o tom

Este projeto guarda os próprios erros por escrito, com nome e medição, inclusive os do mantenedor.
Não é autoflagelação: é que **defeito que não foi escrito volta**. Se você contribuir aqui, vai
encontrar comentários dizendo *"eu estava errado e a medição mostrou"*.

Isso é o padrão. Não hesite em aumentá-lo.
