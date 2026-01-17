import 'package:flutter_test/flutter_test.dart';

import 'package:client/src/core/config/app_config.dart';

void main() {
  test('fromBaseUrl adds default port for localhost', () {
    final config = AppConfig.fromBaseUrl('http://localhost');
    expect(config.apiBaseUrl.toString(), 'http://localhost:8090');
  });

  test('fromBaseUrl adds default port for 127.0.0.1', () {
    final config = AppConfig.fromBaseUrl('http://127.0.0.1');
    expect(config.apiBaseUrl.toString(), 'http://127.0.0.1:8090');
  });

  test('fromBaseUrl adds scheme when missing', () {
    final config = AppConfig.fromBaseUrl('localhost:8090');
    expect(config.apiBaseUrl.toString(), 'http://localhost:8090');
  });

  test('fromBaseUrl keeps explicit port', () {
    final config = AppConfig.fromBaseUrl('http://localhost:8080');
    expect(config.apiBaseUrl.toString(), 'http://localhost:8080');
  });
}
