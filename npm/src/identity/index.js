'use strict'

const { slugify } = require('./slug')
const { knownAgentIds, preset, presetNames } = require('./preset')
const { load, save, agentName, validate, lookup, identityPath, SCHEMA_VERSION } = require('./config')

module.exports = {
  slugify,
  knownAgentIds,
  preset,
  presetNames,
  load,
  save,
  agentName,
  validate,
  lookup,
  identityPath,
  SCHEMA_VERSION,
}
