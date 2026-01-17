class AppConfig {
  final Uri apiBaseUrl;

  const AppConfig({required this.apiBaseUrl});

  factory AppConfig.fromEnv() {
    const baseUrl = String.fromEnvironment(
      'READAPI_BASE_URL',
      defaultValue: 'http://localhost:8090',
    );
    return AppConfig.fromBaseUrl(baseUrl);
  }

  factory AppConfig.fromBaseUrl(String baseUrl) {
    return AppConfig(apiBaseUrl: _normalizeBaseUrl(baseUrl));
  }

  static Uri _normalizeBaseUrl(String baseUrl) {
    final trimmed = baseUrl.trim();
    final normalized = trimmed.isEmpty
        ? 'http://localhost:8090'
        : trimmed.contains('://')
            ? trimmed
            : 'http://$trimmed';
    var uri = Uri.parse(normalized);
    if (!uri.hasPort && uri.scheme == 'http' && uri.host == 'localhost') {
      uri = uri.replace(port: 8090);
    }
    return uri;
  }
}
