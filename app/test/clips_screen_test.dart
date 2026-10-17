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

Map<String, dynamic> clipJson(
  int id,
  String title,
  String quote,
  List<String> tags,
) => {
  'id': id,
  'itemId': 'item-$id',
  'title': title,
  'startMs': 65000,
  'endMs': 125000,
  'quote': quote,
  'note': '',
  'tags': tags,
};

/// Shows the clips screen for a logged-in user. [onClipsRequest] answers GET /api/clips.
Future<void> pumpClips(
  WidgetTester tester,
  http.Response Function(Uri url) onClipsRequest,
) async {
  SharedPreferences.setMockInitialValues({'server_url': 'http://server:8080'});
  final session = await SessionController.restore(
    httpClient: MockClient(
      (r) async => switch (r.url.path) {
        '/api/me' => jsonResponse({'id': 'u1', 'name': 'alice'}),
        '/api/clips' => onClipsRequest(r.url),
        _ => jsonResponse({'error': 'unexpected'}, 404),
      },
    ),
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
}

void main() {
  testWidgets('lists clips with quote, title, time range and tags', (
    tester,
  ) async {
    await pumpClips(
      tester,
      (_) => jsonResponse([
        clipJson(1, 'Rocky (1976)', 'Yo Adrian!', ['classic']),
        clipJson(2, 'Fake Show S01E01 - Pilot', 'Never give up', []),
      ]),
    );

    expect(find.text('"Yo Adrian!"'), findsOneWidget);
    expect(find.text('Rocky (1976)'), findsOneWidget);
    expect(find.text('1:05–2:05'), findsNWidgets(2)); // one badge per clip
    expect(find.text('classic'), findsOneWidget);
    expect(find.text('"Never give up"'), findsOneWidget);
  });

  testWidgets('invites the user to save a first clip', (tester) async {
    await pumpClips(tester, (_) => jsonResponse([]));

    expect(find.textContaining('No clips yet'), findsOneWidget);
  });

  testWidgets('searching asks the server for matching clips', (tester) async {
    final queries = <String?>[];
    await pumpClips(tester, (url) {
      queries.add(url.queryParameters['q']);
      return jsonResponse([]);
    });

    await tester.enterText(find.byType(SearchBar), 'adrian');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await tester.pumpAndSettle();

    expect(queries, [null, 'adrian']);
  });

  testWidgets('shows the server error instead of an empty list', (
    tester,
  ) async {
    await pumpClips(
      tester,
      (_) => jsonResponse({'error': 'internal error'}, 500),
    );

    expect(find.text('internal error'), findsOneWidget);
  });
}
