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
}
