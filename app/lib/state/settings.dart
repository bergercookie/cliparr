import 'package:shared_preferences/shared_preferences.dart';

import '../update/update_checker.dart';
import '../update/version.dart';
import '../version.dart';

const _channelKey = 'update_channel';

/// User preferences stored on the device.
class Settings {
  Settings(this._prefs, {this.currentVersion = appVersion});

  static Future<Settings> load() async =>
      Settings(await SharedPreferences.getInstance());

  final SharedPreferences _prefs;

  /// Version of the running app; decides the default [channel].
  final String currentVersion;

  /// Which releases to follow. Defaults to nightly for nightly builds and
  /// stable for everything else.
  UpdateChannel get channel {
    final saved = _prefs.getString(_channelKey);
    for (final channel in UpdateChannel.values) {
      if (channel.name == saved) return channel;
    }
    return (Version.tryParse(currentVersion)?.isNightly ?? false)
        ? UpdateChannel.nightly
        : UpdateChannel.stable;
  }

  Future<void> setChannel(UpdateChannel channel) =>
      _prefs.setString(_channelKey, channel.name);
}
