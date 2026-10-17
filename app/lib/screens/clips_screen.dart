import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/models.dart';
import '../state/session.dart';
import '../util/time_format.dart';
import 'clip_editor_screen.dart';
import 'clip_view_screen.dart';
import 'settings_screen.dart';

/// Home screen: all saved clips with search.
class ClipsScreen extends StatefulWidget {
  const ClipsScreen({super.key, required this.session});

  final SessionController session;

  @override
  State<ClipsScreen> createState() => _ClipsScreenState();
}

class _ClipsScreenState extends State<ClipsScreen> {
  final _search = TextEditingController();
  late Future<List<Clip>> _clips;

  @override
  void initState() {
    super.initState();
    _clips = _load();
  }

  @override
  void dispose() {
    _search.dispose();
    super.dispose();
  }

  Future<List<Clip>> _load() =>
      widget.session.run((api) => api.listClips(query: _search.text.trim()));

  void _reload() {
    setState(() {
      _clips = _load();
    });
  }

  Future<void> _openEditor({Clip? clip}) async {
    final saved = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => ClipEditorScreen(session: widget.session, clip: clip),
      ),
    );
    if (saved == true) _reload();
  }

  Future<void> _delete(Clip clip) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete clip?'),
        content: Text(clip.title),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await widget.session.run((api) => api.deleteClip(clip.id));
    _reload();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Cliparr'),
        actions: [
          IconButton(
            tooltip: 'Settings',
            icon: const Icon(Icons.settings),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute(
                builder: (_) => SettingsScreen(session: widget.session),
              ),
            ),
          ),
          if (widget.session.user?.viaProxy ?? false)
            Padding(
              padding: const EdgeInsets.only(right: 16),
              child: Center(child: Text(widget.session.user!.name)),
            )
          else
            IconButton(
              tooltip: 'Log out (${widget.session.user?.name})',
              icon: const Icon(Icons.logout),
              onPressed: widget.session.logout,
            ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _openEditor,
        icon: const Icon(Icons.add),
        label: const Text('New clip'),
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(12),
            child: SearchBar(
              controller: _search,
              hintText: 'Search quotes, notes and titles',
              leading: const Icon(Icons.search),
              onSubmitted: (_) => _reload(),
            ),
          ),
          Expanded(
            child: FutureBuilder<List<Clip>>(
              future: _clips,
              builder: (context, snapshot) {
                if (snapshot.hasError) {
                  return Center(child: Text('${snapshot.error}'));
                }
                if (!snapshot.hasData) {
                  return const Center(child: CircularProgressIndicator());
                }
                final clips = snapshot.data!;
                if (clips.isEmpty) {
                  return const Center(
                    child: Text(
                      'No clips yet. Tap "New clip" to save a moment.',
                    ),
                  );
                }
                return RefreshIndicator(
                  onRefresh: () async => _reload(),
                  child: ListView.builder(
                    itemCount: clips.length,
                    itemBuilder: (context, i) => _ClipCard(
                      clip: clips[i],
                      onOpen: () => Navigator.of(context).push(
                        MaterialPageRoute(
                          builder: (_) => ClipViewScreen(
                            session: widget.session,
                            clip: clips[i],
                          ),
                        ),
                      ),
                      onEdit: () => _openEditor(clip: clips[i]),
                      onDelete: () => _delete(clips[i]),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _ClipCard extends StatelessWidget {
  const _ClipCard({
    required this.clip,
    required this.onOpen,
    required this.onEdit,
    required this.onDelete,
  });

  final Clip clip;
  final VoidCallback onOpen;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      child: InkWell(
        onTap: onOpen,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 4, 12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (clip.quote.isNotEmpty)
                      Text(
                        '"${clip.quote}"',
                        style: theme.textTheme.titleMedium,
                      ),
                    const SizedBox(height: 4),
                    Text(
                      '${clip.title}  ·  ${formatMs(clip.startMs)}-${formatMs(clip.endMs)}',
                      style: theme.textTheme.bodySmall,
                    ),
                    if (clip.tags.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.only(top: 8),
                        child: Wrap(
                          spacing: 6,
                          children: [
                            for (final tag in clip.tags) Chip(label: Text(tag)),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
              PopupMenuButton<String>(
                onSelected: (v) => v == 'edit' ? onEdit() : onDelete(),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'edit', child: Text('Edit')),
                  PopupMenuItem(value: 'delete', child: Text('Delete')),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Shows an [ApiException] (or any error) as a snackbar.
void showError(BuildContext context, Object error) {
  final message = error is ApiException ? error.message : '$error';
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
}
