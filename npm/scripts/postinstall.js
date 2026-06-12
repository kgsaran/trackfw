#!/usr/bin/env node

'use strict';

const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const child_process = require('child_process');

// ---------------------------------------------------------------------------
// Plataforma e arquitetura
// ---------------------------------------------------------------------------

const PLATFORM_MAP = { linux: 'linux', darwin: 'darwin', win32: 'windows' };
const ARCH_MAP = { x64: 'amd64', arm64: 'arm64' };

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];

if (!platform || !arch) {
  console.warn('trackfw: plataforma não suportada, pulando instalação do binário');
  process.exit(0);
}

// ---------------------------------------------------------------------------
// Versão
// ---------------------------------------------------------------------------

const pkgPath = path.join(__dirname, '..', 'package.json');
const { version } = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

// ---------------------------------------------------------------------------
// URL — todos os sistemas recebem .tar.gz (Windows 10+ tem tar nativo)
// ---------------------------------------------------------------------------

const archiveName = `trackfw_${version}_${platform}_${arch}.tar.gz`;
const downloadUrl = `https://github.com/kgsaran/trackfw/releases/download/v${version}/${archiveName}`;

// ---------------------------------------------------------------------------
// Destino
// ---------------------------------------------------------------------------

const isWindows = process.platform === 'win32';
const binDir = path.join(__dirname, '..', 'bin');
const binName = isWindows ? 'trackfw-bin.exe' : 'trackfw-bin';
const binDest = path.join(binDir, binName);

if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// ---------------------------------------------------------------------------
// Download com suporte a redirect
// ---------------------------------------------------------------------------

function download(url, destFile) {
  return new Promise((resolve, reject) => {
    function get(currentUrl, attempt) {
      if (attempt > 5) { reject(new Error('Muitos redirects')); return; }
      https.get(currentUrl, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          res.resume();
          get(res.headers['location'], attempt + 1);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} ao baixar ${currentUrl}`));
          return;
        }
        const file = fs.createWriteStream(destFile);
        res.pipe(file);
        file.on('finish', () => file.close(resolve));
        file.on('error', (err) => { fs.unlink(destFile, () => {}); reject(err); });
      }).on('error', reject);
    }
    get(url, 0);
  });
}

// ---------------------------------------------------------------------------
// Busca recursiva pelo binário após extração
// ---------------------------------------------------------------------------

function findBinary(dir, name) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      const found = findBinary(full, name);
      if (found) return found;
    } else if (entry.name === name) {
      return full;
    }
  }
  return null;
}

function cleanup(target) {
  try { fs.rmSync(target, { recursive: true, force: true }); } catch (_) {}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-'));
  const tmpFile = path.join(tmpDir, archiveName);

  try {
    console.log(`trackfw: baixando binário para ${platform}/${arch} v${version}...`);
    console.log(`         ${downloadUrl}`);

    await download(downloadUrl, tmpFile);

    const fileSize = fs.statSync(tmpFile).size;
    if (fileSize < 1000) {
      throw new Error(`Arquivo baixado inválido (${fileSize} bytes) — release v${version} pode não ter sido publicado ainda`);
    }

    console.log('trackfw: extraindo arquivo...');
    // tar está disponível nativamente em Linux, macOS e Windows 10+
    child_process.execSync(`tar -xzf "${tmpFile}" -C "${tmpDir}"`, { stdio: 'pipe' });

    const extractedBinName = isWindows ? 'trackfw.exe' : 'trackfw';
    const extractedBin = findBinary(tmpDir, extractedBinName);

    if (!extractedBin) {
      const files = [];
      const walk = (d) => { for (const e of fs.readdirSync(d, { withFileTypes: true })) { const p = path.join(d, e.name); files.push(p); if (e.isDirectory()) walk(p); } };
      walk(tmpDir);
      throw new Error(`Binário "${extractedBinName}" não encontrado após extração.\nConteúdo extraído:\n${files.join('\n')}`);
    }

    fs.copyFileSync(extractedBin, binDest);
    if (!isWindows) fs.chmodSync(binDest, 0o755);

    console.log('trackfw: instalado com sucesso em ' + binDest);
  } finally {
    cleanup(tmpFile);
    cleanup(tmpDir);
  }
}

main().catch((err) => {
  console.error('\ntrackfw: ERRO ao instalar binário:');
  console.error('  ' + err.message);
  console.error('\nAlternativas:');
  console.error('  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh');
  console.error('  brew install kgsaran/tap/trackfw');
  console.error('  go install github.com/kgsaran/trackfw/cmd/trackfw@latest\n');
  process.exit(0);
});
