class AppConfig {
  final Uri apiBaseUrl;

  const AppConfig({required this.apiBaseUrl});

  static const defaultBaseUrl = 'http://127.0.0.1:8090';

  factory AppConfig.fromEnv() {
    const baseUrl = String.fromEnvironment(
      'READAPI_BASE_URL',
      defaultValue: defaultBaseUrl,
    );
    return AppConfig.fromBaseUrl(baseUrl);
  }

  factory AppConfig.fromBaseUrl(String baseUrl) {
    return AppConfig(apiBaseUrl: _normalizeBaseUrl(baseUrl));
  }

  static Uri _normalizeBaseUrl(String baseUrl) {
    final trimmed = baseUrl.trim();
    final normalized = trimmed.isEmpty
        ? defaultBaseUrl
        : trimmed.contains('://')
            ? trimmed
            : 'http://$trimmed';
    var uri = Uri.parse(normalized);
    if (!uri.hasPort && uri.scheme == 'http') {
      if (uri.host == 'localhost' || uri.host == '127.0.0.1') {
        uri = uri.replace(port: 8090);
      }
    }
    return uri;
  }
}
