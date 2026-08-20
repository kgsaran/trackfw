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
