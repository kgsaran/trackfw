---
status: Accepted
date: 2026-09-04
author: "trackfw_architect (Zeus)"
---

# ADR: Separador POSIX nos artefatos autorados cujo consumidor não é o sistema de arquivos

> Date: 2026-09-04 | Status: Accepted

## Contexto

Cerca de **45 das 217 falhas reais de Windows** — o segundo maior grupo — vêm de o trackfw emitir
separador **nativo** onde o consumidor não é o sistema de arquivos:

```
tildeify        ->  ~\.claude\settings.json    (esperado: ~/.claude/settings.json)
provenanceKey   ->  skills\foo.md              (Go e Python ja normalizam; Node nao)
command do hook ->  C:\Users\x\guard.sh        dentro de string bash
```

Sem uma decisão, cada agente escolhe sozinho e a divergência entre os 3 CLIs volta por outro caminho.

## A pergunta que o KG fez, e que estreitou a decisão

> *"O separador POSIX funciona tanto em Windows quanto em Linux?"*

**Sim** — as APIs Win32 aceitam `/`, e os três runtimes repassam: `os.Open`, `fs.readFileSync` e
`open()` abrem `C:/foo/bar.txt` sem reclamar. Não é tolerância acidental; é comportamento documentado
do Windows.

**Com três exceções reais**, e é por elas que esta ADR **não** diz "sempre POSIX":

| exceção | por quê |
|---|---|
| UNC (`\\server\share`) | a forma exige backslash |
| prefixo de caminho longo (`\\?\`) | exige backslash **exclusivamente** — nem o Windows converte |
| argumento para `cmd.exe` | `cmd` trata `/` como prefixo de **opção**, não de caminho |

**Mas o argumento decisivo é outro, e é mais estreito:** nos três casos deste grupo, **o consumidor
nunca é o sistema de arquivos**.

- **`tildeify`** produz uma variável literalmente chamada `displayPath`
  (`npm/src/commands/update-harness.js:79,116,175`) — é **texto de relatório**. E ele emite `~`, que
  é POSIX-ismo puro: **nenhum shell do Windows expande `~`**. Emitir `~\...` é **incoerente com a
  decisão já tomada** ao escolher o til.
- **`provenanceKey`** é chave de dicionário JSON. Nunca toca o SO. Go
  (`validator_thirdparty_provenance.go:160`) e Python (`validator.py`) **já normalizam** por decisão
  documentada; só o Node não — e por isso ele **passa por acidente**, com fixture nativa e produto
  sem normalização casando entre si.
- **`command` de hook** é **string de comando bash**. Verificado no `settings.json` deste
  repositório: `$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`. Em bash, `\` é **escape**,
  não separador: `C:\Users\foo` vira `C:Usersfoo`. Aqui `/` não é preferência — **é correção**.

## Decisão

### D1 — O critério é o **consumidor**, não o sistema operacional

**O trackfw emite `/` nos artefatos que ele mesmo autora e cujo consumidor não é o sistema de
arquivos.** Onde o consumidor **é** o SO, o separador **nativo** é o correto.

Três categorias emitem `/`:

```
1. texto de relatorio / saida para humano     (displayPath, mensagens)
2. chave de dicionario ou identificador       (provenanceKey, node ID, edge.To)
3. string de comando interpretada por shell   (command de hook, gate de wave)
```

### D2 — `filepath.Join` continua para caminho que o SO abre

Nada nesta ADR muda como o produto **abre arquivo**. `filepath.Join`, `path.join` e `os.path.join`
seguem sendo o certo para caminho que vai a uma syscall.

🔴 **Normalizar cegamente quebraria UNC e o prefixo `\\?\`** — e o prefixo de caminho longo é
justamente o que uma REQ futura de Windows vai precisar. **A normalização é de saída, não de
travessia.**

### D3 — A normalização acontece na **fronteira de emissão**, num ponto único por runtime

Não espalhar `strings.ReplaceAll` pelos chamadores. Uma função por runtime, aplicada onde o valor
**sai** para relatório, chave ou comando — mesmo padrão do ponto único de resolução de REQ
(`ADR-2026-09-03`), e pela mesma razão: **o par emissor/consumidor não pode ter duas noções de
formato.**

### D4 — O contrato é dos 3 CLIs, byte-idêntico

Regra dura de paridade, sem exceção de infra. Hoje o Go e o Python normalizam a chave de proveniência
e o Node não — **essa divergência é o defeito, não uma escolha.**

## Consequências

**Resolve três grupos de uma vez** (~45 testes) e reduz o trabalho de "corrigir 45 falhas" para
**uma função por runtime mais edição de fixture**.

**Torna o `/api/chain` do Node explicável.** Medido: `go=1 nó, node=0, py=0` sobre o mesmo corpus.
Parte disso é indexação por basename (defeito próprio), parte é separador — e a D1 fecha a segunda
metade.

**Não fecha o CRLF.** É ADR separada: lá o defeito é o **parser** ser cego, não a emissão.

**Escopo negativo explícito:** esta ADR **não** autoriza normalizar caminho antes de `os.Open`,
`fs.readFileSync` ou `open()`. Se alguém "aplicar a ADR" ali, quebra UNC e caminho longo —
e o modo de falha seria intermitente, que é o pior.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções** por categoria: com a normalização, os 3 CLIs emitem `/`; sem
  ela, o nativo volta.
- **Controle POSIX:** em macOS e Linux a saída é **byte-idêntica** à de hoje — a normalização é
  no-op onde o separador já é `/`.
- 🔴 **Controle de não-regressão de travessia:** nenhum caminho passado a syscall foi alterado.
  Enumerar os pontos tocados e mostrar que **todos** são de emissão.
