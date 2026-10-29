# PowerShell: função simples engole parâmetro nomeado desconhecido em `$args`

> Data: 2026-10-28 · Domínio: PowerShell, gates · Origem: ML-1A da REQ-2026-10-28 (install.ps1)

## Sintoma
O gate `scripts/check-install-ps1.ps1` montava os tarballs de fixture com
`New-TarGzFixture -Content $ExeContent`, mas o parâmetro declarado é `-EntryContent`. Nada falhou:
o `trackfw.exe` da fixture saía com **0 bytes**. No CI Windows isso teria derrubado só o caminho
feliz (c1), e com uma mensagem que aponta para o instalador, não para a fixture.

## Causa
Numa função **sem** `[CmdletBinding()]` e sem `[Parameter()]`, o PowerShell não valida os nomes
dos parâmetros: o nome desconhecido e o valor dele vão para `$args`, e o parâmetro declarado fica
no default (`$null`). Medido no pwsh 7.6.9:

```
function f { param([byte[]]$EntryContent) "bound=$($null -ne $EntryContent) args=$($args.Count)" }
f -Content ([byte[]](1,2))          → bound=False args=2
function g { [CmdletBinding()] param([byte[]]$EntryContent) "ok" }
g -Content ([byte[]](1,2))          → NamedParameterNotFound
```

## Regra
Toda função auxiliar de gate PowerShell declara `[CmdletBinding()]`. Assim o nome errado vira erro
na chamada, em vez de um `$null` silencioso.

## Ressalva
O comportamento sob Windows PowerShell 5.1 não foi medido aqui. É a mesma regra de binding
documentada para as duas edições, mas não conferida na 5.1.
