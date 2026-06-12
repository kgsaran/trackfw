#!/usr/bin/env node

'use strict';

const https = require('https');
const http = require('http');
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
// URL de download
// TRACKFW_BINARY_URL permite override para redes corporativas / mirrors internos.
// Exemplo: TRACKFW_BINARY_URL=https://mirror.empresa.com/trackfw/v1.0.3/trackfw_1.0.3_windows_amd64.tar.gz
// ---------------------------------------------------------------------------

const isWindows = process.platform === 'win32';
const archiveName = `trackfw_${version}_${platform}_${arch}.tar.gz`;
const defaultUrl = `https://github.com/kgsaran/trackfw/releases/download/v${version}/${archiveName}`;
const downloadUrl = process.env.TRACKFW_BINARY_URL || defaultUrl;

// ---------------------------------------------------------------------------
// Destino
// ---------------------------------------------------------------------------

const binDir = path.join(__dirname, '..', 'bin');
const binName = isWindows ? 'trackfw-bin.exe' : 'trackfw-bin';
const binDest = path.join(binDir, binName);

if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// ---------------------------------------------------------------------------
// Download com suporte a redirect e http/https
// ---------------------------------------------------------------------------

function download(url, destFile) {
  return new Promise((resolve, reject) => {
    function get(currentUrl, attempt) {
      if (attempt > 5) { reject(new Error('Muitos redirects')); return; }
      const client = currentUrl.startsWith('https') ? https : http;
      client.get(currentUrl, (res) => {
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
// Instruções de instalação manual (exibidas em caso de falha)
// ---------------------------------------------------------------------------

function printManualInstructions() {
  const releaseUrl = `https://github.com/kgsaran/trackfw/releases/download/v${version}/${archiveName}`;
  console.error('\n─── Instalação manual ──────────────────────────────────────────');
  console.error(`1. Baixe o binário: ${releaseUrl}`);
  console.error(`2. Extraia e copie o executável para:`);
  console.error(`     ${binDest}`);
  if (isWindows) {
    console.error('\nOu use o install script via WSL/Git Bash:');
    console.error('  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh');
  }
  console.error('\nRede corporativa? Defina TRACKFW_BINARY_URL apontando para um mirror:');
  console.error(`  $env:TRACKFW_BINARY_URL="https://seu-mirror/${archiveName}"`);
  console.error(`  npm install -g trackfw`);
  console.error('────────────────────────────────────────────────────────────────\n');
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
      throw new Error(`Arquivo baixado inválido (${fileSize} bytes)`);
    }

    console.log('trackfw: extraindo arquivo...');
    // tar está disponível nativamente em Linux, macOS e Windows 10+
    child_process.execSync(`tar -xzf "${tmpFile}" -C "${tmpDir}"`, { stdio: 'pipe' });

    const extractedBinName = isWindows ? 'trackfw.exe' : 'trackfw';
    const extractedBin = findBinary(tmpDir, extractedBinName);

    if (!extractedBin) {
      throw new Error(`Binário "${extractedBinName}" não encontrado após extração`);
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
  console.error('\ntrackfw: ERRO ao instalar binário — ' + err.message);
  printManualInstructions();
  // Sai com 0 para não bloquear pipelines de CI que não precisam do binário
  process.exit(0);
});
