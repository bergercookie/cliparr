import 'package:flutter/material.dart';

import '../api/api_client.dart';

/// Shows [message] in the theme's error color, or nothing when it is null.
class ErrorText extends StatelessWidget {
  const ErrorText(this.message, {super.key});

  final String? message;

  @override
  Widget build(BuildContext context) {
    if (message == null) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: Text(
        message!,
        style: TextStyle(color: Theme.of(context).colorScheme.error),
      ),
    );
  }
}

/// Shows an [ApiException] (or any other error) as a snackbar.
void showError(BuildContext context, Object error) {
  final message = error is ApiException ? error.message : '$error';
  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
}
