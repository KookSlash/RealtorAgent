import 'package:flutter/material.dart';

import 'presentation/listings/listings_screen.dart';

class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'RealtorAgent',
      theme: ThemeData(colorSchemeSeed: Colors.blueGrey),
      home: const ListingsScreen(),
    );
  }
}
