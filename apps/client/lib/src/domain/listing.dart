class Listing {
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

  const Listing({
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
}
