import 'package:cliparr/update/update_checker.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'helpers.dart';

void main() {
  final releases = [
    githubRelease('v1.3.3.post2', prerelease: true),
    githubRelease('v1.2.0'),
    githubRelease('v1.3.3.post1', prerelease: true),
    githubRelease('v1.1.0'),
    githubRelease('v9.9.9', draft: true),
    githubRelease('v1.5.0', withApk: false),
    githubRelease('not-a-version'),
  ];

  test(
    'stable channel ignores prereleases, drafts and releases without an APK',
    () async {
      final latest = await checkerWith(releases).latest(UpdateChannel.stable);
      expect(latest!.version.toString(), '1.2.0');
      expect(latest.apkName, 'cliparr-1.2.0.apk');
      expect(
        latest.apkUrl.toString(),
        'https://example.com/v1.2.0/cliparr-1.2.0.apk',
      );
    },
  );

  test('nightly channel picks the newest build of any kind', () async {
    final latest = await checkerWith(releases).latest(UpdateChannel.nightly);
    expect(latest!.version.toString(), '1.3.3.post2');
  });

  test('returns null when there is nothing to install', () async {
    expect(await checkerWith([]).latest(UpdateChannel.stable), isNull);
  });

  test('throws when GitHub cannot be queried', () async {
    final checker = UpdateChecker(
      client: MockClient((_) async => http.Response('rate limited', 403)),
    );
    expect(checker.latest(UpdateChannel.stable), throwsException);
  });

  test('finds the checksum of the APK in SHA256SUMS', () async {
    final checker = checkerWith(
      [githubRelease('v1.2.0')],
      sums: 'aaa111  cliparr_1.2.0_amd64.deb\nbbb222  cliparr-1.2.0.apk\nccc333  other.AppImage\n',
    );
    final release = (await checker.latest(UpdateChannel.stable))!;
    expect(await checker.expectedChecksum(release), 'bbb222');
  });

  test('has no checksum when SHA256SUMS does not list the APK', () async {
    final checker = checkerWith([
      githubRelease('v1.2.0'),
    ], sums: 'aaa111  something-else\n');
    final release = (await checker.latest(UpdateChannel.stable))!;
    expect(await checker.expectedChecksum(release), isNull);
  });
}
