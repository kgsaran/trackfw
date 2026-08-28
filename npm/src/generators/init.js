'use strict'

const fs = require('fs')
const path = require('path')
const os = require('os')
const { readAgentConventions } = require('../config/index.js')

const GOV_DIRS = [
  'docs/adr',
  'docs/req',
  'docs/roadmaps/backlog',
  'docs/roadmaps/analyzing',
  'docs/roadmaps/wip',
  'docs/roadmaps/blocked',
  'docs/roadmaps/done',
  'docs/roadmaps/abandoned',
  'vault/notes',
]

const GLOBAL_ADRS_DIRECTIVE = 'Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.'

/**
 * scaffold(cfg) — cria diretórios de governança e gera arquivos de configuração.
 * cfg = { projectName, projectType, frontend, backend, pkgManager, hooks, ci }
 */
async function scaffold(cfg, cwd) {
  const root = cwd || process.cwd()
  for (const dir of GOV_DIRS) {
    fs.mkdirSync(path.join(root, dir), { recursive: true })
    console.log(`  ✓ ${dir}`)
  }

  generateVaultIndex(root)
  writeTrackfwConfig(cfg, root)
  generateValidateScript(cfg, root)
  generateAttentionScripts(cfg, root)
  generateCredentialGuardScript(root)
  generateGitBranchGuardScript(root)
  generateCIWorkflow(cfg, root)
  generateGitHooks(cfg, root)
  generateCommitMsgHook(cfg, root)
  generateClaudeMD(cfg, root)
  if (cfg.backend === 'java') generatePomXml(cfg, root)
  generateClaudeCommands(root)
  injectHooksDetected(root)
}

/**
 * generateVaultIndex — cria vault/notes/index.md se ainda não existir.
 * @param {string} root
 */
function generateVaultIndex(root) {
  const indexPath = path.join(root || process.cwd(), 'vault', 'notes', 'index.md')
  if (fs.existsSync(indexPath)) return
  const content = `# Vault de Conhecimento

> Ponto de entrada de conhecimento do projeto para agentes e pessoas.
> Cada nota documenta uma causa-raiz, decisão técnica ou restrição não óbvia.
> Crie notas com: trackfw note new "<título>"

## Índice

<!-- As notas serão listadas abaixo. Exemplo:
- [nome-da-nota-YYYY-MM-DD](nome-da-nota-YYYY-MM-DD.md)
-->
`
  fs.writeFileSync(indexPath, content, 'utf8')
  console.log('  ✓ vault/notes/index.md')
}

// ---------------------------------------------------------------------------
// trackfw.yaml
// ---------------------------------------------------------------------------

function writeTrackfwConfig(cfg) {
  const today = new Date().toISOString().slice(0, 10)
  let content = `# trackfw configuration
# generated: ${today}

frontend: ${cfg.frontend || ''}
backend: ${cfg.backend || ''}
backend_framework: ${cfg.backendFramework || ''}
pkg_manager: ${cfg.pkgManager || ''}
hooks: ${cfg.hooks || ''}
ci: ${cfg.ci || ''}${cfg.forge ? `\nforge: ${cfg.forge}` : ''}
require_req_in_commit: ${cfg.requireReqInCommit ? 'true' : 'false'}

# governance paths (edit to match your project structure)
adr_dirs:
  - docs/adr
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
`
  if (cfg.brownfieldMode) {
    const until = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
    content += `governance_mode: lenient\nlenient_until: ${until}\n`
  }
  fs.writeFileSync('trackfw.yaml', content, 'utf8')
  console.log('  ✓ trackfw.yaml')
}

// ---------------------------------------------------------------------------
// scripts/trackfw-validate.sh
// ---------------------------------------------------------------------------

// generateValidateScript — cwd is optional and defaults to process.cwd() so
// existing callers (scaffold(), which always runs with process.cwd() already
// at the project root) keep working unchanged. `trackfw update`'s
// validate-script target passes cwd explicitly so this can be applied
// against a --dry-run sandbox root without ever touching the real project
// tree — this is the SAME canonical generator scaffold() uses for `init`,
// not a separate copy (see npm/src/commands/discover.js's writeValidateScript,
// which is a different, simpler generator used only by `discover`/legacy
// paths and must never be reused here — that mismatch was the ML-6H
// validate-script parity bug: init wrote the rich per-backend script,
// `update` overwrote it with the static 3-line one, so idempotent re-runs
// reported "updated" instead of "skipped").
function generateValidateScript(cfg, cwd) {
  const root = cwd || process.cwd()
  fs.mkdirSync(path.join(root, 'scripts'), { recursive: true })

  const script = buildValidateScript(cfg)
  const scriptPath = path.join(root, 'scripts', 'trackfw-validate.sh')
  fs.writeFileSync(scriptPath, script, { encoding: 'utf8', mode: 0o755 })
  // AC9 (REQ-2026-08-28): writeFileSync's {mode} option applies only on O_CREAT;
  // for an existing file the mode is not changed. fs.chmodSync restores 0o755
  // unconditionally, matching Python's os.chmod behavior (which was already correct).
  fs.chmodSync(scriptPath, 0o755)
  console.log(`  ✓ ${path.join('scripts', 'trackfw-validate.sh')}`)
}

function buildValidateScript(cfg) {
  let base = `#!/usr/bin/env sh
# trackfw governance gate — generated by trackfw init
set -e

echo "→ trackfw: validating governance..."
trackfw validate

`

  switch (cfg.backend) {
    case 'go':
      base += 'echo "→ build check (go)..."\ngo build ./...\n'
      break
    case 'java':
      base += 'echo "→ build check (maven)..."\nmvn compile -q\n'
      break
    case 'node': {
      const pm = cfg.pkgManager || 'npm'
      base += `echo "→ build check (node)..."\n${pm} run build\n`
      break
    }
    case 'python':
      base += "echo \"→ build check (python)...\"\npython3 -c \"import pathlib, py_compile; [py_compile.compile(str(p), doraise=True) for p in pathlib.Path('.').rglob('*.py') if '.venv' not in p.parts and 'venv' not in p.parts]\"\n"
      break
  }

  if (['react', 'vue', 'angular'].includes(cfg.frontend)) {
    const pm = cfg.pkgManager && cfg.pkgManager !== 'none' ? cfg.pkgManager : 'npm'
    base += `echo "→ frontend build check..."\n${pm} run build\n`
  }

  base += '\necho "✓ all checks passed."\n'
  return base
}

// ---------------------------------------------------------------------------
// scripts/trackfw-attention-signal.sh + trackfw-attention-cleanup.sh
// ---------------------------------------------------------------------------

function generateAttentionScripts(cfg, cwd) {
  const { generateAttentionScripts: gen } = require('./hooks')
  gen(cfg, cwd || process.cwd())
}

function generateCredentialGuardScript(cwd) {
  const { generateCredentialGuardScript: gen } = require('./hooks')
  gen(cwd || process.cwd())
}

function generateGitBranchGuardScript(cwd) {
  const { generateGitBranchGuardScript: gen } = require('./hooks')
  gen(cwd || process.cwd())
}

function injectHooksDetected(cwd) {
  const { injectHooksDetected: inject } = require('./hooks')
  inject(cwd || process.cwd())
}

// ---------------------------------------------------------------------------
// CI workflows
// ---------------------------------------------------------------------------

