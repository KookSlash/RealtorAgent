import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:client/src/domain/listing.dart';
import 'package:client/src/domain/listings_page.dart';
import 'package:client/src/domain/listings_repository.dart';
import 'package:client/src/domain/price_history.dart';
import 'package:client/src/presentation/listings/listings_screen.dart';
import 'package:client/src/presentation/providers.dart';

class _ListingsCall {
  final int page;
  final int pageSize;

  const _ListingsCall(this.page, this.pageSize);
}

class FakeListingsRepository implements ListingsRepository {
  FakeListingsRepository({required this.total, required this.items});

  final int total;
  final List<Listing> items;
  final List<_ListingsCall> calls = [];

  @override
  Future<ListingsPage> fetchListings({
    int page = 1,
    int pageSize = 20,
    String sort = 'last_seen_desc',
    String? q,
    double? minPrice,
    double? maxPrice,
    int? minBeds,
    double? minBaths,
    int? minSqft,
    int? maxSqft,
    String? propertyType,
  }) async {
    calls.add(_ListingsCall(page, pageSize));
    return ListingsPage(
      items: items,
      page: page,
      pageSize: pageSize,
      total: total,
    );
  }

  @override
  Future<Listing> fetchListing(String propertyKey) async {
    return items.first;
  }

  @override
  Future<PriceHistory> fetchPriceHistory(String propertyKey) async {
    return PriceHistory(
      propertyKey: propertyKey,
      series: const [],
      stats: PriceHistoryStats(
        firstObservedAt: DateTime.now(),
        lastObservedAt: DateTime.now(),
        firstPrice: 0,
        lastPrice: 0,
        absChange: 0,
        pctChange: 0,
        minPrice: 0,
        maxPrice: 0,
        numObservations: 0,
        numPriceChanges: 0,
        daysSinceLastChange: 0,
        maxDrawdownPct: 0,
      ),
    );
  }
}

void main() {
  testWidgets('ListingsScreen shows loading then data', (tester) async {
    final repo = FakeListingsRepository(
      total: 1,
      items: [
        Listing(
          propertyKey: 'pk-1',
          propertyType: 'HOUSE',
          address: '1 Main St',
          unit: null,
          city: 'Calgary',
          province: 'AB',
          postalCode: 'A1A1A1',
          lat: null,
          lon: null,
          beds: 2,
          baths: 1.0,
          sqft: 800,
          currentPrice: 100000.0,
          ppsf: 125.0,
          ppsfPercentile: 10.0,
          valueScore: 90.0,
          compsCount: 3,
          url: null,
          source: 'REALTOR_CA',
          firstSeenAt: DateTime.parse('2024-01-01T00:00:00Z'),
          lastSeenAt: DateTime.parse('2024-01-01T00:00:00Z'),
          updatedAt: DateTime.parse('2024-01-01T00:00:00Z'),
        ),
      ],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          listingsRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(home: ListingsScreen()),
      ),
    );

    expect(find.byType(CircularProgressIndicator), findsOneWidget);

    await tester.pumpAndSettle();

    expect(find.text('Total listings: 1'), findsOneWidget);
    expect(find.text('1 Main St'), findsOneWidget);
  });

  testWidgets('ListingsScreen next triggers repository call', (tester) async {
    final repo = FakeListingsRepository(
      total: 40,
      items: List.generate(
        20,
        (index) => Listing(
          propertyKey: 'pk-$index',
          propertyType: 'HOUSE',
          address: 'Address $index',
          unit: null,
          city: 'Calgary',
          province: 'AB',
          postalCode: 'A1A1A1',
          lat: null,
          lon: null,
          beds: 2,
          baths: 1.0,
          sqft: 800,
          currentPrice: 100000.0,
          ppsf: 125.0,
          ppsfPercentile: 10.0,
          valueScore: 90.0,
          compsCount: 3,
          url: null,
          source: 'REALTOR_CA',
          firstSeenAt: DateTime.parse('2024-01-01T00:00:00Z'),
          lastSeenAt: DateTime.parse('2024-01-01T00:00:00Z'),
          updatedAt: DateTime.parse('2024-01-01T00:00:00Z'),
        ),
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          listingsRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(home: ListingsScreen()),
      ),
    );

    await tester.pumpAndSettle();
    expect(repo.calls.length, 1);
    expect(repo.calls.first.pageSize, 20);
    expect(repo.calls.first.page, 1);

    await tester.tap(find.text('Next'));
    await tester.pumpAndSettle();

    expect(repo.calls.length, 2);
    expect(repo.calls.last.pageSize, 20);
    expect(repo.calls.last.page, 2);
  });
}
