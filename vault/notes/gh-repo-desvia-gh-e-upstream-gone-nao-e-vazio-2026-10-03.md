# `GH_REPO` desvia o `gh` sem erro, e upstream `[gone]` não é upstream vazio

> Data: 2026-10-03 | REQ-2026-10-03 (#481), PR #515 | Medido pelo hades-tf (Wave 0/2), conferido pelo arquiteto

## 1. `gh pr list` sem `--repo` confia no ambiente

Com `GH_REPO=cli/cli`, `gh pr list` devolve os PRs do **cli/cli**, com exit 0 e JSON válido. Com mais de
um remoto, o `gh` também pode escolher outro repositório. Um consumidor que decide **apagar** com base
nessa resposta apaga com dados alheios.

```bash
GH_REPO=cli/cli gh pr list --state merged --limit 1 --json url --jq '.[0].url'                 # cli/cli/pull/...
gh pr list --repo github.com/kgsaran/trackfw --state merged --limit 1 --json url --jq '.[0].url'  # kgsaran/trackfw
```

**Regra:** toda chamada ao `gh` que alimenta uma decisão passa `--repo HOST/OWNER/REPO` derivado de
`git remote get-url origin`. O `--repo` prevalece sobre `GH_REPO`.

## 2. Upstream `[gone]`

- `git for-each-ref --format='%(upstream:short)' refs/heads/X` **devolve o nome** (`origin/X`) mesmo
  quando o remoto apagou a branch. "Vazio" significa nunca empurrada; `[gone]` aparece em
  `%(upstream:track)`.
- `git rev-parse X@{u}` e `@{u}` **falham com exit 128** numa branch `[gone]`. Não sirvam para
  "tem upstream?".

Um stub de teste que simulava `[gone]` como upstream vazio passou na auditoria do ML-1A. Quem o pegou
foi a medição com git real (ML-2A, L3). Fixture de upstream se faz com remoto bare em `t.TempDir()`.

## 3. Janela do `gh pr list`

O padrão é `--limit 30`. Uma medição feita com os "últimos 100 PRs" deu 0 falsos positivos porque as
branches estavam fora da janela (comentário do consumidor na #481). Resposta com itens == limite é
**incompleta**, não "sem PR".

## 4. Binário instalado ≠ binário da branch

Ao medir o efeito de uma mudança no `push`/`ship`, o `trackfw` do PATH (Homebrew) é a versão
publicada. Em 2026-10-03 ele emitiu o aviso antigo e quase virou um falso bug. Para medir, use
`go build -o <tmp>/tf ./cmd/trackfw` e rode `<tmp>/tf`.
