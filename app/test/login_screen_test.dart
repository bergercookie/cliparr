import 'package:cliparr/screens/login_screen.dart';
import 'package:cliparr/state/session.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'helpers.dart';

/// A Cliparr server that is only as much as the login screen talks to.
class FakeServer {
  FakeServer({this.passwordLogin, this.quickConnect});

  /// What /api/info says; null leaves the field out, like an older server.
  bool? passwordLogin;
  bool? quickConnect;

  /// How the server answers a request to start Quick Connect.
  http.Response Function() startAnswer = () =>
      jsonResponse({'id': 'abc', 'code': '123456', 'expiresInSeconds': 300});

  /// The answer to the n-th poll (counting from 1).
  http.Response Function(int n) pollAnswer = (n) => n < 3
      ? jsonResponse({'status': 'pending'})
      : jsonResponse({
          'status': 'approved',
          'token': 'cliparr-token',
          'user': {'id': 'u1', 'name': 'alice'},
        });

  int polls = 0;

  Future<http.Response> handle(http.Request r) async {
    switch ((r.method, r.url.path)) {
      case ('GET', '/api/info'):
        return jsonResponse({
          'name': 'cliparr',
          'version': 'test',
          'passwordLogin': ?passwordLogin,
          'quickConnect': ?quickConnect,
        });
      case ('POST', '/api/auth/quickconnect'):
        return startAnswer();
      case ('GET', '/api/auth/quickconnect/abc'):
        return pollAnswer(++polls);
    }
    return jsonResponse({'error': 'not logged in'}, 401);
  }
}

Future<SessionController> pumpLogin(
  WidgetTester tester,
  FakeServer server,
) async {
  SharedPreferences.setMockInitialValues({'server_url': 'http://server:8080'});
  final session = await SessionController.restore(
    httpClient: MockClient(server.handle),
  );
  await tester.pumpWidget(
    MaterialApp(
      home: LoginScreen(
        session: session,
        pollEvery: const Duration(milliseconds: 100),
      ),
    ),
  );
  await tester.pump(); // let the screen ask how people may log in
  return session;
}

Future<void> startQuickConnect(WidgetTester tester) async {
  await tester.tap(find.text('Log in with Quick Connect'));
  await tester.pump();
  await tester.pump();
}

void main() {
  testWidgets('offers Quick Connect next to the password when both are on', (
    tester,
  ) async {
    await pumpLogin(
      tester,
      FakeServer(passwordLogin: true, quickConnect: true),
    );

    expect(find.byType(TextField), findsNWidgets(2));
    expect(find.text('Log in'), findsOneWidget);
    expect(find.text('Log in with Quick Connect'), findsOneWidget);
  });

  testWidgets(
    'a server that says nothing about Quick Connect gets the password form',
    (tester) async {
      await pumpLogin(tester, FakeServer());

      expect(find.byType(TextField), findsNWidgets(2));
      expect(find.text('Log in with Quick Connect'), findsNothing);
    },
  );

  testWidgets(
    'without Quick Connect in Jellyfin there is only the password form',
    (tester) async {
      await pumpLogin(
        tester,
        FakeServer(passwordLogin: true, quickConnect: false),
      );

      expect(find.byType(TextField), findsNWidgets(2));
      expect(find.text('Log in with Quick Connect'), findsNothing);
    },
  );

  testWidgets('with passwords turned off only Quick Connect is left', (
    tester,
  ) async {
    await pumpLogin(
      tester,
      FakeServer(passwordLogin: false, quickConnect: true),
    );

    expect(find.byType(TextField), findsNothing);
    expect(find.text('Log in'), findsNothing);
    expect(find.text('Log in with Quick Connect'), findsOneWidget);
  });

  testWidgets('says so when there is no way to log in at all', (tester) async {
    await pumpLogin(
      tester,
      FakeServer(passwordLogin: false, quickConnect: false),
    );

    expect(find.byType(TextField), findsNothing);
    expect(find.textContaining('Logging in is not possible'), findsOneWidget);
  });

  testWidgets('shows the code, waits for the approval and logs in', (
    tester,
  ) async {
    final server = FakeServer(passwordLogin: true, quickConnect: true);
    final session = await pumpLogin(tester, server);

    await startQuickConnect(tester);

    expect(find.text('123 456'), findsOneWidget);
    expect(find.textContaining('Waiting for you'), findsOneWidget);
    expect(
      find.byType(TextField),
      findsNothing,
      reason: 'the form makes way for the code',
    );
    expect(session.loggedIn, isFalse);

    for (var i = 0; i < 3; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }

    expect(server.polls, 3);
    expect(session.loggedIn, isTrue);
    expect(session.user?.name, 'alice');
    expect(session.api.token, 'cliparr-token');
  });

  testWidgets('cancelling goes back to the form and stops asking', (
    tester,
  ) async {
    final server = FakeServer(passwordLogin: true, quickConnect: true);
    final session = await pumpLogin(tester, server);
    await startQuickConnect(tester);
    await tester.pump(const Duration(milliseconds: 100));
    final asked = server.polls;

    await tester.tap(find.text('Cancel'));
    await tester.pump();
    await tester.pump(const Duration(seconds: 2));

    expect(find.byType(TextField), findsNWidgets(2));
    expect(find.text('123 456'), findsNothing);
    expect(server.polls, asked, reason: 'no more questions after cancelling');
    expect(session.loggedIn, isFalse);
  });

  testWidgets('an expired code ends the wait with a message', (tester) async {
    final server = FakeServer(passwordLogin: true, quickConnect: true)
      ..pollAnswer = (_) =>
          jsonResponse({'error': 'this sign-in has expired; start again'}, 410);
    await pumpLogin(tester, server);

    await startQuickConnect(tester);
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump();

    expect(find.text('123 456'), findsNothing);
    expect(find.textContaining('has expired'), findsOneWidget);
    expect(
      find.text('Log in with Quick Connect'),
      findsOneWidget,
      reason: 'one can start again',
    );
  });

  testWidgets('the code runs out on its own after its lifetime', (
    tester,
  ) async {
    final server = FakeServer(passwordLogin: true, quickConnect: true);
    server.startAnswer = () =>
        jsonResponse({'id': 'abc', 'code': '123456', 'expiresInSeconds': 1});
    server.pollAnswer = (_) => jsonResponse({'status': 'pending'});
    await pumpLogin(tester, server);

    await startQuickConnect(tester);
    await tester.pump(const Duration(seconds: 2));

    expect(find.text('123 456'), findsNothing);
    expect(find.textContaining('code expired'), findsOneWidget);
  });

  testWidgets('explains when Jellyfin has Quick Connect turned off', (
    tester,
  ) async {
    final server = FakeServer(passwordLogin: true, quickConnect: true)
      ..startAnswer = () => jsonResponse({
        'error': 'Quick Connect is turned off in Jellyfin. An administrator can turn it on in the Jellyfin dashboard.',
      }, 409);
    await pumpLogin(tester, server);

    await startQuickConnect(tester);

    expect(find.textContaining('turned off in Jellyfin'), findsOneWidget);
    expect(find.byType(TextField), findsNWidgets(2));
  });
}
