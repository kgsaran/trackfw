'use strict'
const path = require('path')
const os = require('os')
const { Command } = require('commander')
const { listREQs, moveREQ } = require('../generators/req')
const { t } = require('../i18n')

const cmd = new Command('req')
cmd.description(t('req.description'))

cmd.command('new <title>')
  .description(t('req.new.description'))
  .action(async (title) => {
    const { input, select } = require('@inquirer/prompts')
    const generators = require('../generators/req')
    const adrGenerators = require('../generators/adr')

    const content = { title, motivation: '', criteria: '', dependsOnADRs: [] }

    if (process.stdin.isTTY) {
      // Form 1 — título + motivação
      content.title = await input({ message: t('req.new.prompt.title'), default: title })
      content.motivation = await input({ message: t('req.new.prompt.motivation'), default: '' })

      // Detectar domínios com base em título + motivação
      const probes = generators.detectDomains(content.title + ' ' + content.motivation)

      // Form 2 — critérios de aceite
      content.criteria = await input({ message: t('req.new.prompt.criteria'), default: '- [ ]\n- [ ]' })

      // Escopo dos ADR drafts desta sessão (uma única pergunta, vale para todos)
      const adrScope = await select({
        message: t('req.new.prompt.adrScope'),
        choices: [
          { name: t('req.new.prompt.adrScopeLocal'), value: 'local' },
          { name: t('req.new.prompt.adrScopeGlobal'), value: 'global' },
        ],
        default: 'local',
      })
      const adrDir = adrScope === 'global'
        ? path.join(os.homedir(), '.trackfw', 'adr')
        : require('../config').load().adrDirs[0]

      // Perguntas dinâmicas por probe
      const generatedADRs = []
      for (const probe of probes) {
        for (const question of probe.questions) {
          const choices = question.options.map(opt => ({
            name: opt.label,
            value: opt.adrSlug || '',
          }))
          const answer = await select({
            message: question.text,
            choices,
          })
          if (answer) {
            try {
              const basename = await adrGenerators.newADRDraft(answer, adrDir)
              if (basename) generatedADRs.push(basename)
            } catch (e) {
              console.warn(t('req.new.adrWarning', { slug: answer, message: e.message }))
            }
          }
        }
      }

      content.dependsOnADRs = [...new Set(generatedADRs)]
    }

    await generators.newREQ(content)

    if (content.dependsOnADRs.length > 0) {
      console.log(`\n${t('req.new.adrDraftsCreated')}`)
      content.dependsOnADRs.forEach(adr => console.log(`  -> ${adr}`))
      console.log(`\n${t('req.new.resolveADRs')}`)
    }
  })

cmd.command('list')
  .description(t('req.list.description'))
  .action(async () => {
    listREQs(require('../config').load())
  })

cmd.command('move <name> <status>')
  .description('Update a REQ status in place')
  .action(async (name, status) => {
    try {
      moveREQ(name, status)
    } catch (err) {
      console.error(`Error: ${err.message}`)
      process.exitCode = 1
    }
  })

module.exports = cmd
