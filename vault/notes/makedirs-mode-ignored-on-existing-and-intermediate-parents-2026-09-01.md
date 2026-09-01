---
date: 2026-09-01
author: hades-tf
tags: [python, filesystem, toctou, security]
---

# `mode=` de `os.makedirs`/`Path.mkdir` não garante o modo pedido em dois casos comuns

## Contexto

`identity/__init__.py`, `integrations/manager.py` e `thirdparty/quarantine.py` chamam
`os.makedirs(directory, exist_ok=True, mode=0o700)` (ou `Path(...).mkdir(parents=True,
exist_ok=True, mode=0o700)`) antes de escrever um arquivo sensível via `_atomic_write`, presumindo
que isso garante um diretório-pai restritivo (`0o700`) — pré-requisito para o argumento de que
`os.fchmod(fd)` não precisa de proteção adicional contra TOCTOU (ver
`docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md`).

## O que é verdade, comprovado por execução

1. **`mode=` é ignorado quando o diretório já existe.** Se outro código já criou `.trackfw` antes
   (ex.: `update_harness.py` instala `~/.trackfw/scripts/*.sh` via `Path(path).parent.mkdir(parents=True,
   exist_ok=True)`, **sem** `mode=`), `.trackfw` nasce em `0o755` (umask padrão). Uma chamada
   posterior com `mode=0o700` para o MESMO diretório **não muda nada** — o `mode=` só se aplica na
   criação.
2. **`mode=` só se aplica à FOLHA, nunca aos pais intermediários criados no mesmo `mkdir(parents=True)`.**
   Mesmo num projeto totalmente novo, `Path(root / ".trackfw" / "thirdparty-quarantine").mkdir(parents=True,
   mode=0o700)` cria `.trackfw` (intermediário) em `0o755` e só `thirdparty-quarantine` (folha) em
   `0o700`.

## Por que isso importa

Qualquer código que presuma "o diretório-pai da minha escrita atômica já é restritivo porque eu
passei `mode=0o700`" está enganado, a menos que o diretório em questão SEMPRE seja a folha de uma
criação `mkdir(parents=True, mode=...)` e NUNCA tenha sido criado antes por outro caminho sem
`mode=`. Em projetos com múltiplos subsistemas escrevendo sob o mesmo `.trackfw/` (scripts, adr,
config, manifestos, identidade), isso quase nunca se sustenta na prática.

## Como verificar rápido

```python
import os, tempfile
d = tempfile.mkdtemp()
os.makedirs(os.path.join(d, "a"), exist_ok=True)              # sem mode
os.makedirs(os.path.join(d, "a"), exist_ok=True, mode=0o700)  # mode= ignorado, já existe
print(oct(os.stat(os.path.join(d, "a")).st_mode & 0o777))     # 0o755, não 0o700
```

## Remédio (não implementado — fora do escopo do ML-0A que gerou esta nota)

Depois de `makedirs`/`mkdir`, aplicar `os.chmod(directory, 0o700)` explicitamente, sempre — nunca
confiar no argumento `mode=` sozinho quando o diretório pode já existir ou ter pais intermediários
criados na mesma chamada.

## Relacionado

Ver `docs/seguranca/2026-09-01-modelo-de-ameaca-da-escrita-atomica-no-windows.md` seções 1 e 5 para
o modelo de ameaça completo (quando isso vira TOCTOU explorável, e sob qual condição — umask não
padrão ou diretório relaxado manualmente).
