# Roadmap: Wrapper npm

> Criado em: 2026-06-11 | Status: ✅ Done

## Contexto

Publicar o `trackfw` CLI no npm como pacote `trackfw` usando o padrão "postinstall binary downloader" — o mesmo usado por Biome, esbuild e similares. O pacote npm é um wrapper fino: no `postinstall`, baixa o binário correto das GitHub Releases (geradas pelo GoReleaser) para a plataforma do usuário; o `bin/trackfw` é um wrapper JS que localiza e executa o binário.

**Repositório:** `github.com/kgsaran/trackfw` (privado por ora)
**Package name npm:** `trackfw`
**Formato de arquivo GoReleaser:** `trackfw_<VERSION>_<os>_<arch>.tar.gz`
**Versão inicial:** `0.1.0`

---

## Wave 1 — Três arquivos em paralelo

> Dependências: independentes entre si

### ML-1A — `npm/package.json`
**Status:** ⬜ Pendente
**Arquivo:** `npm/package.json`
**Conteúdo:**
```json
{
  "name": "trackfw",
  "version": "0.1.0",
  "description": "Governed software delivery framework: ADR → REQ → ROADMAP → kanban",
  "keywords": ["cli", "adr", "roadmap", "governance", "delivery"],
  "homepage": "https://github.com/kgsaran/trackfw",
  "repository": { "type": "git", "url": "https://github.com/kgsaran/trackfw" },
  "license": "MIT",
  "bin": { "trackfw": "./bin/trackfw" },
  "scripts": { "postinstall": "node scripts/postinstall.js" },
  "files": ["bin/", "scripts/"],
  "engines": { "node": ">=14" },
  "os": ["linux", "darwin", "win32"],
  "cpu": ["x64", "arm64"]
}
```

**Critérios de aceite:**
- [ ] JSON válido
- [ ] `bin.trackfw` aponta para `./bin/trackfw`
- [ ] `postinstall` aponta para `node scripts/postinstall.js`
- [ ] `files` contém `bin/` e `scripts/` (não incluir código Go no pacote)

---

### ML-1B — `npm/scripts/postinstall.js`
**Status:** ⬜ Pendente
**Arquivo:** `npm/scripts/postinstall.js`
**Lógica:**
1. Detectar `process.platform` → mapear: `linux` → `linux`, `darwin` → `darwin`, `win32` → `windows`
2. Detectar `process.arch` → mapear: `x64` → `amd64`, `arm64` → `arm64`
3. Ler `version` do `package.json` do próprio pacote (ex: `0.1.0`)
4. Montar URL: `https://github.com/kgsaran/trackfw/releases/download/v${VERSION}/trackfw_${VERSION}_${OS}_${ARCH}.tar.gz` (ou `.zip` no Windows)
5. Baixar com `https` nativo do Node (sem deps externas)
6. Extrair: `tar.gz` → usar `child_process.execSync('tar -xzf ...')` ; `.zip` no Windows → `child_process.execSync('powershell Expand-Archive ...')`
7. Mover binário extraído para `../bin/trackfw` (ou `../bin/trackfw.exe` no Windows)
8. `chmod +x` no binário (exceto Windows)
9. Se plataforma não suportada: `console.warn` e `process.exit(0)` (não falhar a instalação — evitar bloquear CIs)

**Critérios de aceite:**
- [ ] Sem dependências externas (somente módulos nativos do Node)
- [ ] Plataforma não suportada → `exit(0)` (não bloqueia install)
- [ ] Binário extraído em `bin/trackfw` (ou `bin/trackfw.exe`)
- [ ] `chmod +x` aplicado no Unix

---

### ML-1C — `npm/bin/trackfw`
**Status:** ⬜ Pendente
**Arquivo:** `npm/bin/trackfw` (sem extensão; shebang `#!/usr/bin/env node`)
**Lógica:**
1. Determinar caminho do binário: `path.join(__dirname, '..', 'bin', 'trackfw')` (ou `.exe` no Windows)
2. Verificar se o binário existe; se não, exibir mensagem de erro pedindo para reinstalar
3. `child_process.spawnSync(binaryPath, process.argv.slice(2), { stdio: 'inherit' })`
4. `process.exit(result.status ?? 1)`

**Critérios de aceite:**
- [ ] Shebang `#!/usr/bin/env node` na primeira linha
- [ ] Passa todos os args corretamente (`process.argv.slice(2)`)
- [ ] `stdio: 'inherit'` (stdin/stdout/stderr passam direto ao binário)
- [ ] Trata binário não encontrado com mensagem útil

---

## Wave 2 — Validação local

> Dependências: Wave 1 completa

### ML-2A — Testar `npm pack` e estrutura do pacote
**Status:** ⬜ Pendente
**Comandos:**
```bash
cd npm/
chmod +x bin/trackfw
npm pack --dry-run    # listar arquivos que serão publicados
```

**Critérios de aceite:**
- [ ] `npm pack --dry-run` não retorna erro
- [ ] Listagem inclui `bin/trackfw`, `scripts/postinstall.js`, `package.json`
- [ ] NÃO inclui arquivos Go (`.go`, `go.mod`, binários de `dist/`)

---

## Ordem de execução

```
Wave 1: ML-1A ║ ML-1B ║ ML-1C  (paralelo)
               ↓
Wave 2: ML-2A  (validação)
```
