# Wave 6 — ML-6A: `roadmap new` sobrescreve a roadmap criada pelo `req new`

> Data: 2026-10-09 | REQ: REQ-2026-09-09 (AC8, reabertura) | Branch: `fix/req-nasce-orfa-roadmap-new-sobrescreve`

---

## 1. Medição com o binário da main

### Setup

Binário compilado de `HEAD` (branch `fix/req-nasce-orfa-roadmap-new-sobrescreve`):

```
go build -o <scratch>/tf ./cmd/trackfw
```

Projeto temporário isolado em scratch com `trackfw.yaml` mínimo (`roadmap_namespacing: flat`).

---

### (a) Mesmo título — a roadmap é sobrescrita?

```
$ tf req new "Minha Feature Alpha"
created docs/req/REQ-2026-10-09-minha-feature-alpha.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-10-09-minha-feature-alpha.md
✓ linked REQ-2026-10-09-minha-feature-alpha.md → docs/roadmaps/backlog/ROADMAP-2026-10-09-minha-feature-alpha.md

$ tf roadmap new "Minha Feature Alpha"
✓ created docs/roadmaps/backlog/ROADMAP-2026-10-09-minha-feature-alpha.md
```

Estado depois:

| Campo | Valor |
|---|---|
| REQ frontmatter `roadmap:` | `"docs/roadmaps/backlog/ROADMAP-2026-10-09-minha-feature-alpha.md"` (preservado) |
| Roadmap frontmatter `req:` | `""` (destruído) |
| Roadmap body `REQ:` | `` (linha vazia) |

**Sim, a roadmap é sobrescrita.** O vínculo REQ→roadmap sobrevive no frontmatter da REQ (porque `roadmap new` sem `--req` chama `linkREQToRoadmap("")` que retorna imediatamente — `internal/generators/roadmap.go:1513-1514`). O vínculo roadmap→REQ é destruído.

**O validate NÃO detecta o problema enquanto a roadmap está em `backlog/`.** As regras `req_has_roadmap` e `req_roadmap_sync` não disparam porque a REQ ainda aponta para a roadmap. Só ao mover para `wip/` a regra `wip_has_req` detecta: `"roadmap in wip but has no linked REQ"`.

---

### (b) Título diferente — sem colisão

```
$ tf req new "Outro Titulo Diferente"
$ tf roadmap new "Titulo Completamente Diferente"
```

Dois arquivos distintos são criados. Nenhuma colisão. O comportamento é correto quando os slugs diferem.

---

### (c) `roadmap new --from-req` sobre roadmap já existente

```
$ tf roadmap new --from-req docs/req/REQ-2026-10-09-outro-titulo-diferente.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-10-09-outro-titulo-diferente.md
```

O `--from-req` também sobrescreve silenciosamente. Diferença: como `NewRoadmapFromREQ` passa `REQPath` não-vazio para `NewRoadmapFromContent`, o `linkREQToRoadmap` é chamado e os vínculos ficam corretos nos dois lados — mas todo conteúdo editado manualmente na roadmap é destruído sem aviso.

---

### (d) Roadmap já movida para `wip/` com o mesmo nome-base

```
$ tf req new "Feature Em Progresso"
$ tf roadmap move "feature-em-progresso" wip
$ tf roadmap new "Feature Em Progresso"
✓ created docs/roadmaps/backlog/ROADMAP-2026-10-09-feature-em-progresso.md
```

A roadmap em `wip/` é preservada (está em subdiretório diferente). O `roadmap new` cria uma **segunda** roadmap em `backlog/` com o mesmo basename. O `validate` detecta imediatamente:

```
✗ roadmap "ROADMAP-2026-10-09-feature-em-progresso.md" appears in multiple states: [backlog wip]
```

Neste sub-caso o dano é visível pelo validate antes de qualquer commit.

---

### (e) Conteúdo editado à mão — é perdido?

```
$ tf req new "Conteudo Editado"
# edição manual: adicionou "## Trabalho feito manualmente" + "Linha importante..."
$ tf roadmap new "Conteudo Editado"
✓ created docs/roadmaps/backlog/ROADMAP-2026-10-09-conteudo-editado.md
```

Resultado: o conteúdo editado é **completamente destruído**. O `os.WriteFile` sobrescreve sem backup.

---

### Ponto exato da escrita no código

**Arquivo:** `internal/generators/roadmap.go`  
**Linha:** 300  

```go
if err := os.WriteFile(filename, []byte(body), 0644); err != nil {
    return fmt.Errorf("writing roadmap: %w", err)
}
```

