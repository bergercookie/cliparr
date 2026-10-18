import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:video_player/video_player.dart';

import '../api/api_client.dart';
import '../util/time_format.dart';

/// Plays an item's video, optionally restricted to a clip range.
///
/// Use a `GlobalKey<ClipPlayerState>` to read the position or start a range.
///
/// When the browser cannot decode the video's audio (Dolby Digital, DTS), the
/// video plays muted from the original file and the server supplies the sound
/// as MP3 on the side. That sound cannot be seeked, so it is loaded again from
/// the video's position after every jump, and whenever the two drift apart.
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

/// How long the separate sound waits for more jumps before loading, so that
/// dragging the seek bar or holding an arrow key loads it once.
const _soundJumpDelay = Duration(milliseconds: 400);

/// How far the sound may lag behind the picture, or run ahead of it.
const _maxDrift = Duration(milliseconds: 600);

/// The least time between two corrections of the sound, so that a slow
/// connection does not make it restart over and over.
const _driftCooldown = Duration(seconds: 3);

class ClipPlayerState extends State<ClipPlayer> {
  VideoPlayerController? _controller;
  Object? _error;
  int? _loopStartMs;
  int? _loopEndMs;

  // The separate sound, for videos with audio the browser cannot decode.
  bool _separateSound = false;
  bool _soundFailed = false;
  VideoPlayerController? _sound;
  int _soundOffsetMs = 0; // where in the video the sound begins
  int _soundGeneration = 0; // bumped by every load; an older load gives up
  Timer? _soundTimer;
  bool _loadingSound = false;
  DateTime _lastDriftFix = DateTime.fromMillisecondsSinceEpoch(0);

  /// Current playback position in milliseconds.
  int get positionMs => _controller?.value.position.inMilliseconds ?? 0;

  bool get _ready => _controller?.value.isInitialized ?? false;

  @override
  void initState() {
    super.initState();
    HardwareKeyboard.instance.addHandler(_onKey);
    if (widget.startMs != null && widget.endMs != null) {
      _loopStartMs = widget.startMs;
      _loopEndMs = widget.endMs;
    }
    _start();
  }

  @override
  void dispose() {
    HardwareKeyboard.instance.removeHandler(_onKey);
    _soundGeneration++;
    _soundTimer?.cancel();
    _sound?.dispose();
    _controller?.dispose();
    super.dispose();
  }

