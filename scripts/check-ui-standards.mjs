#!/usr/bin/env node

import { readFile, readdir } from 'node:fs/promises';
import { dirname, extname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const sourceRoot = join(root, 'frontend', 'src');
const violations = [];

const walk = async (directory) => {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? walk(path) : path;
  }));
  return files.flat();
};

const lineNumber = (source, index) => source.slice(0, index).split('\n').length;
const reportMatches = (relativePath, source, pattern, message) => {
  for (const match of source.matchAll(pattern)) {
    violations.push(`${relativePath}:${lineNumber(source, match.index ?? 0)} ${message}`);
  }
};

const sourceFiles = (await walk(sourceRoot)).filter((path) => ['.css', '.ts', '.tsx'].includes(extname(path)));
for (const path of sourceFiles) {
  const relativePath = path.slice(root.length + 1);
  const source = await readFile(path, 'utf8');

  reportMatches(relativePath, source, /\bInter\b/g, 'uses the prohibited generic Inter font');
  reportMatches(relativePath, source, /\b(?:purple|violet|indigo)(?:-|\b)/gi, 'uses a prohibited default purple-family visual token');
  reportMatches(relativePath, source, /\b(?:window|globalThis)\.(?:alert|confirm)\s*\(/g, 'uses a native blocking dialog instead of shared feedback');
  reportMatches(relativePath, source, /(^|[^.\w])alert\s*\(/gm, 'uses a native blocking alert instead of shared feedback');
  reportMatches(relativePath, source, /<(?:div|span|tr|li)\b[^>]*\bonClick\s*=/g, 'uses a click handler on a non-interactive element');
  reportMatches(relativePath, source, /\bstyle\s*=/g, 'uses an inline style instead of a design token or reusable class');

  if (extname(path) === '.tsx') {
    reportMatches(relativePath, source, /<label\b(?![^>]*\bfor=)[^>]*>/g, 'has a label that is not associated with an input');
    reportMatches(relativePath, source, /<table\b(?![^>]*\bclass="[^"]*\bdata-table\b)[^>]*>/g, 'has a data table without the responsive data-table behavior');
  }

  if (relativePath.startsWith('frontend/src/pages/')) {
    reportMatches(relativePath, source, /\bfixed\s+inset-0\b/g, 'implements a page-local overlay instead of the accessible Dialog component');
  }
}

const css = await readFile(join(sourceRoot, 'index.css'), 'utf8');
const actionColor = css.match(/--action-primary:\s*(\d+)\s+(\d+)\s+(\d+)/);
if (!actionColor) {
  violations.push('frontend/src/index.css:1 is missing the --action-primary semantic token');
} else {
  const luminance = (values) => {
    const channels = values.map((value) => {
      const normalized = value / 255;
      return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
  };
  const foreground = luminance([255, 255, 255]);
  const background = luminance(actionColor.slice(1).map(Number));
  const contrast = (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05);
  if (contrast < 4.5) violations.push(`frontend/src/index.css:1 primary action contrast is ${contrast.toFixed(2)}:1; WCAG AA requires 4.5:1`);
}

const dialog = await readFile(join(sourceRoot, 'components', 'Dialog.tsx'), 'utf8');
for (const requirement of ['role="dialog"', 'aria-modal="true"', "event.key === 'Escape'", 'previouslyFocused?.focus()']) {
  if (!dialog.includes(requirement)) violations.push(`frontend/src/components/Dialog.tsx:1 accessible dialog requirement is missing: ${requirement}`);
}

if (violations.length > 0) {
  console.error('UI/UX quality gate failed:\n');
  for (const violation of violations) console.error(`- ${violation}`);
  process.exit(1);
}

console.log(`UI/UX quality gate passed (${sourceFiles.length} source files checked).`);
