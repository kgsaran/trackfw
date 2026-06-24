'use strict'

const fs = require('fs')
const path = require('path')

function getAttention(cfg) {
  const attentionPath = path.join(cfg.roadmapDir, '.trackfw-attention.json')
  try {
    const payload = JSON.parse(fs.readFileSync(attentionPath, 'utf8'))
    return { ...payload, active: true }
  } catch (_) {
    return { active: false }
  }
}

function handleAttention(cfg, req, res) {
  res.writeHead(200, {
    'Content-Type': 'application/json',
    'Access-Control-Allow-Origin': '*'
  })
  res.end(JSON.stringify(getAttention(cfg)))
}

module.exports = { getAttention, handleAttention }
