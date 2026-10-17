class User {
  const User({required this.id, required this.name, this.viaProxy = false});

  factory User.fromJson(Map<String, dynamic> json) => User(
    id: json['id'] as String,
    name: json['name'] as String,
    viaProxy: json['viaProxy'] as bool? ?? false,
  );

  final String id;
  final String name;

  /// True when a single sign-on proxy (e.g. Authelia) logged the user in, so
  /// there is nothing to log out of inside Cliparr.
  final bool viaProxy;
}

/// A movie or episode found in the Jellyfin library.
class Item {
  const Item({
    required this.id,
    required this.title,
    required this.type,
    required this.runtimeMs,
  });

  factory Item.fromJson(Map<String, dynamic> json) => Item(
    id: json['id'] as String,
    title: json['title'] as String,
    type: json['type'] as String,
    runtimeMs: json['runtimeMs'] as int,
  );

  final String id;
  final String title;
  final String type;
  final int runtimeMs;
}

/// A saved moment of an [Item].
class Clip {
  const Clip({
    required this.id,
    required this.itemId,
    required this.title,
    required this.startMs,
    required this.endMs,
    required this.quote,
    required this.note,
    required this.tags,
  });

  factory Clip.fromJson(Map<String, dynamic> json) => Clip(
    id: json['id'] as int,
    itemId: json['itemId'] as String,
    title: json['title'] as String,
    startMs: json['startMs'] as int,
    endMs: json['endMs'] as int,
    quote: json['quote'] as String,
    note: json['note'] as String,
    tags: (json['tags'] as List<dynamic>).cast<String>(),
  );

  final int id;
  final String itemId;
  final String title;
  final int startMs;
  final int endMs;
  final String quote;
  final String note;
  final List<String> tags;
}

/// The editable part of a clip, sent when creating or updating.
class ClipDraft {
  const ClipDraft({
    required this.itemId,
    required this.startMs,
    required this.endMs,
    this.quote = '',
    this.note = '',
    this.tags = const [],
  });

  final String itemId;
  final int startMs;
  final int endMs;
  final String quote;
  final String note;
  final List<String> tags;

  Map<String, dynamic> toJson() => {
    'itemId': itemId,
    'startMs': startMs,
    'endMs': endMs,
    'quote': quote,
    'note': note,
    'tags': tags,
  };
}
