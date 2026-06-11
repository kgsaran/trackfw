# Roadmap: GoReleaser — Pipeline de Release de Binários

> Criado em: 2026-06-11 | Status: ✅ Done

## Contexto

Implementar pipeline completo de release de binários para o `trackfw` CLI usando GoReleaser + GitHub Actions. O objetivo é que qualquer `git push tag v*` dispare automaticamente a compilação cross-platform e publique os binários nas GitHub Releases — seguindo o padrão de distribuição de ferramentas como esbuild, Biome e Turbo.

**Repositório:** `github.com/kgsaran/trackfw` (privado por ora)
**Module path:** `github.com/kgsaran/trackfw`
**Entry point:** `cmd/trackfw/main.go`

---

## Wave 1 — GoReleaser Config + GitHub Actions (2 MLs paralelos)

> Dependências: independente entre si

### ML-1A — `.goreleaser.yaml`
**Status:** ✅ Concluído
**Arquivo:** `.goreleaser.yaml` (raiz do projeto)
**Ações:**
- `project_name: trackfw`
- `builds`: entry `./cmd/trackfw`, binário `trackfw`, targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- `archives`: formato `tar.gz` para linux/darwin, `zip` para windows; name template: `trackfw_{{ .Version }}_{{ .Os }}_{{ .Arch }}`
- `checksum`: `sha256sums.txt`
- `changelog`: gerado automaticamente a partir dos commits (filtrar Merge commits)
- `release`: draft `false`, replace existing assets `true`
- Não incluir `snapshot` no config de produção (snapshot é só para teste local)

**Critérios de aceite:**
- [ ] `goreleaser check` passa sem erros (após instalar goreleaser localmente)
- [ ] `goreleaser release --snapshot --clean` gera binários em `dist/` para as 5 plataformas

---

### ML-1B — `.github/workflows/release.yml`
**Status:** ✅ Concluído
**Arquivo:** `.github/workflows/release.yml`
**Ações:**
- Trigger: `on: push: tags: ['v*']`
- Job `release`: `runs-on: ubuntu-latest`
- Steps:
  1. `actions/checkout@v4` com `fetch-depth: 0` (necessário para changelog do goreleaser)
  2. `actions/setup-go@v5` com `go-version-file: go.mod` e `cache: true`
  3. `goreleaser/goreleaser-action@v6` com `version: latest`, `args: release --clean`
  4. Env: `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}`
- Permissões: `contents: write` (para criar releases)

**Critérios de aceite:**
- [ ] YAML válido (sem erros de sintaxe)
- [ ] `actions/checkout` com `fetch-depth: 0` presente
- [ ] `GITHUB_TOKEN` configurado corretamente

---

## Wave 2 — Atualizar `install.sh` para usar GitHub Releases

> Dependências: Wave 1 completa (precisa do formato de arquivo definido no .goreleaser.yaml)

### ML-2A — `scripts/install.sh` (atualizar URL de download)
**Status:** ✅ Concluído
**Arquivo:** `scripts/install.sh`
**Ações:**
- Substituir qualquer URL placeholder por `https://github.com/kgsaran/trackfw/releases/download/v${VERSION}/trackfw_${VERSION}_${OS}_${ARCH}.tar.gz`
- Detectar OS (`uname -s`) e ARCH (`uname -m`) e mapear para os nomes do goreleaser (ex: `arm64` → `arm64`, `x86_64` → `amd64`, `Darwin` → `darwin`, `Linux` → `linux`)
- Fallback para exibir mensagem de erro se a plataforma não for suportada

**Critérios de aceite:**
- [ ] Script detecta corretamente OS e ARCH
- [ ] URL de download segue exatamente o padrão do `.goreleaser.yaml`
- [ ] `shellcheck scripts/install.sh` sem erros críticos

---

## Wave 3 — Teste local do pipeline

> Dependências: Wave 1 + Wave 2 completas

### ML-3A — Instalar GoReleaser e validar snapshot
**Status:** ✅ Concluído
**Comandos:**
```bash
brew install goreleaser
goreleaser check            # valida .goreleaser.yaml
goreleaser release --snapshot --clean   # gera dist/ sem publicar
ls dist/                    # confirmar presença dos 5 binários
```

**Critérios de aceite:**
- [ ] `goreleaser check` sem erros
- [ ] `dist/` contém binários para: `linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64`, `windows_amd64`
- [ ] Build verde: `go build ./...`

---

## Ordem de execução

```
Wave 1: ML-1A ║ ML-1B  (paralelo)
               ↓
Wave 2: ML-2A
               ↓
Wave 3: ML-3A  (validação local)
```
