'use strict';

/**
 * forge/resolve.js — Forge resolution for trackfw ship.
 *
 * Precedence (highest → lowest):
 *  1. --forge flag (explicit override)
 *  2. forge: field in trackfw.yaml
 *  3. Host extracted from git remote get-url origin
 *  4. CI configuration files in repo root (.gitlab-ci.yml / .github/workflows/)
 *  5. "manual" — never an error
 */

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

/** Accepted values for the --forge flag and forge: config field. */
const VALID_FORGES = ['github', 'gitlab', 'bitbucket', 'azure'];

/**
 * Resolve the forge from the given inputs.
 *
 * @param {object} input
 * @param {string} [input.flagForge]   - value from --forge flag
 * @param {string} [input.configForge] - value from trackfw.yaml forge: field
 * @param {string} [input.remoteURL]   - output of "git remote get-url origin"
 * @param {string} [input.repoDir]     - repo root for CI file detection
 * @returns {{ forge: string, source: string }} Resolution object.
 *   forge ∈ {"github", "gitlab", "bitbucket", "azure", "manual"}
 *   source ∈ {"flag", "config", "remote", "ci", "none"}
 * @throws {Error} if flagForge or configForge contains an invalid value.
 */
function resolve(input) {
  const { flagForge = '', configForge = '', remoteURL = '', repoDir = '' } = input || {};

  // 1. Explicit flag wins everything.
  if (flagForge) {
    validateForge(flagForge);
    return { forge: flagForge, source: 'flag' };
  }

  // 2. Config field.
  if (configForge) {
    validateForge(configForge);
    return { forge: configForge, source: 'config' };
  }

  // 3. Remote URL — known hosts only.
  if (remoteURL) {
    const forge = forgeFromRemoteURL(remoteURL);
    if (forge) return { forge, source: 'remote' };
  }

  // 4. CI files — desempate for self-hosted / unknown host.
  if (repoDir) {
    const forge = forgeFromCI(repoDir);
    if (forge) return { forge, source: 'ci' };
  }

  // 5. Manual — not an error.
  return { forge: 'manual', source: 'none' };
}

/**
 * Production entry point: runs "git remote get-url origin" in repoDir,
 * then calls resolve().
 */
function resolveFromRepo(flagForge, configForge, repoDir) {
  const remoteURL = gitRemoteURL(repoDir);
  return resolve({ flagForge, configForge, remoteURL, repoDir });
}

/** @throws {Error} if forge is not in VALID_FORGES */
function validateForge(forge) {
  if (!VALID_FORGES.includes(forge)) {
    throw new Error(
      `invalid forge "${forge}": accepted values are ${VALID_FORGES.join(', ')}`
    );
  }
}

function forgeFromRemoteURL(rawURL) {
  const host = extractHost(rawURL);
  return hostToForge(host);
}

/**
 * Extracts the lowercase hostname from HTTPS, SSH (git@) or ssh:// URLs.
 */
function extractHost(rawURL) {
  if (!rawURL) return '';
  rawURL = rawURL.trim();

  // SSH short form: git@github.com:org/repo.git
  if (rawURL.startsWith('git@')) {
    const rest = rawURL.slice(4); // strip "git@"
    const colonIdx = rest.indexOf(':');
    if (colonIdx >= 0) return rest.slice(0, colonIdx).toLowerCase();
    return '';
  }

  // SSH long form: ssh://git@github.com/org/repo.git
  if (rawURL.startsWith('ssh://')) {
    let rest = rawURL.slice(6); // strip "ssh://"
    const atIdx = rest.indexOf('@');
    if (atIdx >= 0) rest = rest.slice(atIdx + 1); // strip user@
    const slashIdx = rest.indexOf('/');
    return slashIdx >= 0
      ? rest.slice(0, slashIdx).toLowerCase()
      : rest.toLowerCase();
  }

  // HTTPS / HTTP
  if (rawURL.startsWith('https://') || rawURL.startsWith('http://')) {
    let rest = rawURL.replace(/^https?:\/\//, '');
    // strip optional user:pass@
    const atIdx = rest.indexOf('@');
    if (atIdx >= 0) rest = rest.slice(atIdx + 1);
    const slashIdx = rest.indexOf('/');
    return slashIdx >= 0
      ? rest.slice(0, slashIdx).toLowerCase()
      : rest.toLowerCase();
  }

  return '';
}

/**
 * Maps a known hostname to its forge identifier.
 * Azure DevOps uses dev.azure.com (HTTPS) and ssh.dev.azure.com (SSH).
 * Returns '' for unknown / self-hosted hosts.
 */
function hostToForge(host) {
  if (!host) return '';
  if (host === 'github.com') return 'github';
  if (host === 'gitlab.com') return 'gitlab';
  if (host === 'bitbucket.org') return 'bitbucket';
  if (
    host === 'dev.azure.com' ||
    host.endsWith('.dev.azure.com') ||
    host.endsWith('.visualstudio.com')
  ) {
    return 'azure';
  }
  return '';
}

/**
 * Inspects CI indicator files in repoDir.
 * Priority: .gitlab-ci.yml → gitlab; .github/workflows/ → github.
 */
function forgeFromCI(repoDir) {
  try {
    fs.statSync(path.join(repoDir, '.gitlab-ci.yml'));
    return 'gitlab';
  } catch (_) {}

  try {
    const stat = fs.statSync(path.join(repoDir, '.github', 'workflows'));
    if (stat.isDirectory()) return 'github';
  } catch (_) {}

  return '';
}

/**
 * Runs "git remote get-url origin" in repoDir.
 * Returns '' on any error.
 */
function gitRemoteURL(repoDir) {
  try {
    return execSync('git remote get-url origin', {
      cwd: repoDir,
      stdio: ['pipe', 'pipe', 'pipe'],
      encoding: 'utf8',
    }).trim();
  } catch (_) {
    return '';
  }
}

module.exports = {
  VALID_FORGES,
  resolve,
  resolveFromRepo,
  validateForge,
  extractHost,
  hostToForge,
  forgeFromCI,
};
