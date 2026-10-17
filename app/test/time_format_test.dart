import 'package:cliparr/util/time_format.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('formatMs', () {
    expect(formatMs(0), '0:00');
    expect(formatMs(65000), '1:05');
    expect(formatMs(3725000), '1:02:05');
  });

  test('parseTimestamp accepts common formats', () {
    expect(parseTimestamp('1:05'), 65000);
    expect(parseTimestamp('1:02:05'), 3725000);
    expect(parseTimestamp('93'), 93000);
    expect(parseTimestamp(' 93.5 '), 93500);
  });

  test('parseTimestamp rejects garbage', () {
    for (final bad in ['', 'abc', '1:2:3:4', '-5', '1:x']) {
      expect(parseTimestamp(bad), isNull, reason: bad);
    }
  });
}
