import '../../domain/listings_page.dart';
import 'listing_dto.dart';

class ListingsResponseDto {
  final List<ListingDto> items;
  final int limit;
  final int offset;
  final int returned;

  const ListingsResponseDto({
    required this.items,
    required this.limit,
    required this.offset,
    required this.returned,
  });

  factory ListingsResponseDto.fromJson(Map<String, dynamic> json) {
    final items = json['items'];
    final limit = json['limit'];
    final offset = json['offset'];
    final returned = json['returned'];

    if (items is! List) {
      throw const FormatException('Invalid items list');
    }
    if (limit is! num || offset is! num || returned is! num) {
      throw const FormatException('Invalid pagination values');
    }

    final parsedItems = items
        .map((item) => ListingDto.fromJson(item as Map<String, dynamic>))
        .toList(growable: false);

    return ListingsResponseDto(
      items: parsedItems,
      limit: limit.toInt(),
      offset: offset.toInt(),
      returned: returned.toInt(),
    );
  }

  ListingsPage toDomain() {
    return ListingsPage(
      items: items.map((item) => item.toDomain()).toList(growable: false),
      limit: limit,
      offset: offset,
      returned: returned,
    );
  }
}
