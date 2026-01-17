import '../../domain/listing.dart';

class ListingDto {
  final String propertyKey;
  final String propertyType;
  final String address;
  final String? unit;
  final String? city;
  final String? province;
  final String postalCode;
  final double? lat;
  final double? lon;
  final int? beds;
  final double? baths;
  final int? sqft;
  final double? currentPrice;
  final double? ppsf;
  final double? ppsfPercentile;
  final double? valueScore;
  final int compsCount;
  final String? url;
  final String source;
  final DateTime firstSeenAt;
  final DateTime lastSeenAt;
  final DateTime updatedAt;

  const ListingDto({
    required this.propertyKey,
    required this.propertyType,
    required this.address,
    required this.unit,
    required this.city,
    required this.province,
    required this.postalCode,
    required this.lat,
    required this.lon,
    required this.beds,
    required this.baths,
    required this.sqft,
    required this.currentPrice,
    required this.ppsf,
    required this.ppsfPercentile,
    required this.valueScore,
    required this.compsCount,
    required this.url,
    required this.source,
    required this.firstSeenAt,
    required this.lastSeenAt,
    required this.updatedAt,
  });

  factory ListingDto.fromJson(Map<String, dynamic> json) {
    final propertyKey = json['property_key'];
    final propertyType = json['property_type'];
    final address = json['address'];
    final postalCode = json['postal_code'];
    final source = json['source'];
    final firstSeenAt = json['first_seen_at'];
    final lastSeenAt = json['last_seen_at'];
    final updatedAt = json['updated_at'];
    final compsCount = json['comps_count'];

    if (propertyKey is! String ||
        propertyType is! String ||
        address is! String ||
        postalCode is! String ||
        source is! String) {
      throw const FormatException('Invalid listing required fields');
    }
    if (firstSeenAt is! String ||
        lastSeenAt is! String ||
        updatedAt is! String) {
      throw const FormatException('Invalid listing timestamp');
    }
    if (compsCount is! num) {
      throw const FormatException('Invalid comps_count');
    }

    return ListingDto(
      propertyKey: propertyKey,
      propertyType: propertyType,
      address: address,
      unit: _asString(json['unit']),
      city: _asString(json['city']),
      province: _asString(json['province']),
      postalCode: postalCode,
      lat: _asDouble(json['lat']),
      lon: _asDouble(json['lon']),
      beds: _asInt(json['beds']),
      baths: _asDouble(json['baths']),
      sqft: _asInt(json['sqft']),
      currentPrice: _asDouble(json['current_price']),
      ppsf: _asDouble(json['ppsf']),
      ppsfPercentile: _asDouble(json['ppsf_percentile']),
      valueScore: _asDouble(json['value_score']),
      compsCount: compsCount.toInt(),
      url: _asString(json['url']),
      source: source,
      firstSeenAt: DateTime.parse(firstSeenAt),
      lastSeenAt: DateTime.parse(lastSeenAt),
      updatedAt: DateTime.parse(updatedAt),
    );
  }

  Listing toDomain() {
    return Listing(
      propertyKey: propertyKey,
      propertyType: propertyType,
      address: address,
      unit: unit,
      city: city,
      province: province,
      postalCode: postalCode,
      lat: lat,
      lon: lon,
      beds: beds,
      baths: baths,
      sqft: sqft,
      currentPrice: currentPrice,
      ppsf: ppsf,
      ppsfPercentile: ppsfPercentile,
      valueScore: valueScore,
      compsCount: compsCount,
      url: url,
      source: source,
      firstSeenAt: firstSeenAt,
      lastSeenAt: lastSeenAt,
      updatedAt: updatedAt,
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

  static String? _asString(dynamic value) {
    if (value == null) {
      return null;
    }
    if (value is String) {
      return value;
    }
    throw const FormatException('Invalid string value');
  }
}