**Onde o nome do arquivo é computado:** linha 248-250:

```go
slug := toSlug(content.Title)
date := time.Now().Format("2006-01-02")
filename := fmt.Sprintf("%s/ROADMAP-%s-%s.md", backlogDir, date, slug)
```

**Checagem de existência:** inexistente. Entre a linha 248 (cálculo do nome) e a linha 300 (escrita), o código passa por: guard de symlink no diretório (linha 239), guard de symlink no arquivo (linha 296), e `os.WriteFile` — nenhum `os.Stat` ou `os.OpenFile` com `O_EXCL`.

---

## 2. Enumeração de texto gerado que manda rodar `req new` + `roadmap new`

### Sítios em código de produto (gerado — vai para o consumidor)

| # | Arquivo | Linha(s) | Conteúdo | Natureza |
|---|---|---|---|---|
| 1 | `internal/generators/agentfiles.go` | 59–60, 86 | `step1Roadmap := "trackfw roadmap new \"title\""` emitido como passo 1 do Agent Protocol | **GERADO** — vai para AGENTS.md, GEMINI.md, `.github/copilot-instructions.md`, `.windsurfrules`, `.cursor/rules/trackfw.mdc` de todos os projetos consumidores |
| 2 | `internal/generators/claudemd.go` | 74, 77–78 | `"| trackfw roadmap new | Create empty roadmap linked to a REQ |"` (tabela de referência rápida) | **GERADO** — vai para CLAUDE.md dos consumidores |
| 3 | `internal/generators/scaffold.go` | 68 | comentário de código que diz "correct `req new` / `roadmap new` command" | Só neste repo (comentário, não emitido) |

Os sítios 1 e 2 são os críticos: toda vez que um consumidor roda `trackfw init` ou `trackfw update harness`, essas instruções são gravadas nos arquivos de configuração do IDE/agente. A instrução do sítio 1 é a que induz o protocolo quebrado com precisão cirúrgica:

```
trackfw req new "title" → trackfw roadmap new "title" → trackfw roadmap move <name> wip → git checkout -b feat/<branch>
```

### Sítios neste repositório (não gerado — não vai para consumidores)

| # | Arquivo | Linha(s) | Conteúdo |
|---|---|---|---|
| 4 | `CLAUDE.md` (deste repo) | 332 | `` `trackfw req new "title"` → `trackfw roadmap new "title"` → ... `` — Agent Protocol do próprio projeto |
| 5 | `README.md` | 258–261 | Exemplo de fluxo: `trackfw req new "User authentication"` + `trackfw roadmap new "Auth service"` |

Neste repo, o CLAUDE.md (linha 332) e o README.md ensinam o mesmo protocolo que vai para os consumidores via o sítio 1.

---

## 3. Opções de correção com medição de atrito

### Opção A — Recusar com erro quando o arquivo existe

`NewRoadmapFromContent` checa com `os.Stat(filename)` antes de escrever. Se o arquivo existe, retorna:

```
Error: roadmap already exists: docs/roadmaps/backlog/ROADMAP-2026-10-09-minha-feature-alpha.md
       (created by req new — no action needed)
```

**Atrito para consumidor com protocolo antigo:** `roadmap new T` falha com erro claro. O consumidor vê a mensagem, entende que o roadmap já existe e que `req new` o criou. Nenhum dado é destruído.

**Atrito para uso legítimo de `roadmap new` sem `req new` anterior:** zero atrito — arquivo não existe, caminho normal.

**Atrito para `--from-req`:** mesma lógica. Se a roadmap já existe (foi criada por `req new`), recusa. O consumidor pode usar `--force` ou apagar e recriar. Para o caso de uso de "regenerar o roadmap a partir da REQ", o `--force` é necessário.

**Deixa o protocolo antigo inofensivo?** Sim — falha ruidosa em vez de destruição silenciosa. O consumidor que não atualizar o texto verá o erro imediatamente.

### Opção B — Idempotente: pular se já existe, vincular sem sobrescrever

Se o arquivo existe, `roadmap new` imprime `"roadmap already exists at %s — skipping creation, verifying links"` e apenas chama `linkREQToRoadmap` para reparar o backlink se necessário.

**Atrito:** mínimo. O protocolo antigo torna-se silenciosamente correto: `req new T` cria a roadmap vinculada, `roadmap new T` não faz nada (e diz que não fez). O consumidor não precisa atualizar o texto do protocolo para o fluxo comum passar a funcionar.

