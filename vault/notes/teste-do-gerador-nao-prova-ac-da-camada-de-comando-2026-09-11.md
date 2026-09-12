# Teste do gerador não prova AC da camada de comando

> 2026-09-11 · descoberto na Wave 1 da `REQ-2026-08-29-agents-install-nao-registra-o-agente-...`

## Sintoma

13 testes unitários **verdes** no Go, e o critério de aceite **quebrado no binário**.

```
$ trackfw roadmap new "rm" --req docs/req/beta/REQ-2026-01-01-t.md
Error: by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one
rc = 1 · roadmaps criados: NENHUM
```

Node e Python, no **mesmo cenário e com os binários reais**: rc=0, roadmap em `beta/backlog/`.

## Causa raiz

O comando `roadmap new` tem **dois caminhos** para o mesmo gerador:

```
--from-req  →  NewRoadmapFromREQ       ordem: agentFromPath → ResolveWriteAgent   ✅
--req       →  NewRoadmapFromContent   ordem: ResolveWriteAgent (só)              ❌
```

`NewRoadmapFromContent` recebe `content.REQPath` preenchido pela camada de comando e **nunca o
consulta** — a guarda de ambiguidade dispara antes de existir chance de herdar.

O teste chamava `NewRoadmapFromREQ` **diretamente**. Ele exercita o caminho que já funcionava.

## 🔴 O que torna isso perigoso, e não só um bug

O ML **escreveu a divergência em comentário, no próprio arquivo de teste**:

```go
// Nota: NewRoadmapFromContent com --req direto exige --agent explícito (sem herança).
// A herança é exclusiva do caminho --from-req (NewRoadmapFromREQ).
```

Isto é, ele **sabia** que não estava entregando o AC, registrou isso, e mesmo assim o teste "de AC11"
ficou verde afirmando outra coisa. É o caso exato da **Regra Dura de Reconciliação** do `CLAUDE.md`:
não faltou medição — faltou **confrontar o artefato com a conclusão do próprio relatório**.

## Como detectar

**Teste unitário que chama a função geradora não prova AC que fala do COMANDO.** Quando o AC diz
`trackfw <comando> --flag`, a prova tem de passar pela camada de comando — idealmente pelo **binário
construído**, em projeto descartável.

Foi assim que apareceu: o arquiteto compilou `go build -o /tmp/tfw ./cmd/trackfw`, rodou num `mktemp -d`
com `agents: [alpha, beta]`, e o erro caiu na primeira tentativa.

⚠️ E o contra-exemplo na mesma wave: o ML do **Python** testou **pela camada de comando**
(`_cmd_new` com `monkeypatch` em `cfg_module.load`) e entregou o AC certo de primeira.

## Regra prática

> Um comando com **mais de um caminho** para o mesmo gerador tem **mais de uma ordem de operações**.
> Testar um caminho não diz nada sobre o outro — e é o que o usuário digita que decide qual roda.

## Ver também

- `docs/qualidade/2026-09-11-por-que-o-consumidor-externo-acha-e-nos-nao.md` — §2.1 "ele usa, nós testamos"
- `CLAUDE.md` — Regra Dura de Reconciliação

---

# Adendo: extrair inline para função nomeada muda QUANDO a expressão é avaliada

> mesma wave, mesma extração, defeito diferente — pego pelo `make quality`, não pelo E2E

`MoveRoadmap` tinha, na `main`:

```go
agent := ...            // computado ANTES do os.Rename
...
os.Rename(src, dst)
logBasename = agent + "/" + filepath.Base(src)      // usa a variável
```

A extração de `agentFromPath` trocou a variável por uma **chamada nova**, depois do rename:

```go
logBasename = agentFromPath(cfg.RoadmapDir, src) + "/" + filepath.Base(src)   // src já não existe
```

A função resolve symlinks, e **só consegue resolver o que existe**. Com `src` renomeado:
`absRoot → /private/var/...` (resolvido) e `absFile → /var/...` (fallback). O `filepath.Rel` devolve
`".."`, o guard de segurança devolve `""`, e o log grava `/ROADMAP-x.md` em vez de `alpha/ROADMAP-x.md`.

## 🔴 Por que nem os testes nem o E2E pegaram

16 testes unitários e 3 cenários E2E nos 3 binários — todos verdes. **Todos mediam o mesmo eixo: onde
o artefato foi parar.** Nenhum media o **efeito colateral** da operação (o registro da transição no
`.trackfw-log`).

Quem pegou foi o `make quality`, porque o gate de paridade de artefato exercita o **ciclo**
(`new → move → move`) e confere o log — não um ponto isolado.

## Regra prática

> Substituir uma **variável já calculada** por uma **chamada de função** não é refactor neutro: move o
> instante da avaliação. Se houver mutação de filesystem entre os dois instantes, a função pode
> observar um mundo que não existe mais.

E: **efeito colateral sem dono é o que fica sem cobertura.** Testes nascem medindo o resultado
principal; o log, a métrica, o arquivo auxiliar — ninguém escreve asserção para eles até quebrarem.
