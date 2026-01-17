class Listing {
  final String propertyKey;
  final String address;
  final String postalCode;
  final double? price;
  final int? beds;
  final double? baths;
  final int? sqft;
  final String? url;
  final DateTime scrapedAt;

  const Listing({
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
}
