'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const lib = require('../lib.js');

test('normalizeBaseUrl accepts what people type', () => {
  assert.equal(lib.normalizeBaseUrl('192.168.1.5:8080'), 'http://192.168.1.5:8080');
  assert.equal(lib.normalizeBaseUrl('  https://cliparr.example.com/ '), 'https://cliparr.example.com');
  assert.equal(lib.normalizeBaseUrl('https://example.com/jellyfin/'), 'https://example.com/jellyfin');
  assert.equal(lib.normalizeBaseUrl('http://host:8096/?x=1#y'), 'http://host:8096');
});

test('normalizeBaseUrl rejects things that are not http(s) addresses', () => {
  const bad = [
    '',
    '   ',
    null,
    undefined,
    'ftp://host',
    'javascript:alert(1)',
    'http://',
    'not an address', // Chrome would encode the spaces instead of failing
    'http://exa mple.com',
    'http://%20',
    'http://user@',
  ];
  for (const text of bad) {
    assert.equal(lib.normalizeBaseUrl(text), null, String(text));
  }
});

test('normalizeBaseUrl accepts host names, IPs and docker-style names', () => {
  assert.equal(lib.normalizeBaseUrl('jellyfin'), 'http://jellyfin');
  assert.equal(lib.normalizeBaseUrl('my_service:8096'), 'http://my_service:8096');
  assert.equal(lib.normalizeBaseUrl('[::1]:8096'), 'http://[::1]:8096');
  assert.equal(lib.normalizeBaseUrl('HTTPS://Media.Example.com'), 'https://media.example.com');
});

test('permissionPattern covers the whole origin, ignoring the path', () => {
  assert.equal(lib.permissionPattern('https://example.com/jellyfin'), 'https://example.com/*');
  assert.equal(lib.permissionPattern('http://192.168.1.5:8096'), 'http://192.168.1.5:8096/*');
});

test('formatTime', () => {
  assert.equal(lib.formatTime(0), '0:00');
  assert.equal(lib.formatTime(65000), '1:05');
  assert.equal(lib.formatTime(3725000), '1:02:05');
});

const credentials = JSON.stringify({
  Servers: [
    { Id: 'other', ManualAddress: 'http://elsewhere:8096', AccessToken: 'tok-other', UserId: 'u-other' },
    {
      Id: 'srv',
      ManualAddress: 'http://jellyfin.lan:8096/',
      LocalAddress: 'http://192.168.1.5:8096',
      AccessToken: 'tok',
      UserId: 'u1',
    },
  ],
});

test('findCredentials picks the server of the current page', () => {
  const found = lib.findCredentials(credentials, 'http://192.168.1.5:8096/web/index.html#!/video');
  assert.deepEqual(found, { token: 'tok', userId: 'u1', serverId: 'srv', baseUrl: 'http://192.168.1.5:8096' });
});

test('findCredentials falls back to the only logged-in server', () => {
  const single = JSON.stringify({ Servers: [{ Id: 's', AccessToken: 't', UserId: 'u' }] });
  const found = lib.findCredentials(single, 'https://media.example.com/jellyfin/web/index.html');
  assert.equal(found.token, 't');
  assert.equal(found.baseUrl, 'https://media.example.com/jellyfin');
});

test('findCredentials refuses to guess between several servers', () => {
  assert.equal(lib.findCredentials(credentials, 'http://unknown:1234/web/index.html'), null);
});

test('findCredentials copes with missing or broken storage', () => {
  for (const bad of [null, '', 'not json', '{}', '{"Servers":[{"Id":"x"}]}']) {
    assert.equal(lib.findCredentials(bad, 'http://a:1/'), null, String(bad));
  }
});

test('jellyfinBaseFromPage keeps a path prefix', () => {
  assert.equal(lib.jellyfinBaseFromPage('http://h:8096/web/index.html'), 'http://h:8096');
  assert.equal(lib.jellyfinBaseFromPage('https://h/jellyfin/web/'), 'https://h/jellyfin');
  assert.equal(lib.jellyfinBaseFromPage('http://h:8096/'), 'http://h:8096');
});

const playing = (extra) => ({ NowPlayingItem: { Id: 'movie' }, Client: 'Jellyfin Web', UserId: 'u1', ...extra });

