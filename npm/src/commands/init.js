'use strict'
const { Command } = require('commander')
const { t } = require('../i18n')
const identityStore = require('../identity')
const identityWizard = require('./identity-wizard')
const { resolveIdentityPreset, identityFileExists } = identityWizard
const { resolveScope } = require('./integrations')
const { homedir } = require('../homedir')

const cmd = new Command('init')
cmd.description(t('init.description'))
cmd.option('--ai-tools <tools>', 'Comma-separated AI tools to configure (claude,codex,gemini,antigravity,cursor,copilot,windsurf,amazonq,opencode,kiro)', '')
cmd.option('--identity-preset <preset>', `Agent identity preset: none, neutral, ${identityStore.presetNames().join(', ')}`, 'none')
cmd.option('--forge <value>', 'Forge platform: github, gitlab, bitbucket, azure', '')
cmd.action(async (options, command) => {
  const os = require('os')
  const path = require('path')
  const generators = require('../generators/init')

  const home = homedir()

  // A validação e persistência da flag acontecem incondicionalmente, acima
  // do early-return não-TTY abaixo — é isso que faz um --identity-preset
  // inválido falhar alto em CI em vez de silenciosamente não fazer nada.
  const presetChanged = command.getOptionValueSource('identityPreset') === 'cli'
  if (presetChanged) {
    const { cfg, shouldSave } = resolveIdentityPreset(options.identityPreset)
    if (shouldSave) {
      identityStore.validate(cfg, identityStore.knownAgentIds())
      identityStore.save(home, cfg)
    }
  }

  // --forge: validate unconditionally above the non-TTY early return so that
  // an invalid value fails loudly in CI instead of silently no-op'ing.
  const forgeChanged = command.getOptionValueSource('forge') === 'cli'
  if (forgeChanged && options.forge) {
    const { validateForge } = require('../forge/resolve')
    validateForge(options.forge) // throws on invalid value
  }

  // Pula o wizard de identidade inteiramente quando a flag foi passada
  // explicitamente (já tratado acima) ou quando um arquivo de identidade já
  // existe — re-executar init nunca deve sobrescrever silenciosamente uma
  // identidade já configurada.
  const skipIdentityWizard = presetChanged || identityFileExists(home)

  // Modo não-TTY: usar defaults e chamar scaffold diretamente
  if (!process.stdin.isTTY) {
    const cfg = {
      projectName: path.basename(process.cwd()),
      projectType: 'governance',
      frontend: '',
      backend: '',
      pkgManager: 'npm',
      hooks: 'none',
      ci: 'none',
      forge: forgeChanged ? options.forge : '',
    }
    await generators.scaffold(cfg)
    const aiTools = String(options.aiTools || '').split(',').map(tool => tool.trim()).filter(Boolean)
    const supported = new Set(['claude', 'codex', 'gemini', 'antigravity', 'cursor', 'copilot', 'windsurf', 'amazonq', 'opencode', 'kiro'])
    // Sem TTY, o escopo nunca é perguntado: default `global` (ADR D1/D4).
    const scope = await resolveScope({}, { interactive: false })
    const makeOnSkip = () => (_destination, reason) => {
      process.stderr.write(`${reason}\n`)
    }
    for (const tool of aiTools) {
      if (!supported.has(tool)) throw new Error(`Unsupported AI tool: ${tool}`)
      await generators.installIntegrationTarget(tool, process.cwd(), scope, { onSkip: makeOnSkip() })
    }
    console.log(`\n${t('init.success')}`)
    require('../generators/init').printArchitectNextSteps(process.cwd())
    return
  }

  const { input, select, checkbox } = require('@inquirer/prompts')

  // Detect forge from the current dir to prefill the wizard default.
  const { resolveFromRepo: resolveForgeFromRepo } = require('../forge/resolve')
  let detectedForge = ''
  if (!forgeChanged) {
    try {
      const res = resolveForgeFromRepo('', '', process.cwd())
      if (res.source !== 'none') detectedForge = res.forge
    } catch (_) {}
  }

  let projectName, projectType, frontend, pkgManager, backend, backendFramework, hooks, ci, forgeValue, aiTools, requireReqInCommit, scope

  try {
    projectName = await input({
      message: t('init.prompt.projectName'),
      default: path.basename(process.cwd()),
    })

    projectType = await select({
      message: t('init.prompt.projectType'),
      choices: [
        { name: t('init.prompt.projectType_fullstack'), value: 'fullstack' },
        { name: t('init.prompt.projectType_frontend'), value: 'frontend' },
        { name: t('init.prompt.projectType_backend'), value: 'backend' },
        { name: t('init.prompt.projectType_governance'), value: 'governance' },
      ],
    })

    frontend = ''
    pkgManager = ''
    if (projectType === 'fullstack' || projectType === 'frontend') {
      frontend = await select({
        message: t('init.prompt.frontendStack'),
        choices: [
          { name: 'React / Next.js', value: 'react' },
          { name: 'Vue', value: 'vue' },
          { name: 'Angular', value: 'angular' },
        ],
      })
      pkgManager = await select({
        message: t('init.prompt.pkgManager'),
        choices: [
          { name: 'npm', value: 'npm' },
          { name: 'pnpm', value: 'pnpm' },
          { name: 'yarn', value: 'yarn' },
          { name: 'bun', value: 'bun' },
        ],
      })
    }

    backend = ''
    let backendFramework = ''
    if (projectType === 'fullstack' || projectType === 'backend') {
      backend = await select({
        message: t('init.prompt.backendLang'),
        choices: [
          { name: 'Go', value: 'go' },
          { name: 'Java', value: 'java' },
          { name: 'Node.js', value: 'node' },
          { name: 'Python', value: 'python' },
        ],
      })

      const frameworkChoices = {
        go: [
          { name: 'Gin', value: 'gin' },
          { name: 'Echo', value: 'echo' },
          { name: 'Fiber', value: 'fiber' },
          { name: 'Standard library (net/http)', value: 'stdlib' },
        ],
        java: [
          { name: 'Spring Boot', value: 'spring-boot' },
          { name: 'Quarkus', value: 'quarkus' },
          { name: 'Micronaut', value: 'micronaut' },
        ],
        node: [
          { name: 'Express', value: 'express' },
          { name: 'Fastify', value: 'fastify' },
          { name: 'NestJS', value: 'nestjs' },
          { name: 'Koa', value: 'koa' },
        ],
        python: [
          { name: 'FastAPI', value: 'fastapi' },
          { name: 'Django', value: 'django' },
          { name: 'Flask', value: 'flask' },
        ],
      }
      backendFramework = await select({
        message: t('init.prompt.backendFramework'),
        choices: frameworkChoices[backend] || [],
      })
    }

    hooks = await select({
      message: t('init.prompt.gitHooks'),
      choices: [
        { name: 'husky', value: 'husky' },
        { name: 'lefthook', value: 'lefthook' },
        { name: 'None', value: 'none' },
      ],
    })

    ci = await select({
      message: t('init.prompt.ci'),
      choices: [
        { name: 'GitHub Actions', value: 'github-actions' },
        { name: 'GitLab CI', value: 'gitlab-ci' },
        { name: 'None', value: 'none' },
      ],
    })

    if (!forgeChanged) {
      forgeValue = await select({
        message: t('init.prompt.forge'),
        default: detectedForge,
        choices: [
          { name: 'Auto-detect (omit key)', value: '' },
          { name: 'GitHub', value: 'github' },
          { name: 'GitLab', value: 'gitlab' },
          { name: 'Bitbucket', value: 'bitbucket' },
          { name: 'Azure DevOps', value: 'azure' },
        ],
      })
    } else {
      forgeValue = options.forge
    }

    requireReqInCommit = false
    if (hooks !== 'none') {
      const { confirm: confirmPrompt } = require('@inquirer/prompts')
      requireReqInCommit = await confirmPrompt({
        message: t('init.prompt.require_req_in_commit'),
        default: false,
      })
    }

    aiTools = await checkbox({
      message: t('init.prompt.aiTools'),
      choices: [
        { name: 'Claude Code', value: 'claude' },
        { name: 'OpenAI Codex', value: 'codex' },
        { name: 'Gemini CLI', value: 'gemini' },
        { name: 'Google Antigravity', value: 'antigravity' },
        { name: 'Cursor', value: 'cursor' },
        { name: 'GitHub Copilot', value: 'copilot' },
        { name: 'Windsurf', value: 'windsurf' },
        { name: 'Amazon Q Developer', value: 'amazonq' },
        { name: 'OpenCode', value: 'opencode' },
        { name: 'Kiro', value: 'kiro' },
      ],
    })

    // Escopo de instalação (ADR D4): só pergunta quando há de fato algo para
    // instalar. `resolveScope` é a mesma função (mesmas strings/mecanismo de
    // prompt) usada por `agents install` / `skills install`.
    scope = aiTools.length ? await resolveScope({}, { interactive: true }) : 'global'

  } catch (err) {
    // Fallback quando stdin fecha inesperadamente (ex: pipe em TTY simulado)
    const cfg = {
      projectName: path.basename(process.cwd()),
      projectType: 'governance',
      frontend: '',
      backend: '',
      pkgManager: 'npm',
      hooks: 'none',
      ci: 'none',
      forge: forgeChanged ? options.forge : '',
    }
    await generators.scaffold(cfg)
    console.log(`\n${t('init.success')}`)
    require('../generators/init').printArchitectNextSteps(process.cwd())
    return
  }

  // O wizard de identidade roda como seu próprio formulário, logo após o
  // formulário principal — o mesmo componente compartilhado (ADR D1) usado
  // por `agents install`. É pulado inteiramente (nenhuma pergunta) quando a
  // flag já resolveu a identidade acima, ou quando um arquivo de identidade
  // já existe.
  if (!skipIdentityWizard) {
    const { catalog } = require('../integrations')
    try {
      await identityWizard.runIdentityWizard(catalog, home)
    } catch {
      // Fallback quando stdin fecha inesperadamente durante o wizard de
      // identidade (mesma tolerância aplicada ao formulário principal acima).
      await generators.scaffold({
        projectName: path.basename(process.cwd()),
        projectType: 'governance',
        frontend: '',
        backend: '',
        pkgManager: 'npm',
        hooks: 'none',
        ci: 'none',
        forge: forgeChanged ? options.forge : '',
      })
      console.log(`\n${t('init.success')}`)
      require('../generators/init').printArchitectNextSteps(process.cwd())
      return
    }
  }

  const cfg = { projectName, projectType, frontend, backend, backendFramework, pkgManager, hooks, ci, forge: forgeValue || '', requireReqInCommit }
  await generators.scaffold(cfg)

  for (const tool of (aiTools || [])) {
    const onSkip = (_destination, reason) => {
      process.stderr.write(`${reason}\n`)
    }
    await generators.installIntegrationTarget(tool, process.cwd(), scope, { onSkip })
  }

  console.log(`\n${t('init.success')}`)
  require('../generators/init').printArchitectNextSteps(process.cwd())
})

module.exports = cmd
