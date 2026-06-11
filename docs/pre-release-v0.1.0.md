# Pendências para o Release v0.1.0

> Criado em: 2026-06-11 | Responsável: kgsaran

---

## Antes de criar a tag `v0.1.0`

### 1. PAT para o Homebrew tap (obrigatório)

O GoReleaser precisa de permissão para fazer push da fórmula em `kgsaran/homebrew-trackfw`.

**Passos:**
1. GitHub.com → Settings → Developer settings → Personal access tokens → Generate new token
2. Scope: `repo` (acesso de escrita a repositórios)
3. Nome sugerido: `GORELEASER_HOMEBREW_TAP`
4. Repositório `kgsaran/trackfw` → Settings → Secrets and variables → Actions → New repository secret
   - Nome: `HOMEBREW_TAP_GITHUB_TOKEN`
   - Valor: o PAT gerado acima

### 2. Criar e fazer push da tag

```bash
git tag v0.1.0
git push origin v0.1.0
```

Isso aciona o workflow `.github/workflows/release.yml` que:
- Compila binários para linux/darwin amd64+arm64 e windows amd64
- Cria o release no GitHub com os artefatos e checksums
- Atualiza `Formula/trackfw.rb` em `kgsaran/homebrew-trackfw` com URLs e SHA256 reais

### 3. Publicar no npm

```bash
cd npm
npm publish
```

Requer login: `npm login` (conta com acesso ao pacote `trackfw` no npm registry).

### 4. Publicar no PyPI

```bash
cd pypi
python -m build
python -m twine upload dist/*
```

Requer conta no PyPI com o pacote `trackfw` registrado.

---

## Verificação pós-release

- [ ] `curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh` funciona
- [ ] `npm install -g trackfw && trackfw version` funciona
- [ ] `pip install trackfw && trackfw version` funciona
- [ ] `brew install kgsaran/tap/trackfw && trackfw version` funciona
- [ ] Página do release no GitHub tem todos os artefatos e checksums

---

## Estado atual do projeto (2026-06-11)

### Implementado e pronto
- CLI scaffold: `init`, `adr`, `req`, `roadmap`, `status`, `validate`
- `trackfw validate` com 7 regras de governança (violations + warnings)
- Slash commands `trackfw:` para Claude Code e Gemini CLI (adr, req, roadmap, status, validate, move)
- GoReleaser configurado (5 targets de plataforma)
- npm wrapper (postinstall downloader)
- PyPI wrapper (lazy download on first run)
- Homebrew tap configurado (aguardando PAT)
- 14 testes unitários (14/14 PASS)
- README.md completo
- Install script (`scripts/install.sh`)

### Não implementado (roadmap v0.2+)
- `trackfw roadmap show <name>` — render do progresso no terminal
- `trackfw adr list` / `trackfw req list`
- `trackfw log` — histórico de transições de estado
- Plugin system (`trackfw plugins list/add/remove`)
- Detecção de entradas stale em wip/ (em wip por > N dias)
