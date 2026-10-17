import 'package:flutter/material.dart';

import '../api/models.dart';
import '../state/session.dart';
import '../util/time_format.dart';
import 'open_in_jellyfin.dart';

/// A bookmark: a picture from the clip, its title and time range, and what
/// you can do with it (play it here, open it in Jellyfin, edit, delete).
class ClipCard extends StatelessWidget {
  const ClipCard({
    super.key,
    required this.session,
    required this.clip,
    required this.onPlay,
    required this.onEdit,
    required this.onDelete,
  });

  final SessionController session;
  final Clip clip;
  final VoidCallback onPlay;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  /// Cards wider than this put the picture beside the text.
  static const _wideBreakpoint = 640.0;

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final picture = _Thumbnail(
            session: session,
            clip: clip,
            onTap: onPlay,
          );
          final details = _Details(
            session: session,
            clip: clip,
            onPlay: onPlay,
            onEdit: onEdit,
            onDelete: onDelete,
          );
          if (constraints.maxWidth >= _wideBreakpoint) {
            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(width: 300, child: picture),
                Expanded(child: details),
              ],
            );
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [picture, details],
          );
        },
      ),
    );
  }
}

class _Thumbnail extends StatelessWidget {
  const _Thumbnail({
    required this.session,
    required this.clip,
    required this.onTap,
  });

  final SessionController session;
  final Clip clip;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final api = session.api;
    return ClipRRect(
      borderRadius: const BorderRadius.all(Radius.circular(12)),
      child: InkWell(
        onTap: onTap,
        child: AspectRatio(
          aspectRatio: 16 / 9,
          child: Stack(
            fit: StackFit.expand,
            children: [
              ColoredBox(
                color: colors.surfaceContainerHighest,
                child: Image.network(
                  api.thumbnailUrl(clip).toString(),
                  headers: api.authHeaders,
                  fit: BoxFit.cover,
                  semanticLabel: 'Preview of ${clip.title}',
                  errorBuilder: (_, _, _) =>
                      Icon(Icons.movie_outlined, color: colors.outline),
                ),
              ),
              const Center(
                child: Icon(
                  Icons.play_circle_fill,
                  size: 48,
                  color: Colors.white70,
                ),
              ),
              Positioned(
                right: 8,
                bottom: 8,
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: Colors.black54,
                    borderRadius: BorderRadius.circular(4),
                  ),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 6,
                      vertical: 2,
                    ),
                    child: Text(
                      '${formatMs(clip.startMs)}–${formatMs(clip.endMs)}',
                      style: const TextStyle(color: Colors.white, fontSize: 12),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Details extends StatelessWidget {
  const _Details({
    required this.session,
    required this.clip,
    required this.onPlay,
    required this.onEdit,
    required this.onDelete,
  });

  final SessionController session;
  final Clip clip;
  final VoidCallback onPlay;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 4, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(clip.title, style: theme.textTheme.titleMedium),
              ),
              PopupMenuButton<String>(
                tooltip: 'More',
                onSelected: (v) => v == 'edit' ? onEdit() : onDelete(),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit')),
                  PopupMenuItem(value: 'delete', child: Text('Delete')),
                ],
              ),
            ],
          ),
          if (clip.quote.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(right: 12, bottom: 4),
              child: Text(
                '"${clip.quote}"',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontStyle: FontStyle.italic,
                ),
              ),
            ),
          if (clip.tags.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4, right: 12),
              child: Wrap(
                spacing: 6,
                children: [for (final tag in clip.tags) Chip(label: Text(tag))],
              ),
            ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              FilledButton.icon(
                onPressed: onPlay,
                icon: const Icon(Icons.play_arrow),
                label: const Text('Play clip'),
              ),
              OutlinedButton.icon(
                onPressed: () => showOpenInJellyfin(context, session, clip),
                icon: const Icon(Icons.open_in_new),
                label: const Text('Open in Jellyfin'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
