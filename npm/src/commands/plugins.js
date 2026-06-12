'use strict'
const { Command } = require('commander')
const os = require('os')
const path = require('path')
const fs = require('fs')

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
  if (!res.ok) throw new Error(`download failed: HTTP ${res.status} for ${url}`)

  const dir = pluginsDir()
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, pluginName), Buffer.from(await res.arrayBuffer()), { mode: 0o755 })
}

function removePlugin(name) {
  const filePath = path.join(pluginsDir(), name)
  if (!fs.existsSync(filePath)) throw new Error(`plugin "${name}" not found`)
  fs.unlinkSync(filePath)
}

const cmd = new Command('plugins')
cmd.description('Manage trackfw plugins')

cmd.command('list')
  .description('List installed plugins')
  .action(() => {
    const plugins = listPlugins()
    if (plugins.length === 0) {
      console.log('No plugins installed. Use `trackfw plugins add <user/repo>` to install one.')
      return
    }
    plugins.forEach(p => console.log(p))
  })

cmd.command('add <repo>')
  .description('Install a plugin from GitHub Releases (user/repo or user/repo@tag)')
  .action(async (repo) => {
    try {
      console.log(`Installing plugin from ${repo}...`)
      await installPlugin(repo)
      const name = repo.split('@')[0].split('/').pop()
      console.log(`Plugin "${name}" installed successfully.`)
    } catch (err) {
      console.error(`Error: ${err.message}`)
      process.exit(1)
    }
  })

cmd.command('remove <name>')
  .description('Remove an installed plugin')
  .action((name) => {
    try {
      removePlugin(name)
      console.log(`Plugin "${name}" removed.`)
    } catch (err) {
      console.error(`Error: ${err.message}`)
      process.exit(1)
    }
  })

module.exports = cmd