function generateCIWorkflow(cfg) {
  switch (cfg.ci) {
    case 'github-actions':
      generateGitHubActionsWorkflow()
      break
    case 'gitlab-ci':
      generateGitLabCIWorkflow()
      break
  }
}

// buildGitHubActionsWorkflowContent returns the byte-identical content the GitHub Actions
// workflow generator would write. Exported for scaffold doctor (ADR-2026-08-27): the
// comparison uses the same function the write path uses, so drift is structurally impossible.
// The cfg parameter is accepted for API symmetry with Go's buildGitHubActionsWorkflowContent
// (which also ignores cfg for now — the template is not cfg-dependent).
function buildGitHubActionsWorkflowContent(_cfg) {
  return `name: trackfw-gate
on:
  pull_request:
    branches: [main]

jobs:
  governance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install trackfw
        run: |
          curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh

      - name: Governance gate
        run: trackfw validate
`
}

// buildGitLabCIWorkflowContent mirrors buildGitHubActionsWorkflowContent for GitLab CI.
function buildGitLabCIWorkflowContent(_cfg) {
  return `# trackfw governance gate
trackfw-gate:
  stage: test
  image: alpine:latest
  before_script:
    - apk add --no-cache curl
    - curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  script:
    - trackfw validate
  only:
    - merge_requests
`
}

function generateGitHubActionsWorkflow() {
  fs.mkdirSync('.github/workflows', { recursive: true })
  const filePath = '.github/workflows/trackfw-gate.yml'
  fs.writeFileSync(filePath, buildGitHubActionsWorkflowContent(null), 'utf8')
  console.log(`  ✓ ${filePath}`)
}

function generateGitLabCIWorkflow() {
  fs.writeFileSync('.gitlab-ci-trackfw.yml', buildGitLabCIWorkflowContent(null), 'utf8')
  console.log('  ✓ .gitlab-ci-trackfw.yml')
}

// ---------------------------------------------------------------------------
// Git hooks
// ---------------------------------------------------------------------------

function generateGitHooks(cfg) {
  switch (cfg.hooks) {
    case 'husky':
      generateHuskyHook()
      break
    case 'lefthook':
      generateLefthookHook()
      break
  }
}

function generateHuskyHook() {
  fs.mkdirSync('.husky', { recursive: true })
  const content = '#!/usr/bin/env sh\n. "$(dirname -- "$0")/_/husky.sh"\n\ntrackfw validate\n'
  const filePath = '.husky/pre-commit'
  fs.writeFileSync(filePath, content, { encoding: 'utf8', mode: 0o755 })
  console.log(`  ✓ ${filePath}`)
}

function generateLefthookHook() {
  const content = `pre-commit:
  commands:
    trackfw-validate:
      run: trackfw validate
`
  fs.writeFileSync('lefthook.yml', content, 'utf8')
  console.log('  ✓ lefthook.yml')
}

function generateCommitMsgHook(cfg) {
  if (!cfg.requireReqInCommit || cfg.hooks === 'none') return

  const script = [
    '#!/bin/sh',
    '# trackfw: require REQ reference in feat/* and fix/* branches',
    'BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null || echo "")',
    'case "$BRANCH" in',
    '  feat/*|fix/*)',
    '    if ! grep -qE "^(REQ|req): " "$1"; then',
    '      echo "ERROR: Commits in feat/* and fix/* branches require a REQ reference."',
    '      echo "  Add to commit body: REQ: REQ-YYYY-MM-DD-your-req-slug"',
    '      exit 1',
    '    fi',
    '    ;;',
    'esac',
    '',
  ].join('\n')

  if (cfg.hooks === 'husky') {
    fs.mkdirSync('.husky', { recursive: true })
    fs.writeFileSync('.husky/commit-msg', script, { encoding: 'utf8', mode: 0o755 })
    console.log('  ✓ .husky/commit-msg')
  } else if (cfg.hooks === 'lefthook') {
    const lefthookPath = 'lefthook.yml'
    const existing = fs.existsSync(lefthookPath) ? fs.readFileSync(lefthookPath, 'utf8') : ''
    if (!existing.includes('commit-msg:')) {
      const addition = '\ncommit-msg:\n  scripts:\n    "trackfw-req-check.sh":\n      runner: sh\n'
      fs.writeFileSync(lefthookPath, existing + addition, 'utf8')
    }
    fs.mkdirSync('.lefthook/commit-msg', { recursive: true })
    fs.writeFileSync('.lefthook/commit-msg/trackfw-req-check.sh', script, { encoding: 'utf8', mode: 0o755 })
    console.log('  ✓ .lefthook/commit-msg/trackfw-req-check.sh')
  }
}

// ---------------------------------------------------------------------------
// pom.xml (Java / Spring Boot)
// ---------------------------------------------------------------------------

function generatePomXml(cfg) {
  const slug = cfg.projectName
    ? cfg.projectName.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '')
    : 'my-app'
  const name = cfg.projectName || 'My App'
  const content = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>3.3.0</version>
    <relativePath/>
  </parent>
  <groupId>com.example</groupId>
  <artifactId>${slug}</artifactId>
  <version>0.0.1-SNAPSHOT</version>
  <name>${name}</name>
  <description>${name} — generated by trackfw</description>
  <properties>
    <java.version>21</java.version>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-actuator</artifactId>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-test</artifactId>
      <scope>test</scope>
    </dependency>
  </dependencies>
  <build>
    <plugins>
      <plugin>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-maven-plugin</artifactId>
      </plugin>
    </plugins>
  </build>
</project>
`
  fs.writeFileSync('pom.xml', content, 'utf8')
  console.log('  ✓ pom.xml')
}

// ---------------------------------------------------------------------------
// trackfw rules inject-or-update (idempotente via marcadores HTML)
// ---------------------------------------------------------------------------

const RULES_START = '<!-- trackfw:rules:start -->'
const RULES_END = '<!-- trackfw:rules:end -->'

const AGENT_FILES = {
  claude:   'CLAUDE.md',
  codex:    'AGENTS.md',
  gemini:   'GEMINI.md',
  copilot:  '.github/copilot-instructions.md',
  windsurf: '.windsurfrules',
  amazonq:  '.amazonq/developer/guidelines.md',
  cursor:   '.cursor/rules/trackfw.mdc',
}

const AGENT_HEADERS = {
  claude:   '# Project Instructions\n',
  codex:    '# Project Instructions\n',
  gemini:   '# Project Instructions\n',
  copilot:  '# GitHub Copilot Instructions\n',
  windsurf: '# Windsurf Rules\n',
  amazonq:  '# Amazon Q Developer Guidelines\n',
  cursor:   '---\ndescription: trackfw governance rules\nglob: "**/*"\nalwaysApply: true\n---\n',
}

function trackfwRulesBlock(agentConventions) {
  let conventionsSection = ''
  if (agentConventions && String(agentConventions).trim() !== '') {
    conventionsSection = `

