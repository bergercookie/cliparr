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
  show('Saved. Open a video in Jellyfin; the Bookmark button appears on the page.');
});

document.getElementById('test').addEventListener('click', async () => {
  const addresses = readAddresses();
  if (!addresses) return;
  show('Testing...');
  try {
    const response = await fetch(addresses.cliparrUrl + '/api/info');
    const info = await response.json();
    if (info.name !== 'cliparr') throw new Error('not Cliparr');
    show('Connected to Cliparr ' + info.version + '.');
  } catch {
    show('Cannot reach Cliparr at that address. If you have not saved yet, press Save first to grant access.', true);
  }
});

api.storage.local.get(['jellyfinUrl', 'cliparrUrl']).then((stored) => {
  jellyfinInput.value = stored.jellyfinUrl || '';
  cliparrInput.value = stored.cliparrUrl || '';
});
