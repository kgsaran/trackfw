# Triagem por evidência das REQs abertas — 2026-09-18

> Executada a pedido do KG. **31 das 40 REQs abertas** triadas por medição, em dois lotes paralelos,
> com auditoria e reprodução independente do arquiteto nos achados decisivos.

## Resultado

| veredito | qtd | significado |
|---|---|---|
| **OBSOLETA** | **13** | alvo removido pela v8, ou defeito já corrigido por outro trabalho — **fecháveis** |
| **VIVE** | **18** | reproduzido agora, ou código defeituoso presente |
| INCERTA | 0 | justificado por lote, não forçado — ver ressalvas |

**A causa dominante das obsoletas é a v8.0.0** (commit `2eae0a44`), que removeu os CLIs Node e Python.
`git ls-files pypi/trackfw` → **vazio**; `npm/src` **não existe**. 🔴 O diretório `pypi/trackfw/` existe
no disco como **lixo de build não versionado** — um filtro por `ls` erraria aqui, e é preciso usar
`git ls-files`.

## 🔴 Prioridade 1 — segurança, todas reproduzidas

| REQ | defeito | reprodução |
|---|---|---|
| `2026-08-31-guarda-de-folha...` | **escrita fora da árvore do projeto** | `.github` e `scripts` como symlink de diretório → `discover --init` gravou **6 arquivos** em destino externo, **sem aviso**. Reproduzido pelo arquiteto. `os.Lstat` só não segue o **último** componente; `discover.go:77-87` faz `MkdirAll`+`WriteFile` sem inspecionar ancestral |
| `2026-08-30-validacao-de-valor-agent-models...` | **injeção em YAML** | `agent_models: {opus: 'x: y "z"'}` → frontmatter `model: x: y "z"` (YAML inválido) em **12 agentes** de escopo **global** |
| `2026-09-05-o-guard-passa-a-bloquear-staging...` | guard não cobre `git add` | `git add -A` → **rc=0, sem saída**; controle `git push` → rc=2 com deny |
| `2026-09-02-remover-pretooluse...` | remoção do hook é invisível | `validate --json` devolve **zero** achados de guard com **e sem** `hooks.PreToolUse` |

## Prioridade 2 — baratas, reproduzidas

- `2026-09-03-check-referential-integrity...` — gate **vácuo**: `docs/` vazio → `OK`, `rc=0`. É o **quinto** gate vácuo do projeto.
- `2026-09-12-branch-new-cria-do-head...` — `branch.go:124` roda `git checkout -b` do HEAD, sem `--from`, sem `fetch`. Nenhuma AC implementada; o molde já existe em `branch prune`.
- `2026-08-30-debitos-de-higiene` (2 de 4 itens) — `package.json` **8.0.1** × `package-lock.json` **7.6.0** (a defasagem **piorou**: a REQ media 7.3.0 × 6.1.0); e `discover --init` reescreve conteúdo deixando modo **644**, de modo que o hook de pre-commit instalado **não é executável**.

## Ressalvas que mudam decisões

- **`2026-09-01-trackfw-escreve-guard-global...`** vive, **mas não está pronta para despacho**: a própria
  REQ carrega adendo de auditoria externa (2026-09-05) declarando que dois ACs exigem predicados
  diferentes e que *"esta REQ não é implementável"* até a reconciliação ser escrita.
- **`2026-08-21-update-harness...` é duplicata** de `2026-08-30-validacao-de-valor...` — mesma causa
  (sanitização de valor). Absorver, não tratar como item próprio.
- **Duas REQs são *form-level*:** a instância original morreu com a v8, mas a **forma** vive com sujeito
  novo e nomeado — ex.: `agentfiles.go:18-23` injeta rules em **6** alvos e `check-rules-parity.sh:45`
  fixa **4**; `CLAUDE.md` e `AGENTS.md` são escritos pelo produto e **não** conferidos pelo gate.
- **Windows (2 REQs):** o braço de runtime não foi medido (não há Windows aqui), mas o braço de **código**
  é conclusivo e mostra trabalho não feito — `ls scripts/ | grep ps1` → vazio. Os ACs não medidos estão
  nomeados um a um.

## 🔴 O vínculo que fecha um ciclo

O **AC12 da `REQ-2026-09-12-v8-um-binario-muitos-canais`** e os **AC10-AC12 da REQ irmã** pedem
literalmente *"classificar as REQs e os issues abertos em desaparece / barateia / indiferente e fechar
os que a causa removeu"*.

**Esse AC nunca foi executado — e é exatamente esta triagem.** As duas REQs estão obsoletas *por
entrega* (roadmaps em `done/`, v8.0.1 publicada), mas fechá-las sem registrar o vínculo perderia a
rastreabilidade. Colide também com os AC1-AC3 de `REQ-2026-09-12-reqs-de-paridade-nao-distinguem...`:
é o mesmo entregável descrito em duas REQs.

## Higiene descoberta de passagem

- **Dois roadmaps mortos em `backlog/`** descrevem MLs sobre `pypi/trackfw/` e `npm/src/` — caminhos
  inexistentes. Aparecem em `trackfw status` como trabalho pendente inexecutável.
- `scripts/check-tty-detection.sh` **sobrevive com o sujeito deletado**, e o próprio cabeçalho declara
  isso. Vale perguntar se `scripts/check-orphan-gates.sh` dispara sobre os sobreviventes da v8.
- **Nenhuma das 15 REQs do lote 1 cita número de issue** — não têm contraparte rastreável no GitHub.

## Erros de método do arquiteto, corrigidos

1. **O número de órfãs estava errado.** Meu script lia só `^Roadmap:` do **corpo** e ignorava
   `roadmap:` do **frontmatter**: são **24** órfãs, não 30. Ambos os lotes detectaram REQs que eu havia
   classificado como órfãs e **têm** roadmap — incluindo três em `analyzing/`, `backlog/` e `blocked/`.
2. **Erro de transcrição** ao montar o prompt do lote 2: inseri duas REQs que não eram órfãs. Sem perda
   de cobertura; o executor detectou e reportou em vez de apenas obedecer.
3. 🔴 **Varri os issues abertos antes de escrever o roadmap do #392, mas não as REQs abertas** — e a
   Regra Dura de Causa Raiz dá a elas o mesmo peso. Duas REQs tocavam o mecanismo do #392, e uma delas
   (`roadmap move` seguindo symlink) **vive** e incide na função que o ML-3A acabou de reescrever.