### Project Conventions
> Declared by the team in \`trackfw.yaml\`'s \`agent_conventions\` field — NOT
> inferred automatically. trackfw does not impose an architectural standard; it only
> propagates what the project has already decided.

${String(agentConventions).trim()}
`
  }

  return RULES_START + `
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: \`ADR → REQ → ROADMAP\` · States: \`backlog / analyzing / wip / blocked / done / abandoned\`

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   \`trackfw req new "title"\` → \`trackfw roadmap new "title"\` → \`trackfw roadmap move <name> wip\` → \`git checkout -b feat/<branch>\`
   ❌ Never create a branch before REQ + ROADMAP are in wip/
   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ✓ \`trackfw validate\` enforces this via \`branch_has_wip_roadmap\` rule (v2.7.0+)
2. **Before starting:** run \`trackfw context\` · read \`docs/agents-working-context.md\`
3. **After finishing:** update \`docs/agents-working-context.md\` with what changed
4. **Before PR:** \`trackfw validate\` must pass
5. **ML lifecycle — mandatory:**
   - Starting a ML: edit roadmap \`**Status:** ⬜ Pendente\` → \`**Status:** 🔄 Em andamento\` + commit.
   - Completing a ML: edit roadmap → \`**Status:** ✅ Concluído\` + include in ML commit.
   - Analyzing a roadmap: move from \`backlog/\` to \`analyzing/\`; to \`wip/\` only when coding starts.
6. **${GLOBAL_ADRS_DIRECTIVE}**

### Attention Signal (when you need user input during a task)
Write \`docs/roadmaps/.trackfw-attention.json\`:
\`\`\`json
{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}
\`\`\`
Delete the file when resolved. Visible as a live banner in \`trackfw serve\`.

> **Windsurf users:** before asking the user a question or requesting approval, write
> \`<roadmap_dir>/.trackfw-attention.json\` manually — there is no automatic hook for this.
> Delete the file after the user responds.

### Architecture Directives (mandatory)
- **3-layer separation:** frontend / backend / database — never mix concerns
- **No in-memory data:** always database + ORM (never arrays/globals for persistence)
- **Auth from day 1:** never defer — refactoring auth later is very costly
- **Docker + .env from day 1:** containerize early; all config via env vars
- **2-layer validation:** frontend (UX) + backend (security) — never only one
- **API-first:** define OpenAPI contract before coding frontend/backend integration
- **Threat model waves:** every feature roadmap opens with a Wave 0 threat model (before implementation) and closes with a red-team review wave (before release)
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use \`/trackfw:architect\` to define stack before the first REQ
` + conventionsSection + `
### Key Commands
- \`trackfw context\` — current governance state (always run first)
- \`trackfw status\` — all artifacts and states
- \`trackfw validate\` — governance consistency check
- \`trackfw roadmap move <name> <state>\` — transition roadmap state
- \`trackfw serve\` — live Kanban board at http://localhost:4080
` + RULES_END
}

/**
 * injectOrUpdateRules — injeta ou atualiza o bloco de regras trackfw no arquivo.
 * - Arquivo não existe: cria com headerIfNew + bloco de regras
 * - Arquivo existe, sem marcador: appenda o bloco no final
 * - Arquivo existe, com marcador: substitui conteúdo entre marcadores
 */
function injectOrUpdateRules(filePath, headerIfNew, cwd) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true })

  const block = trackfwRulesBlock(readAgentConventions(cwd))

  if (!fs.existsSync(filePath)) {
    let content = headerIfNew || ''
    if (content && !content.endsWith('\n')) content += '\n'
    content += '\n' + block + '\n'
    fs.writeFileSync(filePath, content, 'utf8')
    return
  }

  let content = fs.readFileSync(filePath, 'utf8')

  const start = content.indexOf(RULES_START)
  if (start === -1) {
    // No marker: append
    if (!content.endsWith('\n')) content += '\n'
    content += '\n' + block + '\n'
    fs.writeFileSync(filePath, content, 'utf8')
    return
  }

  // Has marker: replace between start and end
  const end = content.indexOf(RULES_END, start)
  if (end === -1) {
    // Malformed: append new block
    content += '\n' + block + '\n'
    fs.writeFileSync(filePath, content, 'utf8')
    return
  }

  const newContent = content.slice(0, start) + block + content.slice(end + RULES_END.length)
  fs.writeFileSync(filePath, newContent, 'utf8')
}

/**
 * injectRulesForTool — injeta regras trackfw no arquivo de config do tool dado.
 * tool: 'claude' | 'codex' | 'gemini' | 'copilot' | 'windsurf' | 'amazonq' | 'cursor'
 * cwd: diretório raiz do projeto (default: process.cwd())
 */
function injectRulesForTool(tool, cwd) {
  const relPath = AGENT_FILES[tool]
  if (!relPath) return
  const header = AGENT_HEADERS[tool] || ''
  const root = cwd || process.cwd()
  injectOrUpdateRules(path.join(root, relPath), header, root)
}

/**
 * injectRulesDetected — varre cwd por arquivos de config de agentes existentes
 * e injeta as regras trackfw em cada um encontrado.
 * Cursor: injeta também se .cursor/ existir, mesmo que trackfw.mdc não exista ainda.
 */
function injectRulesDetected(cwd) {
  const root = cwd || process.cwd()
  for (const [tool, relPath] of Object.entries(AGENT_FILES)) {
    if (tool === 'cursor') {
      if (fs.existsSync(path.join(root, '.cursor'))) {
        try { injectRulesForTool('cursor', root) } catch (_) {}
      }
      continue
    }
    if (fs.existsSync(path.join(root, relPath))) {
      try { injectRulesForTool(tool, root) } catch (_) {}
    }
  }
}

// ---------------------------------------------------------------------------
// CLAUDE.md
// ---------------------------------------------------------------------------