  Future<void> _start() async {
    try {
      final item = await widget.api.item(widget.itemId);
      _separateSound = !item.audioPlayable;
    } catch (_) {
      // Without the details, play the file as it is.
    }
    if (!mounted) return;
    final controller = VideoPlayerController.networkUrl(
      widget.api.streamUrl(widget.itemId),
      httpHeaders: widget.api.authHeaders,
    )..addListener(_onTick);
    _controller = controller;
    try {
      await controller.initialize();
      if (!mounted) return;
      if (_separateSound) await controller.setVolume(0);
      final start = widget.startMs ?? 0;
      if (start > 0) await controller.seekTo(Duration(milliseconds: start));
      if (widget.autoplay) await _play();
      _syncSound(delay: Duration.zero);
      if (mounted) setState(() {});
    } catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  /// Seeks to [startMs] and loops between [startMs] and [endMs].
  Future<void> playRange(int startMs, int endMs, {bool play = true}) async {
    _loopStartMs = startMs;
    _loopEndMs = endMs;
    await _jumpTo(startMs, delay: Duration.zero);
    if (play) await _play();
  }

  /// Plays, or stays paused when the browser does not allow autoplay.
  Future<void> _play() async {
    try {
      await _controller?.play();
      await _sound?.play();
    } catch (_) {
      // The play button is right there for the user to press.
    }
  }

  Future<void> _pause() async {
    await _controller?.pause();
    await _sound?.pause();
  }

  /// Stops looping so the whole video can be played.
  void clearRange() {
    _loopStartMs = null;
    _loopEndMs = null;
  }

  /// Loads the separate sound again, from where the picture is now, once no
  /// more jumps follow within [delay]. It plays if the picture plays.
  void _syncSound({Duration delay = _soundJumpDelay}) {
    if (!_separateSound) return;
    _soundTimer?.cancel();
    _soundTimer = Timer(delay, _loadSound);
  }

  Future<void> _loadSound() async {
    final video = _controller;
    if (video == null || !mounted) return;
    final generation = ++_soundGeneration;
    _loadingSound = true;
    _lastDriftFix = DateTime.now();
    final startMs = video.value.position.inMilliseconds;
    final next = VideoPlayerController.networkUrl(
      widget.api.audioUrl(widget.itemId, startMs),
      httpHeaders: widget.api.authHeaders,
    );
    try {
      await next.initialize();
    } catch (_) {
      await next.dispose();
      if (generation == _soundGeneration) _loadingSound = false;
      if (generation == _soundGeneration && mounted) {
        setState(() => _soundFailed = true);
      }
      return;
    }
    if (generation != _soundGeneration || !mounted) {
      await next.dispose();
      return;
    }
    _loadingSound = false;
    final old = _sound;
    _lastDriftFix = DateTime.now();
    setState(() {
      _sound = next;
      _soundOffsetMs = startMs;
      _soundFailed = false;
    });
    await old?.dispose();
    // The picture may have moved on while the sound was loading.
    if (video.value.isPlaying) await next.play();
  }

  /// How far the sound is behind (positive) or ahead of the picture.
  int? get _driftMs {
    final sound = _sound;
    if (sound == null || !sound.value.isInitialized) return null;
    return positionMs - (_soundOffsetMs + sound.value.position.inMilliseconds);
  }

  /// Space pauses or resumes; the arrow keys seek. Keys typed into a text
  /// field, or meant for a button or the seek bar that has focus, are left alone.
  bool _onKey(KeyEvent event) {
    if (event is KeyUpEvent || !_ready) return false;
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
  int get _maxMs => _restricted
      ? _loopEndMs!
      : _controller?.value.duration.inMilliseconds ?? 0;
  bool get _restricted =>
      widget.restrictToRange && _loopStartMs != null && _loopEndMs != null;

  void _seekToMs(int ms) => _jumpTo(ms.clamp(_minMs, _maxMs));

  void _seekBy(Duration step) => _seekToMs(positionMs + step.inMilliseconds);

  Future<void> _jumpTo(int ms, {Duration delay = _soundJumpDelay}) async {
    await _controller?.seekTo(Duration(milliseconds: ms));
    // The sound plays on from before the jump until the new one is loaded:
    // better silence than the wrong scene.
    await _sound?.pause();
    _syncSound(delay: delay);
  }

  void _toggle() {
    // Leave the button that was clicked, so the keys keep reaching the player.
    FocusManager.instance.primaryFocus?.unfocus();
    final controller = _controller;
    if (controller == null) return;
    if (controller.value.isPlaying) {
      _pause();
    } else {
      _play();
    }
  }

  void _onTick() {
    final end = _loopEndMs;
    if (end != null && positionMs >= end) {
      _jumpTo(_loopStartMs ?? 0, delay: Duration.zero);
    } else if (_restricted && positionMs < _loopStartMs!) {
      _jumpTo(_loopStartMs!, delay: Duration.zero);
    } else if (_separateSound && (_controller?.value.isPlaying ?? false)) {
      final drift = _driftMs;
      if (drift != null &&
          !_loadingSound &&
          _soundTimer?.isActive != true &&
          drift.abs() > _maxDrift.inMilliseconds &&
          DateTime.now().difference(_lastDriftFix) > _driftCooldown) {
        _syncSound(delay: Duration.zero);
      }
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
    final controller = _controller;
    if (controller == null || !controller.value.isInitialized) {
      return const AspectRatio(
        aspectRatio: 16 / 9,
        child: Center(child: CircularProgressIndicator()),
      );
    }
    final value = controller.value;
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
                    child: VideoPlayer(controller),
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
            if (_soundFailed)
              const Padding(
                padding: EdgeInsets.only(left: 8),
                child: Tooltip(
                  message:
                      'This video has audio your browser cannot play, and the '
                      'server could not supply the sound another way.',
                  child: Icon(Icons.volume_off),
                ),
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
