// ESLint settings for the extension, run by `just lint-extension`.
//
// There is deliberately no `require('globals')` or `@eslint/js` here: the
// extension has no npm dependencies, so this file lists what it needs itself
// and `just lint-extension` fetches a pinned ESLint with npx.

// What the extension's scripts may use from the browser.
const browser = {
  AbortController: 'readonly',
  AbortSignal: 'readonly',
  CustomEvent: 'readonly',
  console: 'readonly',
  document: 'readonly',
  fetch: 'readonly',
  importScripts: 'readonly', // service workers only
  localStorage: 'readonly',
  location: 'readonly',
  setInterval: 'readonly',
  clearInterval: 'readonly',
  setTimeout: 'readonly',
  clearTimeout: 'readonly',
  URL: 'readonly',
};

// What the unit tests and this file may use from Node.
const node = {
  console: 'readonly',
  module: 'writable',
  process: 'readonly',
  require: 'readonly',
};

module.exports = [
  { ignores: ['eslint.config.cjs'] },
  {
    files: ['*.js'],
    languageOptions: { ecmaVersion: 2022, sourceType: 'script', globals: { ...browser, ...node } },
    linterOptions: { reportUnusedDisableDirectives: 'error' },
    rules: {
      // Mistakes: undefined names, dead code, accidental globals.
      'no-undef': 'error',
      'no-unused-vars': 'error',
      'no-redeclare': 'error',
      'no-shadow-restricted-names': 'error',
      'no-dupe-keys': 'error',
      'no-dupe-else-if': 'error',
      'no-unreachable': 'error',
      'no-self-assign': 'error',
      'no-constant-condition': 'error',
      'no-empty': ['error', { allowEmptyCatch: false }],
      'no-fallthrough': 'error',
      'no-useless-escape': 'error',
      // Style that prevents bugs.
      eqeqeq: ['error', 'always', { null: 'ignore' }], // `x == null` means null or undefined
      'no-var': 'error',
      'prefer-const': 'error',
      'no-throw-literal': 'error',
      'no-loss-of-precision': 'error',
      // The extension runs inside people's Jellyfin pages: never run strings as code.
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
      'no-script-url': 'error',
    },
  },
  {
    files: ['test/*.js'],
    languageOptions: { ecmaVersion: 2022, sourceType: 'script', globals: node },
  },
];