function generateClaudeMD(cfg) {
  const today = new Date().toISOString().slice(0, 10)
  const projectName = cfg.projectName || 'my-project'

  let content = `# ${projectName} — Claude Code Instructions\n\n`
  content += `> Generated by trackfw on ${today}. Update this file as the project evolves.\n\n`

  content += '## Project overview\n\n'
  content += '<!-- Describe what this project does in 2-3 sentences. -->\n\n'

  content += '## Governance chain\n\n'
  content += '```\nADR → REQ → ROADMAP → backlog / analyzing / wip / blocked / done / abandoned\n```\n\n'

  content += '## Agent rules (mandatory)\n\n'
  content += 'These rules apply to every agent or AI assistant working in this project:\n\n'
  content += '1. **Never start coding without a REQ and a ROADMAP.** If none exists, create them first.\n'
  content += '2. **Use `/trackfw:implement <req-slug>` to start any implementation.** This skill orchestrates the full flow automatically: finds or generates the roadmap, moves it to `wip/`, executes each ML, updates the roadmap, and moves to `done/`.\n'
  content += '3. **Only one roadmap in `wip/` at a time.** Before starting a new one, complete or move to `blocked/` the current one.\n'
  content += '4. **Ciclo de vida do ML — obrigatório:**\n'
  content += '   - Ao **iniciar** um ML: edite o roadmap alterando `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` e faça commit do roadmap.\n'
  content += '   - Ao **concluir** um ML: edite o roadmap alterando `**Status:** 🔄 Em andamento` → `**Status:** ✅ Concluído` e inclua essa mudança no commit do ML.\n'
  content += '   - Ao **analisar** um roadmap antes de iniciar: mova o arquivo de `backlog/` para `analyzing/`; só mova para `wip/` ao começar a codificar de fato.\n'
  content += '5. **Run `trackfw validate` before every commit.** Zero violations required.\n'
  content += '6. **ADRs before decisions.** Any architectural or technical decision must have an ADR (`/trackfw:adr`).\n'
  content += `7. **${GLOBAL_ADRS_DIRECTIVE}**\n\n`

  content += '## Slash commands (Claude Code)\n\n'
  content += '| Command | When to use |\n'
  content += '|---|---|\n'
  content += '| `/trackfw:architect` | Guide stack and architecture decisions |\n'
  content += '| `/trackfw:implement <req>` | **Start here** — orchestrates full implementation flow |\n'
  content += '| `/trackfw:adr <title>` | Before any architectural decision |\n'
  content += '| `/trackfw:req <title>` | Before any implementation work |\n'
  content += '| `/trackfw:roadmap <req>` | Generate AI roadmap from a REQ |\n'
  content += '| `/trackfw:move <name> <state>` | Move roadmap between states manually |\n'
  content += '| `/trackfw:validate` | Run governance validation |\n'
  content += '| `/trackfw:status` | Check what is in flight |\n'
  content += '| `/trackfw:barrier` | Run the wave-release checklist before liberating the next wave |\n\n'

  content += '## CLI commands (terminal / CI)\n\n'
  content += '| Command | When to use |\n'
  content += '|---|---|\n'
  content += '| `trackfw adr new "title"` | Create ADR |\n'
  content += '| `trackfw req new "title"` | Create REQ |\n'
  content += '| `trackfw roadmap new` | Create empty roadmap linked to a REQ |\n'
  content += '| `trackfw roadmap move <name> <state>` | Move roadmap state |\n'
  content += '| `trackfw validate` | Governance validation gate |\n'
  content += '| `trackfw status` | Show governance status |\n\n'

  // Frontend section
  if (cfg.frontend && cfg.frontend !== 'none' && cfg.frontend !== '') {
    const pm = (cfg.pkgManager && cfg.pkgManager !== 'none') ? cfg.pkgManager : 'npm'
    content += '## Frontend\n\n'
    content += `Stack: ${cfg.frontend}\n`
    content += `Package manager: ${pm}\n\n`
    content += 'Build commands:\n'
    content += `  ${pm} run build     # build\n`
    content += `  ${pm} run dev       # dev server\n`
    content += `  ${pm} run test      # tests\n\n`
    content += 'Rules:\n'
    content += `- Build must pass (\`${pm} run build\`) before marking any ML as done\n`
    content += '- Run `trackfw validate` before committing frontend changes\n\n'
  }

  // Backend section
  if (cfg.backend && cfg.backend !== 'none' && cfg.backend !== '') {
    const [buildCmd, testCmd, lintCmd] = backendCommands(cfg)
    content += '## Backend\n\n'
    content += `Stack: ${cfg.backend}\n\n`
    content += 'Build commands:\n'
    content += `  ${buildCmd}   # build / compile check\n`
    content += `  ${testCmd}   # tests\n`
    content += `  ${lintCmd}   # lint / vet\n\n`
    content += 'Rules:\n'
    content += '- Build must pass before marking any ML as done\n'
    content += '- Run `trackfw validate` before committing backend changes\n\n'
  }

  // Pre-commit checklist
  content += '## Pre-commit checklist\n\n'
  content += 'Before every commit:\n'
  content += '- [ ] `trackfw validate` passes with zero violations\n'
  if (cfg.backend && cfg.backend !== 'none' && cfg.backend !== '') {
    const [buildCmd] = backendCommands(cfg)
    content += `- [ ] \`${buildCmd}\` passes\n`
  }
  if (cfg.frontend && cfg.frontend !== 'none' && cfg.frontend !== '') {
    const pm = (cfg.pkgManager && cfg.pkgManager !== 'none') ? cfg.pkgManager : 'npm'
    content += `- [ ] \`${pm} run build\` passes\n`
  }
  content += '\n'

  // Git hooks section
  content += '## Git hooks\n\n'
  switch (cfg.hooks) {
    case 'husky':
      content += 'Git hook configured in `.husky/pre-commit` — runs `trackfw validate` automatically.\n\n'
      break
    case 'lefthook':
      content += 'Git hook configured in `lefthook.yml` — runs `trackfw validate` automatically.\n\n'
      break
    default:
      content += 'No git hook configured. Run `trackfw validate` manually before every commit.\n\n'
  }

  // CI gate section
  content += '## CI gate\n\n'
  switch (cfg.ci) {
    case 'github-actions':
      content += '`.github/workflows/trackfw-gate.yml` runs `trackfw validate` on every pull request to main.\n'
      break
    case 'gitlab-ci':
      content += '`.gitlab-ci-trackfw.yml` runs `trackfw validate` on every merge request.\n'
      break
    default:
      content += 'No CI gate configured.\n'
  }

  // Harness sections — derived from project governance conventions
  content += '\n## Branch strategy\n\n'
  content += 'One active branch at a time. Name it `feat/<slug>`, `fix/<slug>` or `refactor/<slug>`. '
  content += 'Before creating a new branch, verify no other is genuinely open: run `git fetch origin --prune`, '
  content += 'then `git branch -r --no-merged origin/main`, then for each candidate `git diff origin/main <branch> --stat`. '
  content += 'An empty diff means it was squash-merged — ignore it. '
  content += 'Squash merges do not mark a branch as merged, so `--no-merged` alone is not evidence. '
  content += 'If the branch is stale and the diff looks inflated by main\'s own evolution, '
  content += 'compare only the files the branch itself touched since the merge base.\n\n'

  content += '## Definition of done\n\n'
  content += 'Green build and tests do not close a microbatch. '
  content += 'It is done when the requirement and the roadmap sit in the correct state folder, '
  content += 'their declared status matches that folder, the final validation is recorded with evidence, '
  content += 'no duplicate copy remains in another state, and `trackfw validate` reports no violations.\n\n'

  content += '## Requirement scope\n\n'
  content += 'Every requirement must declare an explicit negative scope: what must not be implemented. '
  content += 'Boundaries prevent an implementing agent from inventing work.\n\n'

  content += '## State requirements\n\n'
  content += '`blocked` requires a reason and an owner. '
  content += '`abandoned` requires a reason and a successor. '
  content += '`wip` must reflect work that is genuinely active; '
  content += 'anything stalled moves to `blocked` or `abandoned` instead of rotting in `wip`.\n\n'

  content += '## Roadmap format\n\n'
  content += 'Organize work as waves of microbatches. '
  content += 'A wave groups microbatches that can run in parallel; a barrier separates waves. '
  content += 'Microbatches sharing any file — including generated trees and build outputs — must be sequential, '
  content += 'and the reason is documented. '
  content += 'Each microbatch declares exact files, exact actions, measurable acceptance criteria and exact validation commands, '
  content += 'so that a small model can execute it without guessing.\n\n'

  content += '## When governance is not required\n\n'
  content += 'A closed list of exemptions: a typo or local variable rename; a documentation-only change; '
  content += 'a configuration tweak with no runtime effect; a direct revert; '
  content += 'answering a question or reviewing without changes. '
  content += 'Additionally, when the user reports a concrete bug, fix it directly and do not open an architectural analysis for it. '
  content += '**This section takes precedence over the general rule that requires a requirement and a roadmap.** '
  content += 'Anything touching business logic, an API contract, a data schema, authentication or authorization, '
  content += 'localization, or user-facing behavior always requires governance, regardless of how few files it touches.\n\n'

  content += '## Production incidents\n\n'
  content += 'Inspect the live environment before proposing a fix: real variables, active credentials, '
  content += 'granted permissions, running processes. '
  content += 'Confirm the root cause against real evidence, then implement the smallest fix. '
  content += 'Never edit static configuration files as a response to a root cause that has not been confirmed in the running environment.\n\n'

  content += '## Iterative prototyping\n\n'
  content += 'For complex or uncertain user-facing work, validate the concept with a disposable, isolated prototype '
  content += 'that the user reviews visually, and only then write the decision record and the production roadmap. '
  content += 'Build and test success is not evidence that an interface is right.\n\n'

  content += '## Autopilot\n\n'
  content += 'Ask everything you need before starting. '
  content += 'Once started, do not interrupt for confirmations that could have been anticipated. '
  content += 'Decide low-risk details autonomously following existing project conventions, '
  content += 'and record autonomous decisions in the commit message.\n'

  content += '\n## Architect responses\n\n'
  content += 'Default: what changed · what was decided · what is needed from you. Three to five lines.\n\n'
  content += 'Scale up only on these three triggers, and only on them: a **blocker** that stops the next wave; '
  content += 'a **pending user decision** that cannot be inferred from context; '
  content += 'an **error the architect made** that cannot be self-corrected.\n\n'
  content += 'Never cut, even when short: measured evidence (command and result), barrier verdict, decision taken and why. '
  content += 'A response that buries a blocker in paragraph seven produced the same effect as not reporting it.\n\n'
  content += 'Cut: restating what an executor already reported, re-explaining reasoning already given, '
  content += 'recapping state that has not changed, closing praise. '
  content += 'Tables and code blocks only when they replace prose, never when they add to it.\n\n'
  content += 'Depth is on demand from the user.\n'

  injectOrUpdateRules('CLAUDE.md', content, '.')
  console.log('  ✓ CLAUDE.md')
}

