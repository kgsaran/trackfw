# Revisão de Qualidade — ML-2A (CONTRIBUTING.md, README.md, PR template)

**Data:** 2026-10-03  
**Revisor:** hefesto-tf  
**Branch:** `docs/contributing-regras-de-contribuicao`  
**Diff base:** `git diff 95deba97..HEAD -- CONTRIBUTING.md README.md .github/PULL_REQUEST_TEMPLATE.md`  
**Roadmap:** ROADMAP-2026-09-01-publicar-a-exigencia-de-governanca-para-prs-no-contributing.md

---

## 1. Verificações factuais

### 1.1 Proteção da `main` — `enforce_admins` e `required_status_checks`

```
gh api repos/kgsaran/trackfw/branches/main/protection \
  --jq '{required_status_checks: .required_status_checks.contexts, enforce_admins: .enforce_admins.enabled}'
```

Resultado:
```json
{
  "required_status_checks": [
    "go", "package-smoke", "windows-integrations-resolve", "parity",
    "governance-install-script", "governance-go-install", "windows-full-suites",
    "shim-byte-identity-gate", "pr-closing-keyword"
  ],
  "enforce_admins": true
}
```

**Veredito:** CORRETO. A afirmação "A `main` exige os status checks e não permite bypass, inclusive para os mantenedores" está verificada — `enforce_admins: true` e 9 checks obrigatórios.

---

### 1.2 Label `req-aberta`

```
gh label list --json name,description | grep req-aberta
```

Resultado:
```json
{"description":"Já existe REQ aberta nossa para esta issue — abra uma Discussion antes de implementar","name":"req-aberta"}
```

**Veredito:** CORRETO. A label existe com o texto exato que o `CONTRIBUTING.md` cita entre aspas.

---

### 1.3 PRs #238 e #240 — "correções de uma linha de gate"

O `CONTRIBUTING.md` (linha 85) afirma: *"Os PRs #238 e #240 eram correções de uma linha de gate."*

```
gh pr view 238 --json title,additions,deletions,files
```

Resultado: PR #238 — `fix(gate): check-roadmap-barrier-contract morre em cp1252 no 11o check`  
Body do PR: **"Um arquivo, 8 linhas"** (8 linhas no script de gate, não 1 linha).  
Arquivos: `scripts/check-roadmap-barrier-contract.sh` (+8 linhas), mais REQ, roadmap e `.trackfw-log`.

```
gh pr view 240 --json title,additions,deletions,files
```

Resultado: PR #240 — `fix(gate): write_fixture_crlf corrompe nao-ASCII no Windows`  
Arquivos: mesmo script (+7/-1 linhas), mais REQ, roadmap e `.trackfw-log`.

**Veredito: INCORRETO.** A afirmação "uma linha" está errada. PR #238 = 8 linhas no gate; PR #240 = 7 linhas no gate. O ponto em si (mudanças pequenas num gate exigiram REQ) é válido — o número "uma linha" é o erro.

---

### 1.4 Anotação `trackfw-contract` e o que `scripts/check-parity-contract-coverage.sh` exige

Formas listadas no `CONTRIBUTING.md` (linha 146):

> `gate=<caminhos>`, `gate=<caminhos> partial=<o que fica de fora>`, `gap reason=<motivo>` e `none reason=<motivo>`

Confirmado no cabeçalho de `scripts/check-parity-contract-coverage.sh` (linhas 8–13):
```
<!-- trackfw-contract: gate=<caminhos> -->
<!-- trackfw-contract: gate=<caminhos> partial=<o que fica de fora> -->
<!-- trackfw-contract: gap reason=<motivo> -->
<!-- trackfw-contract: none reason=<motivo> -->
```

O script também reprova: seção sem anotação, `gate=` sem caminho, chave com valor vazio, chave desconhecida.

**Veredito:** CORRETO. As formas listadas no texto são exatamente as que o script reconhece.

---

### 1.5 Scripts que `trackfw init` gera — nunca triviais

