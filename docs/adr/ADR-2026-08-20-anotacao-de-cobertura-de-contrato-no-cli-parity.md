---
status: Accepted
date: 2026-08-20
author: "Zeus (Arquiteto)"
---

# ADR: anotação de cobertura de contrato no `cli-parity.md`

> Date: 2026-08-20 | Status: Accepted

## Context

O projeto já sustenta o princípio **P4**: *gate sem cenário de falsificação é gate não-verificado* —
e por isso todo gate tem braço de detecção. Falta a análoga, um nível acima:

> **Contrato pinado sem gate nomeado é contrato não-aplicado.**

`docs/cli-parity.md` é o documento de contrato dos 3 CLIs. Medido em 2026-08-20: **53 seções de topo,
122 subseções, 27 scripts de gate**. O documento **não sabe dizer** quais dos seus próprios contratos
estão de fato protegidos — a cobertura cresceu reativamente, existindo gate onde alguém já se queimou.

O custo disso não é hipotético. Entre 18/08, quando a REQ foi aberta com duas instâncias, e hoje, a
lacuna produziu **cinco divergências reais** entre os 3 CLIs — `null` vs `[]` no `--json` do
`doctor`, linha em branco só no Go, stderr do filho descartado pelo `exec.Command().Output()`, erro
de git divergente no Python, timestamp com milissegundos no Node. **Nenhuma** era detectável por
teste por stack, porque cada runtime concorda consigo mesmo. Todas apareceram só quando alguém
lembrou de escrever um gate comparando as três saídas reais.

**A repetição é o dado.** Não há mecanismo que force a existência do gate, então ele depende de
memória — inclusive da memória de quem escreveu o critério.

## Decision

**Toda seção de `cli-parity.md` declara, em bloco legível por máquina, se é contrato e o que a
protege.**

Formato: uma linha imediatamente após o cabeçalho da seção, com prefixo fixo.

```
<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->
<!-- trackfw-contract: none reason=<motivo em uma linha> -->
```

- `gate=<caminho>` — a seção é contrato, e o caminho é o script que a protege. Mais de um gate
  separa-se por vírgula.
- `none reason=<motivo>` — a seção **não** é contrato (justificativa, histórico, nota de contexto).
  **O motivo é obrigatório**; sem ele, inválido.

### As três escolhas de desenho, com o porquê

**Comentário HTML, não campo de frontmatter.** A anotação é **por seção**, e o documento tem 53
delas; frontmatter é por arquivo. Comentário HTML não aparece no render, é trivial de casar por
regex, e não muda a leitura de quem consome o documento hoje.

**`none` exige motivo, e isso é ganho por si.** Força a distinção contrato/prosa a ser **declarada**
em vez de presumida. Hoje o documento mistura as duas sem sinalizar, e o leitor não tem como saber
se uma seção descreve uma garantia ou um histórico.

**O checker aponta para o vazio.** Nomear gate inexistente reprova. Sem isso, a anotação viraria
carimbo — e um carimbo é pior que a ausência, porque compra confiança.

## 🔴 O modo de falha previsível, nomeado

Alguém silencia o checker marcando tudo como `none`. É a saída fácil e destruiria o valor inteiro.

Mitigações: o motivo é **obrigatório**; a contagem de `none` é **reportada** a cada execução; e
mudanças na marcação ficam visíveis no diff. **Nenhuma impede o abuso** — tornam-no **visível**, que
é exatamente a postura que o projeto adota para o `credential-guard` desde o
`ADR-2026-08-12`: detecção ancorada, não prevenção. Não prometer o que não se entrega.

## Consequences

**Positivas**
- O documento passa a saber responder quais dos seus contratos estão protegidos. Hoje não sabe.
- A **lista de contratos sem gate** vira artefato explícito e priorizável — é o produto mais valioso
  da REQ, não subproduto.
- Seção nova sem anotação reprova, então a cobertura para de depender de memória.

**Negativas / riscos**
- **A triagem é julgamento, não mecânica**, e é o grosso do trabalho. Errar para `none` esvazia;
  errar para contrato gera lacunas falsas e ruído que treina o leitor a ignorar.
