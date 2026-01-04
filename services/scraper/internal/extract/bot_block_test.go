package extract

import "testing"

func TestHasRobotsNoIndex(t *testing.T) {
	html := []byte(`
		<html>
			<head>
				<meta name="ROBOTS" content="NOINDEX, NOFOLLOW">
			</head>
			<body>blocked</body>
		</html>
	`)

	if !HasRobotsNoIndex(html) {
		t.Fatalf("expected robots noindex to be detected")
	}
}

func TestHasRobotsNoIndexFalse(t *testing.T) {
	html := []byte(`
		<html>
			<head>
				<meta name="viewport" content="width=device-width">
			</head>
			<body>ok</body>
		</html>
	`)

	if HasRobotsNoIndex(html) {
		t.Fatalf("expected robots noindex to be false")
	}
}
