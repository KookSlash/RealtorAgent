import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/listing.dart';
import '../providers.dart';
import 'listings_controller.dart';

class ListingsScreen extends ConsumerStatefulWidget {
  const ListingsScreen({super.key});

  @override
  ConsumerState<ListingsScreen> createState() => _ListingsScreenState();
}

class _ListingsScreenState extends ConsumerState<ListingsScreen> {
  final _qController = TextEditingController();
  final _minPriceController = TextEditingController();
  final _maxPriceController = TextEditingController();
  final _minBedsController = TextEditingController();

  String _sort = 'last_seen_desc';
  String? _propertyType;

  @override
  void dispose() {
    _qController.dispose();
    _minPriceController.dispose();
    _maxPriceController.dispose();
    _minBedsController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
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
            _FiltersBar(
              qController: _qController,
              minPriceController: _minPriceController,
              maxPriceController: _maxPriceController,
              minBedsController: _minBedsController,
              sort: _sort,
              propertyType: _propertyType,
              onSortChanged: (value) {
                setState(() {
                  _sort = value;
                });
                _applyFilters(controller);
              },
              onPropertyTypeChanged: (value) {
                setState(() {
                  _propertyType = value;
                });
              },
              onApply: () => _applyFilters(controller),
              onClear: () => _clearFilters(controller),
            ),
            const SizedBox(height: 12),
            Text('Total listings: ${state.total}'),
            const SizedBox(height: 12),
            if (state.errorMessage != null)
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(8),
                margin: const EdgeInsets.only(bottom: 8),
                decoration: BoxDecoration(
                  color: Colors.red.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  state.errorMessage!,
                  style: const TextStyle(color: Colors.red),
                ),
              ),
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

  void _applyFilters(ListingsController controller) {
    controller.applyFilters(
      q: _qController.text,
      minPrice: _minPriceController.text,
      maxPrice: _maxPriceController.text,
      minBeds: _minBedsController.text,
      propertyType: _propertyType,
      sort: _sort,
    );
  }

  void _clearFilters(ListingsController controller) {
    _qController.clear();
    _minPriceController.clear();
    _maxPriceController.clear();
    _minBedsController.clear();
    setState(() {
      _propertyType = null;
      _sort = 'last_seen_desc';
    });
    controller.applyFilters(
      q: null,
      minPrice: null,
      maxPrice: null,
      minBeds: null,
      propertyType: null,
      sort: _sort,
    );
  }

  Widget _buildBody(ListingsState state) {
    if (state.isLoading && state.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }
    if (state.items.isEmpty) {
      if (state.isLoading) {
        return const Center(child: CircularProgressIndicator());
      }
      return const Center(child: Text('No listings found.'));
    }

    return ListView.separated(
      itemCount: state.items.length,
      separatorBuilder: (_, __) => const SizedBox(height: 8),
      itemBuilder: (context, index) {
        final item = state.items[index];
        return Card(
          margin: EdgeInsets.zero,
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.address,
                  style: const TextStyle(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 4),
                Text(_primaryLine(item)),
                const SizedBox(height: 4),
                Text(_scoreLine(item)),
                const SizedBox(height: 4),
                Text('Last seen: ${_formatDate(item.lastSeenAt)}'),
              ],
            ),
          ),
        );
      },
    );
  }

  String _primaryLine(Listing item) {
    final price = _formatPrice(item.currentPrice);
    final beds = item.beds?.toString() ?? 'n/a';
    final baths = item.baths?.toStringAsFixed(1) ?? 'n/a';
    final sqft = item.sqft?.toString() ?? 'n/a';
    final postal = item.postalCode.isEmpty ? 'no postal' : item.postalCode;
    return '$price • Beds $beds • Baths $baths • Sqft $sqft • $postal';
  }

  String _scoreLine(Listing item) {
    final ppsf = item.ppsf != null ? item.ppsf!.toStringAsFixed(2) : 'n/a';
    final valueScore =
        item.valueScore != null ? item.valueScore!.toStringAsFixed(2) : 'n/a';
    return 'PPSF $ppsf • Value $valueScore • Comps ${item.compsCount}';
  }

  String _formatPrice(double? price) {
    if (price == null) {
      return 'N/A';
    }
    return '\$${price.toStringAsFixed(0)}';
  }

  String _formatDate(DateTime value) {
    final local = value.toLocal();
    final date = local.toIso8601String();
    return date.split('.').first.replaceFirst('T', ' ');
  }

  String _rangeLabel(ListingsState state) {
    final start =
        state.items.isEmpty ? 0 : (state.page - 1) * state.pageSize + 1;
    final end = start == 0 ? 0 : start + state.items.length - 1;
    return '$start-$end of ${state.total}';
  }
}

class _FiltersBar extends StatelessWidget {
  const _FiltersBar({
    required this.qController,
    required this.minPriceController,
    required this.maxPriceController,
    required this.minBedsController,
    required this.sort,
    required this.propertyType,
    required this.onSortChanged,
    required this.onPropertyTypeChanged,
    required this.onApply,
    required this.onClear,
  });

  final TextEditingController qController;
  final TextEditingController minPriceController;
  final TextEditingController maxPriceController;
  final TextEditingController minBedsController;
  final String sort;
  final String? propertyType;
  final ValueChanged<String> onSortChanged;
  final ValueChanged<String?> onPropertyTypeChanged;
  final VoidCallback onApply;
  final VoidCallback onClear;

  static const sortOptions = [
    'last_seen_desc',
    'price_asc',
    'price_desc',
    'ppsf_asc',
    'ppsf_desc',
    'value_desc',
  ];

  static const propertyTypes = [
    'HOUSE',
    'CONDO',
    'TOWNHOUSE',
    'DUPLEX',
    'LAND',
    'OTHER',
  ];

  @override
  Widget build(BuildContext context) {
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        SizedBox(
          width: 200,
          child: TextField(
            controller: qController,
            decoration: const InputDecoration(
              labelText: 'Search',
              hintText: 'Address',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            controller: minPriceController,
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(
              labelText: 'Min price',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            controller: maxPriceController,
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(
              labelText: 'Max price',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
        SizedBox(
          width: 120,
          child: TextField(
            controller: minBedsController,
            keyboardType: TextInputType.number,
            decoration: const InputDecoration(
              labelText: 'Min beds',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
        ),
        SizedBox(
          width: 180,
          child: DropdownButtonFormField<String>(
            value: sort,
            decoration: const InputDecoration(
              labelText: 'Sort',
              border: OutlineInputBorder(),
              isDense: true,
            ),
            items: sortOptions
                .map((value) => DropdownMenuItem(
                      value: value,
                      child: Text(value),
                    ))
                .toList(growable: false),
            onChanged: (value) {
              if (value != null) {
                onSortChanged(value);
              }
            },
          ),
        ),
        SizedBox(
          width: 180,
          child: DropdownButtonFormField<String?>(
            value: propertyType,
            decoration: const InputDecoration(
              labelText: 'Property type',
              border: OutlineInputBorder(),
              isDense: true,
            ),
            items: [
              const DropdownMenuItem<String?>(
                value: null,
                child: Text('Any'),
              ),
              ...propertyTypes.map(
                (value) => DropdownMenuItem<String?>(
                  value: value,
                  child: Text(value),
                ),
              ),
            ],
            onChanged: onPropertyTypeChanged,
          ),
        ),
        ElevatedButton(
          onPressed: onApply,
          child: const Text('Apply'),
        ),
        TextButton(
          onPressed: onClear,
          child: const Text('Clear'),
        ),
      ],
    );
  }
}
