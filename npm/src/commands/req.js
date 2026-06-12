'use strict'
const { Command } = require('commander')
const { listREQs } = require('../generators/req')

const cmd = new Command('req')
cmd.description('Manage Requirements')

cmd.command('new <title>')
  .description('Create a new REQ')
  .action(async (title) => {
    const { input, select } = require('@inquirer/prompts')
    const generators = require('../generators/req')
    const adrGenerators = require('../generators/adr')

    const content = { title, motivation: '', criteria: '', dependsOnADRs: [] }

    if (process.stdin.isTTY) {
      // Form 1 — título + motivação
      content.title = await input({ message: 'Project requirement', default: title })
      content.motivation = await input({ message: 'Motivation (why is this needed?)', default: '' })

      // Detectar domínios com base em título + motivação
      const probes = generators.detectDomains(content.title + ' ' + content.motivation)

      // Form 2 — critérios de aceite
      content.criteria = await input({ message: 'Acceptance Criteria (one per line)', default: '- [ ]\n- [ ]' })

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
              const basename = await adrGenerators.newADRDraft(answer)
              if (basename) generatedADRs.push(basename)
            } catch (e) {
              console.warn(`warning: could not create ADR draft for ${answer}: ${e.message}`)
            }
          }
        }
      }

      content.dependsOnADRs = [...new Set(generatedADRs)]
    }

    await generators.newREQ(content)

    if (content.dependsOnADRs.length > 0) {
      console.log('\nADR drafts created:')
      content.dependsOnADRs.forEach(adr => console.log(`  -> ${adr}`))
      console.log('\nResolve these ADRs (set Status: Accepted) before creating a roadmap.')
    }
  })

cmd.command('list')
  .description('List all REQs in docs/req/')
  .action(async () => {
    listREQs('docs/req')
  })

module.exports = cmd
