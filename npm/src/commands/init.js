'use strict'
const { Command } = require('commander')
const { t } = require('../i18n')

const cmd = new Command('init')
cmd.description(t('init.description'))
cmd.action(async () => {
  const path = require('path')
  const generators = require('../generators/init')

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
    console.log(`\n${t('init.success')}`)
    return
  }

  const { input, select, checkbox } = require('@inquirer/prompts')

  let projectName, projectType, frontend, pkgManager, backend, backendFramework, hooks, ci, aiTools

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

    aiTools = await checkbox({
      message: t('init.prompt.aiTools'),
      choices: [
        { name: 'Claude Code', value: 'claude' },
        { name: 'Gemini CLI', value: 'gemini' },
        { name: 'Cursor', value: 'cursor' },
        { name: 'GitHub Copilot', value: 'copilot' },
        { name: 'Windsurf', value: 'windsurf' },
        { name: 'Amazon Q Developer', value: 'amazonq' },
      ],
    })
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
    return
  }

  const cfg = { projectName, projectType, frontend, backend, backendFramework, pkgManager, hooks, ci }
  await generators.scaffold(cfg)

  for (const tool of (aiTools || [])) {
    switch (tool) {
      case 'claude':   await generators.installAgents();  break
      case 'gemini':   await generators.installGemini();  break
      case 'cursor':   await generators.installCursor();  break
      case 'copilot':  await generators.installCopilot(); break
      case 'windsurf': await generators.installWindsurf(); break
      case 'amazonq':  await generators.installAmazonQ(); break
    }
  }

  console.log(`\n${t('init.success')}`)
})

module.exports = cmd
