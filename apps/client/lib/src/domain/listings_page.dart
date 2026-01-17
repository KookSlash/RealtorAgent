import 'listing.dart';

class ListingsPage {
  final List<Listing> items;
  final int page;
  final int pageSize;
  final int total;

  const ListingsPage({
    required this.items,
    required this.page,
    required this.pageSize,
    required this.total,
  });
}
