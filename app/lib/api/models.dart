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
    this.updatedAt,
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

  /// When the clip was last edited; changes the thumbnail address so a stale
  /// picture is never shown after the times change.
  final DateTime? updatedAt;
}

/// A running Jellyfin app (TV, phone, browser tab) that can be told to play.
class JellyfinDevice {
  const JellyfinDevice({
    required this.id,
    required this.name,
    required this.client,
  });

  factory JellyfinDevice.fromJson(Map<String, dynamic> json) => JellyfinDevice(
    id: json['id'] as String,
    name: json['name'] as String,
    client: json['client'] as String,
  );

  final String id;
  final String name;
  final String client;
}

/// What the server knows about opening a clip in Jellyfin itself.
class JellyfinHandoff {
  const JellyfinHandoff({
    required this.webUrl,
    required this.startMs,
    required this.resumeMs,
    required this.devices,
  });

  factory JellyfinHandoff.fromJson(Map<String, dynamic> json) =>
      JellyfinHandoff(
        webUrl: json['webUrl'] as String,
        startMs: json['startMs'] as int,
        resumeMs: json['resumeMs'] as int,
        devices: (json['devices'] as List<dynamic>)
            .map((e) => JellyfinDevice.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  /// Opens the item in Jellyfin's web interface.
  final String webUrl;
  final int startMs;

  /// Where the user would currently resume the item, 0 if not started.
  final int resumeMs;
  final List<JellyfinDevice> devices;
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
