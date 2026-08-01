# Armadilhas ao verificar XSS no drawer via navegador

> Data: 2026-07-31 | Autor: Ártemis (via Zeus) | Domínio: serve / testes / segurança

Registrado durante o seam de falsificação da sanitização com DOMPurify (ML-2A da REQ
`REQ-2026-07-31-sanitizar-html-do-drawer-do-dashboard-com-dompurify`). Três comportamentos
que fazem um teste **parecer** que passou — ou que o seam está morto — sem que esteja.

## 1. `<script>` via `innerHTML` NUNCA executa — não é mérito do sanitizador

Ao provar a vivacidade do seam (remover a sanitização e confirmar que o payload volta a
executar), o vetor `<script>window.__FLAG=true</script>` **não dispara nem com a sanitização
removida**. Isso é comportamento da especificação HTML: elementos `<script>` inseridos via
`innerHTML` não são executados pelo navegador.

**Consequência perigosa:** se você concluir "a flag não foi definida, logo estamos protegidos",
a asserção é **vacuosa** — ela passaria igualmente com o sanitizador desligado.

**Como provar corretamente:** use **diferencial de presença do nó**, não de flag.
Com sanitização: `document.querySelector('script')` dentro do drawer → ausente.
Sem sanitização: presente. O que muda é a existência do nó, não a execução.

Para um vetor que realmente executa, use `<img src=x onerror="window.__FLAG=true">` — e
**aguarde** (~700 ms), porque `onerror` é assíncrono.

## 2. Vetores em tags bloqueadas provam pouco

`<img>` e `<script>` estão fora da allowlist, então some a tag inteira. Isso passa mesmo que a
**filtragem de atributos** esteja quebrada. Um seam que só testa isso é fraco.

Teste sempre vetores em tags **permitidas**:

- `<a href="ok.md" onclick="...">` → o `onclick` deve sumir e o `href` deve **sobreviver**.
  Prova a filtragem de atributos e, de quebra, que o interceptador de links `.md` continua tendo
  o `href` de que depende.
- `<a href="javascript:...">` → o `href` deve ser removido/neutralizado, provando o
  `ALLOWED_URI_REGEXP`.

## 3. Duas armadilhas de instrumentação no `app.js`

**`closeDrawer()` não readiciona a classe `hidden`.** Em `app.js` (~992), o fechamento faz
`drawer.style.display = 'none'` mas nunca restaura a classe `hidden` que `openDrawer()` removeu.
Verificar visibilidade só por `classList.contains('hidden')` dá **falso negativo** — o drawer
aparece como "aberto" depois de fechado. Use `getComputedStyle(el).display`.

**`_drawerPath` não é propriedade de `window`.** É declarado com `let` no topo do arquivo, o que
cria binding léxico de módulo/script, não propriedade do objeto global. `window._drawerPath` é
`undefined`. Em `Runtime.evaluate` (CDP) ou no console, use o identificador puro `_drawerPath`.

## ✅ CORRIGIDO — links `.md` relativos com `../` davam 403

O interceptador de links passa o `href` **bruto** para `openDrawer`, e o whitelist de
`api_file.go` rejeita caminhos com `../`. Verificado:
`GET /api/file?path=../roadmaps/done/v2.3-validator-improvements-2026-06-13.md` → **403**.

Documentos reais usavam essa forma — por exemplo
`docs/req/REQ-2026-06-13-validator-improvements.md`.

**Corrigido em 2026-08-01 no PR #98** (ADR
`ADR-2026-08-01-resolucao-de-links-markdown-relativos-ao-documento-aberto-no-drawer`), exatamente
como sugerido aqui: `resolveRelativeMdHref` normaliza o href contra o `dirname` do documento
aberto antes de chamar `openDrawer`. O whitelist do servidor **não** foi alterado — `vault/`
segue fora, e link que resolva para fora dos diretórios permitidos passa a exibir
`Arquivo fora dos diretórios permitidos: <caminho resolvido>` em vez de `Forbidden` cru.

**As três armadilhas de instrumentação acima CONTINUAM VÁLIDAS** — não foram alteradas por aquele
PR, e seguem sendo o motivo principal desta nota existir.

Relacionado: `vault/notes/security-drawer-marked-parse-unsanitized-stored-xss-2026-07-31.md`,
`vault/notes/dashboard-serve-e-light-only-2026-07-31.md`.
