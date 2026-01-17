import 'dart:convert';

import 'package:http/http.dart' as http;

import '../core/config/app_config.dart';
import '../core/http/errors.dart';
import 'dto/listing_dto.dart';
import 'dto/listings_response_dto.dart';
import 'dto/price_history_dto.dart';

class ReadApiClient {
  final AppConfig config;
  final http.Client httpClient;

  ReadApiClient({required this.config, required this.httpClient});

  Future<ListingsResponseDto> fetchListings(
      {int page = 1,
      int pageSize = 20,
      String sort = 'last_seen_desc',
      String? q,
      double? minPrice,
      double? maxPrice,
      int? minBeds,
      double? minBaths,
      int? minSqft,
      int? maxSqft,
      String? propertyType}) async {
    final params = <String, String>{
      'page': page.toString(),
      'page_size': pageSize.toString(),
      'sort': sort,
    };
    _addIfNotEmpty(params, 'q', q);
    _addIfNotEmpty(params, 'min_price', minPrice?.toString());
    _addIfNotEmpty(params, 'max_price', maxPrice?.toString());
    _addIfNotEmpty(params, 'min_beds', minBeds?.toString());
    _addIfNotEmpty(params, 'min_baths', minBaths?.toString());
    _addIfNotEmpty(params, 'min_sqft', minSqft?.toString());
    _addIfNotEmpty(params, 'max_sqft', maxSqft?.toString());
    if (propertyType != null && propertyType.trim().isNotEmpty) {
      params['property_type'] = propertyType.trim().toUpperCase();
    }

    final uri = _buildUri('/v1/listings', params);
    final response = await httpClient.get(uri);
    if (response.statusCode != 200) {
      throw ApiError(statusCode: response.statusCode, body: response.body);
    }

    final data = _decodeJson(response.body);
    return ListingsResponseDto.fromJson(data);
  }

  Future<ListingDto> fetchListing(String propertyKey) async {
    final encoded = Uri.encodeComponent(propertyKey);
    final uri = _buildUri('/v1/listings/$encoded');
    final response = await httpClient.get(uri);
    if (response.statusCode != 200) {
      throw ApiError(statusCode: response.statusCode, body: response.body);
    }

    final data = _decodeJson(response.body);
    return ListingDto.fromJson(data);
  }

  Future<PriceHistoryDto> fetchPriceHistory(String propertyKey) async {
    final encoded = Uri.encodeComponent(propertyKey);
    final uri = _buildUri('/v1/listings/$encoded/price-history');
    final response = await httpClient.get(uri);
    if (response.statusCode != 200) {
      throw ApiError(statusCode: response.statusCode, body: response.body);
    }

    final data = _decodeJson(response.body);
    return PriceHistoryDto.fromJson(data);
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

  void _addIfNotEmpty(
      Map<String, String> params, String key, String? value) {
    if (value == null) {
      return;
    }
    final trimmed = value.trim();
    if (trimmed.isEmpty) {
      return;
    }
    params[key] = trimmed;
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
