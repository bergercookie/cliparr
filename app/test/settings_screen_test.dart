import 'package:cliparr/screens/settings_screen.dart';
import 'package:cliparr/services.dart';
import 'package:cliparr/state/session.dart';
import 'package:cliparr/state/settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'helpers.dart';

/// Shows the settings screen for an app running [currentVersion] against a
/// server 2.0.0, with the given GitHub releases.
Future<FakeInstaller> pumpSettings(
  WidgetTester tester, {
  required String currentVersion,
  required List<Map<String, dynamic>> releases,
  Map<String, Object> prefs = const {},
  String sums = 'abc123  cliparr-1.4.0.apk\n',
}) async {
  SharedPreferences.setMockInitialValues({
    'server_url': 'http://server:8080',
    ...prefs,
  });
  final session = await SessionController.restore(
    httpClient: MockClient(
      (r) async => r.url.path == '/api/info'
          ? jsonResponse({'name': 'cliparr', 'version': '2.0.0'})
          : jsonResponse({'error': 'not logged in'}, 401),
    ),
  );
  final installer = FakeInstaller();
  await tester.pumpWidget(
    MaterialApp(
      home: Services(
        settings: Settings(
          await SharedPreferences.getInstance(),
          currentVersion: currentVersion,
        ),
        updates: checkerWith(releases, sums: sums),
        installer: installer,
        currentVersion: currentVersion,
        child: SettingsScreen(session: session),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return installer;
}

void main() {
  testWidgets('shows the server address and both versions', (tester) async {
    await pumpSettings(
      tester,
      currentVersion: '1.2.0',
      releases: [githubRelease('v1.2.0')],
    );

    expect(find.text('http://server:8080'), findsOneWidget);
    expect(find.text('2.0.0'), findsOneWidget); // server version
    expect(find.text('1.2.0'), findsOneWidget); // app version
  });

  testWidgets('says so when the app is up to date', (tester) async {
    await pumpSettings(
      tester,
      currentVersion: '1.2.0',
      releases: [githubRelease('v1.2.0')],
    );

    expect(find.text('You are running the latest version'), findsOneWidget);
    expect(find.textContaining('Download and install'), findsNothing);
  });

  testWidgets('offers to install a newer release, verified by its checksum', (
    tester,
  ) async {
    final installer = await pumpSettings(
      tester,
      currentVersion: '1.2.0',
      releases: [githubRelease('v1.4.0'), githubRelease('v1.2.0')],
    );

    expect(find.text('Update available: 1.4.0'), findsOneWidget);
    await tester.tap(find.text('Download and install 1.4.0'));
    await tester.pumpAndSettle();

    expect(
      installer.installedUrl.toString(),
      'https://example.com/v1.4.0/cliparr-1.4.0.apk',
    );
    expect(installer.installedSha256, 'abc123');
    expect(
      find.text('Handing over to the Android installer...'),
      findsOneWidget,
    );
  });

  testWidgets('stable users are not offered a nightly, nightly users are', (
    tester,
  ) async {
    final releases = [
      githubRelease('v1.3.3.post4', prerelease: true),
      githubRelease('v1.2.0'),
    ];
    await pumpSettings(tester, currentVersion: '1.2.0', releases: releases);
    expect(find.text('You are running the latest version'), findsOneWidget);

    await tester.tap(find.text('Nightly'));
    await tester.pumpAndSettle();
    expect(find.text('Update available: 1.3.3.post4'), findsOneWidget);
  });

  testWidgets('remembers the chosen channel', (tester) async {
    await pumpSettings(
      tester,
      currentVersion: '1.2.0',
      releases: [githubRelease('v1.2.0')],
    );
    await tester.tap(find.text('Nightly'));
    await tester.pumpAndSettle();

    expect(
      (await SharedPreferences.getInstance()).getString('update_channel'),
      'nightly',
    );
  });

  testWidgets('nightly builds follow the nightly channel by default', (
    tester,
  ) async {
    await pumpSettings(
      tester,
      currentVersion: '1.3.3.post1',
      releases: [githubRelease('v1.3.3.post2', prerelease: true)],
    );
    expect(find.text('Update available: 1.3.3.post2'), findsOneWidget);
  });

  testWidgets('development builds do not claim to be outdated', (tester) async {
    await pumpSettings(
      tester,
      currentVersion: 'dev',
      releases: [githubRelease('v1.2.0')],
    );

    expect(find.text('Development build'), findsOneWidget);
    expect(find.textContaining('Download and install'), findsNothing);
  });

  testWidgets('explains when GitHub cannot be reached', (tester) async {
    await pumpSettings(tester, currentVersion: '1.2.0', releases: const []);
    // An empty list is a valid answer: no release has been published yet.
    expect(find.text('No release found for this channel yet'), findsOneWidget);
  });
}
