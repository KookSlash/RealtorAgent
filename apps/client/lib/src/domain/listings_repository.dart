import 'listing.dart';
import 'listings_page.dart';
import 'price_history.dart';

abstract class ListingsRepository {
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
  });
  Future<Listing> fetchListing(String propertyKey);
  Future<PriceHistory> fetchPriceHistory(String propertyKey);
}
