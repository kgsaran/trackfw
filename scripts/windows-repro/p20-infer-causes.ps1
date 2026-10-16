# p20-infer-causes.ps1 — sonda de causas INFER do ML-W3a (REQ-2026-09-03).
#
# Pergunta 20 do windows-probe.yml.
#
# O que esta sonda mede:
#   INFER-1  TestRenderWithoutIdentityMatchesFrozenGoldens/subagent/architect
#            — golden gerado em macOS (LF, attr/-text) diverge da saida Windows
#              porque go:embed bake CRLF + normalizeMarkdown nao chama NormalizeCRLF
#   INFER-2  TestResolveAgentModelMatchesRender
#            — 16 combinacoes gemini/copilot/windsurf/kiro x 4 agentes;
#              extractFrontmatterField verifica "---\n" mas Render devolve "---\r\n"
#   INFER-3  TestUpdateMigratesKnownCodexAndPreservesUnknown
#            — filepath.Join emite "\" no Windows; manifest JSON serializa como "\\"
#              strings.Contains falha
#   MEASURED TestRenderOpenCodeAgent_CRLFSourceMatchesLF
#   MEASURED TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF
#            — se go:embed ja entrega CRLF, injecao manual gera "\r\r\n";
#              NormalizeCRLF (uma passagem) deixa "\r" residual
#
# Saida grep-friendly:
#   PROBE w3 eol <arquivo> crlf=<n> lf=<n>
#   PROBE w3 test=<nome> exit=<n>
#   PROBE w3 hex_diverge ...
#   PROBE w3 hex_got  <hex>
#   PROBE w3 hex_want <hex>
#   PROBE w3 manifest_key_form=<form>
#
# NAO corrige nada. Sai 0 sempre.
# ---------------------------------------------------------------------------

param(
    [string]$Workspace = $env:GITHUB_WORKSPACE
)

$ErrorActionPreference = 'Continue'

if ($Workspace) {
    Set-Location $Workspace
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "=== P20: Pergunta 20 — causas INFER ML-W3a ==="

# ── (a) EOL config e atributos ────────────────────────────────────────────
Write-Host "=== P20-EOL: git config core.autocrlf ==="
git config --show-origin --get core.autocrlf 2>&1 | Write-Host

Write-Host "=== P20-EOL: git ls-files --eol (assets e goldens) ==="
git ls-files --eol -- `
    internal/integrations/assets/agents/architect.md `
    internal/integrations/assets/agents/backend.md `
    internal/integrations/assets/agents/frontend.md `
    internal/integrations/assets/agents/opencode.md `
    internal/integrations/testdata/architect.subagent.golden.md `
    internal/integrations/testdata/backend.agent-directory.golden.md `
    internal/generators/update_test.go 2>&1 | Write-Host

# ── (b) Contagem de bytes CRLF/LF por arquivo ──────────────────────────
Write-Host "=== P20-EOL: contagem bytes CRLF/LF ==="
python "$scriptDir\p20-eol-count.py" 2>&1 | Write-Host

# ── (c) go test -json + analise hex para cada teste ──────────────────────
$testCases = @(
    [PSCustomObject]@{ name = "TestRenderWithoutIdentityMatchesFrozenGoldens";           pkg = "./internal/integrations/" },
    [PSCustomObject]@{ name = "TestRenderOpenCodeAgent_CRLFSourceMatchesLF";             pkg = "./internal/integrations/" },
    [PSCustomObject]@{ name = "TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF"; pkg = "./internal/integrations/" },
    [PSCustomObject]@{ name = "TestResolveAgentModelMatchesRender";                      pkg = "./internal/integrations/" },
    [PSCustomObject]@{ name = "TestUpdateMigratesKnownCodexAndPreservesUnknown";         pkg = "./internal/generators/" }
)

$hexScript = "$scriptDir\p20-hex-analyze.py"

foreach ($tc in $testCases) {
    $tname = $tc.name
    $pkg   = $tc.pkg
    Write-Host ""
    Write-Host "=== P20 test=$tname ==="
    $jsonLines = go test -count=1 -json -run "^$($tname)$" $pkg 2>&1
    $exitCode  = $LASTEXITCODE
    Write-Host "PROBE w3 test=$tname exit=$exitCode"
    $jsonLines | python "$hexScript" $tname 2>&1 | Write-Host
}

Write-Host ""
Write-Host "=== P20 CONCLUIDA — aguardar saida Windows para confirmar causas ==="
exit 0
