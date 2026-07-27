'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { resolve, extractHost, hostToForge, forgeFromCI, VALID_FORGES } = require('../src/forge/resolve');

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function tmpDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-forge-'));
}

function touch(dir, ...parts) {
  fs.writeFileSync(path.join(dir, ...parts), '');
}

function mkdir(dir, ...parts) {
  fs.mkdirSync(path.join(dir, ...parts), { recursive: true });
}

function cleanup(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
}

// ---------------------------------------------------------------------------
// Precedence
// ---------------------------------------------------------------------------

test('flag wins over config', () => {
  const res = resolve({ flagForge: 'github', configForge: 'gitlab' });
  assert.equal(res.forge, 'github');
  assert.equal(res.source, 'flag');
});

test('flag wins over remote', () => {
  const res = resolve({ flagForge: 'github', remoteURL: 'https://gitlab.com/org/repo.git' });
  assert.equal(res.forge, 'github');
  assert.equal(res.source, 'flag');
});

test('flag wins over CI', () => {
  const dir = tmpDir();
  touch(dir, '.gitlab-ci.yml');
  try {
    const res = resolve({ flagForge: 'bitbucket', repoDir: dir });
    assert.equal(res.forge, 'bitbucket');
    assert.equal(res.source, 'flag');
  } finally {
    cleanup(dir);
  }
});

test('config wins over remote', () => {
  const res = resolve({ configForge: 'bitbucket', remoteURL: 'https://github.com/org/repo.git' });
  assert.equal(res.forge, 'bitbucket');
  assert.equal(res.source, 'config');
});

test('config wins over CI', () => {
  const dir = tmpDir();
  touch(dir, '.gitlab-ci.yml');
  try {
    const res = resolve({ configForge: 'azure', repoDir: dir });
    assert.equal(res.forge, 'azure');
    assert.equal(res.source, 'config');
  } finally {
    cleanup(dir);
  }
});

test('remote wins over CI', () => {
  const dir = tmpDir();
  touch(dir, '.gitlab-ci.yml');
  try {
    const res = resolve({ remoteURL: 'https://github.com/org/repo.git', repoDir: dir });
    assert.equal(res.forge, 'github');
    assert.equal(res.source, 'remote');
  } finally {
    cleanup(dir);
  }
});

// ---------------------------------------------------------------------------
// SSH and HTTPS equivalence for the 4 known hosts
// ---------------------------------------------------------------------------

const knownHosts = [
  { forge: 'github',    https: 'https://github.com/org/repo.git',    ssh: 'git@github.com:org/repo.git' },
  { forge: 'gitlab',    https: 'https://gitlab.com/org/repo.git',    ssh: 'git@gitlab.com:org/repo.git' },
  { forge: 'bitbucket', https: 'https://bitbucket.org/org/repo.git', ssh: 'git@bitbucket.org:org/repo.git' },
  { forge: 'azure',     https: 'https://dev.azure.com/org/project/_git/repo', ssh: 'git@ssh.dev.azure.com:v3/org/project/repo' },
];

for (const tc of knownHosts) {
  test(`SSH and HTTPS resolve equally for ${tc.forge} (https)`, () => {
    const res = resolve({ remoteURL: tc.https });
    assert.equal(res.forge, tc.forge);
    assert.equal(res.source, 'remote');
  });

  test(`SSH and HTTPS resolve equally for ${tc.forge} (ssh)`, () => {
    const res = resolve({ remoteURL: tc.ssh });
    assert.equal(res.forge, tc.forge);
    assert.equal(res.source, 'remote');
  });
}

// ---------------------------------------------------------------------------
// Azure-specific: dev.azure.com and *.visualstudio.com
// ---------------------------------------------------------------------------

test('dev.azure.com resolves to azure', () => {
  const res = resolve({ remoteURL: 'https://dev.azure.com/myorg/_git/myrepo' });
  assert.equal(res.forge, 'azure');
});

test('foo.visualstudio.com resolves to azure', () => {
  const res = resolve({ remoteURL: 'https://foo.visualstudio.com/DefaultCollection/_git/repo' });
  assert.equal(res.forge, 'azure');
});