**Risco desta opção:** cobre a lógica com `os.Stat` — o gap entre `Stat` e `WriteFile` é um TOCTOU. Em uso normal (CLI single-threaded) é inofensivo. Em projetos com dois agentes escrevendo simultaneamente o risco existe; o lock de arquivo seria a proteção correta (fora do escopo deste AC).

**Deixa o protocolo antigo inofensivo?** Sim — melhor ainda que a Opção A: zero erros, zero perda de dados, comportamento correto sem atualização de texto.

### Opção C — `--force` como opt-in explícito de sobrescrita (complemento das opções A ou B)

`roadmap new` usa Opção A ou B por padrão; `roadmap new --force T` sobrescreve. Permite que o caso de uso `--from-req` (regenerar) continue funcionando sem a friação do erro.

**Atrito:** adição de flag; é opt-in, não opt-out. Não introduz regressão.

---

### Qual opção é recomendada

A **Opção B (idempotente)** com a **Opção C (`--force`) como complemento** é a combinação que:

1. Deixa o protocolo antigo completamente inofensivo — zero erros para consumidores que não atualizarem o texto.
2. Não destrói dados em nenhum caso.
3. Oferece um caminho explícito de sobrescrita intencional (`--force`).
4. Reduz a urgência de atualização dos sítios enumerados na Seção 2 (embora ainda devam ser atualizados para eliminar confusão).

A Opção A (erro) é mais conservadora e mais fácil de implementar; a escolha entre A e B é decisão de UX para o ML-6B.

---

## 4. Threat model

### Quem perde trabalho?

**Caso 1 — Agente seguindo o protocolo ensinado (o mais provável)**

O agente lê a instrução do Agent Protocol (gerada no sítio 1 acima) e executa `req new "T"` seguido de `roadmap new "T"`. A roadmap vinculada é destruída em silêncio. O vínculo REQ→roadmap sobrevive no frontmatter da REQ; o `validate` não detecta nada enquanto a roadmap está em `backlog/`. O agente move a roadmap para `wip/`, o `validate` dá `wip_has_req` — mas o agente já escreveu content no roadmap (que agora é o novo, sem vínculo). Dois commits depois, `trackfw push` bloqueia.

**Caso 2 — Conteúdo editado manualmente**

Quem editou o roadmap à mão (acrescentou waves, decisões, contexto) e depois roda `roadmap new T` perde todo esse conteúdo. Não há aviso. O `✓ created` indica sucesso.

