'use strict'

const { Command } = require('commander')
const https = require('https')
const http = require('http')
const fs = require('fs')
const path = require('path')

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

/**
 * Lê um campo de trackfw.yaml (parse linha a linha, sem dependências externas).
 * @param {string} field
 * @returns {string}
 */
function readConfigField(field) {
  try {
    const data = fs.readFileSync('trackfw.yaml', 'utf8')
    const prefix = field + ':'
    for (const line of data.split('\n')) {
      const trimmed = line.trimStart()
      if (trimmed.startsWith(prefix)) {
        let value = trimmed.slice(prefix.length).trim()
        if (value.length >= 2 &&
          ((value[0] === '"' && value[value.length - 1] === '"') ||
           (value[0] === "'" && value[value.length - 1] === "'"))) {
          value = value.slice(1, -1)
        }
        return value
      }
    }
  } catch (_) { /* sem arquivo */ }
  return ''
}

function getConfig(field, envVar) {
  return readConfigField(field) || process.env[envVar] || ''
}

// ---------------------------------------------------------------------------
// HTTP helper
// ---------------------------------------------------------------------------

/**
 * Faz uma requisição HTTP/HTTPS simples com corpo JSON.
 * @returns {Promise<{status: number, body: string}>}
 */
function request(url, options, bodyStr) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url)
    const lib = parsed.protocol === 'https:' ? https : http
    const reqOptions = {
      hostname: parsed.hostname,
      port: parsed.port || (parsed.protocol === 'https:' ? 443 : 80),
      path: parsed.pathname + parsed.search,
      method: options.method || 'GET',
      headers: options.headers || {}
    }
    const req = lib.request(reqOptions, (res) => {
      let data = ''
      res.on('data', (chunk) => { data += chunk })
      res.on('end', () => resolve({ status: res.statusCode, body: data }))
    })
    req.on('error', reject)
    if (bodyStr) req.write(bodyStr)
    req.end()
  })
}

// ---------------------------------------------------------------------------
// Linear client
// ---------------------------------------------------------------------------

/**
 * Cria issue no Linear via GraphQL.
 * @param {string} apiKey
 * @param {string} teamId
 * @param {string} title
 * @param {string} description
 * @returns {Promise<string>} issue identifier (ex: "ENG-123")
 */
async function linearCreateIssue(apiKey, teamId, title, description) {
  const query = `mutation IssueCreate($title: String!, $description: String!, $teamId: String!) {
    issueCreate(input: {title: $title, description: $description, teamId: $teamId}) {
      success
      issue { id identifier }
    }
  }`
  const payload = JSON.stringify({
    query,
    variables: { title, description, teamId }
  })

  const res = await request('https://api.linear.app/graphql', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': apiKey,
      'Content-Length': Buffer.byteLength(payload)
    }
  }, payload)

  if (res.status !== 200) {
    throw new Error(`Linear: unexpected status ${res.status}: ${res.body}`)
  }

  const data = JSON.parse(res.body)
  if (data.errors && data.errors.length > 0) {
    throw new Error(`Linear API error: ${data.errors[0].message}`)
  }
  if (!data.data.issueCreate.success) {
    throw new Error('Linear: issueCreate returned success=false')
  }
  return data.data.issueCreate.issue.identifier
}

// ---------------------------------------------------------------------------
// Jira client
// ---------------------------------------------------------------------------

/**
 * Cria issue no Jira via REST API v3.
 * @returns {Promise<string>} issue key (ex: "ENG-456")
 */
async function jiraCreateIssue(baseUrl, email, token, project, title, description) {
  const payload = JSON.stringify({
    fields: {
      project: { key: project },
      summary: title,
      description: {
        type: 'doc',
        version: 1,
        content: [{
          type: 'paragraph',
          content: [{ type: 'text', text: description }]
        }]
      },
      issuetype: { name: 'Story' }
    }
  })

  const creds = Buffer.from(`${email}:${token}`).toString('base64')
  const url = baseUrl.replace(/\/$/, '') + '/rest/api/3/issue'

  const res = await request(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Accept': 'application/json',
      'Authorization': `Basic ${creds}`,
      'Content-Length': Buffer.byteLength(payload)
    }
  }, payload)

  if (res.status !== 201) {
    throw new Error(`Jira: unexpected status ${res.status}: ${res.body}`)
  }

  const data = JSON.parse(res.body)
  if (!data.key) {
    throw new Error('Jira: response missing issue key')
  }
  return data.key
}

// ---------------------------------------------------------------------------
// REQ file helpers
// ---------------------------------------------------------------------------

function isStatusOpen(text) {
  for (const line of text.split('\n')) {
    if (line.includes('| Status:')) {
      return line.includes('Status: Open')
    }
  }
  return false
}

function extractField(text, field) {
  const prefix = '| ' + field + ':'
  for (const line of text.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith(prefix)) {
      return trimmed.slice(prefix.length).trim()
    }
  }
  return ''
}

function extractTitle(text) {
  for (const line of text.split('\n')) {
    if (line.startsWith('# REQ: ')) {
      return line.slice('# REQ: '.length)
    }
  }
  return ''
}

function extractMotivation(text) {
  const lines = text.split('\n')
  let inSection = false
  const parts = []
  for (const line of lines) {
    if (line.startsWith('## Motivation') || line.startsWith('## Motivação')) {
      inSection = true
      continue
    }
    if (inSection) {
      if (line.startsWith('## ')) break
      parts.push(line)
    }
  }
  return parts.join('\n').trim()
}

function injectField(text, field, value) {
  const prefix = '| ' + field + ':'
  const lines = text.split('\n')

  // se campo já existe, substituir
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].trim().startsWith(prefix)) {
      lines[i] = `| ${field}: ${value}`
      return lines.join('\n')
    }
  }

  // inserir após a linha com | Status:
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].includes('| Status:')) {
      lines.splice(i + 1, 0, `| ${field}: ${value}`)
      return lines.join('\n')
    }
  }

  return text
}

// ---------------------------------------------------------------------------
// Core sync logic
// ---------------------------------------------------------------------------

/**
 * @param {Function} createFn (title, desc) => Promise<issueId>
 * @param {string} issueField
 * @returns {Promise<Array<{reqPath, issueId, skipped, error}>>}
 */
async function syncToProvider(createFn, issueField) {
  const reqDir = 'docs/req'
  let files = []
  try {
    files = fs.readdirSync(reqDir)
      .filter(f => f.endsWith('.md'))
      .map(f => path.join(reqDir, f))
  } catch (_) {
    return []
  }

  const results = []
  for (const f of files) {
    let text
    try {
      text = fs.readFileSync(f, 'utf8')
    } catch (e) {
      results.push({ reqPath: f, skipped: false, error: e })
      continue
    }

    // pular se não é Open
    if (!isStatusOpen(text)) {
      results.push({ reqPath: f, skipped: true })
      continue
    }

    // pular se já tem issue vinculado
    if (extractField(text, issueField) !== '') {
      results.push({ reqPath: f, skipped: true })
      continue
    }

    const title = extractTitle(text)
    const desc = extractMotivation(text)

    try {
      const issueId = await createFn(title, desc)
      const updated = injectField(text, issueField, issueId)
      fs.writeFileSync(f, updated, 'utf8')
      results.push({ reqPath: f, issueId, skipped: false })
    } catch (e) {
      results.push({ reqPath: f, skipped: false, error: e })
    }
  }
  return results
}

async function syncToLinear() {
  const apiKey = getConfig('linear_api_key', 'LINEAR_API_KEY')
  const teamId = getConfig('linear_team_id', 'LINEAR_TEAM_ID')
  if (!apiKey) throw new Error('Linear API key not found. Set LINEAR_API_KEY env var or linear_api_key in trackfw.yaml')
  if (!teamId) throw new Error('Linear Team ID not found. Set LINEAR_TEAM_ID env var or linear_team_id in trackfw.yaml')
  return syncToProvider((title, desc) => linearCreateIssue(apiKey, teamId, title, desc), 'linear_issue')
}

async function syncToJira() {
  const baseUrl = getConfig('jira_base_url', 'JIRA_BASE_URL')
  const email = getConfig('jira_email', 'JIRA_EMAIL')
  const token = getConfig('jira_token', 'JIRA_TOKEN')
  const project = getConfig('jira_project', 'JIRA_PROJECT')
  if (!baseUrl) throw new Error('Jira base URL not found. Set JIRA_BASE_URL env var or jira_base_url in trackfw.yaml')
  if (!email) throw new Error('Jira email not found. Set JIRA_EMAIL env var or jira_email in trackfw.yaml')
  if (!token) throw new Error('Jira API token not found. Set JIRA_TOKEN env var or jira_token in trackfw.yaml')
  if (!project) throw new Error('Jira project key not found. Set JIRA_PROJECT env var or jira_project in trackfw.yaml')
  return syncToProvider((title, desc) => jiraCreateIssue(baseUrl, email, token, project, title, desc), 'jira_issue')
}

// ---------------------------------------------------------------------------
// Commander command
// ---------------------------------------------------------------------------

const syncCmd = new Command('sync')
  .description('Sync Open REQs to a project management tool')
  .requiredOption('--to <target>', 'Target PM tool: linear or jira')
  .action(async (options) => {
    let results
    try {
      switch (options.to) {
        case 'linear':
          results = await syncToLinear()
          break
        case 'jira':
          results = await syncToJira()
          break
        default:
          console.error(`Unknown target "${options.to}" — use --to=linear or --to=jira`)
          process.exit(1)
      }
    } catch (e) {
      console.error(e.message)
      process.exit(1)
    }

    if (!results || results.length === 0) {
      console.log('No REQs found in docs/req/')
      return
    }

    console.log(`${'REQ'.padEnd(55)} ISSUE`)
    console.log(`${'-'.repeat(54)} ${'-'.repeat(10)}`)
    for (const r of results) {
      if (r.skipped) {
        console.log(`${r.reqPath.padEnd(55)} (skipped)`)
      } else if (r.error) {
        console.log(`${r.reqPath.padEnd(55)} ERROR: ${r.error.message}`)
      } else {
        console.log(`${r.reqPath.padEnd(55)} ${r.issueId}`)
      }
    }
  })

module.exports = syncCmd