/**
 * Retorna [buildCmd, testCmd, lintCmd] para o backend configurado.
 */
function backendCommands(cfg) {
  const pm = (cfg.pkgManager && cfg.pkgManager !== 'none') ? cfg.pkgManager : 'npm'
  switch (cfg.backend) {
    case 'go':
      return ['go build ./...', 'go test ./...', 'go vet ./...']
    case 'java':
      return ['mvn package -q', 'mvn test', 'mvn compile -q']
    case 'node':
      return [`${pm} run build`, `${pm} test`, `${pm} run lint`]
    case 'python':
      return [
        "python3 -c \"import pathlib, py_compile; [py_compile.compile(str(p), doraise=True) for p in pathlib.Path('.').rglob('*.py') if '.venv' not in p.parts and 'venv' not in p.parts]\"",
        'python -m pytest',
        'ruff check .',
      ]
    default:
      return ['', '', '']
  }
}

// ---------------------------------------------------------------------------
// .claude/commands/trackfw/ — slash commands
//
// CLAUDE_COMMANDS is the single source of truth for the set of slash
// commands installed by trackfw. Both generateClaudeCommands (normal path,
// idempotent — never overwrites an existing file) and
// generateClaudeCommandsForce (force path — always overwrites) install
// from this same map; they only differ in overwrite behavior. This mirrors
// the Go CLI's installSkillsInner(force) and the Python CLI's single
// generate_claude_commands — one list, one force flag. Do not fork this map
// again: a prior divergence let the force path fall behind the normal path
// (missing roadmap.md, implement.md, barrier.md) undetected.
// ---------------------------------------------------------------------------

