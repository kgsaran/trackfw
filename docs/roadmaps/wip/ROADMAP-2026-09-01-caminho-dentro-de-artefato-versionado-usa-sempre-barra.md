---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Caminho dentro de artefato versionado usa sempre barra

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md`

**Item 10 do issue #216.** `roadmap move` grava `roadmap: docs\roadmaps\wip\ROADMAP-x.md` no
frontmatter da REQ quando roda no Windows. No Windows resolve, porque o `os.Stat` aceita as duas
grafias. **Em Linux não resolve — e o arquivo vai para o git.** Basta commitar no Windows e alguém
dar checkout no Linux para a referência quebrar.

**Caminho dentro de arquivo não é caminho de sistema de arquivos — é dado portável, e tem que ser
sempre `/`.**

Medido no CI (PR #229): **reproduz nos 3 runtimes**, incluindo o caso misto no Python
(`docs/roadmaps\wip\ROADMAP-item10.md`).

## Acceptance Criteria

- [ ] Enumeração real dos pontos que montam caminho **escrito dentro** de conteúdo, nos 3 runtimes
- [ ] Escrita sempre com `/`, independentemente do SO
- [ ] 🔴 **Leitura tolerante**: artefato já gravado com `\` **continua resolvendo**
- [ ] Falsificação nas duas direções, incluindo o controle da leitura tolerante
- [ ] Camada 2 vai de **5** para **4** `REPRODUCED`, com a transição do item 10 explicada e o run citado
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Enumeração e modelo de ameaça
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — Enumerar os pontos que escrevem caminho dentro de conteúdo
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que a enumeração é o entregável, e não um preâmbulo:** a REQ nomeia **dois** pontos conhecidos
(o sync do `roadmap move`, e `pypi/trackfw/commands/roadmap.py:609` no `.trackfw-log`). **Trate essa
lista como conhecidamente incompleta.** Nesta mesma sessão, duas enumerações minhas erraram por uma
ordem de grandeza — 3 guardas contra 187 pontos de escrita, e 3 guardas de folha contra 228
candidatos. O padrão já não é acaso: **grep por sintoma encontra quem já trata o problema, não quem
o ignora.**
**Actions:**
1. Varra os **primitivos de junção** — `filepath.Join`, `path.join`, `os.path.join`, `Path(...) / ...`,
   e concatenação manual com separador — e **filtre pelos que têm o resultado escrito dentro de
   conteúdo de arquivo**, não usados para acessar o sistema de arquivos. **A distinção é a análise
   inteira:** `filepath.Join` para abrir um arquivo está **correto** e não deve ser tocado.
2. Modelo de ameaça: um caminho com `\` num artefato versionado é dado que atravessa máquinas. O que
   quebra, e para quem, quando ele chega ao Linux?
3. **Falsificação nas duas direções**, com atenção especial à direção simétrica: uma normalização
   agressiva demais que **quebre a leitura de artefato já gravado com `\`** troca um defeito por um
   pior — repositórios existentes quebram no upgrade. Nomeie o que **não** pode ser normalizado.
4. Residual declarado.
**Critérios de aceite:**
- [x] Enumeração distingue "caminho escrito em conteúdo" de "caminho usado para acessar arquivo"
- [x] A lista dos 2 pontos da REQ é tratada como ponto de partida, não como escopo
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

**Desta vez a enumeração da REQ estava quase certa — e o que mudou foi mais fino que "faltam pontos".**

### O bug do `.trackfw-log` é só do Python

| runtime | código | situação |
|---|---|---|
| Python `generators/roadmap.py:611` | `os.path.join(agent, basename)` | 🔴 separador **nativo** |
| Go `generators/roadmap.go:467` | `agent + "/" + filepath.Base(src)` | ✅ já correto |
| Node `generators/roadmap.js:269` | `agent + '/' + basename` | ✅ já correto |

A REQ tratava como defeito geral; **é divergência de paridade com dois runtimes já certos.** Verifiquei
as três linhas.

### Achado novo, e é do lado da LEITURA

`internal/validator/validator_thirdparty_provenance.go:142` monta a chave de busca com
`filepath.Rel(root, destination)` — **separador nativo** — e compara contra as chaves de
`.trackfw/thirdparty-provenance.json`, que são **sempre gravadas com `/`**.

O doc-comment ali afirma: *"`filepath.Rel` inverte o `filepath.Join(root, relative)` do
`Manager.resolve` exatamente."* **É verdade para semântica de caminho e falso para casamento de
chave de string** — e é justamente como a chave é usada. Go-only; a regra não foi portada para
Node/Python.

### Três sintomas concretos, dois reproduzidos ao vivo — sem Windows

Ele montou o valor com `\` à mão e rodou o binário Go real em `/tmp`:

1. **`validate` recusa referência que existe:**
   `req "REQ-poc.md" links to Roadmap "docs
