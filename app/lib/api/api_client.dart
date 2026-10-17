import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

/// An error response from the Cliparr server.
class ApiException implements Exception {
  const ApiException(this.status, this.message);

  final int status;
  final String message;

  bool get isUnauthorized => status == 401;

  @override
  String toString() => message;
}

/// Client for the Cliparr HTTP API.
///
/// Native apps authenticate with a bearer [token]. The web build is served by
/// the Cliparr server itself and relies on the session cookie, so its token
/// stays null.
class ApiClient {
  ApiClient({required this.baseUrl, this.token, http.Client? client})
    : _http = client ?? http.Client();

  final Uri baseUrl;
  String? token;
  final http.Client _http;

  /// Headers for requests that bypass this class, e.g. video and images.
  Map<String, String> get authHeaders =>
      token == null ? const {} : {'Authorization': 'Bearer $token'};

  Uri streamUrl(String itemId) => _url('/api/items/$itemId/stream');
  Uri imageUrl(String itemId) => _url('/api/items/$itemId/image');

  /// Returns the server version. Used to verify a server URL before login.
  Future<String> serverVersion() async {
    final json = await _send('GET', '/api/info') as Map<String, dynamic>;
    if (json['name'] != 'cliparr') {
      throw const ApiException(0, 'This URL is not a Cliparr server.');
    }
    return json['version'] as String;
  }

  Future<({User user, String token})> login(
    String username,
    String password,
  ) async {
    final json = await _send(
      'POST',
      '/api/auth/login',
      body: {'username': username, 'password': password},
    ) as Map<String, dynamic>;
    return (
      user: User.fromJson(json['user'] as Map<String, dynamic>),
      token: json['token'] as String,
    );
  }

  Future<void> logout() => _send('POST', '/api/auth/logout');

  Future<User> me() async =>
      User.fromJson(await _send('GET', '/api/me') as Map<String, dynamic>);

  Future<List<Item>> searchItems(String query) async {
    final json = await _send('GET', '/api/items', query: {'q': query});
    return (json as List<dynamic>)
        .map((e) => Item.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<List<Clip>> listClips({String query = '', String tag = ''}) async {
    final json = await _send(
      'GET',
      '/api/clips',
      query: {if (query.isNotEmpty) 'q': query, if (tag.isNotEmpty) 'tag': tag},
    );
    return (json as List<dynamic>)
        .map((e) => Clip.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Clip> createClip(ClipDraft draft) async => Clip.fromJson(
    await _send('POST', '/api/clips', body: draft.toJson())
        as Map<String, dynamic>,
  );

  Future<Clip> updateClip(int id, ClipDraft draft) async => Clip.fromJson(
    await _send('PUT', '/api/clips/$id', body: draft.toJson())
        as Map<String, dynamic>,
  );

  Future<void> deleteClip(int id) => _send('DELETE', '/api/clips/$id');

  Uri _url(String path, [Map<String, String>? query]) {
    final base = baseUrl.path.endsWith('/')
        ? baseUrl.path.substring(0, baseUrl.path.length - 1)
        : baseUrl.path;
    return baseUrl.replace(
      path: '$base$path',
      queryParameters: query == null || query.isEmpty ? null : query,
    );
  }

  Future<Object?> _send(
    String method,
    String path, {
    Map<String, String>? query,
    Map<String, dynamic>? body,
  }) async {
    final request = http.Request(method, _url(path, query))
      ..headers.addAll(authHeaders);
    if (body != null) {
      request.headers['Content-Type'] = 'application/json';
      request.body = jsonEncode(body);
    }

    final http.Response response;
    try {
      response = await http.Response.fromStream(await _http.send(request));
    } on Exception {
      throw const ApiException(0, 'Could not reach the Cliparr server.');
    }

    if (response.statusCode >= 400) {
      throw ApiException(response.statusCode, _errorMessage(response));
    }
    if (response.body.isEmpty) return null;
    return jsonDecode(utf8.decode(response.bodyBytes));
  }

  String _errorMessage(http.Response response) {
    try {
      final json = jsonDecode(utf8.decode(response.bodyBytes));
      if (json is Map<String, dynamic> && json['error'] is String) {
        return json['error'] as String;
      }
    } on FormatException {
      // Not a JSON error body; fall through to the generic message.
    }
    return 'Request failed (${response.statusCode}).';
  }
}
