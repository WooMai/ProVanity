import fs from "node:fs";
import path from "node:path";

const packageGroups = {
  cli: [
    {
      file: "npm/provanity/package.json",
      update(pkg, version) {
        pkg.optionalDependencies = {
          ...pkg.optionalDependencies,
          "@provanity/cli-linux-x64": version,
          "@provanity/cli-win32-x64": version,
        };
      },
    },
    { file: "npm/cli-linux-x64/package.json" },
    { file: "npm/cli-win32-x64/package.json" },
  ],
  worker: [{ file: "npm/worker-linux-x64/package.json" }],
};

packageGroups.all = [...packageGroups.cli, ...packageGroups.worker];

const [firstArg, secondArg] = process.argv.slice(2);
const groupName = secondArg ? firstArg : "all";
const versionInput = secondArg ?? firstArg;
const version = versionInput?.replace(/^v/, "");

if (!version) {
  throw new Error("usage: node scripts/sync-npm-version.mjs [cli|worker|all] <version>");
}

if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(version)) {
  throw new Error(`invalid npm version: ${versionInput}`);
}

const packages = packageGroups[groupName];
if (!packages) {
  throw new Error(`unknown package group: ${groupName}`);
}

for (const entry of packages) {
  const file = entry.file;
  const fullPath = path.resolve(file);
  const pkg = JSON.parse(fs.readFileSync(fullPath, "utf8"));
  const before = JSON.stringify(pkg);
  pkg.version = version;
  entry.update?.(pkg, version);
  if (JSON.stringify(pkg) !== before) {
    fs.writeFileSync(fullPath, `${JSON.stringify(pkg, null, 2)}\n`);
    console.log(`${file}: ${version}`);
  } else {
    console.log(`${file}: already ${version}`);
  }
}
