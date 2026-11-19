// guard-probe-plugin.js — sonda de guard para o OpenCode (dual-export V1+V2)
//
// NÃO é o produto final; é uma sonda de medição para o ML-5B (REQ-2026-11-17).
// REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli
//
// Formato: CommonJS (module.exports) — compatível com node --check e com Bun/OpenCode via interop CJS.
//   Bun importa module.exports como default export; OpenCode V2 detecta {id, setup} por shape.
//   V1 chama server(); V2 chama setup(). Sem duplo disparo.
//   Zero imports de runtime: apenas node:child_process e node:path (nativos; sem npm install).
//
// Variável de ambiente:
//   TRACKFW_PROBE_LOG — caminho do arquivo de log de sonda; opcional.
//     Se não definida ou vazia, o log é ignorado e o plugin só bloqueia (sem efeito colateral de IO).
//     Cada chamada ao guard appenda uma linha JSON com:
//       { tool, guard, arg_keys, project_dir, status, signal, error_code }
//     A linha é escrita ANTES de lançar a exceção, garantindo registro mesmo em caso de bloqueio.

"use strict"

const childProcess = require("node:child_process")
const nodePath = require("node:path")

// SHELL_TOOLS: IDs do tool de shell em V1 ("bash") e V2 ("shell")
const SHELL_TOOLS = new Set(["bash", "shell"])

// FILE_TOOLS: IDs dos tools de arquivo em V1 e V2 (mesmos IDs; apply_patch=V1, patch=V2)
const FILE_TOOLS = new Set(["write", "edit", "read", "patch", "apply_patch"])

/**
 * Appenda uma linha JSON de log em TRACKFW_PROBE_LOG.
 * Nunca lança — erros de IO no log são silenciados.
 * Chamada ANTES de qualquer throw para garantir rastreabilidade mesmo em bloqueio.
 *
 * @param {string} tool        — ID do tool OpenCode (bash, shell, write, etc.)
 * @param {string} guard       — subcomando do guard (credential, git-branch)
 * @param {string[]} argKeys   — chaves de args (sem valores); informa M17/M18 de ML-5A
 * @param {string} projectDir  — cwd passado ao guard (raiz do projeto)
 * @param {{ status: number|null, signal: string|null, error?: {code?: string} }} result
 */
function appendLog(tool, guard, argKeys, projectDir, result) {
  const logPath = process.env.TRACKFW_PROBE_LOG
  if (!logPath) return
  const line = JSON.stringify({
    tool: tool,
    guard: guard,
    arg_keys: argKeys,
    project_dir: projectDir,
    status: result.status,
    signal: result.signal || null,
    error_code: result.error ? (result.error.code || "unknown") : null,
  })
  try {
    require("node:fs").appendFileSync(logPath, line + "\n")
  } catch (_) {
    // Silencia erros de IO no log — nunca deixar o logger bloquear o guard
  }
}

/**
 * Chama trackfw guard <guardCmd> via spawnSync com argv array (nunca via shell).
 * Fail-closed: lança se result.error (ENOENT, ETIMEDOUT, etc.) OU status !== 0.
 *
 * @param {string} guardCmd   — "credential" ou "git-branch"
 * @param {string} tool       — ID do tool OpenCode (para log e tool_name no payload)
 * @param {object} args       — args do tool (output.args em V1; event.input em V2)
 * @param {string} projectDir — raiz do projeto (onde trackfw.yaml está; sem walk-up para credential)
 */
function runGuard(guardCmd, tool, args, projectDir) {
  // Resolve workdir (campo do shell tool) antes de usar como cwd
  // Pode ser relativo ao project dir (M18 aberto); resolvePath garante absoluto.
  const resolvedWorkdir = args.workdir
    ? nodePath.resolve(projectDir, args.workdir)
    : undefined

  // Monta tool_input como spread de args para:
  //   - colocar "command" em tool_input.command (ExtractCommand prioridade 1)
  //   - preservar campos desconhecidos (ex: diff text em apply_patch/patch — M17 aberto)
  //   - incluir file_path absoluto (V1=filePath, V2=path) para cobertura de Layer 1
  const toolInput = Object.assign({}, args)
  if (args.filePath) {
    toolInput.file_path = nodePath.resolve(projectDir, args.filePath)
  } else if (args.path) {
    toolInput.file_path = nodePath.resolve(projectDir, args.path)
  }

  // Payload: tool_input (prioridade 1 do ExtractCommand) + cwd para Layer 2 do credential guard
  //   cwd = workdir resolvido (ou projectDir) — usado por credGlobToken para resolver globs relativos
  const payload = {
    tool_name: tool,
    tool_input: toolInput,
    cwd: resolvedWorkdir || projectDir,
  }

  // credential guard: cwd = projectDir (checa trackfw.yaml diretamente, sem walk-up)
  // git-branch guard: cwd = workdir resolvido (FindProjectRoot faz walk-up)
  const guardCwd = guardCmd === "credential"
    ? projectDir
    : (resolvedWorkdir || projectDir)

  // spawnSync com argv array — nunca via shell (evita D18: errexit herdado bloqueia string poliglota)
  // timeout: 5000 ms; expirado → result.error.code = ETIMEDOUT, status = null → fail-closed
  const result = childProcess.spawnSync("trackfw", ["guard", guardCmd], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    timeout: 5000,
    cwd: guardCwd,
  })

  // Log ANTES de lançar — garante rastreabilidade mesmo em bloqueio
  appendLog(tool, guardCmd, Object.keys(toolInput), projectDir, result)

  // Fail-closed: lança em qualquer erro (ENOENT, timeout, crash) ou status !== 0
  if (result.error || result.status !== 0) {
    throw new Error(
      result.error
        ? "trackfw not found: " + result.error.message
        : (result.stderr || "").trim() || "trackfw guard " + guardCmd + ": operation blocked"
    )
  }
}

/**
 * Verifica o tool e chama os guards aplicáveis.
 * Shell (bash/shell): credential + git-branch.
 * File (write/edit/read/patch/apply_patch): credential only.
 */
function checkTools(tool, args, projectDir) {
  if (SHELL_TOOLS.has(tool)) {
    runGuard("credential", tool, args, projectDir)
    runGuard("git-branch", tool, args, projectDir)
  } else if (FILE_TOOLS.has(tool)) {
    runGuard("credential", tool, args, projectDir)
  }
}

module.exports = {
  id: "trackfw.guard.probe",

  // V2 (OpenCode 2.x) — setup(ctx) chamado pelo loader V2
  // ctx.location.project.directory = raiz do projeto (confirmado em packages/plugin/src/promise/plugin.ts@v2.3.0)
  // NÃO usar ctx.location.directory = workspace cwd (pode ser subdiretório; credential guard sem walk-up)
  setup: async function setup(ctx) {
    const projectDir = ctx.location.project.directory
    await ctx.tool.hook("execute.before", function(event) {
      checkTools(event.tool, event.input || {}, projectDir)
    })
  },

  // V1 (OpenCode 1.x) — server({directory, worktree}) chamado pelo loader V1
  // worktree = raiz git do projeto (onde trackfw.yaml está)
  // directory = cwd do workspace (pode ser subdiretório — NÃO usar para credential guard)
  // Fonte: packages/plugin/src/index.ts@v1.19.5 PluginInput.worktree + .directory
  server: async function server(input) {
    const directory = input.directory
    const worktree = input.worktree
    const projectDir = worktree || directory
    return {
      "tool.execute.before": async function(hookInput, hookOutput) {
        checkTools(hookInput.tool, hookOutput.args || {}, projectDir)
      },
    }
  },
}
