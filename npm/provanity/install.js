"use strict";

const fs = require("node:fs");
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

let target;
try {
  target = packageForPlatform();
} catch (error) {
  console.warn(`${error.message}; provanity binary will be unavailable on this host.`);
  process.exit(0);
}
let pkgJson;
try {
  pkgJson = require.resolve(`${target.name}/package.json`);
} catch {
  console.warn(`Optional package ${target.name} is not installed; provanity binary will be unavailable until platform packages are published or linked.`);
  process.exit(0);
}

const binary = path.join(path.dirname(pkgJson), "bin", target.bin);
if (!fs.existsSync(binary)) {
  console.warn(`Installed package ${target.name} but did not find ${binary}; provanity binary will be unavailable until packaging adds it.`);
}
