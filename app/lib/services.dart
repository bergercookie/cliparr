import 'package:flutter/widgets.dart';

import 'state/settings.dart';
import 'update/apk_installer.dart';
import 'update/update_checker.dart';
import 'version.dart';

/// App-wide helpers that screens look up with `Services.of(context)`.
/// Tests replace them with fakes.
class Services extends InheritedWidget {
  const Services({
    super.key,
    required this.settings,
    required this.updates,
    required this.installer,
    this.currentVersion = appVersion,
    required super.child,
  });

  final Settings settings;
  final UpdateChecker updates;
  final ApkInstaller installer;

  /// The version of the running app.
  final String currentVersion;

  static Services of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<Services>()!;

  @override
  bool updateShouldNotify(Services old) => false;
}
