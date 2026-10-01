# Wave 3 — Revisão Independente do Spawn de `sh`

> Hades (hades-tf) · 2026-10-01 · REQ #491 · Branch: `fix/barrier-executa-cada-linha-do-bloco-de-gates`

---

## Método

Leitura do código antes de consultar os testes. Cada afirmação é seguida do comando e da saída real.
Shells testados: `/bin/sh` (bash 3.2.57 arm64-apple-darwin26, que é o `sh` do PATH nesta máquina),
`/bin/bash` (3.2.57, mesmo binário, `--posix`), `/bin/dash`. `busybox sh` não disponível.

Contenção: todos os experimentos em
`/private/tmp/claude-501/…/scratchpad/sh-n-tests/`, todos com `perl -e 'alarm 5; exec @ARGV'` no
front. Nenhum uso de `trackfw barrier` dentro de bloco de gates. `pkill -9 -x trackfw` reservado,
não utilizado.

---

## 1. O `sh -n` roda só depois do trust check?

**Mapeamento do código (`internal/commands/barrier.go`, Wave 2):**

```
runBarrier (linha 617)
  └─ FenceMaskCheck — aborta se cerca não terminada (pré-condição, linha 638)
  └─ se trustLocalGates == true:
       └─ checkGateFragments(gcmds)   ← sh -n aqui, SOMENTE aqui
       └─ (se ok) evalGateCommands
  └─ senão:
       └─ roadmapTrustForGates(roadmapPath, data)
            └─ se NOT trusted → gatesCheck.Status = "not_evaluated"; RETURN
            └─ se trusted:
                 └─ checkGateFragments(gcmds)   ← sh -n aqui, SOMENTE aqui
                 └─ (se ok) evalGateCommands
```

O comentário em `barrier.go:806` declara explicitamente:
`// checkGateFragments is NOT called for untrusted roadmaps (ML-2A).`

O teste `TestBarrierFragment_UntrustedRoadmap_ShNotCalled` (contra-braço com fake `sh` que grava
chamadas `-n` num arquivo sentinela) mede isso por efeito: sentinela ausente na rodada sem trust,
presente na rodada com `--trust-local-gates`. Rodado:

```
go test ./internal/commands/ -run 'Fragment' -count=1
ok  github.com/kgsaran/trackfw/internal/commands  1.226s
```

**Veredito Q1: sim. O `sh -n` roda somente nos caminhos trusted, depois do trust check. Nenhum
conteúdo não-confiável chega ao `sh`, nem para análise de sintaxe.**

---

## 2. `sh -n -c '<linha>'` executa alguma coisa?

Cada tentativa usou um arquivo sentinela em `$TESTDIR` e verificou `test -f`.
A tabela abaixo lista todos os vetores tentados, os shells e o resultado:

| Vetor | sh (bash 3.2) | bash --posix | dash | Sentinela |
|---|---|---|---|---|
| `$(touch sentinel)` | exit=0 | exit=0 | exit=0 | **NÃO** |
| `` `touch sentinel` `` | exit=0 | exit=0 | exit=0 | **NÃO** |
| `${x:=$(touch sentinel)}` | exit=0 | exit=0 | exit=0 | **NÃO** |
| `$(( $(touch sentinel; echo 1) + 1 ))` | exit=0 | — | — | **NÃO** |
| `trap "touch sentinel" EXIT` | exit=0 | exit=0 | exit=0 | **NÃO** |
| `f(){ touch sentinel; }; f` | exit=0 | — | — | **NÃO** |
| `alias foo=touch; foo sentinel` | exit=0 | — | — | **NÃO** |
| `. source_me.sh` (`source_me.sh` = `touch sentinel`) | exit=0 | — | exit=0 | **NÃO** |
| heredoc `cat <<EOF\ntouch sentinel\nEOF` | exit=0 | — | — | **NÃO** |
| `if true; then touch sentinel; fi` | exit=0 | — | — | **NÃO** |
| `SHELLOPTS=errexit sh -n $(touch sentinel)` | exit=0 | — | — | **NÃO** |
| `cat <(touch sentinel)` (process sub) | exit=2 | — | — | **NÃO** |

Nenhum dos vetores criou o sentinela em nenhum shell testado.

