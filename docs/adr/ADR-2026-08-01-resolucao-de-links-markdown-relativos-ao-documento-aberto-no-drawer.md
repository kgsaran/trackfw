---
status: Accepted
date: 2026-08-01
author: "Zeus"
---

# ADR: Resolucao de links markdown relativos ao documento aberto no drawer

> Date: 2026-08-01 | Status: Accepted

## Context

Clicar num link `.md` dentro do drawer do `trackfw serve` resulta em erro quando o link é
relativo. O interceptador em `internal/serve/static/app.js:966-977` passa o **href bruto** para
`openDrawer`, que o envia como `?path=`. O servidor faz `filepath.Clean` e rejeita qualquer
caminho fora do whitelist.

Reproduzido em 2026-08-01:

```
GET /api/file?path=../roadmaps/done/v2.3-validator-improvements-2026-06-13.md  → 403 Forbidden
GET /api/file?path=docs/roadmaps/done/v2.3-validator-improvements-2026-06-13.md → 200
```

Levantamento de **todas** as formas de link markdown `.md` em `docs/`:

| Forma | Ocorrências |
|---|---|
| `./ARQUIVO.md` | 13 |
| `../../../requisições/claude/ARQUIVO.md` | 5 |
| `ARQUIVO.md` (nu) | 3 |
| `../vault/notes/ARQUIVO.md` | 3 |
| `../roadmaps/done/ARQUIVO.md` | 1 |
| `../../req/ARQUIVO.md` | 1 |

**Nenhuma** é relativa à raiz do repositório. Todas são relativas ao documento — o que torna a
regra de resolução inequívoca e elimina a ambiguidade que existiria se convivessem as duas formas.

Achado por Ártemis durante o seam do XSS (`vault/notes/seam-xss-drawer-armadilhas-de-verificacao-2026-07-31.md`),
reportado sem correção por estar fora daquele escopo.

## Decision

**O href relativo é resolvido contra o diretório do documento atualmente aberto, no cliente.
O whitelist do servidor não muda.**

1. Antes de chamar `openDrawer`, o interceptador resolve o href contra `dirname(_drawerPath)`,
   normalizando `.` e `..`. Exemplo: documento aberto `docs/req/REQ-x.md`, href
   `../roadmaps/done/y.md` → `docs/roadmaps/done/y.md`.
2. **O whitelist de `/api/file` permanece inalterado** — `ADRDirs`, `REQDir`, `RoadmapDir`.
   Em particular, `vault/` **não** é adicionado, embora 3 links apontem para lá. Decisão do
   usuário: manter a superfície de leitura do servidor inalterada.
3. Um link que resolva para fora dos diretórios permitidos passa a exibir **mensagem explicativa**
   no drawer, informando o caminho resolvido e que ele está fora dos diretórios permitidos — em
   vez do `Forbidden` cru ou de um `HTTP 403` sem contexto.
4. A resolução é **puramente cliente**. Nenhum código de servidor muda, em nenhum dos 3 CLIs.
   `internal/serve/static/` é canônico; npm e pypi são espelhos byte-a-byte.

## Consequences

**Positivas**

- Links relativos passam a navegar dentro do drawer, que é o comportamento que o interceptador
  sempre pretendeu ter. Cobre as formas `./x.md`, `x.md` e `../dir/x.md` de uma vez.
- A superfície de leitura do servidor não cresce — relevante logo após o ciclo de correção do XSS.
- Erros deixam de ser opacos: quem clicar num link para fora dos diretórios permitidos entende o
  porquê, e vê o caminho resolvido, o que também ajuda a diagnosticar link quebrado no documento.

**Negativas / aceitas**

- Os 3 links para `../vault/notes/*.md` continuam não abrindo. Aceito conscientemente: passam a
  falhar com mensagem clara em vez de erro cru. Incluir o vault é decisão separada.
- Os 5 links `../../../requisições/claude/*.md` apontam para fora do repositório — legado de outro
  projeto. Continuarão falhando, agora de forma compreensível. **Não** serão corrigidos aqui.
- A resolução depende de `_drawerPath` estar correto no momento do clique. Navegação encadeada
  (A → B → C) precisa resolver cada salto contra o documento **então** aberto, não contra o
  primeiro. Isso é requisito de teste, não detalhe de implementação.

## Alternatives Considered

**Aceitar caminhos relativos no servidor, resolvendo contra as raízes permitidas** — funcionaria
sem tocar no cliente. **Rejeitado:** exigiria afrouxar a checagem anti-path-traversal de
`api_file`, que é justamente o controle que impede leitura arbitrária. Trocar um controle de
segurança por conveniência de navegação é péssima troca, e ainda exigiria implementação idêntica
nos 3 CLIs.

**Reescrever os links dos documentos para a forma raiz-relativa** (`docs/roadmaps/...`) —
resolveria sem código. **Rejeitado:** trata os sintomas atuais e não o mecanismo; qualquer
documento novo com link relativo — a forma natural em markdown, e a que 13 dos 26 links já usam —
voltaria a quebrar. Além disso quebraria a navegação dos arquivos fora do dashboard (editor, GitHub),
onde a forma relativa é a correta.

**Adicionar `vault/` ao whitelist** — faria 3 links a mais funcionarem. **Rejeitado por decisão do
usuário:** amplia o que o servidor lê, logo após um ciclo dedicado a reduzir superfície de ataque.
Pode ser reconsiderado em REQ própria.
