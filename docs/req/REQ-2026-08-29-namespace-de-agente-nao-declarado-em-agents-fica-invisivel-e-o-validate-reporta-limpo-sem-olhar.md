---
status: Done
date: 2026-08-29
author: "trackfw_architect (Zeus)"
adr: "docs/adr/ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-29-lista-de-agentes-complementa-o-disco-e-namespace-nao-declarado-vira-violacao.md"
---

# REQ: Namespace de agente não declarado em `agents:` fica invisível, e o `validate` reporta limpo sem olhar

> Date: 2026-08-29 | Status: Open

## Motivation

Reportado por um agente no projeto **cmdb** — projeto consumidor, não este repositório.

Sintoma visível: `trackfw roadmap move` falha com `not found in any state directory` para um arquivo
**que existe em disco**.

Causa medida (`internal/validator/validator.go:1017-1037`): em `roadmap_namespacing: by_agent`, a
lista `agents:` do `trackfw.yaml` **substitui** a leitura do disco. O fallback para `os.ReadDir` só
acontece quando a lista está **vazia**. No cmdb, `agents:` lista sete agentes e não inclui `zeus` —
`docs/roadmaps/zeus/` e `docs/requisições/zeus/` estavam invisíveis para a ferramenta inteira.

**O dano grave não é o `move` falhar. É o `validate` passar.** Ele reportava `No violations found`
sobre um conjunto que nunca enumerou — incluindo uma REQ de ratificação criada naquela semana. O
`move` foi a única manifestação **visível**; as demais leituras degradam em silêncio.

**Incentivo perverso:** `agents:` vazio é mais seguro que `agents:` incompleto.

**Escala:** a regra está duplicada em **6 funções** só no `validator.go`, e o modo `by_agent` aparece
em 9 arquivos no Go, 11 sítios no Node e 24 no Python. Corrigir um ponto não corrige o defeito.

## Acceptance Criteria

- [ ] **AC1** — Em `by_agent`, a enumeração é a **união** entre `agents:` e os diretórios existentes
      em `roadmap_dir`. Verificável: projeto com `agents: [a, b]` e `docs/roadmaps/zeus/` em disco →
      artefatos de `zeus` são enumerados por `validate`, `status`, `context`, `serve` e `roadmap move`.
- [ ] **AC2** — Mesma união para `req_dir`. No cmdb as duas árvores estavam cegas.
- [ ] **AC3** — `roadmap move` encontra e move roadmap em namespace não declarado, sem erro.
- [ ] **AC4** — Namespace em disco não declarado gera **violação** (não aviso), nomeando o diretório
      e instruindo a acrescentá-lo a `agents:`.
- [ ] **AC5** — **Falsificação nas duas direções:** (a) com o namespace declarado, nenhuma violação;
      (b) com o diretório presente e não declarado, violação — e os artefatos **ainda assim**
      enumerados. A união não pode depender da declaração.
- [ ] **AC6** — **Um resolvedor canônico por runtime**, consumido por todos os pontos que hoje
      resolvem sozinhos. Verificável: `grep` não encontra mais o padrão `len(agents) == 0` (e
      equivalentes) replicado — só dentro do resolvedor.
- [ ] **AC7** — **Não-regressão do caso comum:** projeto com `agents:` completo e correto produz
      exatamente o mesmo resultado de hoje em `validate`, `status` e `context`. Provado por
      comparação antes/depois sobre o corpus deste repositório.
- [ ] **AC8** — Modo `flat` **inalterado**. Nenhuma mudança de comportamento para quem não usa
      `by_agent`.
- [ ] **AC9** — Paridade exata nos 3 CLIs: mesma união, mesma violação, mesma mensagem.
- [ ] **AC10** — Gate falsificável cobrindo AC1, AC4 e AC5 nos 3 runtimes.
- [ ] **AC11** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0, e o CI verde. **Verde
      local não é conclusão** — ver `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`.
