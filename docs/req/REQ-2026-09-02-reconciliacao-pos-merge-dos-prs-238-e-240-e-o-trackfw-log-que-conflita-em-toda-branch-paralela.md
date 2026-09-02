---
status: Open
date: 2026-09-02
author: "kgsaran"
adr: ""
roadmap: ""
---

# REQ: Reconciliação pós-merge dos PRs #238 e #240, e o `.trackfw-log` que conflita em toda branch paralela

> Date: 2026-09-02 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Cinco pontas soltas deixadas pelo merge do PR #250 (item 4 da issue #216) e pelos PRs #238/#240 do
reporter externo. Quatro são dívida nossa de consistência; **a quinta atinge todo adotante do
trackfw** e é a única com urgência própria.

Nenhuma delas bloqueava os merges — todas foram declaradas no momento da decisão, e esta REQ existe
para que "corrigir depois" tenha dono e critério, em vez de evaporar.

### 1. `errors="replace"` no `check-roadmap-barrier-contract.sh` contradiz a decisão do ML-0A

O PR #238 introduz:

```python
sys.stdout.reconfigure(encoding="utf-8", errors="replace", newline="\n")
```

O ML-0A do `ROADMAP-2026-09-02-saida-nao-ascii-...` **mediu e reprovou** `errors="replace"`: troca
degradação limpa e visível por corrupção silenciosa.

**A nuance que evita o veredito preguiçoso:** a reprovação do ML-0A foi sobre encoding **não**-utf8.
Sobre utf-8 o handler quase nunca dispara. A exceção é **surrogate solto**, e ela não é hipotética —
foi medida na barreira final desta mesma sprint (achado S3 de
`docs/seguranca/2026-09-02-parecer-codificacao-declarada.md`): `json.load` preserva `\udXXX`, e com
handler permissivo o byte inválido chega ao artefato.

**Decisão pedida:** trocar para `strict` ou registrar por escrito por que `replace` é aceitável
**aqui**, com a medição. Não deixar implícito.

### 2. A justificativa da allowlist do nosso gate fica falsa no instante do merge

`scripts/check-output-encoding-declared.sh` tem `check-roadmap-barrier-contract.sh` como **única**
entrada de allowlist, com este motivo escrito: *"o PR #238 está aberto sobre o mesmo sítio, e forçar
a codificação lá mataria o crash sem tornar o hash independente do SO"*.

Com o #238 mergeado, **as duas metades do motivo deixam de valer**: a codificação passa a ser
forçada, e o `newline="\n"` torna o hash independente de SO.

🔴 **E o aviso de obsolescência não vai disparar.** O gate avisa "ALLOWLIST OBSOLETA" quando o
arquivo allowlisted **ganha a declaração** — mas ele procura `export PYTHONIOENCODING`, e o #238 usou
`sys.stdout.reconfigure`. O arquivo seguiria allowlisted **em silêncio**, sem cobertura e sem aviso.
É um ponto cego **criado pelo merge**, não pré-existente.

### 3. `PYTHONIOENCODING` não controla fim de linha — buraco possível nos nossos 37 gates

O achado do reporter que nós **não** tínhamos enxergado: sem `newline="\n"`, o Python emite CRLF no
Windows, e um hash calculado sobre essa saída varia **por plataforma**. Corrigir só o encoding troca
um crash barulhento por hash instável — que é pior, porque parece *"o corpus mudou"*.

O nosso mecanismo (`export PYTHONIOENCODING=utf-8`, ML-1B, 37 gates) **não** cobre isso: a variável
age na codificação, não na tradução de fim de linha.

🔴 **Isto é hipótese, não achado.** O impacto existe apenas onde a saída de um gate é **hasheada ou
comparada byte a byte** — não onde ela só é lida por humano. **Medir antes de afirmar**, e antes de
propor qualquer correção em massa.

### 4. Roadmaps entram em `wip` já completos e ficam órfãos

Os PRs do reporter trazem `docs/roadmaps/wip/ROADMAP-*.md` com os MLs marcados `✅ Concluído`.
Verificado no #238: 1 ML concluído, 0 pendentes. No instante do merge, o roadmap entra em `wip`
**completo** e fica lá até alguém mover para `done`.

Com seis PRs abertos, são até seis roadmaps órfãos. É o mesmo erro que já se repetiu cinco vezes
nesta sprint internamente — agora chegando de fora, e sem ninguém encarregado de fechar.