const CLAUDE_COMMANDS = {
    'adr.md': `Execute o seguinte comando bash: \`trackfw adr new "$ARGUMENTS"\`

Se o comando falhar com \`trackfw: command not found\` ou similar, informe ao usuário:

\`\`\`
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
\`\`\``,

    'req.md': `Execute o seguinte comando bash: \`trackfw req new "$ARGUMENTS"\`

Se o comando falhar com \`trackfw: command not found\` ou similar, informe ao usuário:

\`\`\`
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
\`\`\``,

    'validate.md': `Execute o seguinte comando bash: \`trackfw validate\`

Se o comando falhar com \`trackfw: command not found\` ou similar, informe ao usuário:

\`\`\`
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
\`\`\``,

    'status.md': `Execute o seguinte comando bash: \`trackfw status\`

Se o comando falhar com \`trackfw: command not found\` ou similar, informe ao usuário:

\`\`\`
trackfw não está instalado. Instale com uma das opções:

  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw
\`\`\``,

    'move.md': `Execute o seguinte comando bash: \`trackfw roadmap move $ARGUMENTS\`

O formato esperado é: \`<nome-do-roadmap> <estado>\`

Estados válidos: \`backlog\`, \`analyzing\`, \`wip\`, \`blocked\`, \`done\`, \`abandoned\`

Exemplo: \`/trackfw:move meu-roadmap analyzing\`

Se o comando falhar com \`trackfw: command not found\` ou similar, informe ao usuário:
trackfw não está instalado. Instale com:
  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
  npm install -g trackfw
  pip install trackfw`,

    'roadmap.md': `Gere um roadmap de implementação em microlotes para uma REQ do projeto.

## Passos

1. **Listar REQs disponíveis**
   Use Glob para listar \`docs/req/*.md\`. Se nenhum arquivo encontrado, informe:
   > Nenhuma REQ encontrada em \`docs/req/\`. Crie uma primeiro com \`/trackfw:req\`.

2. **Selecionar a REQ**
   - Se \`$ARGUMENTS\` foi fornecido: use como filtro (substring case-insensitive) para encontrar o arquivo
   - Se não foi fornecido ou o filtro não encontrar exatamente um: liste os arquivos disponíveis e pergunte ao usuário qual usar
   - Leia o conteúdo completo do arquivo REQ selecionado

3. **Gerar o roadmap**
   Com base no conteúdo da REQ, gere um roadmap seguindo **estritamente** este formato:

   \`\`\`\`markdown
   ---
   status: backlog
   date: <YYYY-MM-DD>
   req: "docs/req/<arquivo-selecionado>.md"
   squad: ""
   ---

   # Roadmap: <título derivado da REQ>

   > Created: <YYYY-MM-DD> | Status: backlog

   ## Diagnóstico / Contexto
   <resumo do problema, motivação e escopo extraídos da REQ>

   ## Wave 0 — Threat Model
   > Dependencies: none. Blocks all implementation.

   ### ML-0A — Threat model for this roadmap
   **Status:** pending
   **Files affected:**
   **Actions:**
   1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
   2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
   3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
   4. Declared residual — what this design accepts not covering.
   **Acceptance criteria:**
   - [ ] The four sections above answered with evidence, not a one-line assertion
   - [ ] No implementation line written for this ML

   **Gates da wave:**
   \`\`\`bash
   # Wave 0 gate — replace this placeholder with a project-specific check before
   # marking ML-0A done. Do not remove the gate; replace its command (AC13).
   exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
   \`\`\`

   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)
   > Dependências: Independente

   ### ML-1A — <título>
   **Status:** ⬜ Pendente
   **Arquivos afetados:**
   - \`caminho/exato/do/arquivo\`
   **Ações:**
   - Descrição detalhada da ação com valores, chaves e comandos exatos
   **Critérios de aceite:**
   - [ ] build sem erros
   - [ ] testes verdes
   **Comandos de validação:** \`<comando de build e teste do projeto>\`

   ### ML-1B — <título> (se independente de ML-1A)
   ...

   ## Wave 2 — <nome> (depende de Wave 1)
   > Dependências: Wave 1 completa
   ...
   \`\`\`\`

   **Princípios obrigatórios:**
   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra
   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos
   - Waves sequenciais apenas quando há dependência real de resultado
   - Critérios de aceite mensuráveis em cada ML

4. **Salvar o arquivo**
   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais
   - Crie o arquivo em \`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md\`
   - Preencha \`req:\` com o caminho relativo completo da REQ selecionada
   - Use a data de hoje

5. **Confirmar**
   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.
`,

    'implement.md': `Você é o orquestrador de implementação do trackfw. Siga o fluxo abaixo **sem pular etapas**.

## Argumento

\`$ARGUMENTS\` é opcional. Se fornecido, é usado como filtro (substring case-insensitive) sobre os nomes de arquivo das REQs.

---

## Passo 1 — Selecionar a REQ

Use Glob para listar \`docs/req/*.md\`.

- Se **nenhum arquivo encontrado**: informe que não há REQs disponíveis e sugira criar com \`/trackfw:req\`.
- Se **\`$ARGUMENTS\` foi fornecido** e filtra para exatamente uma REQ: use-a diretamente.
- Em **todos os outros casos** (sem argumento, ou argumento ambíguo): apresente a lista de REQs disponíveis e pergunte ao usuário qual deseja implementar.

Leia o conteúdo completo da REQ selecionada.

---

## Passo 2 — Encontrar ou gerar o Roadmap

Verifique se existe um roadmap vinculado à REQ buscando em \`docs/roadmaps/\` (backlog, wip, blocked, done, abandoned) por arquivo cujo nome contenha o slug da REQ.

**Se o roadmap ainda não existe:**
- Informe o usuário: "Nenhum roadmap encontrado para esta REQ. Gerando agora..."
- Execute o fluxo completo de geração do \`/trackfw:roadmap\` (leia o arquivo \`.claude/commands/trackfw/roadmap.md\` para seguir as instruções exatas), passando a REQ já selecionada — não pergunte novamente.
- Salve o roadmap gerado em \`docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md\`.

**Se o roadmap existe e já está em \`done/\` ou \`abandoned/\`:**
- Informe o usuário e pergunte se deseja criar um novo roadmap ou encerrar.

**Se o roadmap existe em \`backlog/\` ou \`blocked/\`:**
- Prossiga para o Passo 3.

**Se já está em \`wip/\`:**
- Prossiga diretamente para o Passo 4 (já está em execução).

---

## Passo 3 — Mover roadmap para WIP

Execute:
\`\`\`bash
trackfw roadmap move <nome-do-roadmap> wip
\`\`\`

Confirme que o arquivo foi movido para \`docs/roadmaps/wip/\`.

---

## Passo 4 — Ler e apresentar o plano

Leia o roadmap (agora em \`wip/\`). Apresente ao usuário:
- Título do roadmap
- Total de Waves e MLs
- Lista resumida dos MLs por Wave

Confirme: "Iniciando implementação. Vou executar cada ML em ordem e atualizar o roadmap a cada conclusão."

---

## Passo 5 — Executar cada ML em ordem

Para cada Wave (em sequência), execute os MLs da Wave:

### Para cada ML:

**5a. Anunciar:** informe qual ML está sendo executado (ex: "Executando ML-1A — Criar client.go").

**5b. Implementar:** execute as ações descritas no ML usando suas ferramentas (Read, Write, Edit, Bash). Siga exatamente os arquivos afetados, ações e critérios de aceite listados no roadmap.

**5c. Validar:** execute os comandos de validação do ML. Se falhar, corrija antes de avançar.

**5d. Atualizar o roadmap:** edite o arquivo de roadmap em \`docs/roadmaps/wip/\` substituindo o status do ML:
- \`**Status:** ⬜ Pendente\` → \`**Status:** ✅ Concluído\`

**5e. Commitar:**
\`\`\`bash
git add -A
git commit -m "feat(<escopo>): <descrição do ML>"
\`\`\`

Só avance para a próxima Wave após todos os MLs da Wave atual estarem ✅.

---

## Passo 6 — Finalizar

Quando todos os MLs estiverem ✅:

**6a.** Execute \`trackfw validate\` — deve passar com zero violations.

**6b.** Mova o roadmap para done:
\`\`\`bash
trackfw roadmap move <nome-do-roadmap> done
\`\`\`

**6c.** Faça o commit final:
\`\`\`bash
git add docs/roadmaps/
git commit -m "docs(trackfw): roadmap <nome> → done"
\`\`\`

**6d.** Informe o usuário:
\`\`\`
✅ Implementação concluída.
Roadmap: docs/roadmaps/done/<nome>.md
Próximo passo: abrir PR com gh pr create
\`\`\``,

    'barrier.md': `Você é o \`trackfw_architect\`, a única autoridade Git deste projeto. Este comando executa o checklist operacional de liberação de uma wave — nenhum outro agente commita, faz push ou libera a próxima wave.

## Argumento

\`$ARGUMENTS\` no formato \`<roadmap> <wave>\`. Se ausente ou incompleto, pergunte ao usuário qual roadmap (em \`docs/roadmaps/wip/\`) e qual número de wave validar.

---

## Núcleo determinístico

Execute primeiro:
\`\`\`bash
trackfw barrier <roadmap> --wave <n> --trust-local-gates --json
\`\`\`

\`--trust-local-gates\` é obrigatório aqui: roadmaps WIP (modificados localmente, ainda não commitados em
\`origin/main\`) são marcados como não confiáveis pela CLI direta por padrão, como proteção contra a
execução de gates de roadmaps chegados por PR de terceiro. O slash command aplica esse flag porque
ele representa o fluxo legítimo do arquiteto operando no próprio repositório — não porque os gates
são inspecionados previamente (o diff ainda é responsabilidade do checklist abaixo).

⚠️ **Não use \`--trust-local-gates\` ao revisar um roadmap chegado por PR de terceiro** — use a CLI
direta sem o flag (\`trackfw barrier <roadmap> --wave <n> --json\`) para que os gates sejam marcados
como \`not_evaluated\` e não executados.

Este comando é **necessário mas não suficiente**. Ele verifica MLs concluídos, evidências e \`trackfw validate\`, mas não substitui as inspeções especializadas nem a auditoria de diff abaixo — nenhuma delas é avaliada pelo binário. Consulte a seção \`trackfw barrier\` em \`docs/cli-parity.md\` para o contrato completo (estados, exit codes, saída JSON).

Se o comando retornar exit code não-zero (\`blocked\` ou erro de resolução): pare, reporte a falha ao usuário e não prossiga no checklist até que a wave passe.

---

## Definição de pronto da barrier — checklist completo

Antes de liberar a próxima wave, confirme cada item com evidência concreta — não presuma:

1. **Todos os MLs da wave concluídos e marcados** — cada ML da wave está com \`**Status:** ✅ Concluído\` no roadmap.
2. **Testes unitários e E2E aplicáveis executados** — rode os comandos de validação declarados em cada ML.
3. **Build aplicável sem erros** — rode o comando de build do(s) workspace(s) afetado(s).
4. **Cada critério de aceite inspecionado com evidência** — leia os arquivos modificados e confirme contra os critérios listados, não apenas contra os testes.
5. **Agente code-quality reportou conformidade, performance, robustez e clareza** — invoque o agente \`code-quality\` quando a mudança introduzir lógica nova, duplicação relevante ou risco de manutenibilidade.
6. **Agente security reportou SAST, privilégios, controle de acesso e camadas aplicáveis** — invoque o agente \`security\` quando a mudança tocar autenticação, segredos, entrada externa ou permissões.
7. **Gates pré-commit declarados pelo projeto executados** — rode os hooks/gates configurados (lint, format, testes de contrato).
8. **\`trackfw validate --json\` aprovado** — execute e confirme zero violações.
9. **Diff auditado contra o escopo** — revise o diff completo; confirme que não há alterações de agentes concorrentes nem arquivos fora do escopo do ML (ex: \`docs/adr/\`, \`docs/req/\`, \`docs/roadmaps/\` quando não autorizado ao especialista).
10. **Resultado registrado antes de liberar a próxima wave** — anote no roadmap ou na resposta ao usuário que a wave passou, com a evidência de cada item acima.

Se qualquer item falhar: bloqueie a próxima wave, identifique o item e o agente responsável, e despache um microlote corretivo. Só repita o checklist depois que o corretivo for concluído.

---

## Autoridade Git

Somente o \`trackfw_architect\` cria branch, audita diff, commita e faz push. Especialistas entregam trabalho sem commit — cabe a este papel revisar, commitar e sugerir a abertura de PR/MR (sem abrir automaticamente sem autorização do usuário).
`,

    'architect.md': `Você é o guia de arquitetura do trackfw. Ajude o usuário a escolher a stack correta e arquitetar a aplicação em linguagem simples, acessível para times não técnicos.

## Passo 1 — Descoberta de Negócio

Faça ao usuário as seguintes perguntas em linguagem simples, uma por vez:

1. "O que sua aplicação vai fazer? Descreva em 2-3 frases como se fosse explicar para alguém de fora da TI."
2. "Quantas pessoas vão usar esse sistema simultaneamente? (< 10 pessoas / 10-100 pessoas / > 100 pessoas)"
3. "Esse sistema vai para produção de verdade ou é um protótipo para validar uma ideia?"
4. "Você precisa de login/autenticação de usuários? (Sim / Não / Não sei)"
5. "Tem alguma restrição de tecnologia ou preferência da empresa? (ex: só Java, só Microsoft, etc.)"

---

## Passo 2 — Recomendação de Stack

Com base nas respostas, escolha **UM** dos combos pré-validados:

### Combo A — Protótipo Rápido
**Quando usar:** prototipagem, validação de ideia, até ~10 usuários, sem pressão de produção.
- **Frontend:** React + Vite
- **Backend:** FastAPI (Python) ou Express (Node.js)
- **Banco:** SQLite + SQLAlchemy / Prisma
- **Auth:** JWT simples quando necessário
- **Docker:** Dockerfile básico para o backend

### Combo B — Sistema Pequeno/Médio em Produção
**Quando usar:** sistema real, 10-100 usuários, robustez e manutenibilidade.
- **Frontend:** Next.js (SSR + rotas prontas)
- **Backend:** FastAPI (Python) ou NestJS (Node.js)
- **Banco:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)
- **Auth:** OAuth2 com JWT (Supabase Auth ou Auth0)
- **Docker:** docker-compose com frontend + backend + banco

### Combo C — Enterprise / Java
**Quando usar:** integração com sistemas corporativos, > 100 usuários, exigência de Java.
- **Frontend:** Angular
- **Backend:** Spring Boot
- **Banco:** PostgreSQL + Hibernate
- **Auth:** Spring Security + OAuth2 (Keycloak ou Azure AD)
- **Docker:** docker-compose com todos os serviços

Apresente o combo recomendado com explicação simples do motivo.

---

## Passo 3 — Arquitetura em Camadas (explicação simples)

Explique a arquitetura com uma metáfora de negócio:

"Pense na aplicação como um restaurante:
- **Frontend** = o salão: o que o cliente vê e interage
- **Backend** = a cozinha: onde as regras de negócio acontecem, nunca exposta diretamente
- **Banco de dados** = a despensa: onde os dados ficam guardados, acessada só pela cozinha"

Reforce as **Architecture Directives** já injetadas no CLAUDE.md deste projeto: separação em 3 camadas sem dados em memória (sempre DB + ORM), auth + Docker + .env desde o dia 1, validação em 2 camadas, contrato OpenAPI antes de codar, wave de segurança em todo roadmap e cobertura mínima de testes (60% protótipo / 80% produção).

---

## Passo 4 — Gerar o ADR de Stack

Execute \`/trackfw:adr\` com o título: \`"Stack e arquitetura em camadas — [nome do projeto]"\`

O ADR deve registrar a stack escolhida (combo e componentes), motivação baseada nas respostas, alternativas descartadas e princípios de arquitetura adotados.

---

## Passo 5 — Próximos Passos

Oriente o usuário:

\`\`\`
✅ Stack definida. Próximos passos:

1. Crie a REQ da primeira feature com /trackfw:req
2. Gere o roadmap em microlotes com /trackfw:roadmap
3. Inicie a implementação com /trackfw:implement
\`\`\``,
}

