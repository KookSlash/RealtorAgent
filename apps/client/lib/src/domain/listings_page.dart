import 'listing.dart';

class ListingsPage {
  final List<Listing> items;
  final int limit;
  final int offset;
  final int returned;

  const ListingsPage({
    required this.items,
    required this.limit,
    required this.offset,
    required this.returned,
  });
}