test('pickSession prefers this browser tab by device id', () => {
  const sessions = [playing({ DeviceId: 'tv', Client: 'Android TV' }), playing({ DeviceId: 'mine' })];
  assert.equal(lib.pickSession(sessions, { deviceId: 'mine', userId: 'u1' }).DeviceId, 'mine');
});

test('pickSession falls back to the only web client that is playing', () => {
  const sessions = [playing({ DeviceId: 'a' }), { DeviceId: 'tv', Client: 'Android TV', NowPlayingItem: { Id: 'x' } }];
  assert.equal(lib.pickSession(sessions, { userId: 'u1' }).DeviceId, 'a');
});

test('pickSession will not guess between several web clients or idle sessions', () => {
  assert.equal(lib.pickSession([playing({ DeviceId: 'a' }), playing({ DeviceId: 'b' })], { userId: 'u1' }), null);
  assert.equal(lib.pickSession([{ DeviceId: 'idle', Client: 'Jellyfin Web' }], { deviceId: 'idle' }), null);
  assert.equal(lib.pickSession(undefined, {}), null);
});

const reported = (ms) => ({ PlayState: { PositionTicks: ms * 10000 } });

test('resolvePosition uses the exact video time when Jellyfin agrees', () => {
  const position = lib.resolvePosition({ videoMs: 80123.4, session: reported(75000) });
  assert.deepEqual(position, { ms: 80123, approximate: false });
});

test('resolvePosition trusts the report when the stream starts part-way in', () => {
  // A transcode started at 1:00:00: the video clock says 0:05, Jellyfin says 1:00:05.
  const position = lib.resolvePosition({ videoMs: 5000, session: reported(3_605_000) });
  assert.deepEqual(position, { ms: 3_605_000, approximate: true });
});

test('resolvePosition works with only one source', () => {
  assert.deepEqual(lib.resolvePosition({ videoMs: 42000, session: null }), { ms: 42000, approximate: false });
  assert.deepEqual(lib.resolvePosition({ videoMs: NaN, session: reported(9000) }), { ms: 9000, approximate: true });
  assert.equal(lib.resolvePosition({ videoMs: NaN, session: null }), null);
});

test('normalizeRange orders the two marks and rejects tiny clips', () => {
  assert.deepEqual(lib.normalizeRange(9000, 4000), { startMs: 4000, endMs: 9000 });
  assert.equal(lib.normalizeRange(5000, 5400), null);
});

test('mark: the first mark sets the start, the second completes the clip', () => {
  const first = lib.mark(null, 'movie', 10000);
  assert.deepEqual(first, { state: { itemId: 'movie', startMs: 10000 }, done: null });

  const second = lib.mark(first.state, 'movie', 25000);
  assert.deepEqual(second, { state: null, done: { itemId: 'movie', startMs: 10000, endMs: 25000 } });
});

test('mark: marking backwards still gives a valid clip', () => {
  const done = lib.mark({ itemId: 'movie', startMs: 30000 }, 'movie', 20000).done;
  assert.deepEqual(done, { itemId: 'movie', startMs: 20000, endMs: 30000 });
});

test('mark: a too-short clip keeps the start so the viewer can try again', () => {
  const result = lib.mark({ itemId: 'movie', startMs: 10000 }, 'movie', 10200);
  assert.equal(result.tooShort, true);
  assert.equal(result.done, null);
  assert.deepEqual(result.state, { itemId: 'movie', startMs: 10000 });
});

test('mark: switching to another movie starts over', () => {
  const result = lib.mark({ itemId: 'old', startMs: 10000 }, 'new', 50000);
  assert.deepEqual(result, { state: { itemId: 'new', startMs: 50000 }, done: null });
});

test('quickRange takes the last ten seconds, and never a negative start', () => {
  assert.deepEqual(lib.quickRange(60000), { startMs: 50000, endMs: 60000 });
  assert.deepEqual(lib.quickRange(3000), { startMs: 0, endMs: 3000 });
  assert.deepEqual(lib.quickRange(200), { startMs: 0, endMs: 1000 });
});

test('saveErrorMessage explains each failure', () => {
  assert.match(lib.saveErrorMessage(0), /cannot be reached/);
  assert.match(lib.saveErrorMessage(401), /log in to Jellyfin again/);
  assert.match(lib.saveErrorMessage(404), /does not let you see/);
  assert.equal(lib.saveErrorMessage(400, 'endMs is past the end of the video'), 'endMs is past the end of the video');
  assert.match(lib.saveErrorMessage(500), /error 500/);
});
