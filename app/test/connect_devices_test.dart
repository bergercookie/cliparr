import 'package:cliparr/widgets/connect_devices.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:qr_flutter/qr_flutter.dart';

void main() {
  // A bare origin parses with a trailing slash in the path when it comes from a page.
  final address = Uri.parse('https://cliparr.example.com/');

  Future<void> pump(WidgetTester tester) => tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(child: ConnectDevices(address: address)),
      ),
    ),
  );

  testWidgets('shows the address as text and as a QR code', (tester) async {
    await pump(tester);

    expect(find.text('https://cliparr.example.com'), findsOneWidget);
    expect(find.byType(QrImageView), findsOneWidget);
  });

  testWidgets('points to the apps and the browser extension', (tester) async {
    await pump(tester);

    expect(find.text('Get the apps'), findsOneWidget);
    expect(find.text('Extension guide'), findsOneWidget);
  });

  testWidgets('copies the address', (tester) async {
    String? copied;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied = (call.arguments as Map)['text'] as String;
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    await pump(tester);

    await tester.tap(find.text('Copy address'));
    await tester.pump();

    expect(copied, 'https://cliparr.example.com');
    expect(find.text('Address copied'), findsOneWidget);
  });
}
