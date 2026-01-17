class PricePoint {
  final DateTime observedAt;
  final double price;

  const PricePoint({required this.observedAt, required this.price});
}

class PriceHistoryStats {
  final DateTime firstObservedAt;
  final DateTime lastObservedAt;
  final double firstPrice;
  final double lastPrice;
  final double absChange;
  final double pctChange;
  final double minPrice;
  final double maxPrice;
  final int numObservations;
  final int numPriceChanges;
  final int daysSinceLastChange;
  final double maxDrawdownPct;

  const PriceHistoryStats({
    required this.firstObservedAt,
    required this.lastObservedAt,
    required this.firstPrice,
    required this.lastPrice,
    required this.absChange,
    required this.pctChange,
    required this.minPrice,
    required this.maxPrice,
    required this.numObservations,
    required this.numPriceChanges,
    required this.daysSinceLastChange,
    required this.maxDrawdownPct,
  });
}

class PriceHistory {
  final String propertyKey;
  final List<PricePoint> series;
  final PriceHistoryStats stats;

  const PriceHistory({
    required this.propertyKey,
    required this.series,
    required this.stats,
  });
}
