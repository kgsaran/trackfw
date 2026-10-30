# Wave 0 — Modelo de Ameaça: Fonte ls-remote na Resolução da Branch Padrão

> Roadmap: ROADMAP-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md
> REQ: REQ-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md
> ADR: ADR-2026-10-30-resolucao-da-branch-padrao-consulta-o-remoto-quando-o-symref-local-falta.md
> Data: 2026-10-30 | Revisor: hades-tf

Mapa ação do roadmap → seção deste documento:

| ação | seção |
|---|---|
| 1. Confiança: hostil, flag injection, `..`, espaço, `-` | Seção 2 — V-1 |
| 2. Rede: prazo, offline, nova chamada onde não havia | Seção 2 — V-2, V-3 |
| 3. Enumeração fechada dos chamadores com efeito em cada sítio | Seção 1 |
| 4. Fakes de gitExec | Seção 2 — V-4 |
| 5. Alvos de falsificação AC1–AC6 | Seção 3 |
| 6. Prescrições e residual | Seções 4–5 |

---

## 1. Completude da Enumeração

O ADR declara: "Usam essa cadeia o `branch new`, o `branch prune` e o `ship`." A enumeração por
`grep -rn "resolveDefaultBranch\|defaultBaseBranch" internal/commands/` expõe cinco sítios:

| sítio | função chamada | recebe o fix do ADR? |
|---|---|---|
| `branch.go:350` (`branch new`) | `resolveDefaultBranch` → `resolveDefaultBranchWithSource` | sim |
| `branch_prune.go:423` (`branch prune`) | `resolveDefaultBranch` → `resolveDefaultBranchWithSource` | sim |
| `ship.go:765` (`detectPendingSquashMerges`) | `resolveDefaultBranch` → `resolveDefaultBranchWithSource` | sim |
| `ship.go:477` (`buildPRBody` via `gitCommitsSince`) | `defaultBaseBranch` em `ship.go:593` | NÃO — sítio separado |
| `release.go:275` | `defaultBaseBranch` | NÃO — sítio separado |

`defaultBaseBranch` (`ship.go:593`) é uma função local que só chama
`git symbolic-ref refs/remotes/origin/HEAD` e cai em `"main"` sem intermediário.
Ela não usa `resolveDefaultBranchWithSource` e não recebe o fix do ADR a não ser que o ML-1A
inclua explicitamente esse sítio.

### Medição A1 — fixture `master` sem symref (HOME isolado)

Estes comandos foram executados com `HOME=$(mktemp -d) GIT_CONFIG_NOSYSTEM=1`; o gitconfig
pessoal não influencia os resultados:

```
$ HOME=$FAKEHOME GIT_CONFIG_NOSYSTEM=1 git fetch origin
# (completa sem output)

$ HOME=$FAKEHOME GIT_CONFIG_NOSYSTEM=1 git symbolic-ref refs/remotes/origin/HEAD
fatal: ref refs/remotes/origin/HEAD is not a symbolic ref
EXIT=128   ← symref AUSENTE após fetch (git 2.54, followRemoteHEAD=never)

$ HOME=$FAKEHOME GIT_CONFIG_NOSYSTEM=1 git ls-remote --symref origin HEAD
ref: refs/heads/master	HEAD
f12f83133990aea3cf2b19cc70376d8a1414f603	HEAD
EXIT=0   ← resposta correta disponível via ls-remote

$ HOME=$FAKEHOME GIT_CONFIG_NOSYSTEM=1 ./tf branch new --dry-run chore/dry-run-master
[dry-run] would create branch "chore/dry-run-master" (base não resolvida: trackfw branch new:
a ref base 'origin/main' não foi encontrada ...)
BUG CONFIRMADO: base resolvida como origin/main (errada) em vez de origin/master

$ HOME=$FAKEHOME GIT_CONFIG_NOSYSTEM=1 ./tf branch prune
trackfw branch prune: origin/main not found — offline, no remote configured, or never fetched.
Refusing to evaluate any branch; nothing deleted.
BUG CONFIRMADO: recusa inteira por referência errada
```

### Medição AC5 — `defaultBaseBranch` no caminho do `ship` (proxy por leitura de código)

