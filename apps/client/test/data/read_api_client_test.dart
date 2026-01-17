import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:client/src/core/config/app_config.dart';
import 'package:client/src/data/read_api_client.dart';

void main() {
  test('fetchListings builds query and parses browse response', () async {
    final client = MockClient((request) async {
      expect(request.url.path, '/v1/listings');
      expect(request.url.queryParameters['page'], '2');
      expect(request.url.queryParameters['page_size'], '25');
      expect(request.url.queryParameters['sort'], 'value_desc');
      expect(request.url.queryParameters['q'], 'Main');
      expect(request.url.queryParameters['min_price'], '100000.0');
      expect(request.url.queryParameters['property_type'], 'HOUSE');
      return http.Response('''{
        "items": [
          {
            "property_key": "pk-1",
            "property_type": "HOUSE",
            "address": "1 Main St",
            "unit": null,
            "city": "Calgary",
            "province": "AB",
            "postal_code": "A1A1A1",
            "lat": 51.0,
            "lon": -114.0,
            "beds": 2,
            "baths": 1.0,
            "sqft": 800,
            "current_price": 100000,
            "ppsf": 125,
            "ppsf_percentile": 10.0,
            "value_score": 90.0,
            "comps_count": 3,
            "url": null,
            "source": "REALTOR_CA",
            "first_seen_at": "2024-01-01T00:00:00Z",
            "last_seen_at": "2024-01-02T00:00:00Z",
            "updated_at": "2024-01-02T00:00:00Z"
          }
        ],
        "page": 2,
        "page_size": 25,
        "total": 1
      }''', 200);
    });

    final api = ReadApiClient(
      config: AppConfig.fromBaseUrl('http://localhost:8090'),
      httpClient: client,
    );

    final response = await api.fetchListings(
      page: 2,
      pageSize: 25,
      sort: 'value_desc',
      q: 'Main',
      minPrice: 100000.0,
      propertyType: 'house',
    );
    expect(response.items.length, 1);
    expect(response.page, 2);
    expect(response.pageSize, 25);
    expect(response.total, 1);
  });

  test('fetchListing parses detail response', () async {
    final client = MockClient((request) async {
      expect(request.url.path, '/v1/listings/pk-1');
      return http.Response('''{
        "property_key": "pk-1",
        "property_type": "HOUSE",
        "address": "1 Main St",
        "unit": null,
        "city": "Calgary",
        "province": "AB",
        "postal_code": "A1A1A1",
        "lat": 51.0,
        "lon": -114.0,
        "beds": 2,
        "baths": 1.0,
        "sqft": 800,
        "current_price": 100000,
        "ppsf": 125,
        "ppsf_percentile": 10.0,
        "value_score": 90.0,
        "comps_count": 3,
        "url": null,
        "source": "REALTOR_CA",
        "first_seen_at": "2024-01-01T00:00:00Z",
        "last_seen_at": "2024-01-02T00:00:00Z",
        "updated_at": "2024-01-02T00:00:00Z"
      }''', 200);
    });

    final api = ReadApiClient(
      config: AppConfig.fromBaseUrl('http://localhost:8090'),
      httpClient: client,
    );

    final listing = await api.fetchListing('pk-1');
    expect(listing.propertyKey, 'pk-1');
    expect(listing.propertyType, 'HOUSE');
  });
}
