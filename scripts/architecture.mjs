import { spawnSync } from 'node:child_process';
import { existsSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../', import.meta.url));
const cli = process.env.ARCHIFY_CLI;
const mode = process.argv[2];
if (!['check', 'build', 'visual'].includes(mode) || !cli || !existsSync(cli)) {
  console.error('Usage: ARCHIFY_CLI=/path/to/archify/bin/archify.mjs node scripts/architecture.mjs check|build|visual');
  process.exit(1);
}
const spec = path.join(root, 'docs/architecture/public.architecture.json');
const html = path.join(root, 'docs/architecture/index.html');
const args = mode === 'visual'
  ? [cli, 'visual-check', html, '--json']
  : [cli, mode === 'build' ? 'deliver' : 'validate', 'architecture', spec,
    ...(mode === 'build' ? [html] : []), '--quality', 'showcase', '--repo-root', root, '--json'];
const result = spawnSync(process.execPath, args, { cwd: root, encoding: 'utf8' });
if (result.stdout) process.stdout.write(result.stdout);
if (result.stderr) process.stderr.write(result.stderr);
if (result.error) console.error(result.error.message);
if (mode === 'build' && result.status === 0) {
  const receipt = JSON.parse(result.stdout);
  // Keep content hashes and evidence, not executor-specific absolute paths.
  const { specification, artifact, validation, evidence } = receipt;
  writeFileSync(path.join(root, 'docs/architecture/validation.json'), JSON.stringify({
    generator: 'Archify 2.17', diagram_type: 'architecture',
    specification, artifact, validation, evidence,
    browser_evidence: 'pending', visual_review: 'pending',
  }, null, 2) + '\n');
}
process.exit(result.status ?? 1);
