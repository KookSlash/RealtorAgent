import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:client/src/domain/listing.dart';
import 'package:client/src/domain/listings_page.dart';
import 'package:client/src/domain/listings_repository.dart';
import 'package:client/src/presentation/listings/listings_screen.dart';
import 'package:client/src/presentation/providers.dart';

class _ListingsCall {
  final int limit;
  final int offset;

  const _ListingsCall(this.limit, this.offset);
}

class FakeListingsRepository implements ListingsRepository {
  FakeListingsRepository({required this.count, required this.items});

  final int count;
  final List<Listing> items;
  final List<_ListingsCall> calls = [];

  @override
  Future<int> fetchCount() async => count;

  @override
  Future<ListingsPage> fetchListings({int limit = 20, int offset = 0}) async {
    calls.add(_ListingsCall(limit, offset));
    return ListingsPage(
      items: items,
      limit: limit,
      offset: offset,
      returned: items.length,
    );
  }
}

void main() {
  testWidgets('ListingsScreen shows loading then data', (tester) async {
    final repo = FakeListingsRepository(
      count: 1,
      items: [
        Listing(
          propertyKey: 'pk-1',
          address: '1 Main St',
          postalCode: 'A1A1A1',
          price: 100000,
          beds: 2,
          baths: 1.0,
          sqft: 800,
          url: null,
          scrapedAt: DateTime.parse('2024-01-01T00:00:00Z'),
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
      count: 40,
      items: List.generate(
        20,
        (index) => Listing(
          propertyKey: 'pk-$index',
          address: 'Address $index',
          postalCode: 'A1A1A1',
          price: 100000,
          beds: 2,
          baths: 1.0,
          sqft: 800,
          url: null,
          scrapedAt: DateTime.parse('2024-01-01T00:00:00Z'),
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
    expect(repo.calls.first.limit, 20);
    expect(repo.calls.first.offset, 0);

    await tester.tap(find.text('Next'));
    await tester.pumpAndSettle();

    expect(repo.calls.length, 2);
    expect(repo.calls.last.limit, 20);
    expect(repo.calls.last.offset, 20);
  });
}
