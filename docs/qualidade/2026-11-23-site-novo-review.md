# Parecer de Qualidade — ML-4A: Site novo do trackfw

> Hefesto-tf | HEAD 910762f6 | Branch `feat/site-do-trackfw-visao-do-produto` | 2026-11-24

---

## Veredito: APROVA com ressalvas

Nenhum achado bloqueante. Todos os gates de medição passam. Dois achados não-bloqueantes têm
correção concreta.

---

## Gates de medição executados

| Gate | Comando | Resultado |
|------|---------|-----------|
| Leak scanner | `bash scripts/check-site-images-leak.sh` | rc=0 — 7 canários ok, 10 arquivos limpos |
| Parity-rest | `make parity-rest` | rc=0 |
| Build do site | `(cd site && npm run build)` | rc=0 — build complete in 1.40s |
| TestQuickStart | `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m -run 'TestQuickStart' ./internal/commands/` | ok (cached) |
| Protótipo removido | `test ! -e site/prototype.md` | OK |
| grep-q gate | `bash scripts/check-no-grep-q-under-pipefail.sh` | OK |
| trap-local-var gate | `bash scripts/check-no-trap-local-var.sh` | OK |
| mktemp-cleanup gate | `bash scripts/check-mktemp-cleanup.sh` | OK |
| orphan-gates | `bash scripts/check-orphan-gates.sh` | OK — `check-site-images-leak.sh` tem consumidor |

---

## Achados

### Achado 1 — Caminho de Chrome hardcoded com username real (não-bloqueante, portabilidade)

**Arquivo:** `scripts/site-images/generate.sh:38`

**Evidência:**
```
CHROME="/Users/kgsaran/Library/Caches/ms-playwright/chromium_headless_shell-1208/chrome-headless-shell-mac-arm64/chrome-headless-shell"
```

O script lida corretamente com Chrome ausente (WARNING + skip), então nenhum outro contribuidor
é bloqueado — mas `board.png` só pode ser regenerado nesta máquina específica. O número de versão
pinado (`1208`) fará o skip silencioso quando o Playwright atualizar.

**Correção sugerida:** substituir a constante por:
```bash
CHROME="${CHROME:-}"
if [ -z "$CHROME" ]; then
  # Tenta o caminho padrão do Playwright no macOS arm64
  _candidate="/Users/${USER}/Library/Caches/ms-playwright/chromium_headless_shell-1208/chrome-headless-shell-mac-arm64/chrome-headless-shell"
  [ -x "$_candidate" ] && CHROME="$_candidate"
fi
```
Ou documentar no comentário do script que o usuário deve exportar `CHROME=<caminho>`.

---

### Achado 2 — Canário G é hollow: não exercita o caminho de FAIL (não-bloqueante, qualidade de teste)

**Arquivo:** `scripts/check-site-images-leak.sh:208-219`

**Evidência — o canário verifica apenas que o arquivo não existe:**
```bash
local transcript_g="$tmpdir/canary-g.txt"
if [ -e "$transcript_g" ]; then
  ...fail...
else
  echo "  SELF-TEST OK: canary G (media without transcript) would be flagged by transcript check"
  pass=$((pass + 1))
fi
```

A declaração "would be flagged" é correta — mas o self-test não chama a lógica real
(`requires_transcript` + loop principal). Se as linhas 305-312 (a checagem de transcript sibling)
fossem removidas acidentalmente, o Canário G continuaria passando.

**Correção sugerida:** fazer o canário rodar a scan completa sobre o `$tmpdir` com apenas
`canary-g.gif` (sem o `.txt`) e verificar que o exit code é 1:
```bash
# Canary G — invocar o scan real sobre tmpdir com só o media stub
_scan_exit=0
( IMG_DIR="$tmpdir" bash "$0" 2>/dev/null ) || _scan_exit=$?
if [ "$_scan_exit" -ne 0 ]; then
  echo "  SELF-TEST OK: canary G (media sem transcript) correctly caught by full scan"
  pass=$((pass + 1))
else
  echo "SELF-TEST FAIL: canary G — full scan não detectou media sem transcript" >&2
  fail=$((fail + 1))
fi
```
Nota: isso exige que o self-test aconteça num tmpdir isolado sem os outros canários (que levariam
o scan a falhar por outros motivos), ou que seja testado em subshell com IMG_DIR apontando
só para o `canary-g.gif`.

