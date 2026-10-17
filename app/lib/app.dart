import 'package:flutter/material.dart';

import 'screens/clips_screen.dart';
import 'screens/login_screen.dart';
import 'screens/server_screen.dart';
import 'services.dart';
import 'state/session.dart';
import 'state/settings.dart';
import 'update/apk_installer.dart';
import 'update/update_checker.dart';
import 'version.dart';

class CliparrApp extends StatelessWidget {
  const CliparrApp({
    super.key,
    required this.session,
    required this.settings,
    required this.updates,
    required this.installer,
    this.currentVersion = appVersion,
  });

  final SessionController session;
  final Settings settings;
  final UpdateChecker updates;
  final ApkInstaller installer;
  final String currentVersion;

  @override
  Widget build(BuildContext context) {
    return Services(
      settings: settings,
      updates: updates,
      installer: installer,
      currentVersion: currentVersion,
      child: MaterialApp(
        title: 'Cliparr',
        theme: ThemeData(
          colorSchemeSeed: Colors.deepPurple,
          useMaterial3: true,
        ),
        darkTheme: ThemeData(
          colorSchemeSeed: Colors.deepPurple,
          brightness: Brightness.dark,
          useMaterial3: true,
        ),
        home: ListenableBuilder(
          listenable: session,
          builder: (context, _) {
            if (session.needsServer) return ServerScreen(session: session);
            if (!session.loggedIn) return LoginScreen(session: session);
            return ClipsScreen(session: session);
          },
        ),
      ),
    );
  }
}
