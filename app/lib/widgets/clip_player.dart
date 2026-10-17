import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

import '../api/api_client.dart';
import '../util/time_format.dart';

/// Plays an item's video, optionally restricted to a clip range.
///
/// Use a `GlobalKey<ClipPlayerState>` to read the position or start a range.
class ClipPlayer extends StatefulWidget {
  const ClipPlayer({
    super.key,
    required this.api,
    required this.itemId,
    this.startMs,
    this.endMs,
    this.autoplay = false,
  });

  final ApiClient api;
  final String itemId;

  /// Initial clip range. When both are set the player seeks to [startMs] and
  /// loops back from [endMs].
  final int? startMs;
  final int? endMs;
  final bool autoplay;

  @override
  State<ClipPlayer> createState() => ClipPlayerState();
}

class ClipPlayerState extends State<ClipPlayer> {
  late final VideoPlayerController _controller;
  Object? _error;
  int? _loopStartMs;
  int? _loopEndMs;

  /// Current playback position in milliseconds.
  int get positionMs => _controller.value.position.inMilliseconds;

  @override
  void initState() {
    super.initState();
    _controller = VideoPlayerController.networkUrl(
      widget.api.streamUrl(widget.itemId),
      httpHeaders: widget.api.authHeaders,
    )..addListener(_onTick);
    _controller
        .initialize()
        .then((_) async {
          if (!mounted) return;
          if (widget.startMs != null && widget.endMs != null) {
            await playRange(
              widget.startMs!,
              widget.endMs!,
              play: widget.autoplay,
            );
          }
          setState(() {});
        })
        .catchError((Object e) {
          if (mounted) setState(() => _error = e);
        });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  /// Seeks to [startMs] and loops between [startMs] and [endMs].
  Future<void> playRange(int startMs, int endMs, {bool play = true}) async {
    _loopStartMs = startMs;
    _loopEndMs = endMs;
    await _controller.seekTo(Duration(milliseconds: startMs));
    if (play) await _controller.play();
  }

  /// Stops looping so the whole video can be played.
  void clearRange() {
    _loopStartMs = null;
    _loopEndMs = null;
  }

  void _onTick() {
    final end = _loopEndMs;
    if (end != null && positionMs >= end) {
      _controller.seekTo(Duration(milliseconds: _loopStartMs ?? 0));
    }
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return const _PlayerMessage(
        'Could not play this video. It may use a format your device cannot play directly.',
      );
    }
    if (!_controller.value.isInitialized) {
      return const AspectRatio(
        aspectRatio: 16 / 9,
        child: Center(child: CircularProgressIndicator()),
      );
    }
    final value = _controller.value;
    return Column(
      children: [
        AspectRatio(
          aspectRatio: value.aspectRatio,
          child: VideoPlayer(_controller),
        ),
        Row(
          children: [
            IconButton(
              tooltip: value.isPlaying ? 'Pause' : 'Play',
              icon: Icon(value.isPlaying ? Icons.pause : Icons.play_arrow),
              onPressed: () =>
                  value.isPlaying ? _controller.pause() : _controller.play(),
            ),
            Expanded(
              child: Slider(
                value: positionMs
                    .clamp(0, value.duration.inMilliseconds)
                    .toDouble(),
                max: value.duration.inMilliseconds.toDouble(),
                onChanged: (v) =>
                    _controller.seekTo(Duration(milliseconds: v.round())),
              ),
            ),
            Text(
              '${formatMs(positionMs)} / ${formatMs(value.duration.inMilliseconds)}',
            ),
            const SizedBox(width: 8),
          ],
        ),
      ],
    );
  }
}

class _PlayerMessage extends StatelessWidget {
  const _PlayerMessage(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => AspectRatio(
    aspectRatio: 16 / 9,
    child: Center(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Text(text, textAlign: TextAlign.center),
      ),
    ),
  );
}