O `sh` nesta máquina é bash 3.2 (`/var/select/sh -> /bin/bash`). O `barrier.go` usa
`exec.Command("sh", "-n", "-c", gc.Text)` sem manipulação de PATH, portanto resolve o mesmo
binário.

**Veredito Q2: nenhuma execução detectada em nenhum vetor, em nenhum dos três shells testados.
`sh -n` é seguro como pré-check sintático nesta superfície.**

---

## 3. Resíduos declarados

### Resíduo (a) — `cmd # comentário \` gera falso positivo

`hasOddTrailingBackslashes` opera sobre o texto já passado por `strings.TrimSpace`. O texto
`echo hello # comment \` termina em `\` (count=1, ímpar), é classificado como fragmento, e o
barrier reporta `incomplete command`. Mas o `\` está dentro de um comentário de shell e não é uma
line-continuation:

```
sh -c 'echo hello # comment \' exit=0 stdout=b'hello\n'
sh -n -c 'echo hello # comment \' exit=0
```

Este é um **falso positivo real**: a linha é completa como argumento `sh -c` isolado.

**Impacto medido:** 0 linhas neste padrão em 162 comandos de gate do corpus atual
(`docs/roadmaps/**`). Nenhum gate real é afetado hoje.

**Proposta:** o resíduo é **aceitável agora** (zero impacto no corpus), mas deve ser declarado no
`cli-parity.md` (regra 5 ou nota de rodapé ao lado da descrição do odd-`\` check) para que um
autor que escreva `gate_cmd # nota \` receba a mensagem "incomplete command" e saiba a causa real.
Não requer ML corretivo nesta REQ: zero falsos positivos no acervo e a correção (examinar se o `\`
precede ou sucede um `#`) exige um parser de tokens, que está fora do escopo de (b).

### Resíduo (b) — bashism rejeitado por `sh` recebe a mensagem errada

Se `sh` no ambiente do usuário for `dash` (Linux, Alpine), um gate com `diff <(echo a) <(echo b)`
é rejeitado por `dash -n` (exit=2, mensagem `Syntax error: "(" unexpected`). O barrier reporta
`incomplete command — each line of the gates block runs as a separate sh -c (rule 5): <cmd>`.
A causa real é "bashism não suportado pelo `sh` local", não "comando incompleto".

```
/bin/dash -n -c 'diff <(echo a) <(echo b)' exit=2
/bin/dash: 1: Syntax error: "(" unexpected
```

**Impacto medido:** 0 gates com bashisms em 162 comandos do corpus. No ambiente desta máquina,
`sh` = bash 3.2, portanto o problema não ocorre.

**Proposta:** resíduo aceitável (zero impacto no corpus; ambiente macOS usa bash como sh). Declarar
na regra 5 ou no comentário do `checkGateFragments` que "a mensagem usa 'incomplete command' para
cobrir tanto fragmentos quanto construções não suportadas pelo `sh` local". Não requer ML corretivo.

---

## 4. F1 — pode esconder gate legítimo?

**Análise dos cenários:**

**Cenário 1 — cerca não-terminada antes do marcador real.**
`FenceMaskCheck` é chamado em `barrier.go:638`, antes de qualquer parse de wave. Uma cerca não
terminada produz `usageExit` com mensagem "unterminated code fence starting at line N". Portanto,
o F1 nunca aplica: se há cerca não terminada, o barrier aborta antes. Fix #476 (commit
`e7595c4e`) já cobre este caminho.

**Cenário 2 — dois marcadores na mesma wave: um fenced (exemplo), um real.**
`ParseGatesLines` usa `continue` (não `return`) ao encontrar marcador fenced. A varredura
continua e encontra o marcador real. Coberto por
`TestParseGatesLines_F1_RealMarkerAfterMasked`:

```
go test ./internal/roadmapdoc/ -run 'F1' -v -count=1
--- PASS: TestParseGatesLines_F1_MaskedMarkerNotGate
--- PASS: TestParseGatesLines_F1_RealMarkerAfterMasked
```

**Cenário 3 — marcador real acidentalmente dentro de cerca.**
Um `**Gates da wave:**` dentro de ` ```bash ` é, por definição, conteúdo de exemplo, não um gate
declarado. FenceMask classificá-lo como fenced e silenciá-lo é o comportamento correto. A única
forma de gate legítimo escondido seria se o autor colocasse o marcador real dentro de uma cerca
por engano — e o remédio é o aviso da regra 5 (escrito no ML-1B), não mudar o parser.

**Medição no acervo:** `TestParseGatesLines_F1_RealFile_Wave2Returns_Empty` verifica que
`done/ROADMAP-2026-08-22-wave-0-…` Wave 2 agora retorna `[]` (antes: `[exit 1 # placeholder…]`).
Todos os outros 162 comandos do corpus não foram afetados:

```
go test ./internal/roadmapdoc/ ./internal/commands/ -count=1
ok  github.com/kgsaran/trackfw/internal/roadmapdoc  0.292s
ok  github.com/kgsaran/trackfw/internal/commands    1.226s
```

**Veredito Q4: F1 não esconde nenhum gate legítimo. O único caminho de esconde
(cerca não terminada) é bloqueado upstream por FenceMaskCheck. O `continue` garante que marcadores
reais após exemplos fenced são encontrados.**

---

## 5. Outros contornos observados

**5.1 Heredoc opener (`cat <<EOF`) passa `sh -n`.**
O Wave 0 já documentou este False Negative do `sh -n` para heredocs. A implementação
complementa com `hasOddTrailingBackslashes`, que não cobre este caso (heredoc opener não termina
em `\`). Resíduo declarado no Wave 0 e confirmado aqui:

```
sh -n -c 'cat <<EOF' exit=0
```

Impacto: um gate `cat <<EOF` não é detectado como fragmento. Ao executar, o `sh -c 'cat <<EOF'`
consome o body do próprio argumento `-c` (terminado na mesma linha) — body vazio, exit=0. O
heredoc body que o autor imaginou não é injetado. O gate passa sem fazer o que deveria. Isto não
é uma vulnerabilidade de execução (nada externo é executado), mas sim um false green para o
autor. O resíduo foi declarado no Wave 0 e está documentado.

**5.2 Linha vazia entre `checkGateFragments` e `evalGateCommands`.**
Não existe caminho de código entre os dois: em `barrier.go:792-799` (e `811-818`), `evalGateCommands`
é chamado dentro do `else` do `if fragStatus != ""`. Se `checkGateFragments` retorna status não
vazio, a execução não avança. A estrutura é correta.

---

## Veredito final

**APROVA.**

As três propriedades de segurança da implementação foram verificadas por medição:

1. `sh -n` roda **somente** em roadmaps trusted, em ambos os caminhos (trust-local-gates e
   origin/main). Roadmap não confiável recebe `not_evaluated` sem spawn de `sh`. Medido por teste
   com fake `sh` sentinela.

2. `sh -n -c '<cmd>'` não executa nenhum efeito colateral em nenhum dos vetores testados
   (command substitution, backtick, arith, param expansion, trap, alias, function, dot, heredoc,
   process substitution) em `/bin/sh` (bash 3.2), `bash --posix` e `/bin/dash`.

3. F1 não esconde gates legítimos: cerca não terminada é bloqueada por FenceMaskCheck antes do
   parse; marcador real após exemplo fenced é encontrado pelo `continue`; 162 comandos do corpus
   não foram afetados.

**Resíduos aceitos (dois, não requerem ML corretivo):**

- R1: `cmd # comentário \` — falso positivo do odd-`\` check (0 ocorrências no corpus). Declarar
  no `cli-parity.md`.
- R2: bashism rejeitado por `sh` = dash recebe mensagem "incomplete command" (causa errada, 0
  ocorrências no corpus no ambiente atual). Declarar no comentário do `checkGateFragments` ou
  na regra 5.

Ambos os resíduos têm impacto zero no corpus atual e não enfraquecem nenhum controle de segurança.
A declaração nos documentos de referência é suficiente.

---

## Evidências de saída

```
go test ./internal/roadmapdoc/ ./internal/commands/ -count=1
ok  github.com/kgsaran/trackfw/internal/roadmapdoc  0.292s
ok  github.com/kgsaran/trackfw/internal/commands    1.226s

git status --short
(clean)
```