oadmaps\wip\ROADMAP-poc.md" which does not exist`.
2. **O board do `serve` perde a aresta:** `/api/chain` desenha o id do nó com `/` e o `to` da aresta
   com `\` — aresta órfã, ligação **some silenciosamente** do grafo.
3. **`metrics` parte um roadmap em dois artefatos** (derivado por leitura, não executado): agrupa
   transições do `.trackfw-log` por string exata de basename, então um roadmap com transições
   misturando `agent/file.md` e `agentile.md` — o bug Python acima — **cai fora do cycle-time**.

O sintoma 2 é o pior dos três: **não falha, some.**

### Ele corrigiu a premissa da minha direção simétrica

Eu tinha escrito que o risco era *"quebrar a leitura de artefato já gravado com `\`"*. **Não existe
tolerância de leitura em lugar nenhum hoje** — então não há o que regredir. **O risco real é o
escopo da normalização nova**, e ele nomeou três limites duros:

| não normalizar | por quê |
|---|---|
| `content_base64` do registro de quarentena de terceiro | âncora de TOCTOU, checksummada, verbatim por design |
| corpo de prosa/código de ADR, REQ e roadmap | normalizar **o valor do campo extraído**, nunca o buffer do arquivo — os artefatos deste repo estão cheios de `\` literal em exemplo e regex |
| chave de caminho absoluto do `integrations-manifest.json` | não-portável por design, contrato já fixado no `cli-parity.md` |

O segundo limite é o que teria estragado tudo: uma normalização ingênua rodando sobre o arquivo
inteiro corromperia a documentação que **descreve** o defeito.

### Residual declarado

O achado do `filepath.Rel` é Go-only e **não testado em Windows real**; o sintoma 3 não foi
executado; e a enumeração seguiu cada `dst`/`logBasename`/chave até o ponto de escrita **em vez de
caminhar exaustivamente os 780+ `Join` do repositório**.

## Wave 1 — A correção (3 MLs, sequência definida pela enumeração)
> Dependências: Wave 0 completa.

### ML-1A — Escrita: separador portável no sync do `roadmap move` e no `.trackfw-log`
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py`
**Actions:**
1. O `newRoadmapPath` escrito no campo `roadmap:`/`Roadmap:` da REQ pareada usa `/` nos 3 runtimes.
2. `pypi/trackfw/generators/roadmap.py:611` — `os.path.join(agent, basename)` vira concatenação
   explícita com `/`, **igualando ao que Go e Node já fazem**. Não é feature nova: é fechar
   divergência de paridade com dois runtimes já corretos.
