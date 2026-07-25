'use strict'
const { Command } = require('commander')
const { t } = require('../i18n')
const identityStore = require('../identity')

// resolveIdentityPreset traduz o valor da flag --identity-preset para uma
// Config a persistir. "none" e "neutral" significam "não gravar nada" — o
// chamador não deve criar ~/.trackfw/identity.json para esses valores. Um
// valor desconhecido é sempre um erro, listando os valores aceitos. Espelha
// internal/commands/init.go:resolveIdentityPreset.
function resolveIdentityPreset(value) {
  if (value === 'none' || value === 'neutral') return { cfg: null, shouldSave: false }
  let cfg
  try {
    cfg = identityStore.preset(value)
  } catch {
    const valid = ['none', 'neutral', ...identityStore.presetNames()]
    throw new Error(`identity-preset invalido "${value}" (validos: ${valid.join(', ')})`)
  }
  return { cfg, shouldSave: true }
}

// identityFileExists reporta se ~/.trackfw/identity.json já existe. Espelha
// internal/commands/init.go:identityFileExists.
function identityFileExists(home) {
  const fs = require('fs')
  const path = require('path')
  return fs.existsSync(path.join(home, '.trackfw', 'identity.json'))
}

// saveWizardIdentity persiste a identidade escolhida através do wizard
// interativo. "neutral" (e o select deixado vazio porque o grupo foi
// ocultado) significam "não gravar nada". Espelha
// internal/commands/init.go:saveWizardIdentity.
function saveWizardIdentity(home, identitySelect, knownAgentIds, customDisplayNames, userNickname) {
  if (identitySelect === '' || identitySelect === 'neutral') return

  let cfg
  if (identitySelect === 'custom') {
    const agents = {}
    knownAgentIds.forEach((id, index) => {
      const slug = identityStore.slugify(customDisplayNames[index])
      agents[id] = { display_name: customDisplayNames[index], slug }
    })
    cfg = { agents }
  } else {
    cfg = identityStore.preset(identitySelect)
  }
  cfg.user_nickname = userNickname

  identityStore.validate(cfg, identityStore.knownAgentIds())
  identityStore.save(home, cfg)
}

const cmd = new Command('init')
cmd.description(t('init.description'))
cmd.option('--ai-tools <tools>', 'Comma-separated AI tools to configure (claude,codex,gemini,antigravity,cursor,copilot,windsurf,amazonq,kiro)', '')
cmd.option('--identity-preset <preset>', `Agent identity preset: none, neutral, ${identityStore.presetNames().join(', ')}`, 'none')
cmd.action(async (options, command) => {
  const os = require('os')
  const path = require('path')
  const generators = require('../generators/init')

  const home = os.homedir()

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
    }
    await generators.scaffold(cfg)
    const aiTools = String(options.aiTools || '').split(',').map(tool => tool.trim()).filter(Boolean)
    const supported = new Set(['claude', 'codex', 'gemini', 'antigravity', 'cursor', 'copilot', 'windsurf', 'amazonq', 'kiro'])
    for (const tool of aiTools) {
      if (!supported.has(tool)) throw new Error(`Unsupported AI tool: ${tool}`)
      await generators.installIntegrationTarget(tool, process.cwd())
    }
    console.log(`\n${t('init.success')}`)
    require('../generators/init').printArchitectNextSteps(process.cwd())
    return
  }

  const { input, select, checkbox } = require('@inquirer/prompts')

  let projectName, projectType, frontend, pkgManager, backend, backendFramework, hooks, ci, aiTools, requireReqInCommit
  let identitySelect = ''
  let customDisplayNames = []
  let userNickname = ''
  const knownAgentIds = identityStore.knownAgentIds()

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
        { name: 'Kiro', value: 'kiro' },
      ],
    })

    // Identidade dos agentes — oculta se resolvida via flag ou já existente
    if (!skipIdentityWizard) {
      identitySelect = await select({
        message: t('init.prompt.identityPreset'),
        choices: [
          { name: 'Panteão grego (Zeus, Apolo, Afrodite...)', value: 'greek' },
          { name: 'Mitologia nórdica (Odin, Thor, Freya...)', value: 'norse' },
          { name: 'Pioneiros da computação (Turing, Codd, Knuth...)', value: 'pioneers' },
          { name: 'Harry Potter (Dumbledore, Snape, Luna...)', value: 'potter' },
          { name: 'Game of Thrones (Tyrion, Jon, Arya...)', value: 'thrones' },
          { name: 'Senhor dos Anéis (Gandalf, Aragorn, Arwen...)', value: 'tolkien' },
          { name: 'Star Wars (Yoda, Leia, Vader...)', value: 'starwars' },
          { name: 'Chaves (Girafales, Madruga, Chiquinha...)', value: 'chaves' },
          { name: 'Turma da Mônica (Franjinha, Cebolinha, Mônica...)', value: 'turma' },
          { name: 'Panteão egípcio (Thoth, Ísis, Anúbis...)', value: 'egyptian' },
          { name: 'Personalizar um a um', value: 'custom' },
          { name: 'Nomes neutros (padrão)', value: 'neutral' },
        ],
      })

      if (identitySelect === 'custom') {
        customDisplayNames = []
        for (const id of knownAgentIds) {
          const value = await input({
            message: `${t('init.prompt.identityCustomName')} (${id})`,
            validate: candidate => {
              let slug
              try {
                slug = identityStore.slugify(candidate)
              } catch (slugErr) {
                return slugErr.message
              }
              for (let index = 0; index < customDisplayNames.length; index += 1) {
                let otherSlug
                try {
                  otherSlug = identityStore.slugify(customDisplayNames[index])
                } catch {
                  continue
                }
                if (otherSlug === slug) return `slug "${slug}" duplicado com o agente "${knownAgentIds[index]}"`
              }
              return true
            },
          })
          customDisplayNames.push(value)
        }
      }

      if (identitySelect !== '' && identitySelect !== 'neutral') {
        userNickname = await input({
          message: t('init.prompt.identityNickname'),
          default: '',
        })
      }
    }
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
    }
    await generators.scaffold(cfg)
    console.log(`\n${t('init.success')}`)
    require('../generators/init').printArchitectNextSteps(process.cwd())
    return
  }

  if (!skipIdentityWizard) {
    saveWizardIdentity(home, identitySelect, knownAgentIds, customDisplayNames, userNickname)
  }

  const cfg = { projectName, projectType, frontend, backend, backendFramework, pkgManager, hooks, ci, requireReqInCommit }
  await generators.scaffold(cfg)

  for (const tool of (aiTools || [])) await generators.installIntegrationTarget(tool, process.cwd())

  console.log(`\n${t('init.success')}`)
  require('../generators/init').printArchitectNextSteps(process.cwd())
})

module.exports = cmd
