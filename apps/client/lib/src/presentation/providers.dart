import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;

import '../core/config/app_config.dart';
import '../data/listings_repository_impl.dart';
import '../data/read_api_client.dart';
import '../domain/listings_repository.dart';
import 'listings/listings_controller.dart';

final appConfigProvider = Provider<AppConfig>((ref) {
  return AppConfig.fromEnv();
});

final httpClientProvider = Provider<http.Client>((ref) {
  final client = http.Client();
  ref.onDispose(client.close);
  return client;
});

final readApiClientProvider = Provider<ReadApiClient>((ref) {
  return ReadApiClient(
    config: ref.watch(appConfigProvider),
    httpClient: ref.watch(httpClientProvider),
  );
});

final listingsRepositoryProvider = Provider<ListingsRepository>((ref) {
  return ListingsRepositoryImpl(client: ref.watch(readApiClientProvider));
});

final listingsControllerProvider =
    StateNotifierProvider<ListingsController, ListingsState>((ref) {
  return ListingsController(ref.watch(listingsRepositoryProvider));
});