O texto afirma: "`scripts/`, `.github/workflows/` e os scripts que `trackfw init` gera são **nunca triviais**".

Confirmado em `internal/generators/scaffold.go`: `trackfw init`/`update` gera
`scripts/trackfw-credential-guard.sh`, `scripts/trackfw-git-branch-guard.sh` e outros hooks.

**Observação de precisão:** os PRs #238 e #240 tocam `scripts/check-roadmap-barrier-contract.sh`, que
é um gate específico do projeto trackfw, **não** um script gerado por `trackfw init`. O exemplo
exemplifica a categoria "`scripts/`" (primeira da lista), não a categoria "scripts que `trackfw init`
gera" (terceira). A afirmação sobre os três tipos é correta; o exemplo prova apenas a primeira
categoria. Não é um erro factual, mas gera ambiguidade.

---

### 1.6 Comandos `trackfw` citados existem

```
bin/trackfw --help
```

Confirmados: `trackfw req`, `trackfw roadmap`, `trackfw validate`, `trackfw branch new`,
`trackfw roadmap move`. Todos presentes.

**Veredito:** CORRETO.

---

## 2. Três pontos levantados pelo arquiteto

### 2(a) AC8(a): "label ou em backlog" — o texto cobre apenas a label

**AC8(a) da REQ diz:**
> "quem for implementar issue com essa label **ou** em backlog abre uma Discussion antes do código"

**Texto atual do `CONTRIBUTING.md`:**
> "Se a issue não tiver essa label mas você tiver dúvida, a Discussion também é o caminho."

GAP: o caso "issue em backlog sem a label" é obrigatório (AC diz "ou em backlog") mas o texto
torna optional ("se você tiver dúvida"). A label é aplicada por nós — um contribuidor externo não
consegue verificar o estado da nossa lista interna de backlog. A frase precisa explicitamente nomear
o que fazer quando há dúvida sobre o backlog, mesmo sem a label.

**Proposta:**
```
Se a issue não tiver essa label mas você perceber que há trabalho nosso em andamento (ou em
backlog) sobre o mesmo problema, a Discussion também é o caminho — sem pressão de prazo.
```

---

### 2(b) "A falha de não ter comunicado antes foi nossa"

**Texto atual:**
> "PR que colide com trabalho em andamento é fechado. O motivo fica registrado, com crédito pelo
> que o PR trouxe — a contribuição não se perde. A falha de não ter comunicado antes foi nossa."

O problema: a frase está escrita como regra geral. Após este documento ser publicado, se um
contribuidor lê as regras, vê a label `req-aberta`, e ainda assim abre um PR conflitante — não é a
nossa falha de comunicação.

A frase foi verdadeira para o caso fundador (2026-10-01, regras não publicadas). Publicadas as
regras, a causa muda.

**Proposta:**
```
PR que colide com trabalho em andamento é fechado. O motivo fica registrado, com crédito pelo que
o PR trouxe — a contribuição não se perde.

⚠️ Esta seção foi acrescentada em 2026-10-03. Antes dela, essas situações aconteciam sem que as
regras estivessem escritas — a falha de comunicação era nossa. Com as regras publicadas, a
Discussion prévia é o caminho para evitar retrabalho dos dois lados.
```

(Alternativa mais curta, mantendo o `⚠️` já presente na seção de abertura: remover a última frase
e deixar o aviso da seção carregar o contexto histórico, já que o `⚠️` diz exatamente isso.)

---

### 2(c) Ponteiro no `README.md` em português — AC6

**Texto inserido (linha 894):**
```markdown
Para contribuir com código, leia [CONTRIBUTING.md](CONTRIBUTING.md) (em português).
```

Posição: primeira linha da seção `## Contributing`, antes do bloco `git clone`.

**Avaliação:** o ponteiro é visível para quem chega à seção Contributing — é a primeira coisa que
aparece antes dos comandos. O `(em português)` informa o idioma, o que é útil para quem lê o README
em inglês. Para AC6 ("de forma visível"), este posicionamento atende.

