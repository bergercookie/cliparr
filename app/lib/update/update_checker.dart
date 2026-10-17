import 'dart:convert';

import 'package:http/http.dart' as http;

import '../version.dart';
import 'version.dart';

/// Which releases the app follows.
enum UpdateChannel {
  /// Tagged releases only.
  stable,

  /// Every build, including the nightly prereleases.
  nightly,
}

/// A published release that contains an Android APK.
class Release {
  const Release({
    required this.version,
    required this.apkName,
    required this.apkUrl,
    this.checksumsUrl,
  });

  final Version version;
  final String apkName;
  final Uri apkUrl;

  /// The release's `SHA256SUMS` file, used to verify the download.
  final Uri? checksumsUrl;
}

/// Looks up the newest release on GitHub.
class UpdateChecker {
  UpdateChecker({http.Client? client, this.repo = releaseRepo})
    : _http = client ?? http.Client();

  final http.Client _http;
  final String repo;

  /// The newest release for [channel], or null if there is none with an APK.
  /// Throws if GitHub cannot be reached.
  Future<Release?> latest(UpdateChannel channel) async {
    final response = await _http.get(
      Uri.https('api.github.com', '/repos/$repo/releases', {'per_page': '30'}),
      headers: {'Accept': 'application/vnd.github+json'},
    );
    if (response.statusCode != 200) {
      throw Exception('GitHub answered ${response.statusCode}.');
    }

    Release? best;
    for (final json in jsonDecode(utf8.decode(response.bodyBytes)) as List) {
      final release = _parse(json as Map<String, dynamic>, channel);
      if (release != null && (best == null || release.version > best.version)) {
        best = release;
      }
    }
    return best;
  }

  /// The expected SHA-256 of [release]'s APK, or null if the release has no checksum file.
  Future<String?> expectedChecksum(Release release) async {
    final url = release.checksumsUrl;
    if (url == null) return null;
    final response = await _http.get(url);
    if (response.statusCode != 200) return null;
    // Lines look like "<hash>  <file name>", as written by sha256sum.
    for (final line in const LineSplitter().convert(response.body)) {
      final parts = line.trim().split(RegExp(r'\s+'));
      if (parts.length == 2 &&
          parts[1].replaceFirst('*', '') == release.apkName) {
        return parts[0];
      }
    }
    return null;
  }

  Release? _parse(Map<String, dynamic> json, UpdateChannel channel) {
    if (json['draft'] == true) {
      return null;
    }
    if (channel == UpdateChannel.stable && json['prerelease'] == true) {
      return null;
    }
    final version = Version.tryParse(json['tag_name'] as String? ?? '');
    if (version == null) return null;

    final assets = (json['assets'] as List? ?? const [])
        .cast<Map<String, dynamic>>();
    Map<String, dynamic>? apk;
    Map<String, dynamic>? sums;
    for (final asset in assets) {
      final name = asset['name'] as String? ?? '';
      if (name.endsWith('.apk')) apk = asset;
      if (name == 'SHA256SUMS') sums = asset;
    }
    if (apk == null) return null;
    return Release(
      version: version,
      apkName: apk['name'] as String,
      apkUrl: Uri.parse(apk['browser_download_url'] as String),
      checksumsUrl: sums == null
          ? null
          : Uri.parse(sums['browser_download_url'] as String),
    );
  }
}
