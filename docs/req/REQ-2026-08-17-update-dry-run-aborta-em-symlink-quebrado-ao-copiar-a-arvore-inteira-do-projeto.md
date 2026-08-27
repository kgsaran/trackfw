---
status: done
date: 2026-08-17
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-27-sandbox-do-update-dry-run-copia-apenas-os-destinos-declarados-nao-a-arvore-do-projeto.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-27-sandbox-do-update-dry-run-por-lista-de-inclusao-dos-destinos-declarados.md"
---

# REQ: `update --dry-run` aborta em symlink quebrado ao copiar a árvore inteira do projeto

> Date: 2026-08-17 | Status: done
| Linear Issue:
| Jira Issue:

## Motivation

Encontrado em 2026-08-17 ao diagnosticar o bug de hooks do CMDB. Antes de rodar `trackfw update` no
projeto do usuário eu quis prever a mudança, e o dry-run **abortou**:

```
$ trackfw update --dry-run
Error: preparing dry-run sandbox: open /Users/.../cmdb/.venv/bin/python: no such file or directory
```

Não havia nada de errado com o trackfw naquele projeto. O que existia era:

```
$ ls -l .venv/bin/python
lrwxr-xr-x  .venv/bin/python -> python3.13      # alvo removido: symlink quebrado
```

### Causa raiz

`copyProjectTree` (`internal/generators/update.go:1662`) monta o sandbox com `filepath.WalkDir` e,
para toda entrada que não seja diretório, faz `os.ReadFile(p)`. `os.ReadFile` **segue o symlink**;
num link quebrado ele falha, o erro sobe pelo `WalkDir` e derruba o dry-run inteiro.

Dois defeitos no mesmo ponto:

1. **Symlink quebrado aborta.** Um link que o trackfw nunca vai tocar impede a operação inteira.
   Symlink quebrado é comum e legítimo: `.venv` de um Python removido, `node_modules` parcial,
   artefato de build limpo pela metade.
2. **O sandbox copia a árvore inteira do projeto.** Só `.git` e `node_modules` são pulados. Tudo o
   mais é lido byte a byte e reescrito no `/tmp`: `.venv`, `dist`, `build`, `target`, `vendor`,
   `__pycache__`, caches, binários. Num monorepo isso é caro em I/O e disco, e não tem relação com o
   que o `update` de fato altera — que é um conjunto pequeno e conhecido de artefatos gerenciados.

### Por que importa mais do que parece

O `--dry-run` é justamente o mecanismo de **previsão segura**: quem não confia no que o `update` vai
escrever roda o dry-run antes. Ele falhar por um symlink irrelevante empurra o usuário a rodar o
`update` de verdade sem previsão — o oposto do objetivo.

No meu caso concreto: sem dry-run, fiz backup manual do `settings.json` antes de rodar. Um usuário
sem esse hábito perde a rede de proteção. Some-se a isso que o caminho de escrita do `update` tem a
janela de gravação parcial descrita em
`REQ-2026-08-17-doctor-detecta-artefato-em-disco-ausente-do-manifesto-apos-janela-de-gravacao-parcial`
— ou seja, é exatamente o comando em que prever antes tem mais valor.

## Escopo

1. **Symlinks não podem derrubar a cópia.** Usar `os.Lstat`/`d.Type()&fs.ModeSymlink` e decidir
   explicitamente: recriar o link, ou pular. Nunca `os.ReadFile` cego num link.
2. **Copiar só o necessário.** O `update` mexe num conjunto conhecido de artefatos; o sandbox deveria
   refletir isso em vez de espelhar o projeto. No mínimo, ampliar a lista de exclusão
   (`.venv`, `venv`, `dist`, `build`, `target`, `vendor`, `__pycache__`, `.next`, `.cache`) — melhor
   ainda, inverter a lógica para lista de inclusão derivada dos targets declarados.
3. **Paridade nos 3 CLIs** — verificar se Node e Python têm o mesmo padrão de cópia; se sim, o mesmo
   defeito existe lá.

### Escopo negativo — declarado

- **Não é mudar o que o `update` escreve** — só como o sandbox de dry-run é montado.
- **Não é resolver a janela de gravação parcial** — é a REQ do `doctor`, citada acima.
- **Não é seguir symlink para fora da raiz do projeto.** Se a decisão for recriar links, um link
  apontando para fora precisa ser tratado com cuidado, não copiado ingenuamente.

## Acceptance Criteria

- [ ] AC1 — Repro: projeto com symlink **quebrado** (`ln -s inexistente alvo`) faz `update --dry-run`
      falhar hoje; depois, completa com sucesso.
- [ ] AC2 — Symlink **válido** também não quebra o dry-run, e o comportamento escolhido (recriar ou
      pular) é **declarado**, não acidental.
- [ ] AC3 — Sandbox deixa de copiar diretórios pesados irrelevantes; a decisão entre lista de
      exclusão e lista de inclusão fica registrada com o motivo.
- [ ] AC4 — Não-regressão: o relatório do `--dry-run` continua idêntico ao de antes para um projeto
      sem symlink quebrado — a correção é do sandbox, **não** do que o dry-run reporta.
- [ ] AC5 — Paridade nos 3 CLIs, com gate; se Node/Python não tiverem o defeito, isso é **declarado**
      com evidência em vez de presumido.
- [ ] AC6 — Cenário de falsificação (P4), baseline + detecção, com o symlink quebrado como
      discriminante.
- [ ] AC7 — `make quality` verde; `trackfw validate` sem novas violações.

## Riscos para quem executar

- **Copiar de menos quebra o dry-run silenciosamente.** Se o sandbox não tiver um arquivo que o
  `update` leria, o relatório sai **errado em vez de falhar** — e um dry-run que mente é pior que um
  que aborta. O AC4 existe por isso.
- **Não testar por leitura de fonte.** O critério é o dry-run rodando num fixture com symlink
  quebrado de verdade.
- **Cuidado com o binário do `PATH`:** medido em 2026-08-17, pode estar velho, e `--version` **não**
  distingue o build. Compilar antes de auditar.

## Linked ADR
ADR: <!-- nenhum; se a opção 2 virar lista de inclusão, avaliar ADR -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
