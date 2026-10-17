/// Formats milliseconds as `m:ss`, or `h:mm:ss` for videos over an hour.
String formatMs(int ms) {
  final total = ms ~/ 1000;
  final h = total ~/ 3600;
  final m = (total % 3600) ~/ 60;
  final s = total % 60;
  final ss = s.toString().padLeft(2, '0');
  if (h > 0) return '$h:${m.toString().padLeft(2, '0')}:$ss';
  return '$m:$ss';
}

/// Parses `h:mm:ss`, `m:ss`, or plain seconds (`93`, `93.5`) into milliseconds.
/// Returns null when the text is not a valid timestamp.
int? parseTimestamp(String text) {
  final parts = text.trim().split(':');
  if (parts.isEmpty || parts.length > 3) return null;
  var seconds = 0.0;
  for (final part in parts) {
    final value = double.tryParse(part);
    if (value == null || value < 0) return null;
    seconds = seconds * 60 + value;
  }
  return (seconds * 1000).round();
}
