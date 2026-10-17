import 'dart:convert';

import 'package:cliparr/update/apk_installer.dart';
import 'package:cliparr/update/update_checker.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

http.Response jsonResponse(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

/// A GitHub "list releases" entry.
Map<String, dynamic> githubRelease(
  String tag, {
  bool prerelease = false,
  bool draft = false,
  bool withApk = true,
}) => {
  'tag_name': tag,
  'prerelease': prerelease,
  'draft': draft,
  'assets': [
    if (withApk)
      {
        'name': 'cliparr-${tag.substring(1)}.apk',
        'browser_download_url':
            'https://example.com/$tag/cliparr-${tag.substring(1)}.apk',
      },
    {
      'name': 'SHA256SUMS',
      'browser_download_url': 'https://example.com/$tag/SHA256SUMS',
    },
  ],
};

/// An [UpdateChecker] that answers from [releases] instead of calling GitHub.
UpdateChecker checkerWith(
  List<Map<String, dynamic>> releases, {
  String sums = '',
}) => UpdateChecker(
  client: MockClient(
    (request) async => request.url.path.endsWith('SHA256SUMS')
        ? http.Response(sums, 200)
        : jsonResponse(releases),
  ),
);

/// Records what was installed instead of touching Android.
class FakeInstaller implements ApkInstaller {
  FakeInstaller({this.supported = true});

  @override
  final bool supported;
  Uri? installedUrl;
  String? installedSha256;

  @override
  Stream<InstallProgress> install(
    Uri url, {
    required String fileName,
    String? sha256,
  }) async* {
    installedUrl = url;
    installedSha256 = sha256;
    yield const InstallProgress.downloading(0.5);
    yield const InstallProgress.installing();
  }
}