🔴 **Normalize o valor do campo, nunca o buffer do arquivo.** Os artefatos deste repositório contêm
`\` literal em exemplo, regex e prosa — inclusive a REQ que descreve este defeito.
**Critérios de aceite:**
- [x] Escrita com `/` nos 3 runtimes, verificada por teste
- [ ] Falsificação nas duas direções, **incluindo o controle**: conteúdo com `\` legítimo no corpo do
      artefato **não** é tocado
- [ ] `make quality` verde

### ML-1B — Leitura: tolerância a `\` já gravado, com limites
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Diagnóstico:** **não existe tolerância de leitura hoje** — este ML cria, não conserta. Sem ela, todo
artefato já commitado por um usuário Windows continua quebrado depois do ML-1A.
**Actions:**
1. Resolução de referência (`validate`, `barrier`, o `/api/chain` do `serve`) aceita `\` e `/`.
2. `internal/validator/validator_thirdparty_provenance.go:142` — a chave de busca é normalizada para
   `/` antes de comparar com as do JSON. **Corrigir também o doc-comment**, que afirma uma
   equivalência verdadeira para caminho e falsa para chave de string.
🔴 **Os três limites duros do ML-0A não são normalizados** — `content_base64`, corpo de prosa/código,
e chave absoluta do `integrations-manifest.json`.
**Critérios de aceite:**
- [x] Referência com `\` resolve; referência com `/` continua resolvendo
- [ ] 🔴 Os três limites duros **verificados por teste**, não por comentário
- [ ] O sintoma 2 (aresta órfã no `serve`) deixa de ocorrer
- [ ] `make quality` verde

### ML-1C — Paridade da regra de provenance
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Diagnóstico:** a regra `thirdparty_artifact_has_provenance` existe **só no Go**. O ML-1B corrige o
Go; se Node e Python não a têm, **a lacuna de paridade é anterior a esta REQ**.
**Actions:**
1. **Confirmar** se a regra está mesmo ausente nos outros dois. Se estiver, isto é achado de paridade
   **fora do escopo desta REQ** — registrar e abrir REQ, **não** implementar aqui.
**Critérios de aceite:**
- [x] Confirmação com evidência
- [ ] Se ausente: REQ aberta, nada implementado

#### Resultado da Wave 1 (apolo-tf, 2026-09-01) — auditado pelo arquiteto

**Duas decisões dele que eu não teria tomado, e as duas certas.**

**1. Rejeitou o `filepath.ToSlash`** — a API idiomática óbvia. O raciocínio ficou no código: em
Linux e macOS o `ToSlash` é **no-op**, porque `filepath.Separator` já é `/`, então **não
normalizaria um valor sujo herdado de um commit feito no Windows** — que é exatamente o defeito a
curar. Usou substituição incondicional. **A API canônica estaria errada precisamente no caso que
importa**, e escolher a menos idiomática exigiu entender por que ela existe.

**2. Documentou o limite no próprio código**, em vez de deixá-lo no roadmap:
*"NÃO deve ser aplicado ao buffer inteiro de um arquivo — só ao valor já extraído de um campo
específico."* O aviso virou parte permanente do código; instrução em roadmap se perde.

### O ML-1C respondeu, e minha hipótese estava errada em outra direção

Eu supus que `thirdparty_artifact_has_provenance` existia **só no Go**. Existe no **Go e no Python**;
falta no **Node** (`npm/src/validator/index.js`: zero ocorrências).

**Isso me fez checar algo que quase passou:** se o Python tem a regra, tem o mesmo bug de separador?
**Tem** — `os.path.relpath` devolve separador nativo igual ao `filepath.Rel`. E ele **corrigiu os
dois**, com o mesmo doc-comment reescrito (`validator.py:3437`). **Se tivesse corrigido só o Go,
teríamos introduzido uma quebra de paridade dentro da REQ que existe para corrigir divergência.**

A ausência no Node é lacuna **anterior** a esta REQ → **REQ própria, não implementada aqui**,
conforme a instrução.

### O doc-comment falso, reescrito

O original afirmava: *"`filepath.Rel` inverte o `filepath.Join(root, relative)` do `Manager.resolve`
exatamente."* A reescrita separa: verdade para **semântica de caminho**, falso para **casamento de
chave de string** — e é assim que o valor é usado duas linhas abaixo. Um comentário que afirma um
invariante falso é pior que nenhum: o próximo leitor confia nele.

### Falsificação nas duas direções — feita pelo arquiteto, não aceita por relato

Fixture em `/tmp` com `roadmap: "docs\roadmaps\wip\ROADMAP-poc.md"` (barra **simples**, como um
commit de Windows produz), e a função de normalização neutralizada em memória:

```
COM a correção   →  0 violações
SEM a correção   →  1 violação — 'links to Roadmap "docs\roadmaps\wip\ROADMAP-poc.md" which does not exist'
```

O defeito volta na forma exata que o ML-0A reproduziu: **`validate` recusando referência que existe
no disco.**

## Wave 2 — Gate falsificável
> Dependências: Wave 1. O gate precisa provar a escrita **sem** máquina Windows — a REQ exige isso
> na AC5, e é o que torna a regressão detectável no CI Linux todo dia.
> 🔴 **O gate nasce ligado ao `Makefile` e com guarda de vacuidade** — contrato escrito em
> `docs/cli-parity.md` nesta sessão, depois de dois gates chegarem inertes.

## Verificação que só o CI fecha

A camada 2 indo de **5** para **4**. Ao contrário dos itens 2 e 3, **este check mede comportamento de
produto** — a saída do `roadmap move` no frontmatter da REQ —, então deve genuinamente virar.
Verifiquei o que ele mede antes de escrever o número, que foi exatamente o passo que faltou da última vez.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.
