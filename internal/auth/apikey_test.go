package auth

import "testing"

func TestGenerateAndValidateAPIKey(t *testing.T) {
	raw, prefix, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey: %v", err)
	}
	if len(raw) > 72 {
		t.Fatalf("raw key length %d exceeds bcrypt limit", len(raw))
	}

	extracted, err := ExtractAPIKeyPrefix(raw)
	if err != nil {
		t.Fatalf("ExtractAPIKeyPrefix: %v", err)
	}
	if extracted != prefix {
		t.Fatalf("expected prefix %q, got %q", prefix, extracted)
	}

	hash, err := HashAPIKey(raw)
	if err != nil {
		t.Fatalf("HashAPIKey: %v", err)
	}

	if err := ValidateAPIKey(raw, hash); err != nil {
		t.Fatalf("ValidateAPIKey: %v", err)
	}

	if err := ValidateAPIKey(raw+"x", hash); err == nil {
		t.Fatal("expected invalid key to fail validation")
	}
}
