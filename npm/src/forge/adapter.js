'use strict';

const fs = require('fs');
const path = require('path');

// ---------------------------------------------------------------------------
// Verificação de disponibilidade de CLI — scan puro no PATH sem subprocess
// ---------------------------------------------------------------------------

/**
 * Verifica se um executável está disponível no PATH, respeitando
 * TRACKFW_DISABLE_EXTERNAL_COMMANDS (mesmo padrão de discover.go).
 * @param {string} name
 * @returns {boolean}
 */
function _defaultAvailFn(name) {
  if (!name) return false;
  if (process.env.TRACKFW_DISABLE_EXTERNAL_COMMANDS === '1') return false;

  const dirs = (process.env.PATH || '').split(path.delimiter).filter(Boolean);
  const isWindows = process.platform === 'win32';
  const exts = isWindows
    ? (process.env.PATHEXT || '.COM;.EXE;.BAT;.CMD').split(';')
    : [''];

  for (const dir of dirs) {
    for (const ext of exts) {
      try {
        const candidate = path.join(dir, name + ext);
        fs.accessSync(candidate, fs.constants.F_OK | fs.constants.X_OK);
        return true;
      } catch (_) {
        // continua
      }
    }
  }
  return false;
}

// ---------------------------------------------------------------------------
// Conversão de remote URL → base HTTPS
// ---------------------------------------------------------------------------

/**
 * Converte qualquer formato de remote git (git@, ssh://, https://, http://)
 * para URL base HTTPS sem .git.
 *
 * Normalização especial para Azure SSH:
 *   git@ssh.dev.azure.com:v3/org/project/repo → https://dev.azure.com/org/project/_git/repo
 *
 * @param {string} rawURL
 * @param {string} forge
 * @returns {string}
 */
function remoteHTTPSBase(rawURL, forge) {
  if (!rawURL) return '';
  rawURL = rawURL.trim();
  if (!rawURL) return '';

  let host = '';
  let pathStr = '';

  if (rawURL.startsWith('git@')) {
    // git@host:path
    const rest = rawURL.slice(4);
    const colonIdx = rest.indexOf(':');
    if (colonIdx < 0) return '';
    host = rest.slice(0, colonIdx).toLowerCase();
    pathStr = rest.slice(colonIdx + 1);

  } else if (rawURL.startsWith('ssh://')) {
    // ssh://[user@]host/path
    let rest = rawURL.slice(6);
    const atIdx = rest.indexOf('@');
    if (atIdx >= 0) rest = rest.slice(atIdx + 1);
    const slashIdx = rest.indexOf('/');
    if (slashIdx < 0) {
      host = rest.toLowerCase();
    } else {
      host = rest.slice(0, slashIdx).toLowerCase();
      pathStr = rest.slice(slashIdx + 1);
    }

  } else if (rawURL.startsWith('https://') || rawURL.startsWith('http://')) {
    let rest = rawURL.replace(/^https?:\/\//, '');
    const atIdx = rest.indexOf('@');
    if (atIdx >= 0) rest = rest.slice(atIdx + 1);
    const slashIdx = rest.indexOf('/');
    if (slashIdx < 0) {
      host = rest.toLowerCase();
    } else {
      host = rest.slice(0, slashIdx).toLowerCase();
      pathStr = rest.slice(slashIdx + 1);
    }

  } else {
    return '';
  }

  pathStr = pathStr.replace(/\.git$/, '').replace(/^\/|\/$/g, '');

  // Normalização Azure SSH
  if (forge === 'azure' && host === 'ssh.dev.azure.com') {
    host = 'dev.azure.com';
    pathStr = pathStr.replace(/^v3\//, '');
    const parts = pathStr.split('/');
    if (parts.length === 3) {
      pathStr = `${parts[0]}/${parts[1]}/_git/${parts[2]}`;
    }
  }

  if (!pathStr) return `https://${host}`;
  return `https://${host}/${pathStr}`;
}

// ---------------------------------------------------------------------------
// Adapter
// ---------------------------------------------------------------------------

/**
 * @typedef {Object} Adapter
 * @property {string} forge
 * @property {string} noun        - "Pull Request" ou "Merge Request"
 * @property {string} cliName     - nome do executável (vazio = sem CLI)
 * @property {string[]} cliArgs   - argumentos para criar PR/MR
 * @property {boolean} available  - false quando CLI ausente
 * @property {function(string, string): string} fallbackURL
 */

/**
 * Retorna o Adapter para o forge informado.
 *
 * @param {string} forge
 * @param {function(string): boolean} [availFn]  - injeta verificação de CLI (para testes)
 * @returns {Adapter}
 */
function forgeAdapter(forge, availFn) {
  if (typeof availFn !== 'function') {
    availFn = _defaultAvailFn;
  }

  let noun = 'Pull Request';
  let cliName = '';
  let cliArgs = [];
  let available = false;

  switch (forge) {
    case 'github':
      cliName = 'gh';
      cliArgs = ['pr', 'create'];
      available = availFn('gh');
      break;
    case 'gitlab':
      noun = 'Merge Request';
      cliName = 'glab';
      cliArgs = ['mr', 'create'];
      available = availFn('glab');
      break;
    case 'azure':
      cliName = 'az';
      cliArgs = ['repos', 'pr', 'create'];
      available = availFn('az');
      break;
    case 'bitbucket':
      // Bitbucket não possui CLI — nunca chama availFn
      break;
    default:
      // forge desconhecido: sem CLI, sem disponibilidade
      break;
  }

  const adapter = { forge, noun, cliName, cliArgs, available };

  adapter.fallbackURL = function (remoteURL, branch) {
    const base = remoteHTTPSBase(remoteURL, forge);
    if (!base) return '';
    switch (forge) {
      case 'github':    return `${base}/compare/${branch}?expand=1`;
      case 'gitlab':    return `${base}/-/merge_requests/new?merge_request[source_branch]=${branch}`;
      case 'bitbucket': return `${base}/pull-requests/new?source=${branch}`;
      case 'azure':     return `${base}/pullrequestcreate?sourceRef=${branch}`;
      default:          return '';
    }
  };

  return adapter;
}

module.exports = { forgeAdapter, remoteHTTPSBase };
