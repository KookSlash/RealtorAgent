import 'dart:convert';

import 'package:http/http.dart' as http;

import '../core/config/app_config.dart';
import '../core/http/errors.dart';
import 'dto/listings_response_dto.dart';

class ReadApiClient {
  final AppConfig config;
  final http.Client httpClient;

  ReadApiClient({required this.config, required this.httpClient});

  Future<int> fetchListingsCount() async {
    final uri = _buildUri('/v1/listings/count');
    final response = await httpClient.get(uri);
    if (response.statusCode != 200) {
      throw ApiError(statusCode: response.statusCode, body: response.body);
    }

    final data = _decodeJson(response.body);
    final count = data['count'];
    if (count is! num) {
      throw DecodeError('Invalid count field');
    }
    return count.toInt();
  }

  Future<ListingsResponseDto> fetchListings(
      {int limit = 20, int offset = 0}) async {
    final uri = _buildUri(
      '/v1/listings',
      {
        'limit': limit.toString(),
        'offset': offset.toString(),
      },
    );
    final response = await httpClient.get(uri);
    if (response.statusCode != 200) {
      throw ApiError(statusCode: response.statusCode, body: response.body);
    }

    final data = _decodeJson(response.body);
    return ListingsResponseDto.fromJson(data);
  }

  Uri _buildUri(String path, [Map<String, String>? queryParameters]) {
    final base = config.apiBaseUrl;
    final basePath = base.path.endsWith('/') ? base.path : '${base.path}/';
    final baseWithSlash = base.replace(path: basePath);
    final resolved =
        baseWithSlash.resolve(path.startsWith('/') ? path.substring(1) : path);
    if (queryParameters == null) {
      return resolved;
    }
    return resolved.replace(queryParameters: queryParameters);
  }

  Map<String, dynamic> _decodeJson(String body) {
    try {
      final decoded = jsonDecode(body);
      if (decoded is! Map<String, dynamic>) {
        throw const FormatException('Expected JSON object');
      }
      return decoded;
    } catch (err) {
      throw DecodeError('Invalid JSON response: $err');
    }
  }
}
