import '../../domain/price_history.dart';

class PriceHistoryDto {
  final String propertyKey;
  final List<PricePoint> series;
  final PriceHistoryStats stats;

  const PriceHistoryDto({
    required this.propertyKey,
    required this.series,
    required this.stats,
  });

  factory PriceHistoryDto.fromJson(Map<String, dynamic> json) {
    final propertyKey = json['property_key'];
    final seriesJson = json['series'];
    final statsJson = json['stats'];

    if (propertyKey is! String || seriesJson is! List || statsJson is! Map) {
      throw const FormatException('Invalid price history payload');
    }

    final series = seriesJson
        .map((item) => _parsePoint(item as Map<String, dynamic>))
        .toList(growable: false);
    final stats = _parseStats(statsJson as Map<String, dynamic>);

    return PriceHistoryDto(
      propertyKey: propertyKey,
      series: series,
      stats: stats,
    );
  }

  PriceHistory toDomain() {
    return PriceHistory(propertyKey: propertyKey, series: series, stats: stats);
  }

  static PricePoint _parsePoint(Map<String, dynamic> json) {
    final observedAt = json['observed_at'];
    final price = json['price'];
    if (observedAt is! String || price is! num) {
      throw const FormatException('Invalid price history point');
    }
    return PricePoint(
      observedAt: DateTime.parse(observedAt),
      price: price.toDouble(),
    );
  }

  static PriceHistoryStats _parseStats(Map<String, dynamic> json) {
    final firstObservedAt = json['first_observed_at'];
    final lastObservedAt = json['last_observed_at'];
    if (firstObservedAt is! String || lastObservedAt is! String) {
      throw const FormatException('Invalid price history stats timestamps');
    }
    return PriceHistoryStats(
      firstObservedAt: DateTime.parse(firstObservedAt),
      lastObservedAt: DateTime.parse(lastObservedAt),
      firstPrice: _asDouble(json['first_price']),
      lastPrice: _asDouble(json['last_price']),
      absChange: _asDouble(json['abs_change']),
      pctChange: _asDouble(json['pct_change']),
      minPrice: _asDouble(json['min_price']),
      maxPrice: _asDouble(json['max_price']),
      numObservations: _asInt(json['num_observations']),
      numPriceChanges: _asInt(json['num_price_changes']),
      daysSinceLastChange: _asInt(json['days_since_last_change']),
      maxDrawdownPct: _asDouble(json['max_drawdown_pct']),
    );
  }

  static double _asDouble(dynamic value) {
    if (value is num) {
      return value.toDouble();
    }
    throw const FormatException('Invalid numeric value');
  }

  static int _asInt(dynamic value) {
    if (value is num) {
      return value.toInt();
    }
    throw const FormatException('Invalid int value');
  }
}
