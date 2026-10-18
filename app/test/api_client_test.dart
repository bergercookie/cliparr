import 'dart:convert';

import 'package:cliparr/api/api_client.dart';
import 'package:cliparr/api/models.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

ApiClient clientWith(
  http.Response Function(http.Request) handler, {
  String? token,
}) => ApiClient(
  baseUrl: Uri.parse('http://server:8080'),
  token: token,
  client: MockClient((request) async => handler(request)),
);

http.Response json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

void main() {
  test('login returns the user and token', () async {
    final api = clientWith((r) {
      expect(r.url.path, '/api/auth/login');
      expect(jsonDecode(r.body), {'username': 'alice', 'password': 'pw'});
      return json({
        'token': 't',
        'user': {'id': 'u1', 'name': 'alice'},
      });
    });
    final result = await api.login('alice', 'pw');
    expect(result.token, 't');
    expect(result.user.name, 'alice');
  });

  test('requests carry the bearer token', () async {
    final api = clientWith((r) {
      expect(r.headers['Authorization'], 'Bearer secret');
      return json([]);
    }, token: 'secret');
    expect(await api.listClips(), isEmpty);
  });

  test('error responses become ApiException with the server message', () async {
    final api = clientWith((_) => json({'error': 'session expired'}, 401));
    expect(
      api.me(),
      throwsA(
        isA<ApiException>()
            .having((e) => e.isUnauthorized, 'isUnauthorized', true)
            .having((e) => e.message, 'message', 'session expired'),
      ),
    );
  });

  test('createClip sends the draft and parses the clip', () async {
    final api = clientWith((r) {
      expect(jsonDecode(r.body)['itemId'], 'movie-1');
      return json({
        'id': 7,
        'itemId': 'movie-1',
        'title': 'Rocky (1976)',
        'startMs': 1000,
        'endMs': 2000,
        'quote': 'Yo',
        'note': '',
        'tags': ['classic'],
      }, 201);
    });
    final clip = await api.createClip(
      const ClipDraft(itemId: 'movie-1', startMs: 1000, endMs: 2000),
    );
    expect(clip.id, 7);
    expect(clip.tags, ['classic']);
  });

  test('serverVersion rejects servers that are not Cliparr', () async {
    final api = clientWith(
      (_) => json({'name': 'something-else', 'version': '1'}),
    );
    expect(api.serverVersion(), throwsA(isA<ApiException>()));
  });

  test('stream URL keeps a sub-path base URL', () {
    final api = ApiClient(baseUrl: Uri.parse('https://home.lan/cliparr/'));
    expect(
      api.streamUrl('abc').toString(),
      'https://home.lan/cliparr/api/items/abc/stream',
    );
  });

  test('the separate sound is asked for from a given time', () {
    final api = clientWith((r) => json({}));
    expect(
      api.audioUrl('abc', 90000).toString(),
      'http://server:8080/api/items/abc/audio?start=90000',
    );
  });

  test('an item says whether the browser can decode its audio', () async {
    final api = clientWith((r) {
      expect(r.url.path, '/api/items/abc');
      return json({
        'id': 'abc',
        'title': 'Dolby Dreams (2021)',
        'type': 'Movie',
        'runtimeMs': 6000000,
        'audioPlayable': false,
      });
    });
    expect((await api.item('abc')).audioPlayable, isFalse);
  });

  test('a server that says nothing about audio gets the plain file', () {
    final item = Item.fromJson({
      'id': 'a',
      'title': 'A',
      'type': 'Movie',
      'runtimeMs': 1,
    });
    expect(item.audioPlayable, isTrue);
  });

  test('thumbnail address changes when the clip is edited', () {
    final api = ApiClient(baseUrl: Uri.parse('http://server:8080'));
    Clip clipAt(DateTime updatedAt) => Clip(
      id: 7,
      itemId: 'i',
      title: 't',
      startMs: 0,
      endMs: 1000,
      quote: '',
      note: '',
      tags: const [],
      updatedAt: updatedAt,
    );

    final before = api.thumbnailUrl(clipAt(DateTime.utc(2026, 1, 1)));
    final after = api.thumbnailUrl(clipAt(DateTime.utc(2026, 1, 2)));

    expect(before.path, '/api/clips/7/thumbnail');
    expect(before.queryParameters['v'], isNot(after.queryParameters['v']));
  });

  test('handoff, play and resume talk to the clip endpoints', () async {
    final seen = <String>[];
    final api = clientWith((r) {
      seen.add('${r.method} ${r.url.path} ${r.body}');
      if (r.method == 'GET') {
        return json({
          'webUrl': 'https://jf.example.com/web/index.html#!/details?id=1',
          'startMs': 65000,
          'resumeMs': 0,
          'devices': [
            {'id': 'tv', 'name': 'Living Room TV', 'client': 'Android TV'},
          ],
        });
      }
      return http.Response('', 204);
    });

    final handoff = await api.jellyfinHandoff(7);
    await api.playOnDevice(7, 'tv');
    await api.setResumePoint(7);

    expect(handoff.devices.single.name, 'Living Room TV');
    expect(handoff.startMs, 65000);
    expect(seen, [
      'GET /api/clips/7/jellyfin ',
      'POST /api/clips/7/jellyfin/play {"deviceId":"tv"}',
      'POST /api/clips/7/jellyfin/resume ',
    ]);
  });
}
