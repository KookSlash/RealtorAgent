import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/listing.dart';
import '../providers.dart';
import 'listings_controller.dart';

class ListingsScreen extends ConsumerWidget {
  const ListingsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(listingsControllerProvider);
    final controller = ref.read(listingsControllerProvider.notifier);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Listings'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Total listings: ${state.count}'),
            const SizedBox(height: 12),
            Expanded(
              child: _buildBody(state),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                ElevatedButton(
                  onPressed: state.canPrev ? controller.prevPage : null,
                  child: const Text('Prev'),
                ),
                const SizedBox(width: 12),
                ElevatedButton(
                  onPressed: state.canNext ? controller.nextPage : null,
                  child: const Text('Next'),
                ),
                const Spacer(),
                Text(_rangeLabel(state)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBody(ListingsState state) {
    if (state.isLoading && state.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.errorMessage != null) {
      return Center(child: Text(state.errorMessage!));
    }
    if (state.items.isEmpty) {
      return const Center(child: Text('No listings found.'));
    }

    return ListView.separated(
      itemCount: state.items.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final item = state.items[index];
        return ListTile(
          title: Text(item.address),
          subtitle: Text(_subtitle(item)),
          trailing: Text(_formatPrice(item.price)),
        );
      },
    );
  }

  String _subtitle(Listing item) {
    final beds = item.beds?.toString() ?? 'n/a';
    final baths = item.baths?.toStringAsFixed(1) ?? 'n/a';
    final sqft = item.sqft?.toString() ?? 'n/a';
    final postal = item.postalCode.isEmpty ? 'no postal' : item.postalCode;
    return 'Beds $beds | Baths $baths | Sqft $sqft | $postal';
  }

  String _formatPrice(double? price) {
    if (price == null) {
      return 'N/A';
    }
    return '\$${price.toStringAsFixed(0)}';
  }

  String _rangeLabel(ListingsState state) {
    final start = state.items.isEmpty ? 0 : state.offset + 1;
    final end = state.offset + state.items.length;
    return '$start-$end of ${state.count}';
  }
}
