import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

import '../api/models.dart';
import '../state/session.dart';
import '../util/time_format.dart';
import 'error_text.dart';

/// Lets the user continue a clip in Jellyfin itself.
///
/// Jellyfin's web links cannot carry a start time, so there are two ways to
/// land on the exact moment: play it on a running Jellyfin app (any position
/// can be requested), or set the Jellyfin resume point to the clip start and
/// open the item, where "Resume" then starts there.
Future<void> showOpenInJellyfin(
  BuildContext context,
  SessionController session,
  Clip clip,
) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => _OpenInJellyfinSheet(session: session, clip: clip),
  );
}

class _OpenInJellyfinSheet extends StatefulWidget {
  const _OpenInJellyfinSheet({required this.session, required this.clip});

  final SessionController session;
  final Clip clip;

  @override
  State<_OpenInJellyfinSheet> createState() => _OpenInJellyfinSheetState();
}

class _OpenInJellyfinSheetState extends State<_OpenInJellyfinSheet> {
  late Future<JellyfinHandoff> _handoff;

  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    _handoff = widget.session.run((api) => api.jellyfinHandoff(widget.clip.id));
  }

  Future<void> _playOn(JellyfinDevice device) async {
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);
    try {
      await widget.session.run(
        (api) => api.playOnDevice(widget.clip.id, device.id),
      );
      navigator.pop();
      messenger.showSnackBar(
        SnackBar(
          content: Text(
            'Playing on ${device.name} from ${formatMs(widget.clip.startMs)}',
          ),
        ),
      );
    } on Exception catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> _openWeb(JellyfinHandoff handoff) async {
    final start = formatMs(widget.clip.startMs);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Open in Jellyfin web'),
        content: Text(
          "Jellyfin's web links cannot start at a given time. Cliparr will set "
          'your resume point for "${widget.clip.title}" to $start, so that '
          'Resume starts there.'
          '${handoff.resumeMs > 0 ? '\n\nYour current resume point '
                    '(${formatMs(handoff.resumeMs)}) will be replaced.' : ''}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Set resume point'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    try {
      await widget.session.run((api) => api.setResumePoint(widget.clip.id));
    } on Exception catch (e) {
      if (mounted) showError(context, e);
      return;
    }
    if (!mounted) return;

    // Opening the link needs its own tap: browsers block pop-ups that are
    // opened after waiting for the network.
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Resume point set'),
        content: Text('In Jellyfin, press Resume to start at $start.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Close'),
          ),
          FilledButton(
            onPressed: () {
              launchUrl(
                Uri.parse(handoff.webUrl),
                mode: LaunchMode.externalApplication,
              );
              Navigator.pop(context);
            },
            child: const Text('Open Jellyfin'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
        child: FutureBuilder<JellyfinHandoff>(
          future: _handoff,
          builder: (context, snapshot) {
            return Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Open in Jellyfin at ${formatMs(widget.clip.startMs)}',
                  style: theme.textTheme.titleLarge,
                ),
                const SizedBox(height: 4),
                Text(widget.clip.title, style: theme.textTheme.bodySmall),
                const SizedBox(height: 16),
                ..._body(context, snapshot),
              ],
            );
          },
        ),
      ),
    );
  }

  List<Widget> _body(
    BuildContext context,
    AsyncSnapshot<JellyfinHandoff> snapshot,
  ) {
    if (snapshot.hasError) {
      return [
        Text('${snapshot.error}'),
        TextButton(
          onPressed: () => setState(_load),
          child: const Text('Try again'),
        ),
      ];
    }
    final handoff = snapshot.data;
    if (handoff == null) {
      return const [Center(child: CircularProgressIndicator())];
    }
    final theme = Theme.of(context);
    return [
      Row(
        children: [
          Expanded(
            child: Text(
              'Play on a Jellyfin device',
              style: theme.textTheme.titleSmall,
            ),
          ),
          IconButton(
            tooltip: 'Look for devices again',
            icon: const Icon(Icons.refresh),
            onPressed: () => setState(_load),
          ),
        ],
      ),
      if (handoff.devices.isEmpty)
        const Padding(
          padding: EdgeInsets.only(bottom: 8),
          child: Text(
            'No Jellyfin app that Cliparr can control is running. Open '
            'Jellyfin on your TV, phone or computer, then look again.',
          ),
        ),
      for (final device in handoff.devices)
        ListTile(
          contentPadding: EdgeInsets.zero,
          leading: const Icon(Icons.tv),
          title: Text(device.name),
          subtitle: Text(device.client),
          onTap: () => _playOn(device),
        ),
      const Divider(),
      ListTile(
        contentPadding: EdgeInsets.zero,
        leading: const Icon(Icons.language),
        title: const Text('Open in Jellyfin web'),
        subtitle: Text(
          'Sets your resume point to ${formatMs(widget.clip.startMs)} and '
          'opens the item. Press Resume there.',
        ),
        onTap: () => _openWeb(handoff),
      ),
    ];
  }
}