/**
 * installClaudeCommandsInner — fonte única de escrita dos slash commands.
 * @param {string} dir diretório de destino (já resolvido)
 * @param {boolean} force quando true, sobrescreve arquivos existentes
 */
function installClaudeCommandsInner(dir, force) {
  fs.mkdirSync(dir, { recursive: true })

  let created = 0
  let skipped = 0
  for (const [filename, content] of Object.entries(CLAUDE_COMMANDS)) {
    const filePath = path.join(dir, filename)
    if (!force && fs.existsSync(filePath)) {
      skipped++
      continue
    }
    fs.writeFileSync(filePath, content, 'utf8')
    created++
  }

  if (force) {
    console.log(`  ✓ ${dir} (${created} slash commands sobrescritos)`)
  } else if (skipped > 0) {
    console.log(`  ✓ ${dir} (${created} slash commands criados, ${skipped} já existiam — não sobrescritos)`)
  } else {
    console.log(`  ✓ ${dir} (${created} slash commands)`)
  }
}

/**
 * generateClaudeCommands — instala os slash commands de forma idempotente:
 * nunca sobrescreve um arquivo já existente. Honra o `rootDir` recebido
 * (mesma convenção do gêmeo forçado generateClaudeCommandsForce e do
 * generate_claude_commands(cwd) do Python); quando omitido, cai no
 * caminho relativo ao cwd do processo, preservando o comportamento
 * histórico consumido pelos testes que chamam sem argumento.
 */
function generateClaudeCommands(rootDir) {
  const dir = rootDir ? path.join(rootDir, '.claude', 'commands', 'trackfw') : '.claude/commands/trackfw'
  installClaudeCommandsInner(dir, false)
}

// ---------------------------------------------------------------------------
// AI tool installers
// ---------------------------------------------------------------------------

/** Compatibility entrypoint backed by the canonical integration engine. */
async function installAgents(cwd = process.cwd()) {
  await installIntegrationTarget('claude', cwd)
}

async function installGemini(cwd = process.cwd()) {
  await installIntegrationTarget('gemini', cwd)
  try {
    injectRulesForTool('gemini', cwd)
    console.log('  ✓ trackfw rules → GEMINI.md')
  } catch (e) {
    console.log(`  ⚠ gemini rules inject: ${e.message}`)
  }
}

async function installCursor(cwd = process.cwd()) {
  await installIntegrationTarget('cursor', cwd)
  try {
    injectRulesForTool('cursor', cwd)
    console.log('  ✓ trackfw rules → .cursor/rules/trackfw.mdc')
  } catch (e) {
    console.log(`  ⚠ cursor rules inject: ${e.message}`)
  }
}

