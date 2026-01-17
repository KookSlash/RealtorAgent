import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../domain/listing.dart';
import '../../domain/listings_page.dart';
import '../../domain/listings_repository.dart';

class ListingsState {
  final bool isLoading;
  final String? errorMessage;
  final int count;
  final int limit;
  final int offset;
  final List<Listing> items;

  const ListingsState({
    required this.isLoading,
    required this.errorMessage,
    required this.count,
    required this.limit,
    required this.offset,
    required this.items,
  });

  factory ListingsState.initial({int limit = 20}) {
    return ListingsState(
      isLoading: true,
      errorMessage: null,
      count: 0,
      limit: limit,
      offset: 0,
      items: const [],
    );
  }

  ListingsState copyWith({
    bool? isLoading,
    String? errorMessage,
    int? count,
    int? limit,
    int? offset,
    List<Listing>? items,
  }) {
    return ListingsState(
      isLoading: isLoading ?? this.isLoading,
      errorMessage: errorMessage,
      count: count ?? this.count,
      limit: limit ?? this.limit,
      offset: offset ?? this.offset,
      items: items ?? this.items,
    );
  }

  bool get canPrev => offset > 0;
  bool get canNext => offset + items.length < count;
}

class ListingsController extends StateNotifier<ListingsState> {
  final ListingsRepository repository;

  ListingsController(this.repository) : super(ListingsState.initial()) {
    load();
  }

  Future<void> load({int? offset}) async {
    final nextOffset = offset ?? state.offset;
    state =
        state.copyWith(isLoading: true, errorMessage: null, offset: nextOffset);
    try {
      final count = await repository.fetchCount();
      final ListingsPage page = await repository.fetchListings(
        limit: state.limit,
        offset: nextOffset,
      );
      state = state.copyWith(
        isLoading: false,
        count: count,
        items: page.items,
        limit: page.limit,
        offset: page.offset,
      );
    } catch (err) {
      state = state.copyWith(isLoading: false, errorMessage: err.toString());
    }
  }

  Future<void> nextPage() async {
    if (!state.canNext) {
      return;
    }
    await load(offset: state.offset + state.limit);
  }

  Future<void> prevPage() async {
    if (!state.canPrev) {
      return;
    }
    final prevOffset = state.offset - state.limit;
    await load(offset: prevOffset < 0 ? 0 : prevOffset);
  }
}
