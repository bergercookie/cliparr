/// Full version of this build, e.g. `1.3.3` or `1.3.3.post2` for nightlies.
/// Release builds set it with `--dart-define=CLIPARR_VERSION=...`; local builds show `dev`.
const appVersion = String.fromEnvironment(
  'CLIPARR_VERSION',
  defaultValue: 'dev',
);

/// GitHub repository whose releases the app checks for updates.
const releaseRepo = String.fromEnvironment(
  'CLIPARR_REPO',
  defaultValue: 'bergercookie/cliparr',
);
