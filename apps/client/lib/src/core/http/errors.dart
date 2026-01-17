class ApiError implements Exception {
  final int statusCode;
  final String body;

  ApiError({required this.statusCode, required this.body});

  @override
  String toString() => 'ApiError(statusCode: $statusCode, body: $body)';
}

class DecodeError implements Exception {
  final String message;

  DecodeError(this.message);

  @override
  String toString() => 'DecodeError($message)';
}
