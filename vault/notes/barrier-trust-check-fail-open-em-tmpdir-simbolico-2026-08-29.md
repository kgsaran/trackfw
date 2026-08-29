# `roadmapTrustForGates` cai em fail-open quando o sandbox de teste vive sob um `$TMPDIR` com componente simbólico (macOS)

> 2026-08-29 · achado escrevendo `scripts/check-roadmap-barrier-contract.sh`
> (ML-3A, `ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md`)
> Domínio: `internal/commands/barrier.go` (`roadmapTrustForGates`) — presumivelmente também
> `npm/src/commands/barrier.js` / `pypi/trackfw/commands/barrier.py`, não confirmado nos outros
> dois runtimes por este achado.

## O sintoma

Um gate que varre roadmaps históricos copiando-os para um sandbox git (sem `--trust-local-gates`,
esperando que o trust-check falhe FECHADO porque o arquivo nunca existe em `origin/main`) travou
por **8+ minutos rodando `make quality` de verdade** — um gate declarado dentro de um dos 144
roadmaps do corpus (`ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`).

O comando `git show origin/main:<path>` manual, rodado à mão no mesmo diretório, retornava
exatamente a mensagem que `roadmapTrustForGates` reconhece como "não confiável":

```
fatal: path '...' exists on disk, but not in 'origin/main'
```

Mesmo assim, `barrier` executou os gates como se o roadmap fosse confiável.

## Causa raiz

`roadmapTrustForGates` (`internal/commands/barrier.go:568-632`) calcula o caminho relativo do
roadmap em relação ao **toplevel físico do git**:

```go
topCmd := exec.Command("git", "rev-parse", "--show-toplevel")   // git SEMPRE resolve symlinks
...
absRoadmap, _ := filepath.Abs(roadmapPath)                       // Abs a partir do cwd do PROCESSO,
                                                                  // que NÃO resolve symlinks
relPath, _ := filepath.Rel(topLevel, absRoadmap)
```

`git rev-parse --show-toplevel` **sempre** devolve o caminho físico (segue todo symlink). Se o
processo do `barrier` foi iniciado (via `exec.Command...Dir = dir`, ou por um `cd` de shell) num
`dir` que contém um componente simbólico — no macOS, `$TMPDIR` por padrão é algo como
`/var/folders/xx/.../T/` → `/private/var/folders/xx/.../T/` — `filepath.Abs` produz um caminho
**não resolvido**, e `filepath.Rel(topLevel-resolvido, absRoadmap-não-resolvido)` produz um
`relPath` incorreto (tipicamente cheio de `../` de volta e o caminho completo de novo).

`git show origin/main:<relPath-errado>` então falha, mas com uma mensagem **diferente** das duas
que o código reconhece (`"does not exist in"` / `"exists on disk, but not in"`) — algo como
`fatal: invalid object name 'origin/main'.` ou um erro de path fora da árvore. Nenhum dos dois
padrões bate, o código cai no ramo genérico:

```go
// Any other failure (origin not configured, ref not fetched) → fail-open.
return gatesTrustVerdict{trusted: true}
```

E os gates do roadmap são executados **de verdade** — incluindo, neste caso, `make quality`
(~25 min) e `bin/trackfw validate --json` (que nem existia no sandbox, exit 127).

## Reprodução mínima

```bash
WORK=$(mktemp -d)                     # sob $TMPDIR simbólico no macOS
cd "$WORK" && git init -q && git config user.email a@a.com && git config user.name a
mkdir -p docs/roadmaps/wip && echo x >README.md && git add README.md
git commit -q -m init && git branch -M main
git init -q --bare "$WORK-origin.git"
git remote add origin "$WORK-origin.git" && git push -q origin main
cp <qualquer-roadmap-com-gate-declarado> docs/roadmaps/wip/x.md
trackfw barrier x --wave 1 --json   # gates EXECUTAM de verdade — bug
```

Corrigido resolvendo o sandbox para o caminho físico antes de qualquer `cd`:

```bash
WORK=$(cd "$WORK" && pwd -P)          # -P: physical, segue e resolve todo symlink
```

Com essa única linha, o mesmo cenário passa a reportar corretamente
`"gates": {"status": "not_evaluated", "failures": ["gates not evaluated: roadmap is not
committed in origin/main — pass --trust-local-gates to evaluate local gates"]}`.

## Por que isto importa além do gate que achou

Este não é só um problema de teste. **O mesmo caminho de código é o que protege o vetor de PR**
descrito em `docs/cli-parity.md` § "Trust and `--trust-local-gates`" e no
`ADR-2026-08-23`: um mantenedor rodando `trackfw barrier` direto (sem a flag) sobre um roadmap de
PR malicioso confia neste trust-check para NÃO executar gates arbitrários. Qualquer ambiente cujo
repositório clonado, `$HOME`, ou diretório de trabalho atravesse um componente simbólico do
sistema operacional (comum em macOS por padrão via `/tmp`→`/private/tmp` e `/var`→`/private/var`,
e em setups corporativos com home montado via automounter/NFS) está potencialmente sujeito ao
mesmo fail-open — **não confirmado em produção**, só neste sandbox de teste, mas o mecanismo é
idêntico e não depende de nada específico de teste.

## Escopo desta nota

Achado durante o ML-3A (que só pode tocar `scripts/`, `docs/cli-parity.md`, `Makefile` —
`internal/` fora de escopo). **Não corrigido aqui.** O gate que achou (
`scripts/check-roadmap-barrier-contract.sh`) mitiga resolvendo seu próprio `$WORK` com `pwd -P`
logo após o `mktemp -d`. O defeito em si é pré-existente em `barrier.go` (não introduzido pelas
Waves 1/2 deste roadmap) e deveria virar REQ própria: trocar `filepath.Abs` por uma resolução que
também siga symlinks (`filepath.EvalSymlinks`) antes do `filepath.Rel`, ou comparar caminhos após
resolver ambos os lados. Não verificado se `npm/src/commands/barrier.js` e
`pypi/trackfw/commands/barrier.py` compartilham o mesmo padrão (prováveis candidatos, mesma
arquitetura de trust-check) — quem abrir a REQ deve conferir os 3 runtimes.

## Relacionado

- `vault/notes/vies-do-tmp-ao-medir-sandbox-de-agente-2026-08-12.md` — mesma família de bug
  (`/tmp` no macOS não é o que parece), causa raiz diferente (lista de diretórios graváveis do
  agente vs. resolução de path do git), mesma lição de fundo: **nunca assumir que `mktemp -d`
  devolve um caminho "limpo"** em macOS.
- `docs/cli-parity.md` § "Trust and `--trust-local-gates`" → "Fail-open cases (declared
  residuals)" — a tabela de fail-opens *declarados* não inclui este caso porque ele não é uma
  decisão de desenho, é um bug de resolução de path.
