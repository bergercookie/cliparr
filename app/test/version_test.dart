import 'package:cliparr/update/version.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Version v(String s) => Version.tryParse(s)!;

  test('parses stable and nightly versions', () {
    expect(v('1.2.3').toString(), '1.2.3');
    expect(v('v1.2.3').toString(), '1.2.3');
    expect(v('1.3.3.post2').toString(), '1.3.3.post2');
    expect(v('1.3.3.post2.20261017').toString(), '1.3.3.post2.20261017');
    expect(v('v1.3.3.post2.20261017').isNightly, isTrue);
    expect(v('1.3.3.post2').isNightly, isTrue);
    expect(v('1.3.3').isNightly, isFalse);
  });

  test('rejects things that are not versions', () {
    for (final bad in [
      'dev',
      '',
      '1.2',
      '1.2.3.4',
      '1.2.3-rc1',
      'latest',
      '1.2.3.post1.2026',
    ]) {
      expect(Version.tryParse(bad), isNull, reason: bad);
    }
  });

  test(
    'orders versions numerically, with nightlies after their base release',
    () {
      final sorted = [
        '1.10.0',
        '1.3.3.post2.20261017',
        '1.3.3',
        '1.3.4',
        '1.9.9',
        '1.3.3.post10.20261101',
        '1.3.3.post2.20261016',
      ].map(v).toList()..sort();
      expect(sorted.map((e) => e.toString()), [
        '1.3.3',
        '1.3.3.post2.20261016',
        '1.3.3.post2.20261017',
        '1.3.3.post10.20261101',
        '1.3.4',
        '1.9.9',
        '1.10.0',
      ]);
    },
  );

  test('equal versions compare equal', () {
    expect(v('1.2.3'), v('v1.2.3'));
    expect(v('1.2.3') > v('1.2.3'), isFalse);
  });
}
