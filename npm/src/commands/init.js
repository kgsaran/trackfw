'use strict'
const { Command } = require('commander')

const cmd = new Command('init')
cmd.description('Initialize trackfw governance in the current project')
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
    console.log("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
    return
  }

  const { input, select, checkbox } = require('@inquirer/prompts')

  let projectName, projectType, frontend, pkgManager, backend, hooks, ci, aiTools

  try {
    projectName = await input({
      message: 'Project name?',
      default: path.basename(process.cwd()),
    })

    projectType = await select({
      message: 'Project type?',
      choices: [
        { name: 'Full-stack (frontend + backend)', value: 'fullstack' },
        { name: 'Frontend only', value: 'frontend' },
        { name: 'Backend only', value: 'backend' },
        { name: 'Governance only (no build stack)', value: 'governance' },
      ],
    })

    frontend = ''
    pkgManager = ''
    if (projectType === 'fullstack' || projectType === 'frontend') {
      frontend = await select({
        message: 'Frontend stack?',
        choices: [
          { name: 'React / Next.js', value: 'react' },
          { name: 'Vue', value: 'vue' },
          { name: 'Angular', value: 'angular' },
        ],
      })
      pkgManager = await select({
        message: 'Package manager?',
        choices: [
          { name: 'npm', value: 'npm' },
          { name: 'pnpm', value: 'pnpm' },
          { name: 'yarn', value: 'yarn' },
          { name: 'bun', value: 'bun' },
        ],
      })
    }

    backend = ''
    if (projectType === 'fullstack' || projectType === 'backend') {
      backend = await select({
        message: 'Backend stack?',
        choices: [
          { name: 'Go', value: 'go' },
          { name: 'Java / Spring Boot', value: 'java' },
          { name: 'Node.js', value: 'node' },
          { name: 'Python', value: 'python' },
        ],
      })
    }

    hooks = await select({
      message: 'Git hooks?',
      choices: [
        { name: 'husky', value: 'husky' },
        { name: 'lefthook', value: 'lefthook' },
        { name: 'None', value: 'none' },
      ],
    })

    ci = await select({
      message: 'CI system?',
      choices: [
        { name: 'GitHub Actions', value: 'github-actions' },
        { name: 'GitLab CI', value: 'gitlab-ci' },
        { name: 'None', value: 'none' },
      ],
    })

    aiTools = await checkbox({
      message: 'Which AI assistants do you use?',
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
    console.log("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
    return
  }

  const cfg = { projectName, projectType, frontend, backend, pkgManager, hooks, ci }
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

  console.log("\n✓ trackfw initialized — run 'trackfw status' to see your governance state.")
})

module.exports = cmd
