# Subprocesso de rede pendurado: o neto prende o **stderr**, e o teste com `cat` mascara a regressão

> 2026-10-12 — Wave 0 do ROADMAP-2026-10-12 (#556), medido pelo hades-tf em macOS; parecer em
> `docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md`, seções 2 e 5.

## O sintoma

`trackfw push` parado 70 min em `git fetch origin --prune`; o processo pendurado era o `ssh`,
**neto** do trackfw.

## O que não é óbvio

1. **`exec.CommandContext` sozinho não retorna.** O contexto mata o `git`, mas o `ssh` herdou o fd 2
   e segura o **stderr**, não o stdout. O `cmd.Output()` instala um pipe interno para stderr (para
   preencher `ExitError.Stderr`) mesmo com `cmd.Stderr == nil`, então o `Wait` fica esperando EOF.
2. **Quatro modos medidos** (prazo 2 s, `WaitDelay` 500 ms):

   | modo | retorna? | órfãos |
   |---|---|---|
   | só `CommandContext` | não | — |
   | + `WaitDelay` | sim | 1 |
   | + `Setpgid` sem `cmd.Cancel` | sim | 2 |
   | + `Setpgid` + `cmd.Cancel = kill(-pgid)` | sim, no prazo | 0 |

   `Setpgid` sem matar o grupo explicitamente **piora**. O `Cancel` padrão mata só o PID.
3. **`Setpgid` com terminal de controle** tira o filho do grupo de primeiro plano. Ao pedir a
   passphrase do `ssh` em `/dev/tty`, ele recebe SIGTTIN, e o Ctrl-C para de alcançá-lo. O
   discriminante é abrir `/dev/tty`, e não `os.Stdin.Stat()&ModeCharDevice`, porque `/dev/null`
   também é char device. Não medido (exige pty).
4. **Teste que não morde:**
   - um neto que **lê stdin** (`cat`) sai no EOF quando o `git` morre, e o modo "só
     `CommandContext`" passa;
   - sem `GIT_SSH_VARIANT=ssh`, o git roda a sonda `ssh -G`, que termina na hora.

   O neto correto é o próprio binário de teste reexecutado, dormindo (`time.Sleep`).
5. **`ps aux` não mostra variáveis de ambiente.** Detecte órfão pelo caminho do binário, capturando o
   `ps` em arquivo antes de filtrar.

6. **`exec.ErrWaitDelay` nunca é `*exec.ExitError`.** Quando o processo sai 0 mas um neto segura o
   pipe além do `WaitDelay`, o `Wait` devolve `exec.ErrWaitDelay` no lugar de nil (`go doc os/exec
   Cmd.WaitDelay`). Um teste com `errors.As(err, &exitErr) && exitErr.ExitCode()==0` é ramo morto: a
   primeira versão do ML-1A fez isso, e um push bem-sucedido cujo hook deixa daemon saía como erro. O
   certo é `errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success()`, avaliado só com
   `ctx.Err()==nil` (`resolveNetworkErr`, `internal/commands/netexec.go`).

## Ver também

- `internal/validator/validator_guard_binary_probe.go:138-147`: precedente do `WaitDelay`.
- `guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24`: orçamento
  total × ocioso. Pelo mesmo motivo, o `git push` ganhou teto próprio (30 min) e não o de 60 s.
