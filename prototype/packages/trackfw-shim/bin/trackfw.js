#!/usr/bin/env node
"use strict";

// trackfw shim — resolves the platform binary at runtime.
// Design: NO postinstall. Binary is located here, at require() time.
// Prior art: esbuild (https://github.com/evanw/esbuild/blob/main/pkg/api/api_impl.go)
//
// Resolution strategy: platform packages carry NO `bin` field — that field would
// create a collision in node_modules/.bin/trackfw (npm install-order dependent,
// nondeterministic). Instead, the shim computes the binary subpath from os.platform()
// directly (same pattern as esbuild: `require.resolve(`${pkg}/${subpath}`)`), then
// joins with the already-known pkgDir. No package metadata consulted for the path.

const path = require("path");
const fs = require("fs");
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

  // Compute binary subpath from platform — platform packages carry no `bin` field
  // (avoids nondeterministic collision in node_modules/.bin/ from install order).
  const binaryName = os.platform() === "win32" ? "trackfw.exe" : "trackfw";
  const resolvedBin = path.join(pkgDir, "bin", binaryName);

  // Fail loudly and name the cause — AC7: no MODULE_NOT_FOUND surprises.
  if (!fs.existsSync(resolvedBin)) {
    process.stderr.write(
      `trackfw: platform binary not found in ${pkg}.\n` +
      `Expected: ${resolvedBin}\n` +
      `The platform package may be corrupted or missing the binary in bin/${binaryName}.\n`
    );
    process.exit(1);
  }

  return resolvedBin;
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
