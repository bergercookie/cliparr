import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import 'links.dart';

/// What a person with no clips yet sees: how to save the first one.
class FirstClipGuide extends StatelessWidget {
  const FirstClipGuide({super.key, required this.onNewClip});

  final VoidCallback onNewClip;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.bookmark_add_outlined,
                size: 56,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(height: 12),
              Text('No clips yet', style: theme.textTheme.titleLarge),
              const SizedBox(height: 8),
              const Text(
                'Pick a movie or episode from your Jellyfin library, mark '
                'where the moment starts and ends, and add the line or a '
                'note about why it matters to you.',
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 16),
              FilledButton.icon(
                onPressed: onNewClip,
                icon: const Icon(Icons.add),
                label: const Text('Save your first clip'),
              ),
              const SizedBox(height: 8),
              TextButton(
                onPressed: () => launchUrl(
                  Uri.parse(extensionGuideUrl),
                  mode: LaunchMode.externalApplication,
                ),
                child: const Text('Or bookmark while watching in Jellyfin'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
