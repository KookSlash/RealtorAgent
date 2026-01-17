import '../domain/listing.dart';
import '../domain/listings_page.dart';
import '../domain/listings_repository.dart';
import '../domain/price_history.dart';
import 'read_api_client.dart';

class ListingsRepositoryImpl implements ListingsRepository {
  final ReadApiClient client;

  ListingsRepositoryImpl({required this.client});

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
    final dto = await client.fetchListings(
      page: page,
      pageSize: pageSize,
      sort: sort,
      q: q,
      minPrice: minPrice,
      maxPrice: maxPrice,
      minBeds: minBeds,
      minBaths: minBaths,
      minSqft: minSqft,
      maxSqft: maxSqft,
      propertyType: propertyType,
    );
    return dto.toDomain();
  }

  @override
  Future<Listing> fetchListing(String propertyKey) async {
    final dto = await client.fetchListing(propertyKey);
    return dto.toDomain();
  }

  @override
  Future<PriceHistory> fetchPriceHistory(String propertyKey) async {
    final dto = await client.fetchPriceHistory(propertyKey);
    return dto.toDomain();
  }
}
