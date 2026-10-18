'use strict';

// Guards the promises the extension makes about what it may touch. A change
// that widens its access has to change these tests too, so it gets noticed.

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.join(__dirname, '..');
const manifest = JSON.parse(fs.readFileSync(path.join(root, 'manifest.base.json'), 'utf8'));

test('it asks for no access to every website', () => {
  assert.equal(JSON.stringify(manifest).includes('<all_urls>'), false);
  assert.equal(manifest.host_permissions, undefined, 'hosts are granted by the user at runtime, not at install');
  assert.equal(manifest.content_scripts, undefined, 'the script is injected only into the addresses the user entered');
});

test('it uses only the permissions it needs', () => {
  assert.deepEqual([...manifest.permissions].sort(), ['scripting', 'storage']);
});

test('pages it may ask to run on are only http and https ones', () => {
  for (const pattern of manifest.optional_host_permissions) {
    assert.match(pattern, /^https?:\/\/\*\/\*$/, `${pattern} is broader than the web`);
  }
});

test('every file the manifest points to exists', () => {
  const files = [...Object.values(manifest.icons), manifest.options_ui.page];
  for (const file of files) {
    assert.ok(fs.existsSync(path.join(root, file)), `${file} is missing`);
  }
});
