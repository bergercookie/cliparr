'use strict';

// Content script: adds a bookmark button to Jellyfin's video player.
//
// It finds out what is playing by asking Jellyfin (with the login the Jellyfin
// page itself keeps in localStorage) and reads the exact time from the video.
// The bookmark is then sent to Cliparr by the background script.

(function () {
  if (globalThis.__cliparrBookmarkLoaded) return; // injected twice (page load and settings change)
  globalThis.__cliparrBookmarkLoaded = true;

  const api = globalThis.browser || globalThis.chrome;
  const lib = globalThis.CliparrLib;

  let pending = null; // the start of a clip that is waiting for its end
  let busy = false;
  let idleTimer = null;

  // --- the button ---------------------------------------------------------

  const host = document.createElement('div');
  host.id = 'cliparr-bookmark';
  host.style.cssText = 'position:fixed;right:16px;bottom:110px;z-index:2147483647;display:none;';
  const shadow = host.attachShadow({ mode: 'open' });
  shadow.innerHTML = `
    <style>
      :host { all: initial; }
      .wrap { display: flex; flex-direction: column; align-items: flex-end; gap: 8px; font: 14px system-ui, sans-serif; transition: opacity .3s; }
      .wrap.idle { opacity: 0; }
      button { display: flex; align-items: center; gap: 8px; padding: 8px 14px; border: 0; border-radius: 999px;
               background: #5b3fd1; color: #fff; font: inherit; cursor: pointer; box-shadow: 0 2px 8px rgba(0,0,0,.5); }
      button:hover { background: #6d52e0; }
      button.marking { background: #c0392b; }
      svg { width: 18px; height: 18px; }
      .toast { max-width: 320px; padding: 8px 12px; border-radius: 8px; background: rgba(20,20,20,.92); color: #fff; }
      .toast.error { background: #8e2a20; }
      .toast:empty { display: none; }
      .toast a, .toast button.link { color: #cbb8ff; background: none; padding: 0; box-shadow: none; text-decoration: underline; font-size: inherit; }
    </style>
    <div class="wrap">
      <div class="toast" role="status" aria-live="polite"></div>
      <button type="button" class="main" title="Bookmark this moment in Cliparr. Press B; Shift+click or Shift+B saves the last 10 seconds.">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path fill="currentColor" d="M6 2h12a2 2 0 0 1 2 2v18l-8-4-8 4V4a2 2 0 0 1 2-2z"/></svg>
        <span class="label">Bookmark</span>
      </button>
    </div>`;
  const wrap = shadow.querySelector('.wrap');
  const button = shadow.querySelector('button.main');
  const label = shadow.querySelector('.label');
  const toast = shadow.querySelector('.toast');
  document.documentElement.appendChild(host);

  function findVideo() {
    const video = document.querySelector('video');
    return video && video.readyState > 0 && !video.ended ? video : null;
  }

  function render() {
    host.style.display = findVideo() ? 'block' : 'none';
    button.classList.toggle('marking', pending !== null);
    label.textContent = pending ? 'Set end · from ' + lib.formatTime(pending.startMs) : 'Bookmark';
  }

  // Like Jellyfin's own controls, fade away when the mouse is still.
  function wake() {
    wrap.classList.remove('idle');
    clearTimeout(idleTimer);
    idleTimer = setTimeout(() => {
      if (!pending) wrap.classList.add('idle');
    }, 3500);
  }

  function say(text, options) {
    toast.className = 'toast' + (options && options.error ? ' error' : '');
    toast.replaceChildren(document.createTextNode(text));
    if (options && options.link) {
      const a = document.createElement('a');
      a.href = options.link.href;
      a.target = '_blank';
      a.rel = 'noopener';
      a.textContent = options.link.text;
      toast.append(' ', a);
    }
    if (options && options.optionsButton) {
      const b = document.createElement('button');
      b.className = 'link';
      b.textContent = 'Open options';
      b.addEventListener('click', () => api.runtime.sendMessage({ type: 'open-options' }));
      toast.append(' ', b);
    }
    wake();
    clearTimeout(say.timer);
    say.timer = setTimeout(() => toast.replaceChildren(), 8000);
  }

  // --- working out what is playing -------------------------------------------

  async function jellyfinGet(credentials, path) {
    // Both ways of presenting the token that Jellyfin understands.
    const headers = {
      Authorization: 'MediaBrowser Token="' + credentials.token + '"',
      'X-Emby-Token': credentials.token,
    };
    const response = await fetch(credentials.baseUrl + path, { headers: headers });
    if (!response.ok) throw new Error('Jellyfin answered ' + response.status + '.');
    return response.json();
  }

  // captureMoment returns the item that plays in this tab and the exact position.
  async function captureMoment() {
    const video = findVideo();
    if (!video) throw new Error('Start a video in Jellyfin first.');
    const credentials = lib.findCredentials(localStorage.getItem('jellyfin_credentials'), location.href);
    if (!credentials) throw new Error('Cannot find your Jellyfin login on this page. Log in to Jellyfin again.');

    const deviceId = localStorage.getItem('_deviceId2') || '';
    const ids = { deviceId: deviceId, userId: credentials.userId };
    let session = null;
    if (deviceId) {
      session = lib.pickSession(await jellyfinGet(credentials, '/Sessions?deviceId=' + encodeURIComponent(deviceId)), ids);
    }
    if (!session) session = lib.pickSession(await jellyfinGet(credentials, '/Sessions'), ids);
    if (!session) {
      throw new Error('Jellyfin has not reported this video yet. Let it play for a few seconds and try again.');
    }

    const position = lib.resolvePosition({ videoMs: video.currentTime * 1000, session: session });
    if (!position) throw new Error('Cannot read the playback position.');
    return {
      token: credentials.token,
      itemId: session.NowPlayingItem.Id,
      positionMs: position.ms,
      approximate: position.approximate,
    };
  }

  // --- saving ------------------------------------------------------------------

  async function save(moment, startMs, endMs) {
    const answer = await api.runtime.sendMessage({
      type: 'save',
      token: moment.token,
      itemId: moment.itemId,
      startMs: startMs,
      endMs: endMs,
    });
    if (answer && answer.ok) {
      const note = moment.approximate ? ' (position approximate: pause first for an exact one)' : '';
      say('Saved ' + lib.formatTime(startMs) + '–' + lib.formatTime(endMs) + note, {
        link: { href: answer.cliparrUrl, text: 'Open Cliparr' },
      });
    } else {
      say((answer && answer.error) || 'Could not save the bookmark.', {
        error: true,
        optionsButton: !!(answer && answer.notConfigured),
      });
    }
  }

  // activate handles a click or the B key; quick saves the last ten seconds in one step.
  async function activate(quick) {
    if (busy) return;
    busy = true;
    try {
      const moment = await captureMoment();
      if (quick) {
        const range = lib.quickRange(moment.positionMs);
        pending = null;
        await save(moment, range.startMs, range.endMs);
        return;
      }
      const step = lib.mark(pending, moment.itemId, moment.positionMs);
      pending = step.state;
      if (step.tooShort) {
        say('That is under a second. Play a little further, then click again.');
      } else if (step.done) {
        await save(moment, step.done.startMs, step.done.endMs);
      } else {
        say('Start set at ' + lib.formatTime(step.state.startMs) + '. Play to the end of the moment and click again. Esc cancels.');
      }
    } catch (e) {
      say(e.message || String(e), { error: true });
    } finally {
      busy = false;
      render();
    }
  }

  button.addEventListener('click', (event) => activate(event.shiftKey));

  document.addEventListener('keydown', (event) => {
    const target = event.target;
    const typing = target && (/^(input|textarea|select)$/i.test(target.tagName) || target.isContentEditable);
    if (typing || event.ctrlKey || event.altKey || event.metaKey) return;
    if (event.key === 'Escape' && pending) {
      pending = null;
      say('Cancelled.');
      render();
    } else if (event.key.toLowerCase() === 'b' && findVideo()) {
      activate(event.shiftKey);
    }
  });

  document.addEventListener('mousemove', wake, { passive: true });
  // Fullscreen shows only the fullscreen element, so move the button into it.
  document.addEventListener('fullscreenchange', () => (document.fullscreenElement || document.documentElement).appendChild(host));
  setInterval(render, 1000);
  render();
  wake();
})();
