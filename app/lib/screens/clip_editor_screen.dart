import 'package:flutter/material.dart';

import '../api/models.dart';
import '../state/session.dart';
import '../util/time_format.dart';
import '../widgets/clip_player.dart';
import '../widgets/error_text.dart';

/// Creates a clip (pick an item, then mark its range) or edits an existing one.
/// Pops `true` when something was saved.
class ClipEditorScreen extends StatefulWidget {
  const ClipEditorScreen({super.key, required this.session, this.clip});

  final SessionController session;
  final Clip? clip;

  @override
  State<ClipEditorScreen> createState() => _ClipEditorScreenState();
}

class _ClipEditorScreenState extends State<ClipEditorScreen> {
  final _player = GlobalKey<ClipPlayerState>();
  final _search = TextEditingController();
  final _start = TextEditingController();
  final _end = TextEditingController();
  final _quote = TextEditingController();
  final _note = TextEditingController();
  final _tags = TextEditingController();

  String? _itemId;
  String? _title;
  List<Item> _results = const [];
  String? _error;
  bool _saving = false;

  /// True between pressing the clip button for the start and for the end.
  bool _startedClip = false;

  Clip? get _existing => widget.clip;

  @override
  void initState() {
    super.initState();
    final clip = _existing;
    if (clip != null) {
      _itemId = clip.itemId;
      _title = clip.title;
      _start.text = formatMs(clip.startMs);
      _end.text = formatMs(clip.endMs);
      _quote.text = clip.quote;
      _note.text = clip.note;
      _tags.text = clip.tags.join(', ');
    }
  }

  @override
  void dispose() {
    for (final c in [_search, _start, _end, _quote, _note, _tags]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _searchItems() async {
    try {
      final results = await widget.session.run(
        (api) => api.searchItems(_search.text.trim()),
      );
      setState(() => _results = results);
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  void _pick(Item item) => setState(() {
    _itemId = item.id;
    _title = item.title;
    _results = const [];
  });

  void _markHere(TextEditingController field) {
    final player = _player.currentState;
    if (player != null) field.text = formatMs(player.positionMs);
  }

  /// The button on the video: the first press marks the start of a clip, the
  /// next one its end.
  void _toggleClip() {
    // Hand the keyboard back to the player, so space keeps pausing.
    FocusManager.instance.primaryFocus?.unfocus();
    final player = _player.currentState;
    if (player == null) return;
    if (!_startedClip) {
      setState(() {
        _start.text = formatMs(player.positionMs);
        _end.clear();
        _startedClip = true;
        _error = null;
      });
      return;
    }
    final start = parseTimestamp(_start.text);
    if (start != null && player.positionMs <= start) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('The end must be after the start.')),
      );
      return;
    }
    setState(() {
      _end.text = formatMs(player.positionMs);
      _startedClip = false;
      _error = null;
    });
  }

  /// Returns the typed range, or null after setting an error message.
  ({int start, int end})? _range() {
    final start = parseTimestamp(_start.text);
    final end = parseTimestamp(_end.text);
    if (start == null || end == null) {
      setState(
        () => _error = 'Enter start and end as m:ss, h:mm:ss or seconds.',
      );
      return null;
    }
    if (end <= start) {
      setState(() => _error = 'The end must be after the start.');
      return null;
    }
    setState(() => _error = null);
    return (start: start, end: end);
  }

  void _preview() {
    final range = _range();
    if (range != null) {
      _player.currentState?.playRange(range.start, range.end);
    }
  }

  Future<void> _save() async {
    final range = _range();
    if (range == null || _itemId == null) return;
    setState(() => _saving = true);
    final draft = ClipDraft(
      itemId: _itemId!,
      startMs: range.start,
      endMs: range.end,
      quote: _quote.text,
      note: _note.text,
      tags: _tags.text.split(','),
    );
    try {
      await widget.session.run(
        (api) => _existing == null
            ? api.createClip(draft)
            : api.updateClip(_existing!.id, draft),
      );
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      if (mounted) {
        setState(() => _error = e.toString());
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(_existing == null ? 'New clip' : 'Edit clip')),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [if (_itemId == null) ..._itemPicker() else ..._editor()],
      ),
    );
  }

  List<Widget> _itemPicker() => [
    TextField(
      controller: _search,
      autofocus: true,
      decoration: const InputDecoration(
        labelText: 'Search your Jellyfin library',
        border: OutlineInputBorder(),
        prefixIcon: Icon(Icons.search),
      ),
      onSubmitted: (_) => _searchItems(),
    ),
    for (final item in _results)
      ListTile(
        title: Text(item.title),
        subtitle: Text(item.type),
        onTap: () => _pick(item),
      ),
  ];

  List<Widget> _editor() => [
    Text(_title!, style: Theme.of(context).textTheme.titleLarge),
    const SizedBox(height: 12),
    ClipPlayer(
      key: _player,
      api: widget.session.api,
      itemId: _itemId!,
      startMs: _existing?.startMs,
      endMs: _existing?.endMs,
      overlay: FilledButton.icon(
        onPressed: _toggleClip,
        icon: Icon(_startedClip ? Icons.stop : Icons.content_cut),
        label: Text(_startedClip ? 'End clip' : 'Start clip'),
      ),
    ),
    const SizedBox(height: 12),
    Row(
      children: [
        Expanded(child: _timeField(_start, 'Start')),
        IconButton(
          tooltip: 'Use current position as start',
          icon: const Icon(Icons.flag_outlined),
          onPressed: () => _markHere(_start),
        ),
        const SizedBox(width: 8),
        Expanded(child: _timeField(_end, 'End')),
        IconButton(
          tooltip: 'Use current position as end',
          icon: const Icon(Icons.flag),
          onPressed: () => _markHere(_end),
        ),
      ],
    ),
    Align(
      alignment: Alignment.centerLeft,
      child: TextButton.icon(
        onPressed: _preview,
        icon: const Icon(Icons.replay),
        label: const Text('Preview clip'),
      ),
    ),
    const SizedBox(height: 8),
    TextField(
      controller: _quote,
      maxLines: 2,
      decoration: const InputDecoration(
        labelText: 'Quote',
        border: OutlineInputBorder(),
      ),
    ),
    const SizedBox(height: 12),
    TextField(
      controller: _note,
      maxLines: 3,
      decoration: const InputDecoration(
        labelText: 'Why does it matter to you?',
        border: OutlineInputBorder(),
      ),
    ),
    const SizedBox(height: 12),
    TextField(
      controller: _tags,
      decoration: const InputDecoration(
        labelText: 'Tags (comma separated)',
        border: OutlineInputBorder(),
      ),
    ),
    ErrorText(_error),
    const SizedBox(height: 16),
    FilledButton(
      onPressed: _saving ? null : _save,
      child: Text(_saving ? 'Saving...' : 'Save clip'),
    ),
  ];

  Widget _timeField(TextEditingController controller, String label) =>
      TextField(
        controller: controller,
        decoration: InputDecoration(
          labelText: label,
          hintText: '0:00',
          border: const OutlineInputBorder(),
        ),
      );
}
