import 'package:cliparr/screens/clips_screen.dart';
import 'package:cliparr/services.dart';
import 'package:cliparr/state/session.dart';
import 'package:cliparr/state/settings.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'helpers.dart';

const _tv = {
  'id': 'tv',
  'name': 'Living Room TV',
  'client': 'Jellyfin Android TV',
};

/// Shows the clips page with one clip (start 1:05). [devices] and [resumeMs]
/// describe what Jellyfin reports; every request lands in [requests].
Future<List<String>> pumpWithClip(
  WidgetTester tester, {
  List<Map<String, String>> devices = const [_tv],
  int resumeMs = 0,
}) async {
  final requests = <String>[];
  SharedPreferences.setMockInitialValues({'server_url': 'http://server:8080'});
  final session = await SessionController.restore(
    httpClient: MockClient((r) async {
      requests.add(
        '${r.method} ${r.url.path}${r.body.isEmpty ? '' : ' ${r.body}'}',
      );
      return switch ((r.method, r.url.path)) {
        ('GET', '/api/me') => jsonResponse({'id': 'u1', 'name': 'alice'}),
        ('GET', '/api/clips') => jsonResponse([
          {
            'id': 1,
            'itemId': 'movie-rocky',
            'title': 'Rocky (1976)',
            'startMs': 65000,
            'endMs': 125000,
            'quote': 'Yo Adrian!',
            'note': '',
            'tags': <String>[],
          },
        ]),
        ('GET', '/api/clips/1/jellyfin') => jsonResponse({
          'webUrl':
              'https://jf.example.com/web/index.html#!/details?id=movie-rocky',
          'startMs': 65000,
          'resumeMs': resumeMs,
          'devices': devices,
        }),
        ('POST', _) => http.Response('', 204),
        _ => jsonResponse({'error': 'unexpected'}, 404),
      };
    }),
  );
  await tester.pumpWidget(
    MaterialApp(
      home: Services(
        settings: Settings(await SharedPreferences.getInstance()),
        updates: checkerWith(const []),
        installer: FakeInstaller(),
        child: ClipsScreen(session: session),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return requests;
}

Future<void> openSheet(WidgetTester tester) async {
  await tester.tap(find.text('Open in Jellyfin'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('each card offers to play the clip and to open it in Jellyfin', (
    tester,
  ) async {
    await pumpWithClip(tester);

    expect(find.text('Play clip'), findsOneWidget);
    expect(find.text('Open in Jellyfin'), findsOneWidget);
  });

  testWidgets(
    'playing on a Jellyfin device starts the clip at its start time',
    (tester) async {
      final requests = await pumpWithClip(tester);
      await openSheet(tester);

      expect(find.text('Open in Jellyfin at 1:05'), findsOneWidget);
      expect(find.text('Living Room TV'), findsOneWidget);

      await tester.tap(find.text('Living Room TV'));
      await tester.pumpAndSettle();

      expect(
        requests,
        contains('POST /api/clips/1/jellyfin/play {"deviceId":"tv"}'),
      );
      expect(find.text('Playing on Living Room TV from 1:05'), findsOneWidget);
    },
  );

  testWidgets('explains what to do when no Jellyfin app is running', (
    tester,
  ) async {
    await pumpWithClip(tester, devices: const []);
    await openSheet(tester);

    expect(find.textContaining('No Jellyfin app'), findsOneWidget);
    expect(find.text('Open in Jellyfin web'), findsOneWidget); // still possible
  });

  testWidgets(
    'web link: asks before replacing the resume point, then sets it',
    (tester) async {
      final requests = await pumpWithClip(tester, resumeMs: 2_432_000);
      await openSheet(tester);

      await tester.tap(find.text('Open in Jellyfin web'));
      await tester.pumpAndSettle();
      expect(
        find.textContaining('current resume point (40:32) will be replaced'),
        findsOneWidget,
      );
      expect(
        requests.where((r) => r.contains('/resume')),
        isEmpty,
        reason: 'nothing happens before confirming',
      );

      await tester.tap(find.text('Set resume point'));
      await tester.pumpAndSettle();

      expect(requests, contains('POST /api/clips/1/jellyfin/resume'));
      expect(find.text('Resume point set'), findsOneWidget);
      expect(find.text('Open Jellyfin'), findsOneWidget);
    },
  );

  testWidgets('cancelling the web link changes nothing', (tester) async {
    final requests = await pumpWithClip(tester);
    await openSheet(tester);

    await tester.tap(find.text('Open in Jellyfin web'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Cancel'));
    await tester.pumpAndSettle();

    expect(requests.where((r) => r.contains('/resume')), isEmpty);
  });
}
