---
name: medir-antes-e-depois-sem-git-stash
description: Para o "antes e depois" literal que o relatório exige, monte a árvore pré-fix com `git archive HEAD | tar -x` num scratch e compile os dois binários — nunca git stash/checkout
metadata:
  type: feedback
---

Quando o relatório exigir medição **antes e depois** com saída literal, não tente reverter a árvore
de trabalho. Extraia a árvore pré-fix para o scratch e compile os dois binários:

```bash
git archive HEAD | tar -x -C "$S/prefix-src"      # leitura pura, não mexe na árvore nem no index
(cd "$S/prefix-src" && go build -o "$S/trackfw-prefix" ./cmd/trackfw)
go build -o "$S/trackfw-fixed" ./cmd/trackfw       # árvore de trabalho, já corrigida
```

Depois exercite os dois **sobre uma CÓPIA do corpus real** (`cp -R docs/roadmaps docs/req` + um
`trackfw.yaml` no fixture), nunca na árvore do projeto — um comando destrutivo pré-fix moveria
arquivo de governança de verdade.

**Why:** este papel não tem autoridade Git e o hook do repositório intercepta `stash`/`checkout --`
por desenho; `git archive` e `git show` são leitura e passam. Sem isso, a única evidência possível
seria mutação temporária do fonte, que é mais frágil (e se a sessão morrer no meio, a árvore fica
sabotada). Em 2026-09-26, no ML-1C, foi esse par de binários que revelou o achado que nenhuma
inspeção de código tinha previsto — o nome **completo e exato** movia o arquivo errado, não só o
nome vazio.

**How to apply:** vale para qualquer ML cujo defeito seja observável pela CLI. Mutação de fonte
(`perl -0pi` + `cp` de restauração) continua sendo a ferramenta certa para falsificar **testes**
(provar que o teste novo reprova sem o fix); `git archive` é a ferramenta certa para medir
**comportamento de produto**. Ver [[metrica-por-artefato]] — meça no artefato em disco, não só no rc.
