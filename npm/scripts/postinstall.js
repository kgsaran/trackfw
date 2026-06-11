#!/usr/bin/env node

'use strict';

const https = require('https');
const fs = require('fs');
const path = require('path');
const os = require('os');
const child_process = require('child_process');
const zlib = require('zlib');

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
  console.warn(
    'trackfw: plataforma não suportada, pulando instalação do binário'
  );
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

// Garantir que o diretório bin existe
if (!fs.existsSync(binDir)) {
  fs.mkdirSync(binDir, { recursive: true });
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Baixa uma URL para um arquivo local, seguindo redirects 301/302.
 * @param {string} url
 * @param {string} destFile
 * @returns {Promise<void>}
 */
function download(url, destFile) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destFile);

    function get(currentUrl) {
      https
        .get(currentUrl, (res) => {
          if (res.statusCode === 301 || res.statusCode === 302) {
            // Seguir redirect
            const location = res.headers['location'];
            if (!location) {
              reject(new Error('Redirect sem Location header'));
              return;
            }
            res.resume(); // descartar body
            get(location);
            return;
          }

          if (res.statusCode !== 200) {
            reject(
              new Error(
                `Falha ao baixar ${currentUrl}: HTTP ${res.statusCode}`
              )
            );
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

/**
 * Extrai um arquivo .tar.gz ou .zip para um diretório.
 * @param {string} archiveFile caminho do arquivo baixado
 * @param {string} destDir diretório de destino da extração
 */
function extract(archiveFile, destDir) {
  if (isWindows) {
    child_process.execSync(
      `powershell -command "Expand-Archive -Path '${archiveFile}' -DestinationPath '${destDir}' -Force"`,
      { stdio: 'inherit' }
    );
  } else {
    child_process.execSync(
      `tar -xzf "${archiveFile}" -C "${destDir}"`,
      { stdio: 'inherit' }
    );
  }
}

/**
 * Remove um arquivo ou diretório silenciosamente.
 * @param {string} target
 */
function cleanup(target) {
  try {
    if (!fs.existsSync(target)) return;
    const stat = fs.statSync(target);
    if (stat.isDirectory()) {
      // Node 14 não tem rm({recursive}) confiável em todas as versões; usar rmdir
      fs.rmdirSync(target, { recursive: true });
    } else {
      fs.unlinkSync(target);
    }
  } catch (_) {
    // Limpeza é best-effort — não falhar por causa disso
  }
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

    console.log('trackfw: extraindo arquivo...');
    extract(tmpFile, tmpDir);

    // GoReleaser extrai o binário com o nome original ("trackfw" / "trackfw.exe")
    const extractedBinName = isWindows ? 'trackfw.exe' : 'trackfw';
    const extractedBin = path.join(tmpDir, extractedBinName);

    if (!fs.existsSync(extractedBin)) {
      throw new Error(
        `Binário não encontrado após extração: ${extractedBin}`
      );
    }

    fs.renameSync(extractedBin, binDest);

    if (!isWindows) {
      fs.chmodSync(binDest, 0o755);
    }

    console.log('trackfw: binário instalado com sucesso');
  } finally {
    cleanup(tmpFile);
    cleanup(tmpDir);
  }
}

main().catch((err) => {
  console.error('trackfw: erro ao instalar binário:', err.message);
  // Sair com 0 para não bloquear CIs que não precisam do binário pré-instalado
  process.exit(0);
});
