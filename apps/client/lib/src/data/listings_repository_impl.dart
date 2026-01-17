import '../domain/listings_page.dart';
import '../domain/listings_repository.dart';
import 'read_api_client.dart';

class ListingsRepositoryImpl implements ListingsRepository {
  final ReadApiClient client;

  ListingsRepositoryImpl({required this.client});

  @override
  Future<int> fetchCount() {
    return client.fetchListingsCount();
  }

  @override
  Future<ListingsPage> fetchListings({int limit = 20, int offset = 0}) async {
    final dto = await client.fetchListings(limit: limit, offset: offset);
    return dto.toDomain();
  }
}
