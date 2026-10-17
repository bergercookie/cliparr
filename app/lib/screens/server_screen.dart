import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../state/session.dart';
import '../widgets/error_text.dart';

/// Native apps only: asks where the Cliparr server lives.
class ServerScreen extends StatefulWidget {
  const ServerScreen({super.key, required this.session});

  final SessionController session;

  @override
  State<ServerScreen> createState() => _ServerScreenState();
}

class _ServerScreenState extends State<ServerScreen> {
  final _url = TextEditingController();
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _url.dispose();
    super.dispose();
  }

  Future<void> _connect() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.session.setServer(_url.text);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 400),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  'Cliparr',
                  style: Theme.of(context).textTheme.headlineLarge,
                ),
                const SizedBox(height: 8),
                const Text('Enter the address of your Cliparr server.'),
                const SizedBox(height: 24),
                TextField(
                  controller: _url,
                  keyboardType: TextInputType.url,
                  autofocus: true,
                  decoration: const InputDecoration(
                    labelText: 'Server address',
                    hintText: 'http://192.168.1.10:8080',
                    border: OutlineInputBorder(),
                  ),
                  onSubmitted: (_) => _connect(),
                ),
                ErrorText(_error),
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: _busy ? null : _connect,
                  child: Text(_busy ? 'Connecting...' : 'Connect'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
