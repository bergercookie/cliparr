import 'dart:convert';

import 'package:cliparr/app.dart';
import 'package:cliparr/state/session.dart';
import 'package:cliparr/state/settings.dart';
import 'package:cliparr/update/update_checker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'helpers.dart';

http.Response _json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

void main() {
  testWidgets('native app asks for the server, then for the login', (
    tester,
  ) async {
    SharedPreferences.setMockInitialValues({});
    final session = await SessionController.restore(
      httpClient: MockClient(
        (r) async => r.url.path == '/api/info'
            ? _json({'name': 'cliparr', 'version': 'test'})
            : _json({'error': 'not logged in'}, 401),
      ),
    );

    await tester.pumpWidget(
      CliparrApp(
        session: session,
        settings: Settings(await SharedPreferences.getInstance()),
        updates: UpdateChecker(),
        installer: FakeInstaller(supported: false),
      ),
    );
    expect(find.text('Server address'), findsOneWidget);

    await tester.enterText(find.byType(TextField), '192.168.1.5:8080');
    await tester.tap(find.text('Connect'));
    await tester.pumpAndSettle();

    expect(find.text('Log in with your Jellyfin account.'), findsOneWidget);
    expect(session.serverUrl.toString(), 'http://192.168.1.5:8080');
  });
}
