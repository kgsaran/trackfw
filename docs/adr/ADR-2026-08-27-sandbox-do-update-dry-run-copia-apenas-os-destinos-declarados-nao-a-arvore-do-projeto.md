---
status: Accepted
date: 2026-08-27
author: "Zeus (Arquiteto)"
---

# ADR: Sandbox do `update --dry-run` copia apenas os destinos declarados, não a árvore do projeto

> Date: 2026-08-27 | Status: Accepted

## Context

`trackfw update --dry-run` monta um sandbox copiando **a árvore inteira do projeto** e aplicando o
update lá para prever o resultado. Medido em `internal/generators/update.go:2121`:

```go
if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") { return filepath.SkipDir }
data, readErr := os.ReadFile(p)   // segue o symlink; link pendurado ABORTA o WalkDir inteiro
```

**KG sentiu o efeito em produção no CMDB, em 2026-08-27:**

```
$ trackfw update --dry-run
Error: preparing dry-run sandbox: open /…/cmdb/.venv/bin/python: no such file or directory

$ ls .venv/bin/    ->  pip3.13 presente, python3.13 AUSENTE
```

O Homebrew atualizou o Python e levou o interpretador; o `.venv` ficou com um link pendurado. **Um
symlink que o trackfw nunca vai tocar derruba a operação inteira.**

O defeito já estava registrado desde 2026-08-17
(`REQ-2026-08-17-update-dry-run-aborta-em-symlink-quebrado-…`), sem roadmap, na fila de "depois da
release".

### Por que o remendo óbvio é o errado

Tolerar symlink pendurado resolve **este** sintoma e mantém a fragilidade: o sandbox continua
copiando `.venv`, `dist`, `build`, `target`, `vendor`, `__pycache__`, `.next`, caches e binários —
tudo o que não seja `.git` ou `node_modules`. Num monorepo isso é caro em I/O e disco, e **nada disso
tem relação com o que o `update` altera**, que é um conjunto pequeno e conhecido de artefatos
gerenciados.

Ampliar a lista de exclusão é o mesmo remendo com mais nomes: a lista nunca fecha, e o próximo
diretório exótico repete o incidente.

## Decision

**O sandbox passa a ser montado por lista de INCLUSÃO derivada dos targets declarados**, não por
espelhamento do projeto com exclusões.

1. O `update` já sabe **quais destinos** cada target escreve. O sandbox copia **apenas esses
   caminhos** — e os que existirem; ausência é estado válido e já tratado (`missing`).
2. **Nenhum `ReadFile` cego.** Onde um destino declarado for symlink, a decisão é explícita
   (`Lstat` / `d.Type()&fs.ModeSymlink`), nunca leitura que segue o link por acidente.
3. **Symlink fora do conjunto declarado deixa de ser problema por construção** — ele não é copiado,
   então não pode abortar nada.
4. **Paridade nos 3 CLIs.** O mesmo padrão de sandbox existe no Python (`update.py:535`); a correção
   é nos três.

## Consequences

**Positivas**
- Fecha a **classe**, não o caso: qualquer diretório exótico, link pendurado, arquivo sem permissão
  ou artefato gigante fora do conjunto declarado deixa de ter efeito.
- `--dry-run` fica proporcional ao que o comando faz — dezenas de arquivos, não o projeto inteiro.
- Devolve a rede de proteção: o dry-run existe para prever antes de escrever, e falhar por motivo
  alheio empurra o usuário a rodar o update **sem** previsão — o oposto do objetivo.

**Negativas e riscos aceitos**
- **A lista de inclusão precisa estar correta.** Se um target escrever um caminho que o sandbox não
  copiou, o dry-run mente por omissão. **É o risco dominante, e a Wave 0 tem de atacá-lo** — a lista
  de destinos declarados já errou três vezes nesta série em outro contexto.
- Comportamento do dry-run muda para quem dependia de o sandbox ser um espelho — não há caso de uso
  conhecido, mas é mudança observável.
- Mais acoplamento entre a montagem do sandbox e a declaração de destinos dos targets.

## Alternatives Considered

**Tolerar symlink pendurado e seguir copiando a árvore.** Rejeitada: resolve o sintoma de hoje e
mantém a superfície. O próximo incidente vem de permissão, arquivo especial ou tamanho.

**Ampliar a lista de exclusão** (`.venv`, `dist`, `build`, `target`, `vendor`, `__pycache__`…).
Rejeitada: lista de exclusão nunca fecha. É a mesma classe de "condição estreita demais" que esta
série já nomeou repetidamente.

**Não usar sandbox — simular sem copiar.** Atraente e mais invasiva: exigiria que todo o caminho de
escrita do `update` soubesse operar em modo simulação, mudando muito mais código do que a montagem do
sandbox. Registrada como caminho futuro se o custo do sandbox voltar a incomodar.
