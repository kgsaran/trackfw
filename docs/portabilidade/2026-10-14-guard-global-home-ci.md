# Guard global com `HOME` ≠ `USERPROFILE` — medição no CI x64 (AC2 da REQ-2026-09-01)

> 2026-10-14 · zeus-tf · `windows-probe.yml`, Pergunta 16, run `38618567001` (`windows-latest` x64),
> branch `fix/guard-global-segue-o-home-que-o-agente-le`. Os dois binários foram compilados no mesmo
> runner: `origin/main` e o commit da branch.

## Resultado

| braço | onde o guard do Claude foi gravado | o que o `validate` diz |
|---|---|---|
| `main` | `HOME` (`D:\a\_temp\p16-fakehome-main\.claude\settings.json`) | nada sobre guard global nem divergência — o falso positivo de saúde |
| branch | `USERPROFILE` (`C:\Users\runneradmin\.claude\settings.json`); `HOME\.claude\settings.json` sem marcador | `home_divergence_windows`: `HOME="D:\\a\\_temp\\p16-fakehome-branch" differs from the OS home (USERPROFILE) "C:\\Users\\runneradmin" — Claude Code reads the OS home (measured)…` |

Linhas brutas: `PROBE guard-global main written=HOME` · `PROBE guard-global branch written=USERPROFILE`.

## Limites

- O Claude Code não está no runner. Que ele lê `USERPROFILE` vem da VM
  (`2026-10-14-guard-global-home-userprofile-vm.md`, `--debug-file`, três terminais).
- Os dois braços terminam com `validate_exit=1` no projeto temporário da sonda — igual nos dois, logo
  não é efeito da mudança. O step da Pergunta 16 sai com 1 por propagar esse código; a sonda é sem
  veredito e o script foi corrigido para sair 0 (ver commit seguinte).
- A Pergunta 5c (Node) também falha no mesmo run: sonda pré-v8, fora desta REQ.
