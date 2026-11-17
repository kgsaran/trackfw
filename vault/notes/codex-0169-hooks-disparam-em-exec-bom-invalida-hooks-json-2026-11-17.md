# codex 0.169.0 — hooks em exec mode (Responses API): achados corrigidos

> Data: 2026-11-17 | Medição por: artemis-tf | ML-4A (REQ-2026-11-15)
> 🔴 NOTA: o título do arquivo foi preservado por rastreabilidade; o achado original foi
> invertido — ver seção "Correção". Renomear é tarefa do arquiteto.

## Achado original (invalidado)

A nota original afirmava: "nenhum PreToolUse hook dispara em `codex exec` mode". Essa conclusão foi
baseada em 6 chamadas ao modelo que não criaram `out\hook_fired.txt`. O contra-braço desta sessão
mostrou que a causa era o `hooks.json` com UTF-8 BOM — não o comportamento do caminho de execução.

## Correção — hooks DISPARAM em exec mode

Chamada 3/4 (SessionStart-only hooks.json, sem BOM): `c3_session_fired.txt` criado → **FIRED**.
Chamada 4/4 (PreToolUse-only hooks.json, matcher `".*"`, sem BOM): `c4_pretool_fired.txt` criado →
**FIRED**.

Ambas as chamadas completaram normalmente (rc=0, ~9s), com JSONL confirmando exec de comando.

## Achado BOM — causa raiz das 6 chamadas originais

`hooks.json` escrito por PowerShell sem codificação explícita ganha BOM `EF BB BF` (UTF-8 BOM).
O serde_json do Codex 0.169.0 rejeita BOM com `"expected value at line 1 column 1"`.

Comportamento segundo flag:
- Com `--dangerously-bypass-hook-trust`: erro emitido como item `error` no JSONL; execução prossegue sem hooks
- Sem o flag: erro silencioso; execução prossegue sem hooks

As 6 chamadas originais tinham `hooks.json` com BOM (escrito via PowerShell). Hooks nunca foram
carregados. O mecanismo "PreToolUse não dispara" estava baseado em medição com hooks desabilitados.

Correção para escrita via PowerShell:
```powershell
$utf8NoBOM = [System.Text.UTF8Encoding]::new($false)
[System.IO.File]::WriteAllText($path, $json, $utf8NoBOM)
```

Ou: escrever o arquivo no macOS e transferir via `scp`.

## Shell e argv empírico (exec mode) — mantido

```
argv[0] = C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe
argv[1] = -Command
argv[2] = '<user_command>'
```

Sem `-NoProfile`, sem `-NonInteractive`. Contradiz leitura de código-fonte de 2026-10-04.
A medição empírica prevalece (ADR D2).

## HOME resolution — mantido

Codex 0.169.0 usa a API do SO Windows (não `HOME` nem `USERPROFILE` env):
- `HOME` env alterado → CODEX_HOME não muda
- `USERPROFILE` env alterado → CODEX_HOME não muda
- `CODEX_HOME` env definido → respeitado (lê `$CODEX_HOME/config.toml` no nível raiz, NÃO `.codex/`)

Fonte: `codex-rs/utils/home-dir/src/lib.rs::find_codex_home()` → `dirs` 6.0.0 →
`dirs-sys` 0.5.0 → `SHGetKnownFolderPath(FOLDERID_Profile)`.

## Shell de hooks (source code — Rust binary)

Hooks em Windows são executados via `cmd.exe /C "<command>"` com `CREATE_NO_WINDOW` e
`env_clear()`. Não usa PowerShell. Fonte: `codex-rs/hooks/src/engine/command_runner.rs:444-510`.

## Implicações para trackfw

1. **Guard de credencial** (`InjectCodexHooks`: `PreToolUse` matcher `"Bash"`) **funciona** em
   `codex exec` mode — desde que o `hooks.json` seja escrito sem BOM.
2. **`HomeFor(CLICodex)`**: diverge de Codex em 3 condições (HOME definido; USERPROFILE override
   com HOME vazio; CODEX_HOME definido). Em ambiente padrão, coincidem.
3. **BOM crítico**: qualquer ferramenta que escreva `hooks.json` via PowerShell no Windows deve
   usar `UTF8Encoding($false)`. Falha silenciosa sem `--dangerously-bypass-hook-trust`.

## Documento de medição completo

`docs/portabilidade/2026-11-17-ml-4a-codex-flags-e-home-windows.md`
