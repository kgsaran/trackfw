---
title: resolver-e-afirmar-contenção — path.resolve() do Node é puramente léxico, e as três primitivas divergem sobre caminho com folha inexistente
tags: [symlink, seguranca, guarda, paridade, gotcha, ancestral]
date: 2026-08-31
related: [[lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31]]
---

## Sintoma que este note previne

Um implementador da Wave 1 de
`ROADMAP-2026-08-31-guarda-de-folha-resolve-o-caminho-e-afirma-contencao-antes-de-escrever.md` lê a
REQ, vê a tripla `filepath.EvalSymlinks` / `fs.realpathSync` / `Path.resolve()` e — sem testar —
implementa o guard Node com `path.resolve()`, porque é o nome mais parecido com `Path.resolve()` do
Python. **Isso produz um guard Node que é um no-op**: passa em todo teste escrito contra o
comportamento do Go, porque `path.resolve()` nunca segue link nenhum.

## Medido (não hipótese)

```js
const path = require('path')
const fs = require('fs')
// caminho por trás de um symlink ANCESTRAL plantado (docs/req -> /outside)
const p = 'poc-project/docs/req/X.md'
path.resolve(p)        // → '.../poc-project/docs/req/X.md'   ← ainda "dentro", ERRADO
fs.realpathSync(p)      // teria que ser chamado, mas ERRA (ver abaixo) porque X.md não existe ainda
```

`path.resolve()` é **puramente léxico** (normaliza `.`/`..`, resolve contra cwd) — não toca o disco,
não segue symlink. O primitivo certo em Node é `fs.realpathSync`, não `path.resolve`.

## A segunda armadilha, ortogonal à primeira: as 3 primitivas divergem sobre folha inexistente

O caso dominante desta REQ é **criar** um arquivo novo (`req new`, `roadmap new`, `claude-skill` no
primeiro `--install-missing`) — a folha, por definição, ainda não existe no momento da checagem.

```
Go   filepath.EvalSymlinks(caminho-completo)   → ERRO "no such file or directory" (mesmo com o
                                                    diretório-pai existindo de verdade)
     filepath.EvalSymlinks(filepath.Dir(caminho)) → resolve sem erro

Node fs.realpathSync(caminho-completo)         → ERRO ENOENT, mesmo padrão do Go
     fs.realpathSync(path.dirname(caminho))     → resolve sem erro

Py   Path(caminho).resolve(strict=False)       → NÃO erra — resolve os componentes que existem e
                                                    mantém a folha inexistente anexada ao final,
                                                    sem precisar do truque de "resolver só o pai"
```

**Consequência de implementação, não teórica:** em Go e Node, o guard tem que resolver
`filepath.Dir(destino)` / `path.dirname(destino)` e só depois juntar a folha antes de comparar contra
`root`. Resolver o caminho completo do arquivo a criar falha com erro de I/O disfarçado de recusa de
segurança — quebra toda criação de arquivo novo, mesmo sem link algum na árvore. Em Python, a mesma
sequência ("resolver só o pai") também funciona, mas não é necessária — `resolve(strict=False)`
resolve o caminho completo direto. **Se a Wave 1 usar a mesma sequência de passos nos 3 runtimes,
Python fica OK e Go/Node quebram** — a REQ acertou ao nomear primitivas diferentes por runtime em vez
de uma fórmula única; isto é a evidência de por quê.

## Terceira armadilha: comparar destino resolvido contra `root` NÃO resolvido

```python
import os
root = '/tmp/x/poc-project'                                    # forma não resolvida
destination = os.path.realpath('/tmp/x/poc-project/docs/req/X.md')
destination.startswith(root)                     # → False, falso positivo — RECUSA legítima
destination.startswith(os.path.realpath(root))   # → True, correto
```

`/tmp` é symlink para `/private/tmp` no macOS (`os.path.islink('/tmp')` → `True`). Qualquer projeto
rodando sob um caminho com prefixo symlink de sistema (ou `$HOME` gerenciado por `chezmoi`/`stow`)
recebe recusa em operação 100% legítima se o guard resolver só um dos dois lados da comparação.
**Resolver os dois lados (root e destino) antes de comparar não é opcional.**

## Onde isto foi encontrado

`docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md`, ML-0A da REQ
`REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-...`. Os três achados (primitiva
Node errada, folha inexistente, root não resolvido) foram medidos ao vivo antes de escrever o
parecer, não inferidos.

## Rastreamento

- `docs/seguranca/2026-08-31-modelo-de-ameaca-da-guarda-de-ancestral.md` — parecer completo, inclui
  dois PoCs de exploração ao vivo (`trackfw req new` e `trackfw update harness` escrevendo fora da
  árvore/`$HOME` via symlink ancestral).
- `docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md`
