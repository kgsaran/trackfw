'use strict'
const { Command } = require('commander')
const os = require('os')
const path = require('path')
const fs = require('fs')
const { t } = require('../i18n')

function pluginsDir() {
  return path.join(os.homedir(), '.trackfw', 'plugins')
}

function platformOS() {
  if (process.platform === 'win32') return 'windows'
  if (process.platform === 'darwin') return 'darwin'
  return 'linux'
}

function platformArch() {
  if (process.arch === 'x64') return 'amd64'
  return process.arch
}

function listPlugins() {
  const dir = pluginsDir()
  fs.mkdirSync(dir, { recursive: true })
  return fs.readdirSync(dir).filter(f => fs.statSync(path.join(dir, f)).isFile())
}

async function installPlugin(repo) {
  let base = repo
  let tag = 'latest'
  const atIdx = repo.indexOf('@')
  if (atIdx !== -1) {
    base = repo.slice(0, atIdx)
    tag = repo.slice(atIdx + 1)
  }
  const pluginName = path.basename(base)
  const assetName = `trackfw-plugin-${pluginName}-${platformOS()}-${platformArch()}`
  const url = tag === 'latest'
    ? `https://github.com/${base}/releases/latest/download/${assetName}`
    : `https://github.com/${base}/releases/download/${tag}/${assetName}`

  const res = await fetch(url)
  if (!res.ok) throw new Error(t('errors.downloadFailed', { status: res.status, url }))

  const dir = pluginsDir()
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, pluginName), Buffer.from(await res.arrayBuffer()), { mode: 0o755 })
}

function removePlugin(name) {
  const filePath = path.join(pluginsDir(), name)
  if (!fs.existsSync(filePath)) throw new Error(t('errors.pluginNotFound', { name }))
  fs.unlinkSync(filePath)
}

const cmd = new Command('plugins')
cmd.description(t('plugins.description'))

cmd.command('list')
  .description(t('plugins.list.description'))
  .action(() => {
    const plugins = listPlugins()
    if (plugins.length === 0) {
      console.log(t('plugins.list.empty'))
      return
    }
    plugins.forEach(p => console.log(p))
  })

cmd.command('add <repo>')
  .description(t('plugins.add.description'))
  .action(async (repo) => {
    try {
      console.log(t('plugins.add.installing', { repo }))
      await installPlugin(repo)
      const name = repo.split('@')[0].split('/').pop()
      console.log(t('plugins.add.success', { name }))
    } catch (err) {
      console.error(`Error: ${err.message}`)
      process.exit(1)
    }
  })

cmd.command('remove <name>')
  .description(t('plugins.remove.description'))
  .action((name) => {
    try {
      removePlugin(name)
      console.log(t('plugins.remove.success', { name }))
    } catch (err) {
      console.error(`Error: ${err.message}`)
      process.exit(1)
    }
  })

module.exports = cmd
