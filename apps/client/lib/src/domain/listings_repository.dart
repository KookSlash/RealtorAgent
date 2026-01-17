import 'listing.dart';
import 'listings_page.dart';

abstract class ListingsRepository {
  Future<int> fetchCount();
  Future<ListingsPage> fetchListings({int limit = 20, int offset = 0});
}
