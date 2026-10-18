import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../services.dart';
import '../state/session.dart';
import '../update/update_checker.dart';
import '../update/version.dart';
import '../widgets/connect_devices.dart';

/// Server address, versions and app updates.
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key, required this.session});

  final SessionController session;

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  late Future<String> _serverVersion;
  bool _started = false;

  late UpdateChannel _channel;
  bool _checking = false;
  Release? _latest;
  String? _checkError;

  bool _installing = false;
  double? _progress;
  String? _installMessage;

  @override
  void initState() {
    super.initState();
    _serverVersion = widget.session.api.serverVersion();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_started) {
      return;
    }
    _started = true;
    _channel = Services.of(context).settings.channel;
    if (_showUpdates) _checkForUpdate();
  }

  /// The web app is served by the server itself, so it never updates on its own.
  bool get _showUpdates => !kIsWeb;

  Version? get _current =>
      Version.tryParse(Services.of(context).currentVersion);

  bool get _updateAvailable {
    final current = _current;
    final latest = _latest;
    return current != null && latest != null && latest.version > current;
  }

  Future<void> _checkForUpdate() async {
    setState(() {
      _checking = true;
      _checkError = null;
      _installMessage = null;
    });
    try {
      final latest = await Services.of(context).updates.latest(_channel);
      if (mounted) setState(() => _latest = latest);
    } on Exception catch (e) {
      if (mounted) {
        setState(() => _checkError = 'Could not check for updates: $e');
      }
    } finally {
      if (mounted) setState(() => _checking = false);
    }
  }

  Future<void> _selectChannel(UpdateChannel channel) async {
    setState(() => _channel = channel);
    await Services.of(context).settings.setChannel(channel);
    await _checkForUpdate();
  }

  Future<void> _install(Release release) async {
    final services = Services.of(context);
    setState(() {
      _installing = true;
      _progress = null;
      _installMessage = null;
    });
    try {
      final checksum = await services.updates.expectedChecksum(release);
      await for (final p in services.installer.install(
        release.apkUrl,
        fileName: release.apkName,
        sha256: checksum,
      )) {
        if (!mounted) return;
        setState(() {
          _progress = p.fraction;
          _installMessage = p.message;
        });
        if (p.failed) break;
      }
    } on Exception catch (e) {
      if (mounted) setState(() => _installMessage = 'Update failed: $e');
    } finally {
      if (mounted) setState(() => _installing = false);
    }
  }

  Future<void> _changeServer() async {
    final controller = TextEditingController(
      text: widget.session.serverUrl.toString(),
    );
    String? error;
    final changed = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('Server address'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: controller,
                autofocus: true,
                keyboardType: TextInputType.url,
                decoration: InputDecoration(errorText: error),
              ),
              const SizedBox(height: 8),
              const Text('You will have to log in again.'),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () async {
                try {
                  await widget.session.setServer(controller.text);
                  if (context.mounted) Navigator.pop(context, true);
                } on ApiException catch (e) {
                  setDialogState(() => error = e.message);
                }
              },
              child: const Text('Connect'),
            ),
          ],
        ),
      ),
    );
    controller.dispose();
    // Changing the server logs the user out, so go back to the login screen.
    if (changed == true && mounted) {
      Navigator.of(context).popUntil((route) => route.isFirst);
    }
  }

  @override
  Widget build(BuildContext context) {
    final services = Services.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('Settings')),
      body: ListView(
        children: [
          const _SectionTitle('Server'),
          ListTile(
            title: const Text('Server address'),
            subtitle: Text('${widget.session.serverUrl}'),
            trailing: kIsWeb
                ? null
                : IconButton(
                    tooltip: 'Change server address',
                    icon: const Icon(Icons.edit),
                    onPressed: _changeServer,
                  ),
          ),
          FutureBuilder<String>(
            future: _serverVersion,
            builder: (context, snapshot) => ListTile(
              title: const Text('Server version'),
              subtitle: Text(
                snapshot.hasError
                    ? 'Unknown (could not reach the server)'
                    : snapshot.data ?? 'Checking...',
              ),
            ),
          ),
          ListTile(
            title: const Text('App version'),
            subtitle: Text(services.currentVersion),
          ),
          if (kIsWeb && widget.session.serverUrl != null) ...[
            const _SectionTitle('Other devices'),
            ConnectDevices(address: widget.session.serverUrl!),
          ],
          if (_showUpdates) ..._updateSection(services),
        ],
      ),
    );
  }

  List<Widget> _updateSection(Services services) {
    return [
      const _SectionTitle('App updates'),
      Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: SegmentedButton<UpdateChannel>(
          segments: const [
            ButtonSegment(value: UpdateChannel.stable, label: Text('Stable')),
            ButtonSegment(value: UpdateChannel.nightly, label: Text('Nightly')),
          ],
          selected: {_channel},
          onSelectionChanged: _installing
              ? null
              : (s) => _selectChannel(s.first),
        ),
      ),
      _updateStatus(),
      if (_installing)
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: LinearProgressIndicator(value: _progress),
        ),
      if (_installMessage != null)
        ListTile(
          title: Text(_installMessage!),
          leading: const Icon(Icons.info_outline),
        ),
      Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: Wrap(
          spacing: 8,
          children: [
            OutlinedButton(
              onPressed: _checking || _installing ? null : _checkForUpdate,
              child: const Text('Check now'),
            ),
            if (_updateAvailable && services.installer.supported)
              FilledButton(
                onPressed: _installing ? null : () => _install(_latest!),
                child: Text('Download and install ${_latest!.version}'),
              ),
          ],
        ),
      ),
    ];
  }

  Widget _updateStatus() {
    if (_checking) {
      return const ListTile(
        leading: SizedBox(
          width: 24,
          height: 24,
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
        title: Text('Checking for updates...'),
      );
    }
    if (_checkError != null) {
      return ListTile(
        leading: const Icon(Icons.error_outline),
        title: Text(_checkError!),
      );
    }
    if (_current == null) {
      return const ListTile(
        leading: Icon(Icons.build_outlined),
        title: Text('Development build'),
        subtitle: Text('Update checks only work for released versions.'),
      );
    }
    final latest = _latest;
    if (latest == null) {
      return const ListTile(
        leading: Icon(Icons.help_outline),
        title: Text('No release found for this channel yet'),
      );
    }
    if (_updateAvailable) {
      return ListTile(
        leading: const Icon(Icons.system_update),
        title: Text('Update available: ${latest.version}'),
        subtitle: Text(
          'You are running ${Services.of(context).currentVersion}.',
        ),
      );
    }
    return ListTile(
      leading: const Icon(Icons.check_circle_outline),
      title: const Text('You are running the latest version'),
      subtitle: Text('Newest ${_channel.name} release: ${latest.version}'),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 24, 16, 4),
    child: Text(text, style: Theme.of(context).textTheme.titleSmall),
  );
}
