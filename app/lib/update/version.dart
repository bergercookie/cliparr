/// A Cliparr version: `major.minor.patch`, optionally followed by
/// `.postN.YYYYMMDD` for nightly builds (build counter, then the build date).
///
/// A nightly sorts after the stable release with the same numbers, so
/// `1.3.3 < 1.3.3.post1.20261016 < 1.3.3.post2.20261017 < 1.3.4`.
class Version implements Comparable<Version> {
  const Version(
    this.major,
    this.minor,
    this.patch, [
    this.post = 0,
    this.date = 0,
  ]);

  static final _pattern = RegExp(
    r'^v?(\d+)\.(\d+)\.(\d+)(?:\.post(\d+)(?:\.(\d{8}))?)?$',
  );

  /// Parses `1.2.3`, `v1.2.3`, `1.2.3.post4` or `1.2.3.post4.20261017`.
  /// Returns null for anything else (for example `dev` builds).
  static Version? tryParse(String text) {
    final m = _pattern.firstMatch(text.trim());
    if (m == null) return null;
    return Version(
      int.parse(m[1]!),
      int.parse(m[2]!),
      int.parse(m[3]!),
      m[4] == null ? 0 : int.parse(m[4]!),
      m[5] == null ? 0 : int.parse(m[5]!),
    );
  }

  final int major;
  final int minor;
  final int patch;
  final int post;

  /// Build date of a nightly as `YYYYMMDD`, or 0 when unknown.
  final int date;

  bool get isNightly => post > 0;

  @override
  int compareTo(Version other) {
    for (final (a, b) in [
      (major, other.major),
      (minor, other.minor),
      (patch, other.patch),
      (post, other.post),
      (date, other.date),
    ]) {
      if (a != b) return a.compareTo(b);
    }
    return 0;
  }

  bool operator >(Version other) => compareTo(other) > 0;

  @override
  bool operator ==(Object other) => other is Version && compareTo(other) == 0;

  @override
  int get hashCode => Object.hash(major, minor, patch, post, date);

  @override
  String toString() {
    final base = '$major.$minor.$patch';
    if (post == 0) return base;
    return date == 0 ? '$base.post$post' : '$base.post$post.$date';
  }
}
