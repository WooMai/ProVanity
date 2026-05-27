import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "..");
const sourceReadme = path.join(repoRoot, "README.md");
const defaultPackageDirs = ["npm/provanity"];

if (!fs.existsSync(sourceReadme)) {
  throw new Error(`missing source README: ${sourceReadme}`);
}

const args = process.argv.slice(2);
const packageDirs =
  args.length > 0
    ? args.map((packageDir) => path.resolve(process.cwd(), packageDir))
    : defaultPackageDirs.map((packageDir) => path.join(repoRoot, packageDir));

for (const packageDir of packageDirs) {
  const stat = fs.statSync(packageDir, { throwIfNoEntry: false });
  if (!stat?.isDirectory()) {
    throw new Error(`npm package directory does not exist: ${packageDir}`);
  }

  const targetReadme = path.join(packageDir, "README.md");
  fs.copyFileSync(sourceReadme, targetReadme);
  console.log(`copied README.md to ${path.relative(repoRoot, targetReadme)}`);
}