---

### Achado 3 — Ícones de feature card sem aria-hidden (não-bloqueante, acessibilidade)

**Arquivos:** `site/index.md:387,392,397,402` e `site/pt/index.md:387,392,397,402`

**Evidência — 4 divs em cada arquivo, 8 ocorrências totais:**
```html
<div class="icon">🔗</div>
<div class="icon">🤖</div>
<div class="icon">🌐</div>
<div class="icon">📦</div>
```

Emojis decorativos sem `aria-hidden="true"` são anunciados por screen readers (VoiceOver: "link
symbol", "robot face" etc.). WCAG 1.1.1: conteúdo não-textual decorativo deve ser escondido do AT.

**Correção sugerida:** em ambos os arquivos:
```html
<div class="icon" aria-hidden="true">🔗</div>
```

---

## Itens verificados sem achado

| Item | Verificação | Resultado |
|------|-------------|-----------|
| Protótipo removido (D8) | `test ! -e site/prototype.md` | OK |
| Permissões do deploy (D6) | `build: contents: read` apenas; `deploy: pages/id-token write` | OK |
| Sem permissões no nível do workflow | `deploy-docs.yml` | OK — bloco workflow-level ausente |
| Paridade EN/PT | Contagem de elementos estruturais, links, imagens | Idênticos |
| `<video controls>` sem autoplay/loop (D7, WCAG 2.2.2) | EN e PT | OK |
| `<video>` com poster, aria-label e fallback | EN e PT | OK |
| Transcript do vídeo em `<details>` | EN e PT | OK |
| chain-diagram com aria-label e arrows com aria-hidden | EN e PT | OK |
| Links internos EN e PT com prefixo de locale correto | Todos os hrefs verificados | OK |
| Paths de imagem `/img/...` com base `/trackfw/` | Verificado no HTML compilado (`dist/index.html`) | OK — VitePress prefixa para `/trackfw/img/...` |
| scanner leak: 7 canários (A–G) + varredura | 10 arquivos, nenhum vazio | OK |
| `generate.sh`: isolamento (HOME, PS1, TZ, GIT_AUTHOR/COMMITTER, tokens unset) | Leitura do script | OK — conformidade com D2 |
| `rewrite_tape`: substitui Output .gif/webm/mp4 e .txt e Screenshot | Leitura do script | OK |
| `demo.tape` não tem Screenshot pré-existente | `grep -n Screenshot demo.tape` | OK — awk append é único |
| Transcript do Quick start termina com 2 warnings | Verificado pela geração + gate no generate.sh | OK (conferido por ML-3D) |
| `guard.tape`: setup hidden, init com `</dev/null >/dev/null 2>&1` | Leitura do tape | OK |
| `quickstart.tape`: função `trackfw()` não-TTY, init suprimido | Leitura do tape | OK |
| Canários A–F do scanner exercitam caminhos de FAIL reais | Leitura do self-test | OK (só G é hollow) |
| `check-no-grep-q-under-pipefail.sh` nos novos scripts | rc=0 | OK |
| `check-no-trap-local-var.sh` nos novos scripts | rc=0 | OK |
| `check-mktemp-cleanup.sh` nos novos scripts | rc=0 | OK |
| `check-site-images-leak.sh` registrado como não-órfão | Consumidor: `generate.sh:502` | OK |
| Peso total das imagens | Soma: ~1,5 MB (10 arquivos) | Dentro do residual declarado no ADR |

---

## Contagem de achados

| Classe | Quantidade |
|--------|-----------|
| Bloqueante | 0 |
| Não-bloqueante | 3 |

**Achados não-bloqueantes:**
1. Chrome hardcoded com username e versão — portabilidade
2. Canário G hollow — não exercita o caminho de FAIL real
3. Ícones de feature card sem aria-hidden — WCAG 1.1.1

---

## Estado do git após a revisão

```
git status --porcelain: (clean, apenas o arquivo de parecer)
```

Verificado: `git status --porcelain` retornou vazio antes da criação deste arquivo.
Este arquivo é o único artefato escrito nesta sessão.