**Ressalva menor:** o README é em inglês; um contribuidor internacional que escana com `Ctrl+F` por
"contributing" encontraria o heading `## Contributing` e em seguida uma frase em português. A
legibilidade imediata depende do contexto — o `(em português)` sinaliza o idioma, mas o link em si
não diz "read the contributing guide." Para contribuidores que não leem português, sugerir adicionar
um subtítulo em inglês ou mudar para bilíngue:

**Proposta opcional** (não bloqueia):
```markdown
Para contribuir com código, leia [CONTRIBUTING.md](CONTRIBUTING.md) (em português — see link for
contribution guidelines).
```

Ou, mais limpo:
```markdown
See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines (in Portuguese).
```

---

## 3. Clareza para quem nunca viu o projeto

### Trecho dependente de contexto interno — `CONTRIBUTING.md` linha ~83

> "Isso vale para os mantenedores também. A conclusão deste documento é governada por
> `docs/req/REQ-2026-09-01-…` — a REQ foi reaberta em 2026-10-03 porque uma versão anterior foi
> escrita fora da cadeia."

Um contribuidor externo não tem acesso a `docs/req/` (é um arquivo do repo, não um link clicável).
O caminho de arquivo completo não é útil para quem não clonou o repo. Além disso, a frase depende
de saber o que "reaberta" significa na terminologia interna.

**Proposta:**
```
Isso vale para os mantenedores também — este documento foi escrito sob a cadeia de governança que
descreve.
```

(Remove a referência interna ao número de REQ e ao "reaberta", que não agrega para o leitor externo
e que pode ser verificada por quem já está no repo.)

---

### Seção "Se você está adicionando um gate" — dependência implícita do `docs/cli-parity.md`

> "Quando o gate descreve um contrato documentado no `docs/cli-parity.md`..."

Um contribuidor pode não saber o que é o `docs/cli-parity.md`. A frase presume que ele sabe onde
esse arquivo fica e o que significa "contrato documentado". Sugestão: adicionar um link para o arquivo
ou uma frase de contexto:

```
Quando o gate descreve um contrato documentado em [`docs/cli-parity.md`](docs/cli-parity.md) — o
registro de paridade de comportamento do CLI entre canais de distribuição —, ...
```

---

## Resumo de achados

| # | Arquivo | Linha | Severidade | Descrição |
|---|---|---|---|---|
| A1 | `CONTRIBUTING.md` | 85 | **Médio** | "correções de uma linha" — PR #238 foi 8 linhas, PR #240 foi 7 linhas; texto deve dizer "poucas linhas" |
| A2 | `CONTRIBUTING.md` | ~53 | **Médio** | AC8(a) exige Discussion para issue "em backlog" também; texto torna isso opcional ("se tiver dúvida") |
| A3 | `CONTRIBUTING.md` | ~57 | **Baixo** | "A falha de não ter comunicado antes foi nossa" está como regra geral; era o caso fundador; pós-publicação, a responsabilidade muda |
| A4 | `CONTRIBUTING.md` | ~83 | **Baixo** | Caminho interno `docs/req/REQ-2026-09-01-…` não é útil para contribuidor externo; simplificar |
| A5 | `CONTRIBUTING.md` | ~137 | **Baixo** | `docs/cli-parity.md` sem contexto para quem não conhece o projeto; adicionar breve descrição ou link |
| A6 | `README.md` | 894 | **Baixo** | Ponteiro em português num README em inglês — atende AC6, mas pode confundir; proposta bilíngue opcional |

---

## Veredito: APROVA COM AJUSTES

A1 é factualmente errado e deve ser corrigido antes do merge. A2 é gap direto contra o AC8(a) da
REQ e deve ser corrigido. A3–A6 são clareza e não bloqueiam, mas devem entrar no mesmo commit para
não gerar ML corretivo.
