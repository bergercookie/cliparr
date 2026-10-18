import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:url_launcher/url_launcher.dart';

import 'links.dart';

/// How to use Cliparr on other devices: the server address to enter, as text
/// to copy and as a QR code to scan with a phone, and where to get the apps
/// and the browser extension.
class ConnectDevices extends StatelessWidget {
  const ConnectDevices({super.key, required this.address});

  final Uri address;

  /// The address as people type it: without the slash a bare origin ends in.
  String get _text => address.toString().replaceFirst(RegExp(r'/$'), '');

  Future<void> _copy(BuildContext context) async {
    final messenger = ScaffoldMessenger.of(context);
    await Clipboard.setData(ClipboardData(text: _text));
    messenger.showSnackBar(const SnackBar(content: Text('Address copied')));
  }

  @override
  Widget build(BuildContext context) {
    final textTheme = Theme.of(context).textTheme;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('Other devices need this address to find your server.'),
          const SizedBox(height: 12),
          Wrap(
            spacing: 16,
            runSpacing: 12,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              // A QR code must stay dark on light to be readable, in dark mode too.
              Container(
                color: Colors.white,
                padding: const EdgeInsets.all(8),
                child: QrImageView(
                  data: _text,
                  size: 140,
                  semanticsLabel: 'QR code with the server address',
                ),
              ),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  SelectableText(_text, style: textTheme.titleSmall),
                  TextButton.icon(
                    onPressed: () => _copy(context),
                    icon: const Icon(Icons.copy),
                    label: const Text('Copy address'),
                  ),
                ],
              ),
            ],
          ),
          const SizedBox(height: 12),
          const Text(
            'Android and Linux apps: install one, open it and enter the address '
            'above (on a phone, scan the code).',
          ),
          TextButton(
            onPressed: () => launchUrl(
              Uri.parse(releasesUrl),
              mode: LaunchMode.externalApplication,
            ),
            child: const Text('Get the apps'),
          ),
          const Text(
            "Browser extension: bookmark moments straight from Jellyfin's "
            "player. Enter the address above in the extension's settings.",
          ),
          TextButton(
            onPressed: () => launchUrl(
              Uri.parse(extensionGuideUrl),
              mode: LaunchMode.externalApplication,
            ),
            child: const Text('Extension guide'),
          ),
        ],
      ),
    );
  }
}
