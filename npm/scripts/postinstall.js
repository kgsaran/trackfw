#!/usr/bin/env node

'use strict';

const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const child_process = require('child_process');

// ---------------------------------------------------------------------------
// Mapeamento de plataforma e arquitetura
// ---------------------------------------------------------------------------

const PLATFORM_MAP = {
  linux: 'linux',
  darwin: 'darwin',
  win32: 'windows',
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64',
};

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];

if (!platform || !arch) {
  console.warn('trackfw: plataforma não suportada, pulando instalação do binário');
  process.exit(0);
}

// ---------------------------------------------------------------------------
// Versão — lida do package.json do wrapper npm
// ---------------------------------------------------------------------------

const pkgPath = path.join(__dirname, '..', 'package.json');
const { version } = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

// ---------------------------------------------------------------------------
// URL de download
// ---------------------------------------------------------------------------

const isWindows = platform === 'windows';
const ext = isWindows ? '.zip' : '.tar.gz';
const archiveName = `trackfw_${version}_${platform}_${arch}${ext}`;
const downloadUrl = `https://github.com/kgsaran/trackfw/releases/download/v${version}/${archiveName}`;

// ---------------------------------------------------------------------------
// Destino final do binário
// ---------------------------------------------------------------------------

const binDir = path.join(__dirname, '..', 'bin');
const binName = isWindows ? 'trackfw-bin.exe' : 'trackfw-bin';
const binDest = path.join(binDir, binName);

if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function download(url, destFile) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destFile);

    function get(currentUrl) {
      https
        .get(currentUrl, (res) => {
          if (res.statusCode === 301 || res.statusCode === 302) {
            const location = res.headers['location'];
            if (!location) {
              reject(new Error('Redirect sem Location header'));
              return;
            }
            res.resume();
            // reabrir o arquivo para a requisição seguinte não acumular lixo
            file.close(() => {
              const file2 = fs.createWriteStream(destFile);
              file2.on('finish', () => file2.close(resolve));
              file2.on('error', (err) => { fs.unlink(destFile, () => {}); reject(err); });
              https.get(location, (res2) => {
                if (res2.statusCode !== 200) {
                  reject(new Error(`Falha ao baixar ${location}: HTTP ${res2.statusCode}`));
                  return;
                }
                res2.pipe(file2);
              }).on('error', (err) => { fs.unlink(destFile, () => {}); reject(err); });
            });
            return;
          }

          if (res.statusCode !== 200) {
            reject(new Error(`Falha ao baixar ${currentUrl}: HTTP ${res.statusCode}`));
            return;
          }

          res.pipe(file);
          file.on('finish', () => file.close(resolve));
          file.on('error', (err) => {
            fs.unlink(destFile, () => {});
            reject(err);
          });
        })
        .on('error', (err) => {
          fs.unlink(destFile, () => {});
          reject(err);
        });
    }

    get(url);
  });
}

function extract(archiveFile, destDir) {
  if (isWindows) {
    child_process.execSync(
      `powershell -NoProfile -Command "Expand-Archive -LiteralPath '${archiveFile}' -DestinationPath '${destDir}' -Force"`,
      { stdio: 'pipe' }
    );
  } else {
    child_process.execSync(
      `tar -xzf "${archiveFile}" -C "${destDir}"`,
      { stdio: 'pipe' }
    );
  }
}

// Busca recursiva pelo binário em qualquer subdiretório após extração
function findBinary(dir, name) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
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
  try {
    if (!fs.existsSync(target)) return;
    const stat = fs.statSync(target);
    if (stat.isDirectory()) {
      fs.rmSync(target, { recursive: true, force: true });
    } else {
      fs.unlinkSync(target);
    }
  } catch (_) {}
}

// ---------------------------------------------------------------------------
// Função principal
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
      throw new Error(`Arquivo baixado suspeito (${fileSize} bytes) — verifique a conexão ou se a versão v${version} foi publicada no GitHub`);
    }

    console.log('trackfw: extraindo arquivo...');
    extract(tmpFile, tmpDir);

    const extractedBinName = isWindows ? 'trackfw.exe' : 'trackfw';
    const extractedBin = findBinary(tmpDir, extractedBinName);

    if (!extractedBin) {
      const files = [];
      function listAll(d) {
        for (const e of fs.readdirSync(d, { withFileTypes: true })) {
          const p = path.join(d, e.name);
          files.push(p);
          if (e.isDirectory()) listAll(p);
        }
      }
      listAll(tmpDir);
      throw new Error(
        `Binário "${extractedBinName}" não encontrado após extração.\nArquivos encontrados:\n${files.join('\n')}`
      );
    }

    fs.renameSync(extractedBin, binDest);

    if (!isWindows) {
      fs.chmodSync(binDest, 0o755);
    }

    console.log('trackfw: binário instalado com sucesso em ' + binDest);
  } finally {
    cleanup(tmpFile);
    cleanup(tmpDir);
  }
}

main().catch((err) => {
  console.error('\ntrackfw: ERRO ao instalar binário:');
  console.error('  ' + err.message);
  console.error('\nAlternativas de instalação:');
  console.error('  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh');
  console.error('  brew install kgsaran/tap/trackfw  (macOS/Linux)');
  console.error('  go install github.com/kgsaran/trackfw/cmd/trackfw@latest\n');
  // Sair com 0 para não bloquear npm install em CIs sem acesso ao GitHub
  process.exit(0);
});
