'use strict';

const { test } = require('node:test');
const assert = require('node:assert/strict');
const { forgeAdapter } = require('../src/forge/adapter.js');

const availTrue  = () => true;
const availFalse = () => false;

/** Cria um spy que registra chamadas e retorna ret */
function makeSpy(calls, ret) {
  return (name) => { calls.push(name); return ret; };
}

// ---------------------------------------------------------------------------
// Nouns e CLIName
// ---------------------------------------------------------------------------

test('github: noun=Pull Request, cliName=gh', () => {
  const a = forgeAdapter('github', availTrue);
  assert.equal(a.noun, 'Pull Request');
  assert.equal(a.cliName, 'gh');
  assert.deepEqual(a.cliArgs, ['pr', 'create']);
});

test('gitlab: noun=Merge Request, cliName=glab', () => {
  const a = forgeAdapter('gitlab', availTrue);
  assert.equal(a.noun, 'Merge Request');
  assert.equal(a.cliName, 'glab');
  assert.deepEqual(a.cliArgs, ['mr', 'create']);
});

test('azure: noun=Pull Request, cliName=az', () => {
  const a = forgeAdapter('azure', availTrue);
  assert.equal(a.noun, 'Pull Request');
  assert.equal(a.cliName, 'az');
  assert.deepEqual(a.cliArgs, ['repos', 'pr', 'create']);
});

test('bitbucket: noun=Pull Request, sem cliName', () => {
  const a = forgeAdapter('bitbucket', availTrue);
  assert.equal(a.noun, 'Pull Request');
  assert.equal(a.cliName, '');
  assert.deepEqual(a.cliArgs, []);
});

// ---------------------------------------------------------------------------
// Disponibilidade via availFn injetada
// ---------------------------------------------------------------------------

test('github/gitlab/azure: available=true quando availFn retorna true', () => {
  for (const forge of ['github', 'gitlab', 'azure']) {
    const a = forgeAdapter(forge, availTrue);
    assert.equal(a.available, true, `${forge}: available deve ser true`);
  }
});

test('github/gitlab/azure: available=false quando availFn retorna false', () => {
  for (const forge of ['github', 'gitlab', 'azure']) {
    const a = forgeAdapter(forge, availFalse);
    assert.equal(a.available, false, `${forge}: available deve ser false`);
  }
});

// ---------------------------------------------------------------------------
// Bitbucket nunca chama availFn
// ---------------------------------------------------------------------------

test('bitbucket: nunca chama availFn', () => {
  const calls = [];
  const a = forgeAdapter('bitbucket', makeSpy(calls, true));
  assert.equal(a.available, false, 'bitbucket: available deve ser sempre false');
  assert.equal(calls.length, 0, `availFn não deve ser chamada; chamada com ${JSON.stringify(calls)}`);
});

test('github: chama availFn com "gh"', () => {
  const calls = [];
  forgeAdapter('github', makeSpy(calls, true));
  assert.deepEqual(calls, ['gh']);
});

test('gitlab: chama availFn com "glab"', () => {
  const calls = [];
  forgeAdapter('gitlab', makeSpy(calls, false));
  assert.deepEqual(calls, ['glab']);
});

test('azure: chama availFn com "az"', () => {
  const calls = [];
  forgeAdapter('azure', makeSpy(calls, false));
  assert.deepEqual(calls, ['az']);
});

// ---------------------------------------------------------------------------
// Forge desconhecido
// ---------------------------------------------------------------------------

test('forge desconhecido: available=false, cliName vazio', () => {
  const a = forgeAdapter('unknown-forge', availTrue);
  assert.equal(a.available, false);
  assert.equal(a.cliName, '');
});

// ---------------------------------------------------------------------------
// fallbackURL — casos principais
// ---------------------------------------------------------------------------

const BRANCH = 'feat/my-feature';

const fallbackCases = [
  // GitHub — HTTPS
  ['github', 'https://github.com/org/repo.git',
    'https://github.com/org/repo/compare/feat/my-feature?expand=1'],
  // GitHub — SSH
  ['github', 'git@github.com:org/repo.git',
    'https://github.com/org/repo/compare/feat/my-feature?expand=1'],
  // GitHub — self-hosted
  ['github', 'https://git.company.com/org/repo.git',
    'https://git.company.com/org/repo/compare/feat/my-feature?expand=1'],
  // GitLab — HTTPS
  ['gitlab', 'https://gitlab.com/org/repo.git',
    'https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature'],
  // GitLab — SSH
  ['gitlab', 'git@gitlab.com:org/repo.git',
    'https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature'],
  // GitLab — self-hosted
  ['gitlab', 'https://gitlab.company.com/org/repo.git',
    'https://gitlab.company.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature'],
  // Bitbucket — HTTPS
  ['bitbucket', 'https://bitbucket.org/org/repo.git',
    'https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature'],
  // Bitbucket — SSH
  ['bitbucket', 'git@bitbucket.org:org/repo.git',
    'https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature'],
  // Azure — HTTPS
  ['azure', 'https://dev.azure.com/org/project/_git/repo',
    'https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature'],
  // Azure — SSH (normalização)
  ['azure', 'git@ssh.dev.azure.com:v3/org/project/repo',
    'https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature'],
  // Azure — self-hosted
  ['azure', 'https://azdo.company.com/org/project/_git/repo',
    'https://azdo.company.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature'],
];

for (const [forge, remoteURL, want] of fallbackCases) {
  test(`fallbackURL forge=${forge} remote=${remoteURL}`, () => {
    const a = forgeAdapter(forge, availFalse);
    const got = a.fallbackURL(remoteURL, BRANCH);
    assert.equal(got, want);
  });
}

// ---------------------------------------------------------------------------
// fallbackURL — casos de borda
// ---------------------------------------------------------------------------

test('fallbackURL: remote vazio retorna ""', () => {
  const a = forgeAdapter('github', availFalse);
  assert.equal(a.fallbackURL('', 'main'), '');
});

test('fallbackURL: forge desconhecido retorna ""', () => {
  const a = forgeAdapter('unknown', availFalse);
  assert.equal(a.fallbackURL('https://example.com/org/repo.git', 'main'), '');
});
