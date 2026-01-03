package normalize

import (
	"testing"
)

func TestNormalizeText(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "trim and collapse", input: "  123   Main  St ", expect: "123 main st"},
		{name: "punctuation", input: "123 Main-St.", expect: "123 mainst"},
		{name: "unit", input: "Unit #5B", expect: "unit 5b"},
		{name: "tabs newlines", input: "123\tMain\nSt", expect: "123 main st"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeText(tc.input); got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestNormalizePostal(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "spaces", input: "t2p 1a1", expect: "T2P1A1"},
		{name: "dash", input: "T2P-1A1", expect: "T2P1A1"},
		{name: "mixed", input: " t2p\t1a1 ", expect: "T2P1A1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizePostal(tc.input); got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}

func TestCanonicalIdentity(t *testing.T) {
	got := CanonicalIdentity("123 Main-St.", "t2p 1a1", "Unit #5B")
	if got != "123 mainst|T2P1A1|unit 5b" {
		t.Fatalf("unexpected canonical identity: %q", got)
	}
}

func TestPropertyKeyDeterminism(t *testing.T) {
	key1 := PropertyKey("123 Main St", "T2P1A1", "101")
	key2 := PropertyKey("123 Main St", "T2P1A1", "101")
	if key1 != key2 {
		t.Fatalf("expected deterministic property key")
	}

	key3 := PropertyKey("123 Main St", "T2P1A1", "102")
	if key1 == key3 {
		t.Fatalf("expected different key for different unit")
	}

	if len(key1) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(key1))
	}
}

func TestSnapshotHashChanges(t *testing.T) {
	base := SnapshotHash("500000", "2", "1.5", "900", "L-1", "CONDO")

	cases := []struct {
		name string
		val  string
		fn   func(string) string
	}{
		{name: "price", val: "510000", fn: func(v string) string { return SnapshotHash(v, "2", "1.5", "900", "L-1", "CONDO") }},
		{name: "beds", val: "3", fn: func(v string) string { return SnapshotHash("500000", v, "1.5", "900", "L-1", "CONDO") }},
		{name: "baths", val: "2", fn: func(v string) string { return SnapshotHash("500000", "2", v, "900", "L-1", "CONDO") }},
		{name: "sqft", val: "1000", fn: func(v string) string { return SnapshotHash("500000", "2", "1.5", v, "L-1", "CONDO") }},
		{name: "source_listing_id", val: "L-2", fn: func(v string) string { return SnapshotHash("500000", "2", "1.5", "900", v, "CONDO") }},
		{name: "property_type", val: "HOUSE", fn: func(v string) string { return SnapshotHash("500000", "2", "1.5", "900", "L-1", v) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.val); got == base {
				t.Fatalf("expected snapshot hash change for %s", tc.name)
			}
		})
	}
}

func FuzzNormalizeAddress(f *testing.F) {
	f.Add("123 Main St")
	f.Add("Unit #5B")
	f.Fuzz(func(t *testing.T, input string) {
		first := NormalizeText(input)
		second := NormalizeText(first)
		if first != second {
			t.Fatalf("expected NormalizeText to be idempotent")
		}
	})
}

func FuzzCanonicalIdentityDeterminism(f *testing.F) {
	f.Add("123 Main St", "T2P1A1", "101")
	f.Fuzz(func(t *testing.T, address, postal, unit string) {
		identity1 := CanonicalIdentity(address, postal, unit)
		identity2 := CanonicalIdentity(address, postal, unit)
		if identity1 != identity2 {
			t.Fatalf("expected deterministic canonical identity")
		}
	})
}

func FuzzPropertyKeyDeterminism(f *testing.F) {
	f.Add("123 Main St", "T2P1A1", "101")
	f.Fuzz(func(t *testing.T, address, postal, unit string) {
		key1 := PropertyKey(address, postal, unit)
		key2 := PropertyKey(address, postal, unit)
		if key1 != key2 {
			t.Fatalf("expected deterministic property key")
		}
		if len(key1) != 64 {
			t.Fatalf("expected sha256 hex length 64, got %d", len(key1))
		}
	})
}
