import 'package:flutter/material.dart';
import 'package:video_player_media_kit/video_player_media_kit.dart';

import 'app.dart';
import 'state/session.dart';
import 'state/settings.dart';
import 'update/apk_installer.dart';
import 'update/update_checker.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // video_player has no Linux backend of its own; media_kit provides one.
  VideoPlayerMediaKit.ensureInitialized(linux: true);
  final session = await SessionController.restore();
  final settings = await Settings.load();
  runApp(
    CliparrApp(
      session: session,
      settings: settings,
      updates: UpdateChecker(),
      installer: AndroidApkInstaller(),
    ),
  );
}