**Caso 3 — Roadmap já em wip/**

Cria uma segunda roadmap com o mesmo basename em `backlog/`. O `validate` detecta imediatamente (`multiple states`), então o dano é visível — mas o estado do projeto fica inconsistente e requer limpeza manual.

### Pode um agente contornar o bloqueio com `rm`?

Sim. Se `roadmap new` passar a recusar a sobrescrita (Opção A ou B), um agente que leia esse erro poderia:

1. `rm docs/roadmaps/backlog/ROADMAP-...md`
2. `trackfw roadmap new "T"` — cria novo arquivo sem link, sem erro

Este vetor existe porque `roadmap new` não protege artefatos já vinculados — ele apenas recusa criar um arquivo no mesmo path. A proteção real é o `wip_has_req` ao mover para `wip/`, que ainda bloqueia.

Mas este é um vetor de agente determinadamente errando, não de protocolo induzindo o erro. O defeito atual produz o mesmo resultado **sem intenção**, o que é significativamente pior.

### Alvos de falsificação nas duas direções

#### Direção falso-negativo (fix aceita o que deveria recusar)

| Superfície | Onde o bypass entra | Qual gate deveria capturar | Status |
|---|---|---|---|
| `roadmap new T` quando arquivo existe em `backlog/` | `os.WriteFile` sem checagem → sobrescreve | `os.Stat` antes de `WriteFile` (fix do ML-6B) | **Não capturado hoje** |
| `roadmap new --from-req REQ` quando arquivo existe | Mesmo sítio, mesmo mecanismo | Mesmo fix | **Não capturado hoje** |
| `roadmap move <slug> wip` após sobrescrita | `req:` está `""` no roadmap | `wip_has_req` no `validate` | Captura — mas só após a perda, não antes |
| `req_has_roadmap` para a REQ sobrescrita | REQ ainda tem `roadmap:` apontando para o caminho (preservado) | `req_roadmap_sync` (verifica divergência) | **Não dispara** porque `req:` no roadmap é `""` — a regra checa divergência entre dois valores não-vazios |

#### Direção falso-positivo (fix recusa o que deveria aceitar)

| Superfície | Cenário legítimo | Qual gate garante que funciona |
|---|---|---|
| `roadmap new T` quando arquivo NÃO existe | Primeiro uso do roadmap (caso comum) | `os.Stat` retorna erro → prossegue normalmente |
| `roadmap new --force T` | Regeneração intencional | Flag explícito; nenhum dado perdido por acidente |
| `roadmap new T` com slug diferente mesmo que título seja parecido | `toSlug` produz nomes distintos | Nomes distintos → arquivos distintos → sem conflito |

### Residual declarado

Este design aceita explicitamente não cobrir:

1. **TOCTOU entre `os.Stat` e `os.WriteFile`**: dois agentes escrevendo simultaneamente para o mesmo path poderiam ambos passar pelo `Stat` antes de qualquer um escrever. Em uso normal (CLI single-threaded) é inofensivo. Proteção real requereria lock de arquivo, que está fora do escopo.

2. **`rm` + `roadmap new`**: um agente que deliberadamente apaga o arquivo antes de recriar contorna qualquer checagem de existência. O `wip_has_req` ainda captura ao mover para `wip/`. Não é novo risco introduzido pelo fix.

3. **Atualização dos sítios de texto enumerados na Seção 2**: o fix em `roadmap new` torna o protocolo antigo inofensivo, mas os sítios continuam emitindo a instrução redundante. ML-6B deve atualizar o texto dos sítios 1 e 2 (gerado) e os sítios 4 e 5 (neste repo). Enquanto não forem atualizados, consumidores verão o protocolo antigo na documentação — que agora é inofensivo, mas ainda é confuso.

4. **`roadmap new --from-req` sobre roadmap em estado `wip/` ou `done/`**: a roadmap com aquele basename está em outro subdiretório; `NewRoadmapFromContent` computa o path em `backlog/` e cria um arquivo novo lá. Gera o `multiple states` no `validate`. Este sub-caso (Cenário D acima) é capturado pelo gate existente — mas antes de o gate disparar, o consumidor pode ter feito commits.

---

## Veredito

**O defeito é confirmado, real e medido.** A causa é uma linha única:

```
internal/generators/roadmap.go:300
os.WriteFile(filename, []byte(body), 0644)
```

sem nenhuma checagem de existência. O `filename` é computado por slug + data de forma determinística; quando `req new T` e `roadmap new T` rodam no mesmo dia, produzem o mesmo path. A sobrescrita é silenciosa, `✓ created` impresso como sucesso, e o vínculo REQ↔roadmap fica quebrado de um lado sem que o `validate` detecte enquanto a roadmap está em `backlog/`.

A severidade é alta para o fluxo automatizado de agentes: o texto do Agent Protocol gerado pelo trackfw (sítio 1: `agentfiles.go:59-86`) instrui explicitamente os dois comandos em sequência. Todo consumidor que rode `trackfw init` ou `trackfw update harness` recebe essa instrução. O protocolo correto é rodar apenas `req new` — mas esse protocolo correto não está escrito nos sítios gerados.

**Especificação para ML-6B:**

1. `internal/generators/roadmap.go:248–300` — antes de `os.WriteFile`, adicionar:
   - Se arquivo existe E `--force` não está ativo: comportamento a decidir entre Opção A (erro) ou Opção B (pular com aviso). Recomendação: Opção B (pular + verificar link) + Opção C (`--force`).
   - Mensagem de pular deve dizer que `req new` cria a roadmap e o `roadmap new` é redundante.
   - `--force` flag em `newRoadmapNewCmd` (`internal/commands/roadmap.go:27`) e propagação para `NewRoadmapFromContent` via `RoadmapContent`.
2. Atualizar sítios de texto:
   - `internal/generators/agentfiles.go:59-60, 86` — remover `step1Roadmap` do passo 1 do Agent Protocol; o passo 1 passa a ser só `req new "title"`.
   - `internal/generators/claudemd.go:74, 77-78` — atualizar descrição de `roadmap new` para deixar claro que `req new` já cria a roadmap, e `roadmap new` é para casos onde não há REQ.
   - `CLAUDE.md:332` — atualizar passo 1 do protocolo.
   - `README.md:258-261` — atualizar exemplo de fluxo.
3. Falsificação obrigatória em testes:
   - `req new T` + `roadmap new T` no mesmo diretório: a roadmap mantém `req:` preenchido (não sobrescrita).
   - `roadmap new T` sem REQ anterior: cria normalmente.
   - `roadmap new T --force` com arquivo existente: sobrescreve (caso explícito permitido).
4. `make quality` verde após as mudanças.
