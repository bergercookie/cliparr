import 'package:flutter/foundation.dart';
import 'package:ota_update/ota_update.dart';

/// Progress of a download-and-install.
class InstallProgress {
  const InstallProgress.downloading(this.fraction)
    : message = null,
      failed = false;
  const InstallProgress.installing()
    : fraction = null,
      message = 'Handing over to the Android installer...',
      failed = false;
  const InstallProgress.failed(this.message) : fraction = null, failed = true;

  /// 0..1 while downloading, null otherwise.
  final double? fraction;
  final String? message;
  final bool failed;
}

/// Downloads an APK and starts the system installer.
abstract class ApkInstaller {
  /// Whether this platform can install APKs (Android only).
  bool get supported;

  /// Downloads [url] (verifying [sha256] when given) and opens the installer.
  Stream<InstallProgress> install(
    Uri url, {
    required String fileName,
    String? sha256,
  });
}

/// Installs through Android's package installer using the `ota_update` plugin.
/// Android refuses the install unless the APK is signed with the same key as
/// the running app, so only genuine Cliparr releases can replace it.
class AndroidApkInstaller implements ApkInstaller {
  @override
  bool get supported =>
      !kIsWeb && defaultTargetPlatform == TargetPlatform.android;

  @override
  Stream<InstallProgress> install(
    Uri url, {
    required String fileName,
    String? sha256,
  }) async* {
    final events = OtaUpdate().execute(
      url.toString(),
      destinationFilename: fileName,
      sha256checksum: sha256,
    );
    await for (final event in events) {
      switch (event.status) {
        case OtaStatus.DOWNLOADING:
          final percent = double.tryParse(event.value ?? '');
          yield InstallProgress.downloading(
            percent == null ? null : percent / 100,
          );
        case OtaStatus.INSTALLING:
        case OtaStatus.INSTALLATION_DONE:
          yield const InstallProgress.installing();
        case OtaStatus.CHECKSUM_ERROR:
          yield const InstallProgress.failed(
            'The download is corrupted (checksum mismatch). Nothing was installed.',
          );
        case OtaStatus.PERMISSION_NOT_GRANTED_ERROR:
          yield const InstallProgress.failed(
            'Android blocked the install. Allow Cliparr to install apps in the system settings, then try again.',
          );
        case OtaStatus.CANCELED:
          yield const InstallProgress.failed('The download was canceled.');
        case OtaStatus.ALREADY_RUNNING_ERROR:
        case OtaStatus.INSTALLATION_ERROR:
        case OtaStatus.INTERNAL_ERROR:
        case OtaStatus.DOWNLOAD_ERROR:
          yield InstallProgress.failed(
            'Update failed: ${event.value ?? event.status.name}',
          );
      }
    }
  }
}