- Anotação é metadado que pode apodrecer — daí o checker validar que o gate **existe**, não só que
  foi nomeado.

## Alternatives Considered

- **Frontmatter por arquivo** — rejeitada: a granularidade errada. O contrato é por seção.
- **Tabela central de cobertura** — rejeitada: separa a anotação do que ela descreve, e diverge na
  primeira seção movida. O acoplamento local é a propriedade que se quer.
- **Inferir cobertura por menção do gate na prosa** — rejeitada: hoje 18 seções mencionam gate em
  texto livre, e inferência sobre prosa é frágil de um jeito silencioso. Explícito reprova; implícito
  adivinha.
- **Bloquear desde o primeiro dia** — rejeitada em favor de modo relatório durante a triagem. Gate
  vermelho por semanas é gate que se aprende a ignorar, e perderíamos justamente o que se quer criar.

---

## Emenda 1 (2026-08-20) — três estados, não dois; e cobertura parcial

> Esta ADR está `Accepted`. A emenda **acrescenta**; nada acima foi reescrito.

O ML-1A aplicou o formato em 3 pilotos e encontrou o que o piloto existia para encontrar: **o
formato original tem só dois estados, e o caso central da REQ é um terceiro.**

### O buraco

`gate=<caminho>` e `none reason=<motivo>` não cobrem *"isto é contrato e **nada** o protege"* — que
é **exatamente** o que a REQ existe para revelar. O executor contornou anotando `gate=` com valor
vazio, e sinalizou a decisão em vez de escondê-la. Foi a escolha certa diante da alternativa de
inventar um caminho de script inexistente, que esta própria ADR chama de carimbo.

Mas valor vazio é a solução errada, por um motivo específico: **é indistinguível de omissão**. O
checker não consegue separar "declarei que não há gate" de "esqueci de preencher" — e a segunda é
justamente a falha que se quer detectar.

### Decisão: três estados explícitos

```
<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->     contrato protegido
<!-- trackfw-contract: gap reason=<motivo> -->                     contrato SEM gate
<!-- trackfw-contract: none reason=<motivo> -->                    não é contrato
```

`gap` é distinto, greppável e **contável**. A contagem de `gap` é o produto mais valioso da REQ, e
precisa ser um número que se possa acompanhar cair ao longo do tempo. Com valor vazio isso não
existe.

### Cobertura parcial

Medido no piloto 2: `## Vault de conhecimento` tem um gate que cobre a mecânica de criação de nota,
mas **não** cobre a semântica da regra `note_orphan`. "Protegido pela metade" colapsava em
`gate=` vazio, perdendo a informação.

```
<!-- trackfw-contract: gate=<caminhos> partial=<o que fica de fora> -->
```

`partial` é opcional. Quando presente, a seção conta como **coberta com ressalva**, e o `partial`
entra no mesmo relatório do `gap` — porque lacuna declarada e lacuna parcial têm o mesmo destino:
virar trabalho priorizável.

### Regra de desempate para seção ambígua

O piloto 3 encontrou seção que **se autodeclara** não-contrato e mesmo assim fixa um fato
falsificável sobre comportamento de CLI. Vai recorrer em escala, e precisa de regra, não de
julgamento caso a caso:

> **Se a seção fixa um fato falsificável sobre o comportamento dos CLIs, ela é contrato** — a
> autodeclaração não prevalece.

O motivo é o mesmo princípio da ADR: prosa que afirma comportamento **é** contrato, esteja ou não
rotulada assim. Quem quiser que não seja, remove a afirmação.

### Universo real da triagem

Medido: `##` 53 · `###` 122 · **`####` 17** — três níveis, não dois. O `####` não aparecia na REQ
nem no roadmap. E o estado de contrato **não acompanha a profundidade**: há `####` de não-contrato
dentro de `##` de contrato.

**A anotação vale para os três níveis. O universo da triagem é ~192, não 175.** O ML-2A é
proporcionalmente maior do que estava dimensionado.