test('ssh.dev.azure.com resolves to azure', () => {
  const res = resolve({ remoteURL: 'ssh://git@ssh.dev.azure.com/v3/org/project/repo' });
  assert.equal(res.forge, 'azure');
});

// ---------------------------------------------------------------------------
// Self-hosted + CI desempate
// ---------------------------------------------------------------------------

test('self-hosted + .gitlab-ci.yml → gitlab (source: ci)', () => {
  const dir = tmpDir();
  touch(dir, '.gitlab-ci.yml');
  try {
    const res = resolve({ remoteURL: 'https://git.empresa.com.br/org/repo.git', repoDir: dir });
    assert.equal(res.forge, 'gitlab');
    assert.equal(res.source, 'ci');
  } finally {
    cleanup(dir);
  }
});

test('self-hosted + .github/workflows/ → github (source: ci)', () => {
  const dir = tmpDir();
  mkdir(dir, '.github', 'workflows');
  try {
    const res = resolve({ remoteURL: 'https://git.empresa.com.br/org/repo.git', repoDir: dir });
    assert.equal(res.forge, 'github');
    assert.equal(res.source, 'ci');
  } finally {
    cleanup(dir);
  }
});

test('self-hosted + no CI → manual (source: none), no error', () => {
  const dir = tmpDir();
  try {
    const res = resolve({ remoteURL: 'https://git.empresa.com.br/org/repo.git', repoDir: dir });
    assert.equal(res.forge, 'manual');
    assert.equal(res.source, 'none');
  } finally {
    cleanup(dir);
  }
});

// ---------------------------------------------------------------------------
// No remote (new repo)
// ---------------------------------------------------------------------------

test('no remote + CI decides', () => {
  const dir = tmpDir();
  touch(dir, '.gitlab-ci.yml');
  try {
    const res = resolve({ remoteURL: '', repoDir: dir });
    assert.equal(res.forge, 'gitlab');
    assert.equal(res.source, 'ci');
  } finally {
    cleanup(dir);
  }
});

test('no remote + no CI → manual', () => {
  const dir = tmpDir();
  try {
    const res = resolve({ remoteURL: '', repoDir: dir });
    assert.equal(res.forge, 'manual');
    assert.equal(res.source, 'none');
  } finally {
    cleanup(dir);
  }
});

// ---------------------------------------------------------------------------
// Manual is not an error
// ---------------------------------------------------------------------------

test('all empty inputs → manual, no throw', () => {
  const res = resolve({});
  assert.equal(res.forge, 'manual');
  assert.equal(res.source, 'none');
});

// ---------------------------------------------------------------------------
// Invalid forge value → error with list of valid values
// ---------------------------------------------------------------------------

test('invalid flagForge throws with valid values listed', () => {
  assert.throws(
    () => resolve({ flagForge: 'notaforge' }),
    (err) => {
      assert.ok(err.message.includes('notaforge'));
      for (const v of VALID_FORGES) {
        assert.ok(err.message.includes(v), `error should mention "${v}"`);
      }
      return true;
    }
  );
});

test('invalid configForge throws', () => {
  assert.throws(
    () => resolve({ configForge: 'svn' }),
    (err) => {
      assert.ok(err.message.includes('svn'));
      return true;
    }
  );
});

// ---------------------------------------------------------------------------
// extractHost edge cases
// ---------------------------------------------------------------------------

test('extractHost: empty string returns empty', () => {
  assert.equal(extractHost(''), '');
});

test('extractHost: HTTPS with credentials', () => {
  assert.equal(extractHost('https://user:pass@github.com/org/repo.git'), 'github.com');
});

test('extractHost: SSH long form', () => {
  assert.equal(extractHost('ssh://git@github.com/org/repo.git'), 'github.com');
  assert.equal(extractHost('ssh://git@gitlab.com/org/repo.git'), 'gitlab.com');
  assert.equal(extractHost('ssh://git@bitbucket.org/org/repo.git'), 'bitbucket.org');
});