### 5. 🔴 `.trackfw-log` conflita em toda branch paralela — e isso atinge todo adotante

O `docs/roadmaps/.trackfw-log` é append-only e **toda escrita cai na última linha**. Duas branches
que movam qualquer roadmap conflitam **sempre**. Não é azar: é propriedade do formato.

Medido no #238: ele acrescentou uma linha (10:46); a `main` acrescentou duas (10:45 e 11:21)
enquanto o PR estava aberto; conflito garantido. O #240 reproduz o mesmo.

**Não existe `.gitattributes` neste repositório, e o `trackfw init` não gera nenhum** — verificado.
Logo o defeito não é do nosso repo: é do produto, e todo projeto que adota o trackfw herda um arquivo
que conflita sempre que duas pessoas trabalham em paralelo.

O remédio é uma linha, com driver nativo do git:

```
docs/roadmaps/.trackfw-log merge=union
```

Conflito em arquivo de log é ruído puro — não há disputa semântica, as duas linhas devem sobreviver.

## Acceptance Criteria

- [ ] **AC1** — `errors="replace"` vira `strict` **ou** o motivo de mantê-lo está escrito no código
      com a medição que o sustenta. Falsificação: entrada com surrogate solto demonstra o
      comportamento nos dois casos.
- [ ] **AC2** — A entrada de allowlist de `check-output-encoding-declared.sh` é removida **ou** seu
      motivo é reescrito para o que passa a ser verdade. Se o arquivo sair da allowlist, o gate passa
      a cobri-lo e isso é verificado por execução.
- [ ] **AC3** — 🔴 O aviso de "allowlist obsoleta" passa a reconhecer **também** `sys.stdout.reconfigure`
      como declaração, e não só `export PYTHONIOENCODING`. Falsificado nas duas direções.
- [ ] **AC4** — O item 3 é **medido**: enumerar onde saída de gate é hasheada ou comparada byte a
      byte, e reportar quantos dos 37 têm exposição real a CRLF. **Zero é resultado válido** e
      encerra o item. Correção só se a medição mostrar exposição.
- [ ] **AC5** — Os roadmaps trazidos pelos PRs do reporter estão em `done`, e a política de quem faz
      essa transição para PR externo está escrita (candidata natural ao `CONTRIBUTING.md`).
- [ ] **AC6** — `.gitattributes` com `merge=union` para `.trackfw-log` existe **neste** repositório
      **e** é gerado pelo `trackfw init` nos **3 CLIs** (regra dura de paridade). Falsificação nas
      duas direções: duas branches que movem roadmaps mergeiam sem conflito com o arquivo, e
      conflitam sem ele.
- [ ] **AC7** — 🔴 **Controle do AC6:** o `merge=union` **não** engole conteúdo — as linhas dos dois
      lados sobrevivem, verificado por igualdade de conjunto, não por ausência de conflito.

## Negative Scope

- **Não** revisar os PRs #245, #247, #248 e #249 aqui — são trabalho independente do reporter e cada
  um traz a própria cadeia de governança.
- **Não** mexer no `CORPUS_HASH` nem no corpus pinado. O item 3 é sobre **medir** exposição a CRLF,
  não sobre repinar hash.
- **Não** varrer os demais gates atrás de `sys.stdin.read()` sem encoding explícito — o próprio
  reporter declarou isso como fora de escopo no #240, e provavelmente o ML-1B já cobre a classe pelo
  `PYTHONIOENCODING` agir no decode do stdin. **Hipótese; se virar trabalho, é REQ própria.**
- **Não** escrever o `CONTRIBUTING.md` aqui. O AC5 só exige que a política de transição de roadmap
  para PR externo esteja **decidida**; publicá-la é a REQ pausada
  `REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing.md`.

## Linked ADR
<!-- Reconciliação de dívida declarada e correção de defeito de harness; sem decisão de arquitetura
     nova a registrar. O AC6 muda artefato gerado pelo init nos 3 CLIs, mas segue contrato de
     paridade já estabelecido. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap deliberadamente NÃO criado ainda: `trackfw roadmap move` escreve em
     docs/roadmaps/.trackfw-log, exatamente o arquivo em conflito nos PRs #238/#240. Criar o roadmap
     antes de os dois mergearem geraria um terceiro conflito no PR do reporter — que é o defeito
     descrito no item 5 desta própria REQ. Criar após os merges. -->
Roadmap:
