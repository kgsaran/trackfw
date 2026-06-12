'use strict'
const path = require('path')
const fs = require('fs')

function detectLocale() {
  const raw = process.env.LANG || process.env.LC_ALL || process.env.LANGUAGE || ''
  // Mapeia pt_BR.UTF-8 → pt-BR, es_ES.UTF-8 → es-ES, en_US.UTF-8 → en-US
  const map = { pt: 'pt-BR', es: 'es-ES' }
  const code = raw.split('.')[0].replace('_', '-')  // pt-BR
  const lang = code.split('-')[0]                   // pt
  if (map[lang]) return map[lang]
  // Fallback para Windows: usar Intl
  try {
    const loc = Intl.DateTimeFormat().resolvedOptions().locale
    const l = loc.split('-')[0]
    if (map[l]) return map[l]
  } catch (_) {}
  return 'en-US'
}

let _locale = null
let _messages = null

function load() {
  if (_messages) return
  _locale = detectLocale()
  const filePath = path.join(__dirname, 'locales', `${_locale}.json`)
  const fallback = path.join(__dirname, 'locales', 'en-US.json')
  try {
    _messages = JSON.parse(fs.readFileSync(fs.existsSync(filePath) ? filePath : fallback, 'utf8'))
  } catch (_) {
    _messages = {}
  }
}

function t(key, vars = {}) {
  load()
  const keys = key.split('.')
  let val = _messages
  for (const k of keys) {
    val = val?.[k]
    if (val === undefined) break
  }
  if (typeof val !== 'string') return key
  return val.replace(/\{\{(\w+)\}\}/g, (_, k) => vars[k] ?? `{{${k}}}`)
}

function locale() {
  load()
  return _locale
}

module.exports = { t, locale }