async function installCopilot(cwd = process.cwd()) {
  await installIntegrationTarget('copilot', cwd)
  try {
    injectRulesForTool('copilot', cwd)
    console.log('  ✓ trackfw rules → .github/copilot-instructions.md')
  } catch (e) {
    console.log(`  ⚠ copilot rules inject: ${e.message}`)
  }
}

async function installWindsurf(cwd = process.cwd()) {
  await installIntegrationTarget('windsurf', cwd)
  try {
    injectRulesForTool('windsurf', cwd)
    console.log('  ✓ trackfw rules → .windsurfrules')
  } catch (e) {
    console.log(`  ⚠ windsurf rules inject: ${e.message}`)
  }
}

async function installAmazonQ(cwd = process.cwd()) {
  await installIntegrationTarget('amazonq', cwd)
  try {
    injectRulesForTool('amazonq', cwd)
    console.log('  ✓ trackfw rules → .amazonq/developer/guidelines.md')
  } catch (e) {
    console.log(`  ⚠ amazonq rules inject: ${e.message}`)
  }
}

/**
 * Install the canonical agents and skills for one AI target.
 *
 * `scope` defaults to "project" for backward compatibility with the
 * compatibility entrypoints below (installAgents, installGemini, etc.), that
 * historical tests exercise directly without going through the `init`
 * wizard's scope prompt. `trackfw init` itself always passes the scope it
 * resolved (ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-
 * skills, D4).
 */
async function installIntegrationTarget(target, cwd = process.cwd(), scope = 'project', { onSkip } = {}) {
  const { execute, buildPlans } = require('../integrations')
  const { resolveAgentModels } = require('../config')
  const { models: agentModels, warning: agentModelsWarning } = resolveAgentModels(scope, os.homedir(), cwd)
  if (agentModelsWarning) process.stderr.write(agentModelsWarning + '\n')
  const roots = { projectRoot: cwd }
  const options = { targets: [target], scope, onSkip, agentModels }
  // D5 — transparency: print resolved destinations before writing anything.
  // buildPlans has no side effects, so it is safe to call here purely to
  // enumerate destinations; the actual write happens below via execute().
  // Mirrors printResolvedDestinations in commands/integrations.js and Go's
  // installAITools (internal/commands/init.go) — this call site (init's AI
  // tools install) was the one place that stayed silent (divergence #2,
  // ROADMAP-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills).
  const destinations = [
    ...buildPlans('agents', options).map(plan => plan.destination),
    ...buildPlans('skills', options).map(plan => plan.destination),
  ]
  console.log(`Destino (${scope}):`)
  for (const destination of [...new Set(destinations)].sort()) console.log(`  ${destination}`)
  execute('agents', 'install', options, roots)
  execute('skills', 'install', options, roots)
  console.log(`  ✓ ${target} agents and skills`)
}

/**
 * generateClaudeCommandsForce — re-gera todos os slash commands, sobrescrevendo
 * arquivos existentes. Instala exatamente o mesmo conjunto de CLAUDE_COMMANDS
 * usado por generateClaudeCommands — só o comportamento de sobrescrita difere.
 */
function generateClaudeCommandsForce(rootDir) {
  const dir = rootDir ? path.join(rootDir, '.claude', 'commands', 'trackfw') : '.claude/commands/trackfw'
  installClaudeCommandsInner(dir, true)
}

/**
 * installSkillsForce — sobrescreve ~/.claude/skills/trackfw/SKILL.md sempre.
 */
function installSkillsForce() {
  const skillDir = path.join(os.homedir(), '.claude', 'skills', 'trackfw')
  fs.mkdirSync(skillDir, { recursive: true })

  const content = `---
name: trackfw
description: "trackfw — Governed Software Delivery: ADR → REQ → ROADMAP → kanban"
signature: "📦 trackfw - Governed Delivery"
---

# trackfw — Modo de Operação

Este projeto usa **trackfw** para governança de entrega de software.
Cadeia: **ADR → REQ → ROADMAP** · Estados: \`backlog / wip / blocked / done / abandoned\`

## Comandos principais

- \`trackfw context\` — contexto de trabalho atual (sempre execute primeiro)
- \`trackfw status\` — todos os artefatos e estados
- \`trackfw validate\` — valida consistência de governança
- \`trackfw roadmap move <nome> <estado>\` — transição de estado
- \`trackfw serve\` — board Kanban em http://localhost:4080

## Protocolo de agente

1. Antes de iniciar: \`trackfw context\` + ler \`docs/agents-working-context.md\`
2. Após concluir: atualizar \`docs/agents-working-context.md\`
3. Antes de PR: \`trackfw validate\` deve passar com zero violations
`

  const dest = path.join(skillDir, 'SKILL.md')
  fs.writeFileSync(dest, content, 'utf8')
  console.log(`  ✓ skill global atualizada em ${dest}`)
}

function printArchitectNextSteps(cwd) {
  const candidates = [
    { file: 'CLAUDE.md',                              cmd: 'claude' },
    { file: '.cursor/rules/trackfw.mdc',              cmd: 'cursor .' },
    { file: '.windsurfrules',                         cmd: 'windsurf .' },
    { file: '.github/copilot-instructions.md',        cmd: 'code . (Copilot)' },
    { file: '.amazonq/developer/guidelines.md',       cmd: 'code . (Amazon Q)' },
    { file: 'GEMINI.md',                              cmd: 'gemini' },
    { file: 'AGENTS.md',                              cmd: 'codex' },
  ]

  const detected = candidates.filter(t => {
    try { return fs.existsSync(path.join(cwd, t.file)) } catch { return false }
  })

  const tools = detected.length > 0 ? detected : [{ cmd: 'claude' }]

  console.log()
  console.log('Próximo passo — inicie com o guia de arquitetura:')
  console.log()
  for (const t of tools) {
    console.log(`  ${t.cmd}`)
  }
  console.log()
  console.log('  Execute /trackfw:architect no chat do seu assistente de IA.')
  console.log()
}

module.exports = {
  GOV_DIRS,
  GLOBAL_ADRS_DIRECTIVE,
  scaffold,
  writeTrackfwConfig,
  generateValidateScript,
  generateAttentionScripts,
  generateCredentialGuardScript,
  generateGitBranchGuardScript,
  generateCIWorkflow,
  generateGitHooks,
  generateCommitMsgHook,
  generateClaudeMD,
  generateClaudeCommands,
  generateClaudeCommandsForce,
  generateVaultIndex,
  installSkillsForce,
  installAgents,
  installGemini,
  installCursor,
  installCopilot,
  installWindsurf,
  installAmazonQ,
  installIntegrationTarget,
  injectRulesForTool,
  injectRulesDetected,
  injectHooksDetected,
  trackfwRulesBlock,
  printArchitectNextSteps,
  // Template constants and builders — exported for scaffold doctor (ADR-2026-08-27).
  // Same single-source-of-truth principle as hooks.js exports: comparing against the
  // same object the write path uses makes drift structurally impossible.
  CLAUDE_COMMANDS,
  buildValidateScript,
  // CI workflow content builders (inline content from generateGitHub/GitLabCIWorkflow)
  buildGitHubActionsWorkflowContent,
  buildGitLabCIWorkflowContent,
}
