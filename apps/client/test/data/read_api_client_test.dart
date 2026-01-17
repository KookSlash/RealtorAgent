import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:client/src/core/config/app_config.dart';
import 'package:client/src/data/read_api_client.dart';

void main() {
  test('fetchListingsCount builds URL and parses count', () async {
    final client = MockClient((request) async {
      expect(request.url.toString(), 'http://localhost:8090/v1/listings/count');
      return http.Response('{"count":3}', 200);
    });

    final api = ReadApiClient(
      config: AppConfig.fromBaseUrl('http://localhost:8090'),
      httpClient: client,
    );

    final count = await api.fetchListingsCount();
    expect(count, 3);
  });

  test('fetchListingsCount handles base URL trailing slash', () async {
    final client = MockClient((request) async {
      expect(request.url.toString(), 'http://localhost:8090/v1/listings/count');
      return http.Response('{"count":2}', 200);
    });

    final api = ReadApiClient(
      config: AppConfig.fromBaseUrl('http://localhost:8090/'),
      httpClient: client,
    );

    final count = await api.fetchListingsCount();
    expect(count, 2);
  });

  test('fetchListings parses browse response', () async {
    final client = MockClient((request) async {
      expect(request.url.toString(),
          'http://localhost:8090/v1/listings?limit=20&offset=0');
      return http.Response('''{
        "items": [
          {
            "property_key": "pk-1",
            "address": "1 Main St",
            "postal_code": "A1A1A1",
            "price": 100000,
            "beds": 2,
            "baths": 1.0,
            "sqft": 800,
            "url": null,
            "scraped_at": "2024-01-01T00:00:00Z"
          }
        ],
        "limit": 20,
        "offset": 0,
        "returned": 1
      }''', 200);
    });

    final api = ReadApiClient(
      config: AppConfig.fromBaseUrl('http://localhost:8090'),
      httpClient: client,
    );

    final response = await api.fetchListings();
    expect(response.items.length, 1);
    expect(response.limit, 20);
    expect(response.offset, 0);
    expect(response.returned, 1);
  });
}
