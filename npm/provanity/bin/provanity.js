#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");
const path = require("node:path");

function packageForPlatform() {
  const key = `${process.platform}-${process.arch}`;
  switch (key) {
    case "linux-x64":
      return { name: "@provanity/cli-linux-x64", bin: "provanity" };
    case "win32-x64":
      return { name: "@provanity/cli-win32-x64", bin: "provanity.exe" };
    default:
      throw new Error(`Unsupported platform: ${key}`);
  }
}

function resolveBinary() {
  const target = packageForPlatform();
  const pkgJson = require.resolve(`${target.name}/package.json`);
  return path.join(path.dirname(pkgJson), "bin", target.bin);
}

const binary = resolveBinary();
const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  throw result.error;
}
if (typeof result.status === "number") {
  process.exit(result.status);
}
process.exit(1);
