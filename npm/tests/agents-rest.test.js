const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { installCursor, installWindsurf, installAmazonQ } = require('../src/generators/init')

async function assertIdempotentToolInstall(toolName, installer, expectedFiles) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), `trackfw-${toolName}-`))
  await installer(root)
  const snapshots = expectedFiles.map(file => fs.readFileSync(path.join(root, file), 'utf8'))
  await installer(root)
  expectedFiles.forEach((file, index) => {
    assert.equal(fs.existsSync(path.join(root, file)), true, file)
    assert.equal(fs.readFileSync(path.join(root, file), 'utf8'), snapshots[index], `${toolName}:${file}`)
  })
}

test('installCursor creates idempotent Cursor rules', async () => {
  await assertIdempotentToolInstall('cursor', installCursor, ['.cursor/rules/trackfw.mdc'])
})

test('installWindsurf creates idempotent Windsurf rules', async () => {
  await assertIdempotentToolInstall('windsurf', installWindsurf, ['.windsurfrules'])
})

test('installAmazonQ creates idempotent Amazon Q rules', async () => {
  await assertIdempotentToolInstall('amazonq', installAmazonQ, ['.amazonq/developer/guidelines.md'])
})
