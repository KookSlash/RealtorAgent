import '../../domain/listing.dart';

class ListingDto {
  final String propertyKey;
  final String address;
  final String postalCode;
  final double? price;
  final int? beds;
  final double? baths;
  final int? sqft;
  final String? url;
  final DateTime scrapedAt;

  const ListingDto({
    required this.propertyKey,
    required this.address,
    required this.postalCode,
    required this.price,
    required this.beds,
    required this.baths,
    required this.sqft,
    required this.url,
    required this.scrapedAt,
  });

  factory ListingDto.fromJson(Map<String, dynamic> json) {
    final propertyKey = json['property_key'];
    final address = json['address'];
    final postalCode = json['postal_code'];
    final scrapedAt = json['scraped_at'];

    if (propertyKey is! String || address is! String || postalCode is! String) {
      throw const FormatException('Invalid listing required fields');
    }
    if (scrapedAt is! String) {
      throw const FormatException('Invalid scraped_at');
    }

    return ListingDto(
      propertyKey: propertyKey,
      address: address,
      postalCode: postalCode,
      price: _asDouble(json['price']),
      beds: _asInt(json['beds']),
      baths: _asDouble(json['baths']),
      sqft: _asInt(json['sqft']),
      url: json['url'] is String ? json['url'] as String : null,
      scrapedAt: DateTime.parse(scrapedAt),
    );
  }

  Listing toDomain() {
    return Listing(
      propertyKey: propertyKey,
      address: address,
      postalCode: postalCode,
      price: price,
      beds: beds,
      baths: baths,
      sqft: sqft,
      url: url,
      scrapedAt: scrapedAt,
    );
  }

  static double? _asDouble(dynamic value) {
    if (value == null) {
      return null;
    }
    if (value is num) {
      return value.toDouble();
    }
    throw const FormatException('Invalid numeric value');
  }

  static int? _asInt(dynamic value) {
    if (value == null) {
      return null;
    }
    if (value is num) {
      return value.toInt();
    }
    throw const FormatException('Invalid int value');
  }
}
