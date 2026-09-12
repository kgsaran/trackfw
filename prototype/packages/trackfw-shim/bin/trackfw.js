#!/usr/bin/env node
"use strict";

// trackfw shim — resolves the platform binary at runtime.
// Design: NO postinstall. Binary is located here, at require() time.
// Prior art: esbuild (https://github.com/evanw/esbuild/blob/main/pkg/api/api_impl.go)

const path = require("path");
const { spawnSync } = require("child_process");
const os = require("os");

function getPlatformPackage() {
  const platform = os.platform();   // 'linux', 'darwin', 'win32'
  const arch = os.arch();           // 'x64', 'arm64'
  // Dynamic: package name is constructed from platform+arch.
  // If the platform package is not installed, resolveBinaryPath handles
  // the error with a clear message. No hardcoded allowlist needed.
  return `@trackfw-bin/${platform}-${arch}`;
}

function resolveBinaryPath(pkg) {
  // Resolve the platform package from node_modules relative to this shim.
  // This works with or without postinstall, under --ignore-scripts.
  let pkgPath;
  try {
    pkgPath = require.resolve(`${pkg}/package.json`);
  } catch (e) {
    const platform = os.platform();
    const arch = os.arch();
    process.stderr.write(
      `trackfw: platform package for ${platform}/${arch} (${pkg}) is not installed.\n` +
      `This usually means the optional dependency was skipped during install.\n` +
      `Try: npm install (without --ignore-scripts) or install ${pkg} directly.\n`
    );
    process.exit(1);
  }

  const pkgDir = path.dirname(pkgPath);
  const pkgJson = require(pkgPath);

  // Extract bin path from platform package.json
  const binEntry = pkgJson.bin;
  if (!binEntry) {
    process.stderr.write(`trackfw: platform package ${pkg} has no bin entry.\n`);
    process.exit(1);
  }
  const binRelPath = typeof binEntry === "string" ? binEntry : binEntry.trackfw;
  return path.join(pkgDir, binRelPath);
}

const pkg = getPlatformPackage();
const binPath = resolveBinaryPath(pkg);

// Pass all argv through to the binary.
const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  env: process.env,
});

if (result.error) {
  process.stderr.write(`trackfw: failed to run binary: ${result.error.message}\n`);
  process.exit(1);
}

process.exit(result.status ?? 1);
