# Guarda que reporta ausência precisa distinguir "não achei" de "não consegui procurar"

> 2026-09-10 · seis instâncias no mesmo dia, três delas em guardas que **nós** escrevemos e **eu**
> aprovei em auditoria.

## O padrão

Uma guarda observa **um sinal** e publica **uma conclusão**. Quando o sinal é *ausência* — o nome não
apareceu, o comando saiu != 0, o arquivo não abriu — **dois estados diferentes produzem o mesmo
observável**:

```
o alvo não existe          ← a conclusão que a guarda publica
não consegui procurar      ← o que de fato aconteceu
```

🔴 **A guarda mede "não encontrei" e declara "não existe".** É a Regra Dura de Reconciliação do
projeto aplicada ao gate: *o artefato afirma mais do que mediu*.

**Falha na direção de tranquilizar**: a mensagem manda a pessoa procurar no lugar errado, ou — pior —
o gate degrada para aviso e a verificação **simplesmente não roda**, sem ninguém notar.

## As seis instâncias, todas de 2026-09-10

| # | a guarda dizia | o que era |
|---|---|---|
| **#307** | `git does not resolve on NO_FORGE_PATH` | o `python3` **copiado não iniciava** (DLL ausente) |
| corretivo ML-R2c2 | `gh.exe shim does NOT execute and return marker` | `python3: command not found` — o interpretador do probe |
| **ML-2B** (CI) | *"normal em PRs que adicionam o arquivo pela primeira vez"* | `origin/main` **não existe como ref** (checkout raso, `fetch-depth: 1`) |
| **#274** | `pass 0 / fail 1` | **suíte não carregou** *ou* **teste reprovou** — indistinguíveis |
| **ML-2B** (núcleo) | nome sumiu da lista ⇒ "corrigido" | pode ser **"deixou de executar"** |
| **#309** | `if 'newline' in call: continue` | verifica **presença**, declara **valor** — `newline="\r\n"` passa |

**Três foram achadas por consumidor externo. Três achamos sozinhos, no mesmo dia, depois de a
primeira nos ensinar o padrão** — e ainda assim reintroduzimos o defeito no corretivo da própria
correção.

## A regra

🔴 **Toda guarda que conclui a partir de uma ausência tem de responder duas perguntas separadas:**

1. **consegui procurar?** — o instrumento funcionou;
2. **achei?** — o alvo estava lá.

E **cada resposta tem mensagem própria**. Se as duas compartilham a mesma mensagem, o defeito já
existe, mesmo que ninguém tenha tropeçado ainda.

## Como aplicar — o padrão de correção

**Sonda de sanidade do instrumento ANTES da pergunta real**, e ela falha com mensagem própria:

```bash
# 1. o instrumento funciona?
"$REAL_PYTHON3" -c 'import sys; sys.exit(0)' || die "interpretador não inicia: $REAL_PYTHON3"

# 2. só então, a pergunta
PATH="$P" "$REAL_PYTHON3" -c '...git...'      || die "git não resolve em $P"
```

**Separar descoberta de visibilidade.** O `PATH` restrito existe para governar **o que o filho
resolve** — não para achar o interpretador. Interpretador **por caminho absoluto**; `PATH` só para o
filho. Misturar os dois papéis na mesma variável foi a causa de duas das seis.

**Em CI, "não consegui procurar" é FATAL, nunca aviso:**

```
ref indisponível        → fatal   (a verificação NÃO rodou)
ref ok + alvo ausente   → aviso   (a verificação rodou e não achou nada)
```

Degradar para aviso quando o instrumento falhou é **fail-open com aparência de tolerância**.

**Quando o discriminante não existir, DECLARE.** O `ML-2B` fez certo: sem `-rA`, o Python não
distingue "passou" de "não existe mais" — o gate **emite aviso e pula** aquela verificação, em vez de
fabricar distinção. Precedente contrário: o achado **A1** da auditoria externa de 2026-09-05, em que
medimos que `ENOTDIR` era indistinguível **e criamos um teste afirmando o contrário na mesma
entrega**.

## Como detectar em revisão

Ao ler uma guarda, pergunte: **"que outra coisa produz este mesmo observável?"**

- `exit != 0` → pode ser o comando falhando **ou o binário não existindo**;
- arquivo não abriu → pode não existir **ou o caminho estar errado** (ou o volume desmontado);
- nome não apareceu → pode ter passado **ou não ter rodado**;
- `git show <ref>:<path>` falhou → `path` ausente **ou `ref` ausente**;
- argumento presente → não diz **nada** sobre o valor dele (#309).

Se a resposta for "mais de uma coisa" e a mensagem é uma só, **é este defeito**.

## Sinal de alerta na redação da mensagem

Mensagem que explica **por que está tudo bem** é suspeita:

> *"normal em PRs que adicionam o arquivo pela primeira vez"*

Ela foi escrita para o caso benigno e **passou a cobrir o maligno**. Se a guarda precisa tranquilizar
o leitor, ela provavelmente está agrupando dois estados.

## Relacionado

- `CLAUDE.md` — Regra Dura de Reconciliação (o artefato não pode afirmar mais do que mediu)
- `ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem`
  — **D3**: *"o discriminante é o tipo do evento, nunca a contagem"*
- [bash-resolve-o-que-o-processo-filho-nativo-nao-resolve-no-windows-2026-09-09](bash-resolve-o-que-o-processo-filho-nativo-nao-resolve-no-windows-2026-09-09.md)
  — `command -v` acha o que o `CreateProcess` não acha: mesma família, no eixo de *quem procura*
