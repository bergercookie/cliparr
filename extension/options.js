'use strict';

// Options page: where Jellyfin and Cliparr are. Saving asks the browser for
// permission to talk to exactly those two addresses and nothing else.

const api = globalThis.browser || globalThis.chrome;
const lib = globalThis.CliparrLib;

const form = document.getElementById('form');
const jellyfinInput = document.getElementById('jellyfin');
const cliparrInput = document.getElementById('cliparr');
const status = document.getElementById('status');

function show(text, isError) {
  status.textContent = text;
  status.className = isError ? 'error' : '';
}

// readAddresses returns both addresses normalized, or null after showing what is wrong.
function readAddresses() {
  const jellyfinUrl = lib.normalizeBaseUrl(jellyfinInput.value);
  const cliparrUrl = lib.normalizeBaseUrl(cliparrInput.value);
  if (!jellyfinUrl || !cliparrUrl) {
    show('Enter both addresses, for example http://192.168.1.10:8096.', true);
    return null;
  }
  return { jellyfinUrl: jellyfinUrl, cliparrUrl: cliparrUrl };
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const addresses = readAddresses();
  if (!addresses) return;

  // Must be the first await: the browser only allows the question right after a click.
  const origins = [lib.permissionPattern(addresses.jellyfinUrl), lib.permissionPattern(addresses.cliparrUrl)];
  const granted = await api.permissions.request({ origins: origins });
  if (!granted) {
    show('Without that permission the extension cannot work. Press Save again and allow it.', true);
    return;
  }
  await api.storage.local.set(addresses);
  await api.runtime.sendMessage({ type: 'apply-settings' });
  show('Saved. Checking that Cliparr answers...');
  const check = await checkCliparr(addresses.cliparrUrl);
  if (check.ok) {
    show('Saved. ' + check.text + ' Open a video in Jellyfin; the Bookmark button appears on the page.');
  } else {
    show('Saved, but it does not work yet. ' + check.text, true);
  }
});

// checkCliparr asks the Cliparr at an address who it is, and says how that went.
async function checkCliparr(cliparrUrl) {
  // A host that never answers should not leave the page waiting for ever.
  const timeout = new AbortController();
  const timer = setTimeout(() => timeout.abort(), 8000);
  let response;
  try {
    response = await fetch(cliparrUrl + '/api/info', { signal: timeout.signal });
  } catch {
    return { ok: false, text: lib.unreachableMessage(cliparrUrl) };
  } finally {
    clearTimeout(timer);
  }
  let info = null;
  try {
    info = await response.json();
  } catch {
    // Something other than Cliparr answered.
  }
  if (!info || info.name !== 'cliparr') {
    return { ok: false, text: 'Something answers at ' + cliparrUrl + ', but it is not Cliparr. Check the address and the port.' };
  }
  return { ok: true, text: 'Connected to Cliparr ' + info.version + '.' };
}

document.getElementById('test').addEventListener('click', async () => {
  const addresses = readAddresses();
  if (!addresses) return;
  // Without this permission the browser treats the request as a cross-origin one and refuses it.
  const pattern = lib.permissionPattern(addresses.cliparrUrl);
  if (!(await api.permissions.contains({ origins: [pattern] }))) {
    show('Press Save first: the browser has not yet allowed the extension to use that address.', true);
    return;
  }
  show('Testing...');
  const check = await checkCliparr(addresses.cliparrUrl);
  show(check.text, !check.ok);
});

api.storage.local.get(['jellyfinUrl', 'cliparrUrl']).then((stored) => {
  jellyfinInput.value = stored.jellyfinUrl || '';
  cliparrInput.value = stored.cliparrUrl || '';
});
