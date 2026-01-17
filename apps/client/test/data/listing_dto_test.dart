import 'package:flutter_test/flutter_test.dart';

import 'package:client/src/data/dto/listing_dto.dart';
import 'package:client/src/data/dto/listings_response_dto.dart';

void main() {
  test('ListingDto parses full payload', () {
    final dto = ListingDto.fromJson({
      'property_key': 'pk-123',
      'property_type': 'HOUSE',
      'address': '123 Test St',
      'unit': 'Unit 1',
      'city': 'Calgary',
      'province': 'AB',
      'postal_code': 'T1T1T1',
      'lat': 51.0,
      'lon': -114.0,
      'current_price': 500000.0,
      'beds': 3,
      'baths': 2.5,
      'sqft': 1200,
      'ppsf': 416.7,
      'ppsf_percentile': 10.0,
      'value_score': 90.0,
      'comps_count': 8,
      'url': 'https://example.com/listing',
      'source': 'REALTOR_CA',
      'first_seen_at': '2024-01-01T10:00:00Z',
      'last_seen_at': '2024-01-02T10:00:00Z',
      'updated_at': '2024-01-02T10:00:00Z',
    });

    final listing = dto.toDomain();
    expect(listing.propertyKey, 'pk-123');
    expect(listing.propertyType, 'HOUSE');
    expect(listing.address, '123 Test St');
    expect(listing.postalCode, 'T1T1T1');
    expect(listing.currentPrice, 500000.0);
    expect(listing.beds, 3);
    expect(listing.baths, 2.5);
    expect(listing.sqft, 1200);
    expect(listing.url, 'https://example.com/listing');
    expect(listing.firstSeenAt.toUtc().toIso8601String(),
        '2024-01-01T10:00:00.000Z');
  });

  test('ListingDto parses missing optional fields', () {
    final dto = ListingDto.fromJson({
      'property_key': 'pk-456',
      'property_type': 'CONDO',
      'address': '456 Example Ave',
      'postal_code': '',
      'current_price': null,
      'beds': null,
      'baths': null,
      'sqft': null,
      'url': null,
      'ppsf': null,
      'ppsf_percentile': null,
      'value_score': null,
      'comps_count': 0,
      'source': 'REALTOR_CA',
      'first_seen_at': '2024-01-02T12:30:00Z',
      'last_seen_at': '2024-01-02T12:30:00Z',
      'updated_at': '2024-01-02T12:30:00Z',
    });

    final listing = dto.toDomain();
    expect(listing.currentPrice, isNull);
    expect(listing.beds, isNull);
    expect(listing.baths, isNull);
    expect(listing.sqft, isNull);
    expect(listing.url, isNull);
  });

  test('ListingsResponseDto parses envelope', () {
    final dto = ListingsResponseDto.fromJson({
      'items': [
        {
          'property_key': 'pk-1',
          'property_type': 'HOUSE',
          'address': '1 Main St',
          'postal_code': 'A1A1A1',
          'current_price': 100000,
          'beds': 2,
          'baths': 1.0,
          'sqft': 800,
          'ppsf': 125.0,
          'ppsf_percentile': 20.0,
          'value_score': 80.0,
          'comps_count': 5,
          'url': null,
          'source': 'REALTOR_CA',
          'first_seen_at': '2024-01-01T00:00:00Z',
          'last_seen_at': '2024-01-01T00:00:00Z',
          'updated_at': '2024-01-01T00:00:00Z',
        }
      ],
      'page': 1,
      'page_size': 20,
      'total': 1,
    });

    expect(dto.items.length, 1);
    expect(dto.page, 1);
    expect(dto.pageSize, 20);
    expect(dto.total, 1);
  });
}
