import 'package:flutter/material.dart';

import '../api/models.dart';
import '../state/session.dart';
import '../util/time_format.dart';
import '../widgets/clip_player.dart';
import '../widgets/open_in_jellyfin.dart';

/// Plays one saved clip on a loop.
class ClipViewScreen extends StatelessWidget {
  const ClipViewScreen({super.key, required this.session, required this.clip});

  final SessionController session;
  final Clip clip;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(clip.title),
        actions: [
          IconButton(
            tooltip: 'Open in Jellyfin',
            icon: const Icon(Icons.open_in_new),
            onPressed: () => showOpenInJellyfin(context, session, clip),
          ),
        ],
      ),
      body: ListView(
        children: [
          ClipPlayer(
            api: session.api,
            itemId: clip.itemId,
            startMs: clip.startMs,
            endMs: clip.endMs,
            autoplay: true,
          ),
          Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (clip.quote.isNotEmpty)
                  Text('"${clip.quote}"', style: theme.textTheme.headlineSmall),
                const SizedBox(height: 8),
                Text(
                  'Clip ${formatMs(clip.startMs)}-${formatMs(clip.endMs)}',
                  style: theme.textTheme.bodySmall,
                ),
                if (clip.note.isNotEmpty) ...[
                  const SizedBox(height: 16),
                  Text(clip.note),
                ],
                if (clip.tags.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 16),
                    child: Wrap(
                      spacing: 6,
                      children: [
                        for (final t in clip.tags) Chip(label: Text(t)),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
