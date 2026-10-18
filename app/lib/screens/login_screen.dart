import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/models.dart';
import '../state/session.dart';
import '../widgets/error_text.dart';

/// Logs in with a Jellyfin password or, where Jellyfin allows it, a Quick Connect code.
class LoginScreen extends StatefulWidget {
  const LoginScreen({
    super.key,
    required this.session,
    this.pollEvery = const Duration(seconds: 2),
  });

  final SessionController session;

  /// How often to ask whether the user approved a Quick Connect code.
  final Duration pollEvery;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  bool _busy = false;
  String? _error;

  LoginOptions _options = LoginOptions.passwordOnly;
  QuickConnectRequest? _quickConnect;
  Timer? _poll;
  Timer? _expiry;
  bool _asking = false;

  @override
  void initState() {
    super.initState();
    _loadOptions();
  }

  @override
  void dispose() {
    _stopTimers();
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _loadOptions() async {
    try {
      final options = await widget.session.api.loginOptions();
      if (mounted) setState(() => _options = options);
    } on ApiException {
      // Keep the password form: it is what every server offers.
    }
  }

  Future<void> _login() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.session.login(_username.text.trim(), _password.text);
      // Lets the browser offer to save the password.
      TextInput.finishAutofillContext();
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _startQuickConnect() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final request = await widget.session.api.startQuickConnect();
      if (!mounted) return;
      setState(() => _quickConnect = request);
      _poll = Timer.periodic(widget.pollEvery, (_) => _askIfApproved());
      _expiry = Timer(request.expiresIn, () {
        _stopQuickConnect(error: 'The code expired. Start again.');
      });
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _askIfApproved() async {
    final request = _quickConnect;
    if (request == null || _asking) return;
    _asking = true;
    try {
      // On success the session changes and the app replaces this screen.
      final signedIn = await widget.session.pollQuickConnect(request.id);
      if (signedIn) {
        _stopQuickConnect();
      }
    } on ApiException catch (e) {
      _stopQuickConnect(error: e.message);
    } finally {
      _asking = false;
    }
  }

  void _stopTimers() {
    _poll?.cancel();
    _expiry?.cancel();
    _poll = _expiry = null;
  }

  /// Stops asking Jellyfin; the code then simply runs out there.
  void _stopQuickConnect({String? error}) {
    _stopTimers();
    if (!mounted) return;
    setState(() {
      _quickConnect = null;
      _error = error;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 400),
          child: Padding(
            padding: const EdgeInsets.all(24),
            // Tells browsers and password managers which fields belong together.
            child: AutofillGroup(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Cliparr',
                    style: Theme.of(context).textTheme.headlineLarge,
                  ),
                  const SizedBox(height: 8),
                  if (_quickConnect != null)
                    _QuickConnectPanel(
                      request: _quickConnect!,
                      onCancel: _stopQuickConnect,
                    )
                  else
                    ..._forms(context),
                  if (!kIsWeb)
                    TextButton(
                      onPressed: widget.session.forgetServer,
                      child: Text(
                        'Change server (${widget.session.serverUrl})',
                      ),
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _forms(BuildContext context) {
    final quickConnectButton = _options.password
        ? OutlinedButton.icon(
            onPressed: _busy ? null : _startQuickConnect,
            icon: const Icon(Icons.devices),
            label: const Text('Log in with Quick Connect'),
          )
        : FilledButton.icon(
            onPressed: _busy ? null : _startQuickConnect,
            icon: const Icon(Icons.devices),
            label: const Text('Log in with Quick Connect'),
          );
    return [
      Text(
        _options.password
            ? 'Log in with your Jellyfin account.'
            : 'Log in with your Jellyfin account, using Quick Connect.',
      ),
      const SizedBox(height: 24),
      if (_options.password) ...[
        TextField(
          controller: _username,
          autofocus: true,
          autofillHints: const [AutofillHints.username],
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: 'Username',
            border: OutlineInputBorder(),
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _password,
          obscureText: true,
          autofillHints: const [AutofillHints.password],
          decoration: const InputDecoration(
            labelText: 'Password',
            border: OutlineInputBorder(),
          ),
          onSubmitted: (_) => _login(),
        ),
      ],
      ErrorText(_error),
      const SizedBox(height: 16),
      if (_options.password)
        FilledButton(
          onPressed: _busy ? null : _login,
          child: Text(_busy ? 'Logging in...' : 'Log in'),
        ),
      if (_options.password && _options.quickConnect) ...[
        const SizedBox(height: 8),
        const Center(child: Text('or')),
        const SizedBox(height: 8),
      ],
      if (_options.quickConnect) quickConnectButton,
      if (!_options.password && !_options.quickConnect)
        const Text(
          'Logging in is not possible: this server turned off passwords, and '
          'Quick Connect is off in Jellyfin. Ask whoever runs the server.',
        ),
    ];
  }
}

/// The code to type into a Jellyfin app, while Cliparr waits for the user to do so.
class _QuickConnectPanel extends StatelessWidget {
  const _QuickConnectPanel({required this.request, required this.onCancel});

  final QuickConnectRequest request;
  final VoidCallback onCancel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final code = request.code.length == 6
        ? '${request.code.substring(0, 3)} ${request.code.substring(3)}'
        : request.code;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'In Jellyfin, open the user menu or the settings, choose Quick '
          'Connect and enter this code:',
        ),
        const SizedBox(height: 16),
        Center(
          child: SelectableText(
            code,
            style: theme.textTheme.displaySmall?.copyWith(letterSpacing: 4),
          ),
        ),
        const SizedBox(height: 16),
        const Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            SizedBox(
              width: 16,
              height: 16,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            SizedBox(width: 12),
            Flexible(child: Text('Waiting for you to enter it...')),
          ],
        ),
        const SizedBox(height: 16),
        Text(
          'Only enter a code that you started yourself on this screen.',
          style: theme.textTheme.bodySmall,
        ),
        const SizedBox(height: 16),
        OutlinedButton(onPressed: onCancel, child: const Text('Cancel')),
      ],
    );
  }
}
