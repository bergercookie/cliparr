/// A Cliparr version: `major.minor.patch`, optionally followed by `.postN` for nightly builds.
///
/// A nightly sorts after the stable release with the same numbers, so
/// `1.3.3 < 1.3.3.post1 < 1.3.3.post2 < 1.3.4`.
class Version implements Comparable<Version> {
  const Version(this.major, this.minor, this.patch, [this.post = 0]);

  static final _pattern = RegExp(r'^v?(\d+)\.(\d+)\.(\d+)(?:\.post(\d+))?$');

  /// Parses `1.2.3`, `v1.2.3` or `1.2.3.post4`. Returns null for anything else
  /// (for example `dev` builds).
  static Version? tryParse(String text) {
    final m = _pattern.firstMatch(text.trim());
    if (m == null) return null;
    return Version(
      int.parse(m[1]!),
      int.parse(m[2]!),
      int.parse(m[3]!),
      m[4] == null ? 0 : int.parse(m[4]!),
    );
  }

  final int major;
  final int minor;
  final int patch;
  final int post;

  bool get isNightly => post > 0;

  @override
  int compareTo(Version other) {
    for (final (a, b) in [
      (major, other.major),
      (minor, other.minor),
      (patch, other.patch),
      (post, other.post),
    ]) {
      if (a != b) return a.compareTo(b);
    }
    return 0;
  }

  bool operator >(Version other) => compareTo(other) > 0;

  @override
  bool operator ==(Object other) => other is Version && compareTo(other) == 0;

  @override
  int get hashCode => Object.hash(major, minor, patch, post);

  @override
  String toString() => '$major.$minor.$patch${post > 0 ? '.post$post' : ''}';
}
