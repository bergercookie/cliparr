import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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
    this.autoplay = true,
    this.restrictToRange = false,
    this.overlay,
  });

  final ApiClient api;
  final String itemId;

  /// Initial clip range. When both are set the player seeks to [startMs] and
  /// loops back from [endMs].
  final int? startMs;
  final int? endMs;

  /// Start playing as soon as the video is ready. Browsers may refuse this
  /// until the user has interacted with the page; the video then stays paused.
  final bool autoplay;

  /// Only allow seeking between [startMs] and [endMs], and show times within
  /// the clip. For viewing a clip; the editor needs the whole video.
  final bool restrictToRange;

  /// Controls shown on top of the video, in its top right corner.
  final Widget? overlay;

  @override
  State<ClipPlayer> createState() => ClipPlayerState();
}

/// How far the left and right arrow keys seek.
const _seekStep = Duration(seconds: 5);

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
    HardwareKeyboard.instance.addHandler(_onKey);
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
          } else if (widget.autoplay) {
            await _play();
          }
          if (mounted) setState(() {});
        })
        .catchError((Object e) {
          if (mounted) setState(() => _error = e);
        });
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_onKey);
    _controller.dispose();
    super.dispose();
  }

  /// Seeks to [startMs] and loops between [startMs] and [endMs].
  Future<void> playRange(int startMs, int endMs, {bool play = true}) async {
    _loopStartMs = startMs;
    _loopEndMs = endMs;
    await _controller.seekTo(Duration(milliseconds: startMs));
    if (play) await _play();
  }

  /// Plays, or stays paused when the browser does not allow autoplay.
  Future<void> _play() async {
    try {
      await _controller.play();
    } catch (_) {
      // The play button is right there for the user to press.
    }
  }

  /// Stops looping so the whole video can be played.
  void clearRange() {
    _loopStartMs = null;
    _loopEndMs = null;
  }

  /// Space pauses or resumes; the arrow keys seek. Keys typed into a text
  /// field, or meant for a button or the seek bar that has focus, are left alone.
  bool _onKey(KeyEvent event) {
    if (event is KeyUpEvent || !_controller.value.isInitialized) return false;
    final focus = FocusManager.instance.primaryFocus?.context;
    if (focus != null &&
        (focus.findAncestorWidgetOfExactType<EditableText>() != null ||
            focus.widget is EditableText ||
            focus.findAncestorWidgetOfExactType<ButtonStyleButton>() != null ||
            focus.findAncestorWidgetOfExactType<IconButton>() != null ||
            focus.findAncestorWidgetOfExactType<Slider>() != null)) {
      return false;
    }
    final key = event.logicalKey;
    if (key == LogicalKeyboardKey.space && event is KeyDownEvent) {
      _toggle();
    } else if (key == LogicalKeyboardKey.arrowLeft) {
      _seekBy(-_seekStep);
    } else if (key == LogicalKeyboardKey.arrowRight) {
      _seekBy(_seekStep);
    } else {
      return false;
    }
    return true;
  }

  /// The part of the video that may be played: the clip, or all of it.
  int get _minMs => _restricted ? _loopStartMs! : 0;
  int get _maxMs =>
      _restricted ? _loopEndMs! : _controller.value.duration.inMilliseconds;
  bool get _restricted =>
      widget.restrictToRange && _loopStartMs != null && _loopEndMs != null;

  void _seekToMs(int ms) =>
      _controller.seekTo(Duration(milliseconds: ms.clamp(_minMs, _maxMs)));

  void _seekBy(Duration step) => _seekToMs(positionMs + step.inMilliseconds);

  void _toggle() {
    // Leave the button that was clicked, so the keys keep reaching the player.
    FocusManager.instance.primaryFocus?.unfocus();
    if (_controller.value.isPlaying) {
      _controller.pause();
    } else {
      _play();
    }
  }

  void _onTick() {
    final end = _loopEndMs;
    if (end != null && positionMs >= end) {
      _controller.seekTo(Duration(milliseconds: _loopStartMs ?? 0));
    } else if (_restricted && positionMs < _loopStartMs!) {
      _controller.seekTo(Duration(milliseconds: _loopStartMs!));
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
        // Keeps the play button and the seek bar on screen: the video only
        // takes what is left of a share of the window's height.
        ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: (MediaQuery.sizeOf(context).height * 0.4).clamp(
              160.0,
              480.0,
            ),
          ),
          child: AspectRatio(
            aspectRatio: value.aspectRatio,
            child: Stack(
              children: [
                Positioned.fill(
                  child: _ClickToToggle(
                    playing: value.isPlaying,
                    onToggle: _toggle,
                    child: VideoPlayer(_controller),
                  ),
                ),
                if (widget.overlay != null)
                  Positioned(top: 8, right: 8, child: widget.overlay!),
              ],
            ),
          ),
        ),
        Row(
          children: [
            IconButton(
              tooltip: value.isPlaying ? 'Pause' : 'Play',
              icon: Icon(value.isPlaying ? Icons.pause : Icons.play_arrow),
              onPressed: _toggle,
            ),
            Expanded(
              child: Slider(
                value: positionMs.clamp(_minMs, _maxMs).toDouble(),
                min: _minMs.toDouble(),
                max: _maxMs.toDouble(),
                onChanged: (v) => _seekToMs(v.round()),
              ),
            ),
            Text(
              '${formatMs(positionMs - _minMs)} / ${formatMs(_maxMs - _minMs)}',
            ),
            const SizedBox(width: 8),
          ],
        ),
      ],
    );
  }
}

/// Clicking the video pauses or resumes it. While paused, a big play button
/// sits on top of it.
class _ClickToToggle extends StatelessWidget {
  const _ClickToToggle({
    required this.playing,
    required this.onToggle,
    required this.child,
  });

  final bool playing;
  final VoidCallback onToggle;
  final Widget child;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    label: playing ? 'Pause' : 'Play',
    child: GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onToggle,
      child: Stack(
        alignment: Alignment.center,
        children: [
          child,
          if (!playing)
            const IgnorePointer(
              child: CircleAvatar(
                radius: 32,
                backgroundColor: Colors.black54,
                child: Icon(Icons.play_arrow, size: 44, color: Colors.white),
              ),
            ),
        ],
      ),
    ),
  );
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
