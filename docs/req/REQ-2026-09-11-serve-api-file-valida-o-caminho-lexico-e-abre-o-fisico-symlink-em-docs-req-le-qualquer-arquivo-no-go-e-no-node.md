---
status: Open
date: 2026-09-11
author: ""
adr: ""
roadmap: ""
---

# REQ: serve /api/file valida o caminho lexico e abre o fisico: symlink em docs/req/ le qualquer arquivo no Go e no Node

> Date: 2026-09-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 

## Motivation

Achado **H-01** da auditoria de segurança `hades-tf` do **Codex**
(`docs/qualidade/2026-09-11-auditoria-hades-codex-dos-fontes.md`), **reproduzido pelo arquiteto** em
2026-09-11 contra os três binários reais, com `serve` de pé e `curl`:

```
GO    HTTP 200  HADES_SECRET_TOKEN_ABC123      🔴 vulneravel
NODE  HTTP 200  HADES_SECRET_TOKEN_ABC123      🔴 vulneravel
PY    HTTP 403                                 defendido
```

Cenário: projeto com `req_dir: docs/req` e um symlink `docs/req/link.md → /fora/secret.txt`.
`GET /api/file?path=docs/req/link.md` devolve **o conteúdo do arquivo de fora**.

## Causa

**A verificação ocorre antes da resolução física.** O código valida o **nome** que vai abrir; o sistema
operacional abre o **destino**.

```
Node    npm/src/serve/api_file.js:46-55    path.resolve()  → so lexico  → readFileSync
Go      internal/serve/api_file.go:32-52   filepath.Clean/Join + prefixo → os.ReadFile
Python  pypi/trackfw/serve/api_file.py:11  os.path.realpath NOS DOIS LADOS   ✅
```

🔴 **É a mesma classe que este projeto vem pagando o dia inteiro:** a guarda mede uma coisa e o uso
consome outra. Aqui a distância entre as duas é um `readlink`.

## O Python é a referência — não inventar desenho

`_is_safe_path` já faz o certo: `realpath` na raiz **e** no arquivo, e compara com separador ao final
para não casar `/docs/adr` com `/docs/adr2`. **Portar essa função**, como fizemos no `serve` do browser,
onde os ramos corretos do Python serviram de modelo.

## M-03 entra AQUI — mesma causa

`npm/src/commands/serve.js:141-164` serve `/static/...` com `path.resolve()` e `readFileSync`, sem
`realpath`. Severidade menor (o diretório vem do pacote instalado), **mas é o mesmo mecanismo**. Por
`Regra Dura de Causa Raiz`: mesma causa ⇒ mesma REQ ⇒ mesmo PR.

## ⚠️ Nota de método — quase descartei este achado

A minha primeira reprodução deu **403** e eu quase classifiquei o relatório como falso positivo. O
motivo era o caminho: usei `$(mktemp -d)` cru (`/var/...`) enquanto o `process.cwd()` do Node devolve
`/private/var/...`. Com `pwd -P`, o mesmo teste devolve **200 e o segredo**.

🔴 **Terceira vez em 24h que a divergência `/var` × `/private/var` no macOS produziu leitura falsa** —
e esta teria custado um achado de severidade alta. Vale nota de vault própria.

## Acceptance Criteria

- [ ] **AC1** — Go e Node canonicalizam **a raiz autorizada e o arquivo pedido** antes de comparar, e
      recusam destino cuja canonicalização caia fora. Manter a regra de prefixo **com separador**.
- [ ] **AC2** — M-03: o servidor de estáticos do Node usa o mesmo tratamento.
- [ ] **AC3** — resposta **403** e 🔴 **ausência do conteúdo no corpo** — asserir as duas coisas. Só
      checar o status deixa passar um vazamento com código errado.
- [ ] **AC4** — 🔴 **Contra-braço:** arquivo legítimo dentro da raiz continua devolvendo **200** com o
      conteúdo. Guarda que só recusa é indistinguível de guarda que recusa sempre.
- [ ] **AC5** — teste do vetor nos **3 runtimes**, inclusive no Python, que hoje passa: a defesa dele
      não tem teste que a prove, e defesa sem teste é a próxima a ser "simplificada".
- [ ] **AC6** — gate de segurança do servidor cobre o caso, com falsificação: revertendo o `realpath`,
      o gate **reprova**.
- [ ] **AC7** — 🔴 **Varredura:** derivar **todo** sítio dos 3 runtimes que valida caminho
      lexicamente e depois abre. **Comando escrito.** `api_file` e os estáticos são dois; a pergunta é
      se são os únicos.

## Fora desta REQ

M-04 (ausência de autenticação e CORS `*` quando `--host` não é loopback) — o próprio parecer classifica
como **residual de desenho declarado**, não defeito oculto. Merece decisão registrada, não este PR.