`defaultBaseBranch` não foi executada via `ship` — a medição abaixo é por leitura de código
e verificação de efeito de cadeia, não por execução do comando `ship`. Classificada como proxy.

```
$ git symbolic-ref refs/remotes/origin/HEAD
fatal: ref refs/remotes/origin/HEAD is not a symbolic ref  ← defaultBaseBranch devolve "main"

$ git rev-parse --verify main
fatal: Needed a single revision
main ref ABSENT

$ git log main..HEAD --format=%H
fatal: ambiguous argument 'main..HEAD': unknown revision or path not in the working tree.
EXIT=128
```

`gitCommitsSince("main", ...)` recebe exit 128, devolve `nil`. `buildPRBody` produz corpo
mínimo sem lista de commits — falha silenciosa.

Para `release.go:275` (ADR-2026-08-19 Emenda 1): `base = defaultBaseBranch(deps.execGit)`
devolve `"main"`. `rev-parse origin/main` falha. `localSHA = ""`. A verificação de
desatualização da branch local é pulada por inteiro (comentário no código: "failure to resolve
it... must not block reaching the forge resolution below"). A forge ainda decide o commit-target.
Efeito: não catastrófico, mas a salvaguarda local é silenciosamente inerte.

**Veredicto de completude:** a enumeração do ADR está INCOMPLETA para `defaultBaseBranch`. A
AC5 da REQ cobre ambos os sítios — se o resultado estiver errado (medido acima: está), o ML-1A
deve corrigir. Ver discussão de independência na Seção 5 (Prescrições, P-3).

---

## 2. Modelo de Ameaça

### V-1: Resultado hostil do ls-remote — nome de branch arbitrário

O remoto `origin` controla integralmente a saída de `git ls-remote --symref origin HEAD`.
Um remoto hostil (ou um `origin` substituído) pode servir qualquer string na linha
`ref: refs/heads/<nome>\tHEAD`. Esta superfície é mais aberta que o symref local: o symref
passa pela API de refs do git (que valida nomes ao escrever), enquanto a saída do ls-remote é
texto de protocolo bruto, sem validação no lado do cliente.

**Vetores medidos via forja direta de `$BARE2/HEAD`:**

| entrada forjada | saída de ls-remote | o que rev-parse recebe | resultado |
|---|---|---|---|
| `ref: refs/heads/-broken\tHEAD` | `ref: refs/heads/-broken\tHEAD` | `origin/-broken` (começa por `origin/`, não por `-`) | ref aceita; exit 0; branch padrão errada, não injeção de flag |
| `ref: refs/heads/../master\tHEAD` | (vazio — git server rejeita `..`) | cai no literal | seguro via rejeição do servidor padrão |
| `ref: refs/heads/nome com espaço\tHEAD` | (vazio — git server rejeita) | cai no literal | seguro via rejeição do servidor padrão |

**Atenção:** "seguro" para `..` e espaço descreve o comportamento de um servidor git padrão.
Um remoto hostil pode implementar um protocolo próprio e servir qualquer bytes. O que
garante segurança não é a rejeição do servidor, mas a validação do lado do cliente (P-2).

**Injeção de flag:** `branch.go:353` usa `--end-of-options`; `branch_prune.go:425` não usa.
Em ambos os casos o valor passado ao `rev-parse` é `"origin/" + localName`, que começa por
`"origin/"` e nunca por `"-"`. Injeção de flag não é viável por esse caminho.

**Risco de revisão relativa:** `branch.go:353` constrói `remoteRef+"^{commit}"` e passa para
`rev-parse`. Se `localName = "main~1"`, o argumento fica `"origin/main~1^{commit}"` — que o
git interpreta como o commit um passo antes de `origin/main`. Sem injection, mas com resultado
silenciosamente errado (base da branch nova = commit anterior ao topo de main). Vetores
similares: `main^`, `main@{0}`, `a:b`, `a*b`. P-2 usa lista de permissão que os rejeita.

**Por que lista de permissão, não lista de negação:** a lista de negação pode ser esgotada por
caracteres não antecipados que o git interpreta como revisão. A lista de permissão garante que
apenas caracteres sem semântica especial no git passam.

### V-2: Hang de rede — GIT_TERMINAL_PROMPT não configurado

`defaultGitExec` (`netexec.go:160–210`) não configura `GIT_TERMINAL_PROMPT=0` para nenhuma
operação de rede. Grep confirma: zero ocorrências em `netexec.go`.

```
grep -n "GIT_TERMINAL_PROMPT" internal/commands/netexec.go
(sem resultado)
```

Um origin HTTPS que exige autenticação pode abrir prompt ou aguardar até o prazo de 60 s
(`networkSubcmds["ls-remote"] = "metadata"`). Esta lacuna já afeta `fetch` — a nova chamada
`ls-remote` herda o mesmo comportamento. **Não é uma nova classe de problema.**

**Comportamento offline (medido):**

```
timeout 10 git ls-remote --symref https://127.0.0.1:19999/x HEAD
fatal: unable to access '...': Failed to connect to 127.0.0.1 port 19999 after 0 ms
EXIT=128
```

Recusa de conexão imediata → `ls-remote` falha instantaneamente → cai no literal. Prazo de
60 s só é atingido quando o endereço é alcançável mas lento, não em offline puro.

### V-3: Nova chamada de rede onde antes havia apenas chamada local

Antes do fix, `detectPendingSquashMerges` (ship.go:765) chamava `resolveDefaultBranch` →
`resolveDefaultBranchWithSource` que, quando a forge não responde, fazia só `symbolic-ref`
(local). Depois do fix, quando a forge não responde e o symref está ausente, haverá uma chamada
adicional de `ls-remote` (rede) de até 60 s. Não afeta ambientes com symref presente (caso mais
comum) nem ambientes com forge respondendo (symref nunca alcançado).

P-3 também adiciona ls-remote a `defaultBaseBranch`, chamada de `buildPRBody` (ship.go:477) e
`release.go:275`. Quando o symref está ausente nessas rotas — por exemplo, clone raso ou repo
`master` sem `followRemoteHEAD` — ambas passam a fazer uma chamada de rede adicional. Em
`release.go`, essa chamada ocorre antes da resolução da forge (linha 347), mas depois do
`git fetch origin --prune` (linha 271) — a janela de rede já estava aberta.

### V-4: Fakes de gitExec — o default case silencia o ls-remote

Os fakes de `gitExec` em testes de `internal/commands/` têm três formas de tratar
subcomandos não declarados:

| fake | arquivo | comportamento com nova chamada `ls-remote --symref origin HEAD` | resultado |
|---|---|---|---|
| `mockGit.exec` (switch/case, default `return "", nil`) | `ship_test.go` | ls-remote retorna `"", nil` → sem linha `ref:` → literal | sem regressão |
| `mockReleaseGit.exec` (map lookup, default `return "", nil`) | `release_test.go` | ls-remote não está no map → `return "", nil` → literal | sem regressão |
| `makeRemoteGitExec` via `ghExec` (forge responde primeiro) | testes de `resolveDefaultBranchWithSource` | forge intercepta; ls-remote nunca alcançado na cadeia da forge | sem regressão |

**`TestDefaultBranch_NeitherAvailable_FallsBackToMain`** usa `default: fmt.Errorf(...)` para
todos os comandos. Após o fix, ls-remote também recebe erro (consumido); o teste passa, mas o
nome não documenta que ls-remote foi tentado. Não é regressão — é ausência de documentação de caso.

**`TestReleaseTag_NoLocalSymref_ForgeBranchNameUsedWithoutFalseRefusal`** (release_test.go)
coloca `symbolic-ref refs/remotes/origin/HEAD` no mapa de erros. Após P-3, o ls-remote é chamado;
`"ls-remote --symref origin HEAD"` não está no mapa → `return "", nil` → literal "main". Mesmo
resultado de antes, mas agora o ls-remote é exercitado silenciosamente. O teste não declara esse
caminho — P-4 deve acrescentar um case explícito neste teste também.

**`TestDefaultBaseBranch_SymbolicRefFails_FallsBackToMain`** (ship_test.go:413) usa
`return "", errors.New(...)` para todos os args. Após P-3, ls-remote recebe erro, cai no literal.
Passa, mas o nome está desatualizado — ML-1A deve renomeá-lo ou adicionar variante nomeada.

---

## 3. Alvos de Falsificação

### AC1 — `TestBranchNew_DryRunFetchesMasterDefault` reproduz sem-2.48 em qualquer git

**Falso negativo:** o fixture não tem `followRemoteHEAD never`; em git ≥ 2.48 o fetch recria
o symref e o teste passa mesmo sem ls-remote. Porta: o comentário
`"after fetch, origin/HEAD is updated"` permanece afirmando a premissa errada.
Gate: verificar que `remote.origin.followRemoteHEAD = never` está no fixture E o comentário foi
corrigido.

**Falso positivo:** com `followRemoteHEAD never`, o fixture mantém o symref ausente em qualquer
git. Medido: após fetch no git 2.54 com essa config, `refs/remotes/origin/HEAD` está ausente.
Gate fechado por construção.

### AC2 — contra-braço: sem ls-remote, o AC1 reprova

**Falso negativo:** o contra-braço remove ls-remote mas usa `ghExec` que responde com a forge
→ a forge mascara o bug; o teste passa mesmo sem ls-remote. Gate: o contra-braço deve ter
`ghExec = nil` ou a forge devolvendo erro, para forçar o caminho symref → ls-remote → literal.

**Falso positivo:** a remoção de ls-remote no contra-braço é incompleta (fake ainda devolve valor).
Gate: a remoção deve ser do bloco de código, não do fake. Verificar que o fixture tem
`followRemoteHEAD never` e `ghExec = nil`.

### AC3 — teste de `branch prune` num repositório `master` sem symref

**Falso negativo:** a forge mascara o bug. Para que o teste seja válido, `ghExec = nil` no
fixture. Gate: confirmar que `resolveDefaultBranchWithSource` não recebe ghExec não-nil no teste.

**Falso positivo:** o teste reprova porque `origin/master` não existe no fixture (teste não
criou a branch remota corretamente). Gate: verificar que o bare tem `refs/heads/master` e que
`git ls-remote --symref origin HEAD` retorna `ref: refs/heads/master`.

### AC4 — testes unitários do parse

**Falso negativo:** o parse aceita `ref: refs/heads/-bad\tHEAD` sem rejeitar o nome;
ou aceita `ref: refs/heads/main\tOTHER` sem verificar que o segundo campo é `HEAD`.
Gate: os vetores de P-2 devem incluir `"-bad"` esperando rejeição, `"main~1"` esperando rejeição,
e o parse-level vector `ref: refs/tags/v1\tHEAD` esperando cair no literal.

**Falso positivo (R-2):** a lista de permissão rejeita nome legítimo unicode ou com caractere
incomum (`ä`, `+`, `@{upstream}`); a cadeia cai no literal e o bug #633 é recriado para esse
repositório. Esse residual está declarado em R-2.
Gate: o ML-1A deve listar os caracteres aceitos e deixar claro que unicode cai no literal.

### AC5 — `ship defaultBaseBranch` medido

**Falso negativo:** o ML-1A entrega o fix de `resolveDefaultBranchWithSource` sem tocar
`defaultBaseBranch`, e a auditoria aceita porque `ship.go` não está na lista de arquivos afetados.
Gate: a AC5 da REQ exige medição (ou execução de `ship`). A medição por proxy neste documento
confirma comportamento errado — o ML-1A deve incluir `ship.go` como arquivo afetado.

**Falso positivo:** o fix de `defaultBaseBranch` alarga a assinatura da função adicionando um
parâmetro de forge, alterando ambos os chamadores desnecessariamente.
Gate: P-3 proíbe explicitamente isso. `defaultBaseBranch` deve usar apenas `execGit` via helper
compartilhado (`parseLsRemoteSymref` + `isValidBranchLocalName`). Nenhum parâmetro novo deve ser
adicionado à assinatura.

### AC6 — pacote `internal/commands` inteiro verde

**Falso negativo:** um fake com `default: return "", nil` para ls-remote faz o teste passar sem
exercitar o caso de ls-remote com sucesso.
Gate: o fake de AC3 e AC4 deve incluir case explícito para `"ls-remote"`.

**Falso positivo:** os fakes de `mockGit` e `mockReleaseGit` usam `return "", nil` como default
— não `t.Fatalf`. Ls-remote é consumido silenciosamente em ambos (V-4), sem regressão. Fakes de
`resolveDefaultBranchWithSource` com forge interceptam antes do ls-remote — também sem regressão.
Gate: se um novo teste de `defaultBaseBranch` usar um fake com `default: t.Fatalf`, ele falhará
ao receber ls-remote inesperado. Declarar o case explicitamente em qualquer fake que imponha
`t.Fatalf` para comandos não declarados.

---

## 4. Residual Declarado

**R-1: `GIT_TERMINAL_PROMPT=0` ausente em todas as chamadas de rede (pré-existente).**
A nova chamada `ls-remote` herda essa lacuna. Não é nova; fecha por REQ própria.

**R-2: Validação de `localName` não cobre toda a gramática de refs do git.**
A prescrição P-2 usa uma lista de permissão restrita. Nomes de branch legítimos com caracteres
fora do padrão (ex: `%`, `@{`) são rejeitados e caem no literal. Melhor cair no literal que
aceitar nomes malformados.

**R-3: `release.go:275` — cross-check de forge não afetado; staleness check restaurado.**
Leitura de `release.go:338–383` (ADR-2026-08-19 Emenda 1): o commit-target vem do forge via
`repoInfo.DefaultBranch` (linha 353, de `gh api repos/{owner}/{repo}`). A comparação de
independência (`forgeLocalSHA` vs `commitObj.SHA`, linha 380) usa `repoInfo.DefaultBranch`
— não `base`. Se `base` estiver errado, apenas o staleness check (verificação de que a branch
local está atualizada com o remote) é silenciosamente pulado; o cross-check permanece intacto.

Adicionando ls-remote a `defaultBaseBranch` via helper compartilhado (P-3): `base` passa a ser
`master` (correto). O staleness check é restaurado. O cross-check `forgeLocalSHA` vs
`commitObj.SHA` continua usando `repoInfo.DefaultBranch` — independência preservada.

Os comentários que deixam de ser verdadeiros são listados em P-3 (três sítios). O ML-1A
deve atualizá-los todos.

**R-5: `gitCommitsSince` usa o nome nu (`base`) sem prefixo `origin/` — falha em fresh clone (pré-existente).**
`gitCommitsSince(base, execGit)` executa `git log <base>..HEAD`. Em um clone raso ou init+fetch
sem checkout, `refs/heads/<base>` não existe — o nome nu falha com exit 128 mesmo que
`origin/<base>` resolva. Medido: `git rev-parse --verify master` EXIT=128; `git rev-parse
--verify origin/master` EXIT=0 em fixture init+fetch. Essa limitação existe HOJE com `"main"`:
se `refs/heads/main` não existir, `git log main..HEAD` falha e `gitCommitsSince` devolve nil.
P-3 não introduce nenhum comportamento novo neste plano. Fecha por REQ própria; não é escopo do ML-1A.

**R-4: Windows — `ls-remote` contra remote local requer MSYS `/usr/bin` em PATH.**
Detalhado em `vault/notes/path-curado-no-windows-tres-medicoes-que-invertem-o-diagnostico-2026-09-24.md`.
Quando o PATH é reduzido a apenas `git-core` (sem `sh.exe` + `msys-2.0.dll`), `git ls-remote`
contra um remote local falha com exit 139 / `0xC0000005` (violação de acesso). O mesmo sintoma
afeta `git fetch` — os testes E2E de fetch já carregam essa exposição. `ls-remote` não adiciona
nova classe de problema. Remotes HTTPS não são afetados (não precisam de `sh.exe`).

---

## Prescrições

As prescrições abaixo são obrigatórias para o ML-1A. Nenhuma linha de implementação foi escrita
neste documento — as prescrições são regras comportamentais com vetores de teste.

**P-1: Nova fonte entre symref e literal em `resolveDefaultBranchWithSource`**

Posição: entre o bloco que consulta `symbolic-ref --short refs/remotes/origin/HEAD` e o bloco
de fallback literal. Comportamento exigido:
- Chamar `gitExec("ls-remote", "--symref", "origin", "HEAD")`.
- Iterar as linhas da saída.
- Para cada linha, aceitar SOMENTE o padrão `ref: refs/heads/<nome>\tHEAD`, onde o segundo
  campo separado por tab é exatamente `HEAD` (não `refs/heads/HEAD`, não `HEAD2`, não ausente).
- Extrair `<nome>` e passá-lo por `isValidBranchLocalName` (P-2).
- Qualquer saída vazia, erro de processo, timeout ou linha sem o prefixo `ref: refs/heads/`
  cai no fallback literal — sem mensagem de erro ao usuário.
- Este bloco só é atingido quando `symbolic-ref refs/remotes/origin/HEAD` retorna erro (`err != nil`).
  Saída vazia ou prefixo inválido do symbolic-ref NÃO disparam ls-remote (P-3, caminhos b e c).
- O resultado retorna `fromForge=false`.

**P-2: Função `isValidBranchLocalName`**

Regras de aceitação (lista de permissão, não lista de negação):
Lista de permissão (não lista de negação — razão: caracteres de revisão git, como `~`, `^`,
`@{`, `:`, `*`, podem ser servidos por um remoto hostil; uma lista de negação é esgotável):

- Cada caractere deve estar em `[a-zA-Z0-9._/\-]`.
- String não vazia.
- Não começa com `-`.
- Não contém `..` (`.` está na lista de permissão, portanto `a..b` passa pela verificação de
  caractere isolada; a verificação explícita de `..` é necessária — não é defesa em profundidade).

Vetores de teste obrigatórios:

| entrada | esperado | razão |
|---|---|---|
| `""` | rejeitar | vazio |
| `"-bad"` | rejeitar | lida como flag |
| `"a..b"` | rejeitar | revisão relativa |
| `"main~1"` | rejeitar | fora da lista de permissão (`~`) |
| `"main^"` | rejeitar | fora da lista de permissão (`^`) |
| `"main@{0}"` | rejeitar | fora da lista de permissão (`@{`) |
| `"a:b"` | rejeitar | fora da lista de permissão (`:`) |
| `"a*b"` | rejeitar | fora da lista de permissão (`*`) |
| `"main"` | aceitar | nome simples |
| `"master"` | aceitar | nome simples |
| `"release/7.2"` | aceitar | barra permitida (branch com componentes) |
| `"feat/my-feature"` | aceitar | hífen permitido |

Vetores de parse obrigatórios para P-1 (formato da linha ls-remote):

| linha | segundo campo | esperado |
|---|---|---|
| `"ref: refs/heads/main\tHEAD"` | `HEAD` | aceitar → `"main"` |
| `"ref: refs/tags/v1\tHEAD"` | `HEAD` (mas não `heads`) | cair no literal |
| `"ref: refs/heads/main\tHEADX"` | `HEADX` ≠ `HEAD` | cair no literal |
| `"ref: refs/heads/main"` (sem tab) | ausente | cair no literal |
| `"ref: refs/heads/main\tHEAD\r"` (CRLF) | strip `\r` por linha antes de comparar | aceitar |

**P-3: `defaultBaseBranch` em `ship.go` — helper compartilhado, sem forge**

AC5 mostra resultado errado. O sítio deve ser atualizado com a MESMA lógica de ls-remote de P-1/P-2,
usando um helper compartilhado (`parseLsRemoteSymref` e `isValidBranchLocalName`) de modo que
`resolveDefaultBranchWithSource` e `defaultBaseBranch` não possam divergir.

`defaultBaseBranch` tem três caminhos que devolvem `"main"`: (a) `err != nil` após `symbolic-ref`,
(b) prefixo ausente, (c) nome vazio após strip. O ls-remote entra no caminho (a) apenas — quando o
`symbolic-ref` falha com erro. Os caminhos (b) e (c) indicam saída malformada do git local e não
devem tentar a rede.

Proibição explícita: `defaultBaseBranch` NÃO deve chamar `resolveDefaultBranchWithSource`.
Três razões concretas:

1. **Assinatura incompatível.** `defaultBaseBranch(execGit)` recebe apenas `execGit`; forge é
   acessado via `ghExec`/`execForgeAPI` — um parâmetro diferente. Chamar
   `resolveDefaultBranchWithSource` exigiria adicionar esse parâmetro a `defaultBaseBranch` e
   atualizar os dois chamadores (`ship.go:477` e `release.go:275`), alargando o escopo sem ganho.

2. **Ordenação em `release.go`.** `defaultBaseBranch` é chamado na linha 275, antes da resolução
   da forge (linha 347) e antes da verificação de disponibilidade do adaptador (linha 334). Chamar
   a forge dentro de `defaultBaseBranch` colocaria uma chamada REST no caminho de inicialização,
   antes das pré-condições que a protegem.

3. **Desnecessário.** O staleness check (linhas 285-296) compara dois refs locais:
   `refs/heads/<base>` contra `origin/<base>`. Nenhuma forge participa. Ls-remote (transporte
   local ou de rede sobre o protocolo git) é suficiente — e o que AC5 exige.

O cross-check de forge descrito em R-3 (`forgeLocalSHA` vs `commitObj.SHA`) usa
`repoInfo.DefaultBranch` — não `base`. A independência desse cross-check não é afetada por P-3.

ML-1A deve atualizar os três comentários a seguir para refletir que ls-remote pode ser consultado
quando o symref está ausente:

- `ship.go:584-591` (docstring de `defaultBaseBranch`): "It tries `git symbolic-ref
  refs/remotes/origin/HEAD` and falls back to 'main' when that fails" + "This is a LOCAL,
  gravable ref".
- `release.go:268-270`: "base is symref-derived — a LOCAL, gravable ref" + "(a) the value the
  forge's default_branch must agree with".
- `release.go:341-342`: "defaultBaseBranch then falls back to 'main'".

**P-4: Testes que precisam declarar o case ls-remote explicitamente**

Os testes abaixo exercitam o caminho `err != nil` de `defaultBaseBranch` e, após P-3, receberão
uma chamada `ls-remote --symref origin HEAD`. Cada um deve incluir case explícito no fake e
declarar a frase de reconciliação no relatório do ML-1A.

- **`TestDefaultBranch_NeitherAvailable_FallsBackToMain`** (`internal/commands/branch_prune_default_branch_test.go:373`):
  fake com `default: return "", fmt.Errorf(...)` → ls-remote também falha → cai no literal.
  Frase: "este teste afirma que, quando forge, symref local e ls-remote falham, a cadeia cai
  no literal `main`."

- **`TestReleaseTag_NoLocalSymref_ForgeBranchNameUsedWithoutFalseRefusal`** (`release_test.go`):
  `symbolic-ref` no mapa de erros → após P-3, ls-remote chamado → mock retorna `"", nil` (default)
  → sem linha `ref:` → literal "main". O case ls-remote deve ser declarado explicitamente como
  `"ls-remote --symref origin HEAD": ""` (vazio, não `master`). **Atenção:** se o case devolver
  `"ref: refs/heads/master\tHEAD"`, `base` passa a ser `"master"`, `rev-parse -q --verify
  refs/heads/master` retorna `"", nil` (não está nos erros do mock), o staleness check dispara,
  e o teste falha. Declarar o case como vazio preserva o comportamento atual do teste.
  Frase: "este teste afirma que, quando symref está ausente e ls-remote não devolve symref válido,
  `defaultBaseBranch` cai em `main` sem bloquear a tag."

- **`TestDefaultBaseBranch_SymbolicRefFails_FallsBackToMain`** (`ship_test.go:413`): fake retorna
  erro para todos os args → ls-remote também falha. O ML-1A deve renomear o teste para
  `TestDefaultBaseBranch_SymrefAndLsRemoteFail_FallsBackToMain` ou adicionar variante nomeada.
  Frase: "este teste afirma que, quando symref e ls-remote falham, `defaultBaseBranch` cai em
  `main`."

**Sobre call-sequence assertions (medido):** `ship_test.go` e `release_test.go` têm loops
`for _, c := range g.calls` que verificam presença/ausência de `push`, `commit` e `add` — nenhum
verifica ls-remote nem conta total de chamadas. P-3 não quebra nenhum teste existente por esse
mecanismo.

**P-5: `GIT_TERMINAL_PROMPT=0` — não exigido neste ML**

Pré-existente. Abrir issue separada. E2E contra fixture local não são afetados.
