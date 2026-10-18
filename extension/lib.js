// Logic shared by the extension's scripts. It has no browser dependencies, so
// it also runs under Node for the unit tests (extension/test/lib.test.js).
//
// Loaded as a classic script (`globalThis.CliparrLib`) in the browser and with
// require() in Node.
(function (root) {
  'use strict';

  // The last position Jellyfin was told about can lag by about this much.
  const REPORT_LAG_MS = 15000;
  const MIN_CLIP_MS = 1000;
  const QUICK_CLIP_MS = 10000;

  // normalizeBaseUrl turns what a person typed ("192.168.1.5:8080/") into
  // "http://192.168.1.5:8080", or null if it is not an http(s) address.
  function normalizeBaseUrl(input) {
    let text = String(input == null ? '' : input).trim();
    // Browsers differ on odd input (Chrome encodes a space in a host name where
    // Node rejects it), so refuse anything with whitespace before parsing.
    if (!text || /\s/.test(text)) return null;
    if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(text)) text = 'http://' + text;
    let url;
    try {
      url = new URL(text);
    } catch {
      return null;
    }
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
    // A host name, an IPv4 address or a bracketed IPv6 address; nothing else.
    if (!/^([a-z0-9._-]+|\[[0-9a-f:.]+\])$/i.test(url.hostname)) return null;
    return url.origin + url.pathname.replace(/\/+$/, '');
  }

  // permissionPattern is the host permission that covers a base URL.
  function permissionPattern(baseUrl) {
    return new URL(baseUrl).origin + '/*';
  }

  // formatTime shows milliseconds as m:ss or h:mm:ss.
  function formatTime(ms) {
    const total = Math.floor(ms / 1000);
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = String(total % 60).padStart(2, '0');
    return h > 0 ? h + ':' + String(m).padStart(2, '0') + ':' + s : m + ':' + s;
  }

  // jellyfinBaseFromPage guesses Jellyfin's address from the page address,
  // keeping a path prefix such as /jellyfin that precedes /web/.
  function jellyfinBaseFromPage(pageUrl) {
    const url = new URL(pageUrl);
    const prefix = url.pathname.split('/web/')[0].replace(/\/(web)?$/, '');
    return url.origin + (prefix === '/' ? '' : prefix);
  }

  // findCredentials reads the login that Jellyfin's web client keeps in
  // localStorage ("jellyfin_credentials") for the server the page belongs to.
  function findCredentials(storedJson, pageUrl) {
    let data;
    try {
      data = JSON.parse(storedJson);
    } catch {
      return null;
    }
    const servers = (data && Array.isArray(data.Servers) ? data.Servers : []).filter(
      (s) => s && s.AccessToken && s.UserId
    );
    const origin = new URL(pageUrl).origin;
    const addresses = (s) =>
      [s.ManualAddress, s.LocalAddress, s.RemoteAddress].map(normalizeBaseUrl).filter(Boolean);
    const sameOrigin = (a) => new URL(a).origin === origin;

    let server = servers.find((s) => addresses(s).some(sameOrigin));
    if (!server && servers.length === 1) server = servers[0];
    if (!server) return null;

    return {
      token: server.AccessToken,
      userId: server.UserId,
      serverId: server.Id,
      baseUrl: addresses(server).find(sameOrigin) || jellyfinBaseFromPage(pageUrl),
    };
  }

  // pickSession chooses which of the user's Jellyfin sessions is this browser
  // tab: the one with our device id, or else the only web client that is playing.
  function pickSession(sessions, ids) {
    const playing = (Array.isArray(sessions) ? sessions : []).filter(
      (s) => s && s.NowPlayingItem && s.NowPlayingItem.Id
    );
    if (ids.deviceId) {
      const own = playing.find((s) => s.DeviceId === ids.deviceId);
      if (own) return own;
    }
    const web = playing.filter(
      (s) => (!ids.userId || s.UserId === ids.userId) && /web/i.test(s.Client || '')
    );
    return web.length === 1 ? web[0] : null;
  }

  // resolvePosition decides the position to bookmark, in milliseconds.
  //
  // The video element knows the exact time, but for a transcoded stream that
  // started part-way in, its clock is relative to the start of the stream.
  // Jellyfin's last report is always absolute, but may lag. If the two agree
  // (within the lag) the stream starts at zero and the video time is exact;
  // otherwise the report is the best answer, flagged as approximate.
  function resolvePosition(input) {
    const ticks = input.session && input.session.PlayState && input.session.PlayState.PositionTicks;
    const reportedMs = Number.isFinite(ticks) ? Math.round(ticks / 10000) : null;
    const videoMs = Number.isFinite(input.videoMs) ? Math.round(input.videoMs) : null;

    if (videoMs !== null && (reportedMs === null || Math.abs(reportedMs - videoMs) <= REPORT_LAG_MS)) {
      return { ms: Math.max(0, videoMs), approximate: false };
    }
    if (reportedMs !== null) return { ms: Math.max(0, reportedMs), approximate: true };
    return null;
  }

  // normalizeRange orders two positions and rejects ranges that are too short.
  function normalizeRange(a, b) {
    const startMs = Math.min(a, b);
    const endMs = Math.max(a, b);
    return endMs - startMs >= MIN_CLIP_MS ? { startMs: startMs, endMs: endMs } : null;
  }

  // mark advances the "mark start, then mark end" flow. `state` is null or the
  // pending start; it returns the new state and, when the clip is complete, `done`.
  function mark(state, itemId, positionMs) {
    if (!state || state.itemId !== itemId) {
      return { state: { itemId: itemId, startMs: positionMs }, done: null };
    }
    const range = normalizeRange(state.startMs, positionMs);
    if (!range) return { state: state, done: null, tooShort: true };
    return { state: null, done: { itemId: itemId, startMs: range.startMs, endMs: range.endMs } };
  }

  // quickRange is the "save the last ten seconds" shortcut.
  function quickRange(positionMs) {
    const startMs = Math.max(0, positionMs - QUICK_CLIP_MS);
    return { startMs: startMs, endMs: Math.max(positionMs, startMs + MIN_CLIP_MS) };
  }

  // reachHelp says what to check when a request to Cliparr does not complete. A browser
  // reports a refused connection, a wrong name and a certificate it does not trust in the
  // same way (Firefox calls all of them "CORS request did not succeed"), and an extension
  // cannot show the certificate warning, so the text has to cover them all.
  function reachHelp(baseUrl) {
    const help = 'Check the address and that Cliparr is running.';
    if (!/^https:\/\//i.test(baseUrl || '')) return help;
    return (
      help +
      ' If it uses a certificate your browser does not trust (self-signed, or from your own certificate authority), ' +
      'open ' + baseUrl + ' in a browser tab and accept the certificate once: the extension cannot show the warning itself.'
    );
  }

  // unreachableMessage is what the options page says when Cliparr does not answer.
  function unreachableMessage(baseUrl) {
    return 'Cannot reach Cliparr at ' + baseUrl + '. ' + reachHelp(baseUrl);
  }

  // saveErrorMessage explains a failed save in words a viewer can act on.
  function saveErrorMessage(status, serverMessage, cliparrUrl) {
    if (status === 0) return 'Cliparr cannot be reached. ' + reachHelp(cliparrUrl);
    if (status === 401) return 'Jellyfin did not accept your login. Reload the page or log in to Jellyfin again.';
    if (status === 404) return 'Jellyfin does not let you see this item, so it cannot be bookmarked.';
    return serverMessage || 'Cliparr could not save the bookmark (error ' + status + ').';
  }

  const lib = {
    REPORT_LAG_MS: REPORT_LAG_MS,
    normalizeBaseUrl: normalizeBaseUrl,
    permissionPattern: permissionPattern,
    formatTime: formatTime,
    jellyfinBaseFromPage: jellyfinBaseFromPage,
    findCredentials: findCredentials,
    pickSession: pickSession,
    resolvePosition: resolvePosition,
    normalizeRange: normalizeRange,
    mark: mark,
    quickRange: quickRange,
    saveErrorMessage: saveErrorMessage,
    unreachableMessage: unreachableMessage,
  };

  if (typeof module !== 'undefined' && module.exports) module.exports = lib;
  root.CliparrLib = lib;
})(typeof globalThis !== 'undefined' ? globalThis : this);
