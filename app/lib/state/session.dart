import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

import '../api/api_client.dart';
import '../api/models.dart';

const _serverKey = 'server_url';
const _tokenKey = 'token';

/// Holds the server URL and login state, and persists them between runs.
///
/// On the web the app is served by the Cliparr server, so the server URL is
/// the page origin and login rides on a cookie. Native apps let the user pick
/// the server and keep a bearer token.
class SessionController extends ChangeNotifier {
  SessionController._(this._prefs, this._httpClient);

  /// Restores a previous session, if any.
  static Future<SessionController> restore({
    SharedPreferences? prefs,
    http.Client? httpClient,
  }) async {
    final controller = SessionController._(
      prefs ?? await SharedPreferences.getInstance(),
      httpClient,
    );
    await controller._restore();
    return controller;
  }

  final SharedPreferences _prefs;
  final http.Client? _httpClient;
  ApiClient? _api;
  User? _user;

  /// Whether the user still has to tell the app where the server is.
  bool get needsServer => _api == null;
  bool get loggedIn => _user != null;
  User? get user => _user;
  Uri? get serverUrl => _api?.baseUrl;

  /// The API client. Only valid when [needsServer] is false.
  ApiClient get api => _api!;

  /// Verifies and stores the server URL (native apps only).
  Future<void> setServer(String input) async {
    final uri = _parseServerUrl(input);
    final candidate = ApiClient(baseUrl: uri, client: _httpClient);
    await candidate.serverVersion();
    await _prefs.setString(_serverKey, uri.toString());
    await _prefs.remove(_tokenKey);
    _api = candidate;
    _user = null;
    notifyListeners();
  }

  /// Forgets the server so the user can pick another one.
  Future<void> forgetServer() async {
    await _prefs.remove(_serverKey);
    await _prefs.remove(_tokenKey);
    _api = null;
    _user = null;
    notifyListeners();
  }

  Future<void> login(String username, String password) async {
    final result = await api.login(username, password);
    if (!kIsWeb) {
      api.token = result.token;
      await _prefs.setString(_tokenKey, result.token);
    }
    _user = result.user;
    notifyListeners();
  }

  Future<void> logout() async {
    try {
      await api.logout();
    } on ApiException {
      // The session may already be gone on the server; log out locally anyway.
    }
    await _clearLogin();
  }

  /// Runs [call] and logs the user out locally if the server rejects the session.
  Future<T> run<T>(Future<T> Function(ApiClient api) call) async {
    try {
      return await call(api);
    } on ApiException catch (e) {
      if (e.isUnauthorized) await _clearLogin();
      rethrow;
    }
  }

  Future<void> _restore() async {
    if (kIsWeb) {
      _api = ApiClient(baseUrl: _origin(Uri.base), client: _httpClient);
    } else {
      final saved = _prefs.getString(_serverKey);
      if (saved == null) return;
      _api = ApiClient(
        baseUrl: Uri.parse(saved),
        token: _prefs.getString(_tokenKey),
        client: _httpClient,
      );
    }
    try {
      _user = await _api!.me();
    } on ApiException {
      // Not logged in (or server unreachable): the login screen takes over.
    }
  }

  Future<void> _clearLogin() async {
    await _prefs.remove(_tokenKey);
    _api?.token = null;
    _user = null;
    notifyListeners();
  }
}

/// Turns user input like `192.168.1.5:8080` into a normalized server URL.
Uri _parseServerUrl(String input) {
  var text = input.trim();
  if (text.isEmpty) throw const ApiException(0, 'Enter the server address.');
  if (!text.contains('://')) text = 'http://$text';
  final uri = Uri.tryParse(text);
  if (uri == null || uri.host.isEmpty) {
    throw const ApiException(0, 'That does not look like a valid address.');
  }
  return Uri(
    scheme: uri.scheme,
    host: uri.host,
    port: uri.hasPort ? uri.port : null,
    path: uri.path,
  );
}

/// The scheme, host and port of [uri] with no path, query or fragment.
Uri _origin(Uri uri) => Uri(
  scheme: uri.scheme,
  host: uri.host,
  port: uri.hasPort ? uri.port : null,
  path: '/',
);
