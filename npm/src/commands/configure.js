'use strict'
const { Command } = require('commander')
const readline = require('readline')
const fs = require('fs')
const path = require('path')

const DEFAULTS = {
  adr_dirs: 'docs/adr',
  req_dir: 'docs/req',
  roadmap_dir: 'docs/roadmaps',
  wip_limit: '1',
  link_req: 'REQ:'
}

/**
 * Pergunta ao usuário via readline e retorna a resposta como Promise<string>.
 * Se o usuário pressionar Enter sem digitar nada, retorna o valor padrão.
 * @param {readline.Interface} rl
 * @param {string} question
 * @param {string} defaultValue
 * @returns {Promise<string>}
 */
function ask(rl, question, defaultValue) {
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      resolve(answer.trim() || defaultValue)
    })
  })
}

/**
 * Executa o wizard interativo e grava trackfw.yaml.
 * Exportada separadamente para facilitar testes.
 * @param {readline.Interface} [rl] — opcional; se não fornecido, cria um novo
 * @param {string} [cwd] — diretório de trabalho (padrão: process.cwd())
 * @returns {Promise<{ fieldsWritten: number, filePath: string }>}
 */
async function runConfigure(rl, cwd) {
  const dir = cwd || process.cwd()
  const yamlPath = path.join(dir, 'trackfw.yaml')
  let ownRl = false

  if (!rl) {
    rl = readline.createInterface({ input: process.stdin, output: process.stdout })
    ownRl = true
  }

  try {
    // Verificar se já existe e perguntar se recria
    if (fs.existsSync(yamlPath)) {
      const answer = await ask(rl, 'trackfw.yaml já existe. Recriar do zero? (s/N) ', 'N')
      if (answer.toLowerCase() !== 's') {
        console.log('Operação cancelada.')
        return { fieldsWritten: 0, filePath: yamlPath }
      }
    }

    // Coletar campos via prompts
    const adrDirs = await ask(rl, `ADR dirs [${DEFAULTS.adr_dirs}]: `, DEFAULTS.adr_dirs)
    const reqDir = await ask(rl, `REQ dir [${DEFAULTS.req_dir}]: `, DEFAULTS.req_dir)
    const roadmapDir = await ask(rl, `Roadmap dir [${DEFAULTS.roadmap_dir}]: `, DEFAULTS.roadmap_dir)
    const wipLimit = await ask(rl, `WIP limit [${DEFAULTS.wip_limit}]: `, DEFAULTS.wip_limit)
    const linkReq = await ask(rl, `Marcador de REQ [${DEFAULTS.link_req}]: `, DEFAULTS.link_req)

    // Montar campos customizados (somente diferenças dos defaults)
    const custom = {}
    if (adrDirs !== DEFAULTS.adr_dirs) custom.adr_dirs = adrDirs
    if (reqDir !== DEFAULTS.req_dir) custom.req_dir = reqDir
    if (roadmapDir !== DEFAULTS.roadmap_dir) custom.roadmap_dir = roadmapDir
    if (wipLimit !== DEFAULTS.wip_limit) custom.wip_limit = wipLimit
    if (linkReq !== DEFAULTS.link_req) custom['link_fields.req'] = linkReq

    // Gerar conteúdo do YAML
    let content = '# trackfw.yaml — gerado por trackfw configure\n'
    const fieldsWritten = Object.keys(custom).length

    if (fieldsWritten > 0) {
      for (const [key, value] of Object.entries(custom)) {
        if (key === 'adr_dirs') {
          content += `adr_dirs:\n  - ${value}\n`
        } else if (key === 'link_fields.req') {
          content += `link_fields:\n  req:\n    - "${value}"\n`
        } else {
          content += `${key}: ${value}\n`
        }
      }
    }

    fs.writeFileSync(yamlPath, content, 'utf8')
    console.log(`trackfw.yaml gravado com ${fieldsWritten} campos customizados`)

    return { fieldsWritten, filePath: yamlPath }
  } finally {
    if (ownRl) rl.close()
  }
}

const cmd = new Command('configure')
cmd.description('Wizard interativo para criar/atualizar trackfw.yaml')
cmd.action(async () => {
  await runConfigure()
})

module.exports = cmd
module.exports.runConfigure = runConfigure
module.exports.DEFAULTS = DEFAULTS
