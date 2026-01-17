import '../../domain/listings_page.dart';
import 'listing_dto.dart';

class ListingsResponseDto {
  final List<ListingDto> items;
  final int page;
  final int pageSize;
  final int total;

  const ListingsResponseDto({
    required this.items,
    required this.page,
    required this.pageSize,
    required this.total,
  });

  factory ListingsResponseDto.fromJson(Map<String, dynamic> json) {
    final items = json['items'];
    final page = json['page'];
    final pageSize = json['page_size'];
    final total = json['total'];

    if (items is! List) {
      throw const FormatException('Invalid items list');
    }
    if (page is! num || pageSize is! num || total is! num) {
      throw const FormatException('Invalid pagination values');
    }

    final parsedItems = items
        .map((item) => ListingDto.fromJson(item as Map<String, dynamic>))
        .toList(growable: false);

    return ListingsResponseDto(
      items: parsedItems,
      page: page.toInt(),
      pageSize: pageSize.toInt(),
      total: total.toInt(),
    );
  }

  ListingsPage toDomain() {
    return ListingsPage(
      items: items.map((item) => item.toDomain()).toList(growable: false),
      page: page,
      pageSize: pageSize,
      total: total,
    );
  }
}
