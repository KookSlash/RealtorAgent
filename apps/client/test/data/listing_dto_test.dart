import 'package:flutter_test/flutter_test.dart';

import 'package:client/src/data/dto/listing_dto.dart';
import 'package:client/src/data/dto/listings_response_dto.dart';

void main() {
  test('ListingDto parses full payload', () {
    final dto = ListingDto.fromJson({
      'property_key': 'pk-123',
      'address': '123 Test St',
      'postal_code': 'T1T1T1',
      'price': 500000.0,
      'beds': 3,
      'baths': 2.5,
      'sqft': 1200,
      'url': 'https://example.com/listing',
      'scraped_at': '2024-01-01T10:00:00Z',
    });

    final listing = dto.toDomain();
    expect(listing.propertyKey, 'pk-123');
    expect(listing.address, '123 Test St');
    expect(listing.postalCode, 'T1T1T1');
    expect(listing.price, 500000.0);
    expect(listing.beds, 3);
    expect(listing.baths, 2.5);
    expect(listing.sqft, 1200);
    expect(listing.url, 'https://example.com/listing');
    expect(listing.scrapedAt.toUtc().toIso8601String(),
        '2024-01-01T10:00:00.000Z');
  });

  test('ListingDto parses missing optional fields', () {
    final dto = ListingDto.fromJson({
      'property_key': 'pk-456',
      'address': '456 Example Ave',
      'postal_code': '',
      'price': null,
      'beds': null,
      'baths': null,
      'sqft': null,
      'url': null,
      'scraped_at': '2024-01-02T12:30:00Z',
    });

    final listing = dto.toDomain();
    expect(listing.price, isNull);
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
          'address': '1 Main St',
          'postal_code': 'A1A1A1',
          'price': 100000,
          'beds': 2,
          'baths': 1.0,
          'sqft': 800,
          'url': null,
          'scraped_at': '2024-01-01T00:00:00Z',
        }
      ],
      'limit': 20,
      'offset': 0,
      'returned': 1,
    });

    expect(dto.items.length, 1);
    expect(dto.limit, 20);
    expect(dto.offset, 0);
    expect(dto.returned, 1);
  });
}
