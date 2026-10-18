'use strict';

// Background script: keeps the settings, adds the bookmark button to the
// user's Jellyfin pages (and only those), and sends bookmarks to Cliparr.
// It runs as a service worker in Chrome and as an event page in Firefox.

if (typeof importScripts === 'function') importScripts('lib.js'); // Firefox loads lib.js from the manifest

const api = globalThis.browser || globalThis.chrome;
const lib = globalThis.CliparrLib;
const SCRIPT_ID = 'cliparr-jellyfin';

async function getSettings() {
  const stored = await api.storage.local.get(['cliparrUrl', 'jellyfinUrl']);
  return { cliparrUrl: stored.cliparrUrl || '', jellyfinUrl: stored.jellyfinUrl || '' };
}

// registerButton makes the browser inject the bookmark button into the pages
// of the configured Jellyfin server, and into tabs that are already open.
async function registerButton() {
  try {
    await api.scripting.unregisterContentScripts({ ids: [SCRIPT_ID] });
  } catch {
    // Nothing was registered yet.
  }
  const { jellyfinUrl } = await getSettings();
  if (!jellyfinUrl) return;

  const pattern = lib.permissionPattern(jellyfinUrl);
  if (!(await api.permissions.contains({ origins: [pattern] }))) return;

  await api.scripting.registerContentScripts([
    {
      id: SCRIPT_ID,
      matches: [pattern],
      js: ['lib.js', 'content.js'],
      runAt: 'document_idle',
      persistAcrossSessions: true,
    },
  ]);
  for (const tab of await api.tabs.query({ url: pattern })) {
    api.scripting.executeScript({ target: { tabId: tab.id }, files: ['lib.js', 'content.js'] }).catch(() => {});
  }
}

// save sends one bookmark to Cliparr, logged in with the Jellyfin token that
// the content script found in the Jellyfin page.
async function save(message, sender) {
  const { cliparrUrl, jellyfinUrl } = await getSettings();
  if (!cliparrUrl || !jellyfinUrl) {
    return { ok: false, notConfigured: true, error: 'Set your Cliparr address in the extension options first.' };
  }
  // Only pages of the configured Jellyfin server may use the token this way.
  if (!sender.url || new URL(sender.url).origin !== new URL(jellyfinUrl).origin) {
    return { ok: false, error: 'This page is not the Jellyfin server set in the extension options.' };
  }

  let response;
  try {
    response = await fetch(cliparrUrl + '/api/clips', {
      method: 'POST',
      headers: { Authorization: 'Jellyfin ' + message.token, 'Content-Type': 'application/json' },
      body: JSON.stringify({ itemId: message.itemId, startMs: message.startMs, endMs: message.endMs }),
    });
  } catch {
    return { ok: false, error: lib.saveErrorMessage(0) };
  }
  let body = null;
  try {
    body = await response.json();
  } catch {
    // Not a JSON answer; the status is enough.
  }
  if (!response.ok) return { ok: false, error: lib.saveErrorMessage(response.status, body && body.error) };
  return { ok: true, cliparrUrl: cliparrUrl };
}

api.runtime.onMessage.addListener((message, sender, sendResponse) => {
  const handlers = {
    save: () => save(message, sender),
    'apply-settings': () => registerButton().then(() => ({ ok: true })),
    'open-options': () => api.runtime.openOptionsPage().then(() => ({ ok: true })),
  };
  const handler = handlers[message && message.type];
  if (!handler) return false;
  handler().then(sendResponse, (e) => sendResponse({ ok: false, error: String(e) }));
  return true; // the answer comes later
});

api.action.onClicked.addListener(() => api.runtime.openOptionsPage());
api.runtime.onInstalled.addListener(() => registerButton());
api.runtime.onStartup.addListener(() => registerButton());
