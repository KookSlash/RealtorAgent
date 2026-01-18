import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../domain/listing.dart';
import '../../domain/price_history.dart';
import '../providers.dart';

class ListingDetailScreen extends ConsumerStatefulWidget {
  final String propertyKey;

  const ListingDetailScreen({super.key, required this.propertyKey});

  @override
  ConsumerState<ListingDetailScreen> createState() =>
      _ListingDetailScreenState();
}

class _ListingDetailScreenState extends ConsumerState<ListingDetailScreen> {
  late Future<_DetailPayload> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Listing Detail'),
      ),
      body: FutureBuilder<_DetailPayload>(
        future: _future,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Text('Failed to load listing: ${snapshot.error}'),
              ),
            );
          }
          final data = snapshot.data;
          if (data == null) {
            return const Center(child: Text('Listing not found.'));
          }

          return _DetailBody(
            listing: data.listing,
            history: data.history,
            onOpenUrl: _openUrl,
          );
        },
      ),
    );
  }

  Future<_DetailPayload> _load() async {
    final repo = ref.read(listingsRepositoryProvider);
    final listing = await repo.fetchListing(widget.propertyKey);
    final history = await repo.fetchPriceHistory(widget.propertyKey);
    return _DetailPayload(listing: listing, history: history);
  }

  Future<void> _openUrl(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) {
      return;
    }
    await launchUrl(uri, mode: LaunchMode.platformDefault);
  }
}

class _DetailBody extends StatelessWidget {
  final Listing listing;
  final PriceHistory history;
  final Future<void> Function(String) onOpenUrl;

  const _DetailBody({
    required this.listing,
    required this.history,
    required this.onOpenUrl,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            listing.address,
            style: theme.textTheme.headlineSmall,
          ),
          const SizedBox(height: 4),
          Text(_secondaryLine(listing)),
          const SizedBox(height: 8),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              _InfoChip(label: 'Price', value: _formatPrice(listing)),
              _InfoChip(label: 'Beds', value: _formatBeds(listing)),
              _InfoChip(label: 'Baths', value: _formatBaths(listing)),
              _InfoChip(label: 'Sqft', value: _formatSqft(listing)),
              _InfoChip(label: 'PPSF', value: _formatPpsf(listing)),
              _InfoChip(label: 'Value', value: _formatValue(listing)),
              _InfoChip(
                label: 'Comps',
                value: listing.compsCount.toString(),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Text('Last seen: ${_formatDate(listing.lastSeenAt)}'),
          const SizedBox(height: 12),
          if (listing.url != null)
            ElevatedButton.icon(
              onPressed: () => onOpenUrl(listing.url!),
              icon: const Icon(Icons.open_in_new),
              label: const Text('Open on Zolo'),
            ),
          const SizedBox(height: 24),
          Text(
            'Price history',
            style: theme.textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          if (history.series.isEmpty)
            const Text('No price history available.')
          else ...[
            SizedBox(
              height: 220,
              child: _PriceHistoryChart(series: history.series),
            ),
            const SizedBox(height: 8),
            Text(
              'From ${_formatDate(history.series.first.observedAt)} to '
              '${_formatDate(history.series.last.observedAt)}',
            ),
          ],
          const SizedBox(height: 16),
          Text(
            'Stats',
            style: theme.textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          _StatsGrid(stats: history.stats),
        ],
      ),
    );
  }

  String _secondaryLine(Listing listing) {
    final parts = <String>[
      listing.propertyType,
      if (listing.city != null && listing.city!.isNotEmpty) listing.city!,
      if (listing.province != null && listing.province!.isNotEmpty)
        listing.province!,
      listing.postalCode,
    ];
    return parts.join(' | ');
  }

  String _formatPrice(Listing listing) {
    final price = listing.currentPrice;
    if (price == null) {
      return 'N/A';
    }
    return '\$${price.toStringAsFixed(0)}';
  }

  String _formatBeds(Listing listing) {
    return listing.beds?.toString() ?? 'n/a';
  }

  String _formatBaths(Listing listing) {
    return listing.baths?.toStringAsFixed(1) ?? 'n/a';
  }

  String _formatSqft(Listing listing) {
    return listing.sqft?.toString() ?? 'n/a';
  }

  String _formatPpsf(Listing listing) {
    return listing.ppsf != null ? listing.ppsf!.toStringAsFixed(2) : 'n/a';
  }

  String _formatValue(Listing listing) {
    return listing.valueScore != null
        ? listing.valueScore!.toStringAsFixed(2)
        : 'n/a';
  }

  String _formatDate(DateTime value) {
    final local = value.toLocal();
    final date = local.toIso8601String();
    return date.split('.').first.replaceFirst('T', ' ');
  }
}

class _InfoChip extends StatelessWidget {
  final String label;
  final String value;

  const _InfoChip({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.blueGrey.withOpacity(0.12),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text('$label: $value'),
    );
  }
}

class _StatsGrid extends StatelessWidget {
  final PriceHistoryStats stats;

  const _StatsGrid({required this.stats});

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 12,
      runSpacing: 8,
      children: [
        _InfoChip(label: 'First price', value: _formatPrice(stats.firstPrice)),
        _InfoChip(label: 'Last price', value: _formatPrice(stats.lastPrice)),
        _InfoChip(label: 'Abs change', value: _formatPrice(stats.absChange)),
        _InfoChip(
          label: 'Pct change',
          value: '${stats.pctChange.toStringAsFixed(2)}%',
        ),
        _InfoChip(label: 'Min price', value: _formatPrice(stats.minPrice)),
        _InfoChip(label: 'Max price', value: _formatPrice(stats.maxPrice)),
        _InfoChip(
          label: 'Price changes',
          value: stats.numPriceChanges.toString(),
        ),
        _InfoChip(
          label: 'Days since change',
          value: stats.numPriceChanges == 0
              ? 'n/a'
              : stats.daysSinceLastChange.toString(),
        ),
        _InfoChip(
          label: 'Max drawdown',
          value: '${stats.maxDrawdownPct.toStringAsFixed(2)}%',
        ),
      ],
    );
  }

  String _formatPrice(double value) {
    return '\$${value.toStringAsFixed(0)}';
  }
}

class _PriceHistoryChart extends StatelessWidget {
  final List<PricePoint> series;

  const _PriceHistoryChart({required this.series});

  @override
  Widget build(BuildContext context) {
    final spots = <FlSpot>[];
    for (var i = 0; i < series.length; i++) {
      spots.add(FlSpot(i.toDouble(), series[i].price));
    }
    final maxX = spots.isEmpty ? 0.0 : spots.length.toDouble() - 1;
    return LineChart(
      LineChartData(
        minX: 0,
        maxX: maxX,
        gridData: const FlGridData(show: true),
        titlesData: const FlTitlesData(show: false),
        borderData: FlBorderData(show: false),
        lineBarsData: [
          LineChartBarData(
            spots: spots,
            isCurved: true,
            barWidth: 2,
            color: Colors.blueGrey,
            dotData: const FlDotData(show: false),
          ),
        ],
      ),
    );
  }
}

class _DetailPayload {
  final Listing listing;
  final PriceHistory history;

  const _DetailPayload({required this.listing, required this.history});
}
