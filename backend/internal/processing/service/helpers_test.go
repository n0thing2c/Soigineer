package service

import "testing"

func TestNormalizeMessage(t *testing.T) {
	input := "  User JOHN@example.com from 10.0.0.1 hit id 550e8400-e29b-41d4-a716-446655440000 with token abcdefghijklmnop and code 500  "
	original, normalized := NormalizeMessage(input)

	if original != input {
		t.Fatalf("original = %q, want input", original)
	}
	want := "user {email} from {ip} hit id {uuid} with token {id} and code {number}"
	if normalized != want {
		t.Fatalf("normalized = %q, want %q", normalized, want)
	}
}

func TestClassifyCategories(t *testing.T) {
	tests := []struct {
		message string
		want    ErrorCategory
	}{
		{"jwt token unauthorized", AuthError},
		{"database sql deadlock", DatabaseError},
		{"deadline exceeded timeout", NetworkTimeout},
		{"upstream external api failed", ThirdPartyAPIError},
		{"validation failed bad request", ValidationError},
		{"panic nil pointer runtime error", InternalServerError},
		{"plain informational message", UnknownError},
	}

	for _, tt := range tests {
		t.Run(string(tt.want), func(t *testing.T) {
			if got := Classify(tt.message); got != tt.want {
				t.Fatalf("Classify(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	if !ContainsAny("hello world", "nope", "world") {
		t.Fatal("ContainsAny() = false, want true")
	}
	if ContainsAny("hello world", "nope", "missing") {
		t.Fatal("ContainsAny() = true, want false")
	}
}

func TestGenerateFingerprintDeterministic(t *testing.T) {
	first := GenerateFingerprint("app", "ERROR", "DATABASE_ERROR", "db failed")
	second := GenerateFingerprint("app", "ERROR", "DATABASE_ERROR", "db failed")
	changed := GenerateFingerprint("app", "ERROR", "DATABASE_ERROR", "other")

	if first != second {
		t.Fatalf("fingerprint changed for same input: %q vs %q", first, second)
	}
	if first == changed {
		t.Fatalf("fingerprint did not change for different input: %q", first)
	}
	if len(first) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(first))
	}
}