- [ ] **AC12** — A enumeração **não segue symlink**. Verificável nos 3 CLIs: namespace que é symlink
      apontando para fora do projeto → `roadmap move` **não** escreve fora da árvore. Reproduzido
      hoje: Go imune, **Node e Python escapam**. Primitivas exigidas: `os.ReadDir` + `entry.IsDir()`
      (Go, preservar), `readdirSync(..., {withFileTypes:true})` + `dirent.isDirectory()` (Node),
      `os.scandir` + `is_dir(follow_symlinks=False)` (Python).
- [ ] **AC13** — Bloqueia a AC1: a união **não pode** ser entregue sem a AC12. Hoje o escape é
      condicionado a `agents:` vazia; a união o tornaria incondicional para todo projeto `by_agent`.

## Negative Scope

- **Não** deprecar nem alterar a semântica de `agents:` para ordenação/exibição no `serve`.
- **Não** mexer no modo `flat`.
- **Não** migrar `trackfw.yaml` de projeto nenhum automaticamente. A violação instrui; quem decide é
  o usuário.
- **Não** criar diretório de namespace ausente. A união é de **leitura**.
- **Não** resolver aqui o `roadmapTrustForGates` que falha aberto nem os defeitos de Windows da
  issue #216 — REQs próprias, já rastreadas.

## Observação de método

O defeito foi encontrado **por um usuário fora deste repositório**, usando a ferramenta em trabalho
real, e a manifestação que o revelou (`move` falhando) é a única que não era silenciosa. É o segundo
defeito de harness em produção descoberto assim em dois dias — o primeiro foi a issue #216. Vale como
evidência de que o nosso próprio uso não é amostra suficiente: aqui usamos `flat`, e por isso nunca
vimos.

---

## Nota de correção (2026-08-31) — o AC12 é verdadeiro no Linux e **falso no Windows**

> Anexada por `zeus-tf`. **A REQ não é reaberta e o AC12 acima não é reescrito** — o histórico do que
> acreditávamos, e por quê, vale mais do que um artefato limpo.

O **AC12** afirma que *"a enumeração **não segue symlink**. Verificável nos 3 CLIs"*, e exige as
primitivas `entry.IsDir()` (Go), `dirent.isDirectory()` (Node) e `is_dir(follow_symlinks=False)`
(Python). **Isso foi medido e é verdade — no Linux.**

A sonda de Windows (`windows-probe.yml`, run
[`33338382066`](https://github.com/kgsaran/trackfw/actions/runs/33338382066), primeira execução
pós-merge do #221) mediu:

```
lstat-symlink   →  ModeSymlink=true    ModeIrregular=false   ModeDir=false
lstat-junction  →  ModeSymlink=false   ModeIrregular=TRUE    ModeDir=false
stat-junction   →  ModeDir=true   (seguindo o link)
```

**Uma *junction* do Windows não é reportada como symlink.** E ela é criada por `mklink /J`, que
**não exige privilégio algum** — ao contrário de `os.Symlink`, que exige Developer Mode. No Windows,
portanto, a defesa cobre o caso que exige privilégio e deixa passar o que não exige.

O AC12 não está errado sobre o que mediu; está **incompleto sobre onde vale**. "Verificável nos 3
CLIs" foi lido como "verificado em toda plataforma", e nenhum dos três CLIs foi exercitado em
Windows quando esta REQ fechou — o instrumento que torna Windows mensurável só existiu depois
(#221). Registrar isso importa porque a mesma frase aparece em outras REQs desta família.

**O que esta nota NÃO faz:** não classifica as guardas nem propõe remédio. A separação por classe —
guarda de diretório, guarda salva por acidente, guarda de folha que nunca olha ancestral — está na
`REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-...`, junto com a descoberta de que o freio acidental
existe **só no Go**. A correção depende de medição que só roda pós-merge.

Ver `vault/notes/lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31.md`.

## Linked ADR
ADR: docs/adr/ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-29-lista-de-agentes-complementa-o-disco-e-namespace-nao-declarado-vira-violacao.md
