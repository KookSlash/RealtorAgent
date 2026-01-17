import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/listing.dart';
import '../../domain/listings_page.dart';
import '../../domain/listings_repository.dart';

class ListingsFilters {
  final String? q;
  final double? minPrice;
  final double? maxPrice;
  final int? minBeds;
  final String? propertyType;

  const ListingsFilters({
    required this.q,
    required this.minPrice,
    required this.maxPrice,
    required this.minBeds,
    required this.propertyType,
  });

  factory ListingsFilters.empty() {
    return const ListingsFilters(
      q: null,
      minPrice: null,
      maxPrice: null,
      minBeds: null,
      propertyType: null,
    );
  }
}

class ListingsState {
  final bool isLoading;
  final String? errorMessage;
  final int total;
  final int page;
  final int pageSize;
  final String sort;
  final ListingsFilters filters;
  final List<Listing> items;

  const ListingsState({
    required this.isLoading,
    required this.errorMessage,
    required this.total,
    required this.page,
    required this.pageSize,
    required this.sort,
    required this.filters,
    required this.items,
  });

  factory ListingsState.initial({int pageSize = 20}) {
    return ListingsState(
      isLoading: true,
      errorMessage: null,
      total: 0,
      page: 1,
      pageSize: pageSize,
      sort: 'last_seen_desc',
      filters: ListingsFilters.empty(),
      items: const [],
    );
  }

  ListingsState copyWith({
    bool? isLoading,
    String? errorMessage,
    int? total,
    int? page,
    int? pageSize,
    String? sort,
    ListingsFilters? filters,
    List<Listing>? items,
  }) {
    return ListingsState(
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
      total: total ?? this.total,
      page: page ?? this.page,
      pageSize: pageSize ?? this.pageSize,
      sort: sort ?? this.sort,
      filters: filters ?? this.filters,
      items: items ?? this.items,
    );
  }

  bool get canPrev => page > 1;
  bool get canNext => (page - 1) * pageSize + items.length < total;
}

class ListingsController extends StateNotifier<ListingsState> {
  final ListingsRepository repository;

  ListingsController(this.repository) : super(ListingsState.initial()) {
    load();
  }

  Future<void> load({
    int? page,
    ListingsFilters? filters,
    String? sort,
  }) async {
    final nextPage = page ?? state.page;
    final nextFilters = filters ?? state.filters;
    final nextSort = sort ?? state.sort;
    state = state.copyWith(
      isLoading: true,
      errorMessage: null,
      page: nextPage,
      sort: nextSort,
      filters: nextFilters,
    );
    try {
      final ListingsPage pageData = await repository.fetchListings(
        page: nextPage,
        pageSize: state.pageSize,
        sort: nextSort,
        q: nextFilters.q,
        minPrice: nextFilters.minPrice,
        maxPrice: nextFilters.maxPrice,
        minBeds: nextFilters.minBeds,
        propertyType: nextFilters.propertyType,
      );
      state = state.copyWith(
        isLoading: false,
        total: pageData.total,
        items: pageData.items,
        page: pageData.page,
        pageSize: pageData.pageSize,
        sort: nextSort,
        filters: nextFilters,
      );
    } catch (err) {
      state = state.copyWith(isLoading: false, errorMessage: err.toString());
    }
  }

  Future<void> applyFilters({
    required String? q,
    required String? minPrice,
    required String? maxPrice,
    required String? minBeds,
    required String? propertyType,
    required String sort,
  }) async {
    final parsedMinPrice = _parseDouble(minPrice);
    if (parsedMinPrice == null && _hasText(minPrice)) {
      state = state.copyWith(
          isLoading: false, errorMessage: 'Invalid min price');
      return;
    }
    final parsedMaxPrice = _parseDouble(maxPrice);
    if (parsedMaxPrice == null && _hasText(maxPrice)) {
      state = state.copyWith(
          isLoading: false, errorMessage: 'Invalid max price');
      return;
    }
    final parsedMinBeds = _parseInt(minBeds);
    if (parsedMinBeds == null && _hasText(minBeds)) {
      state = state.copyWith(
          isLoading: false, errorMessage: 'Invalid min beds');
      return;
    }

    final filters = ListingsFilters(
      q: _normalizeText(q),
      minPrice: parsedMinPrice,
      maxPrice: parsedMaxPrice,
      minBeds: parsedMinBeds,
      propertyType: _normalizeText(propertyType),
    );

    await load(page: 1, filters: filters, sort: sort);
  }

  Future<void> nextPage() async {
    if (!state.canNext) {
      return;
    }
    await load(page: state.page + 1);
  }

  Future<void> prevPage() async {
    if (!state.canPrev) {
      return;
    }
    await load(page: state.page - 1);
  }

  bool _hasText(String? value) {
    return value != null && value.trim().isNotEmpty;
  }

  String? _normalizeText(String? value) {
    if (!_hasText(value)) {
      return null;
    }
    return value!.trim();
  }

  double? _parseDouble(String? value) {
    if (!_hasText(value)) {
      return null;
    }
    return double.tryParse(value!.trim());
  }

  int? _parseInt(String? value) {
    if (!_hasText(value)) {
      return null;
    }
    return int.tryParse(value!.trim());
  }
}
