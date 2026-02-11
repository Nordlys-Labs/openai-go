package apijson

import (
	"testing"

	"github.com/Nordlys-Labs/openai-go/v3/packages/param"
)

type paramUnion = param.APIUnion

// Test struct union with missing discriminator field
type TestMessageUnion struct {
	OfEasyMessage *TestEasyMessage `json:",omitzero,inline"`
	OfFullMessage *TestFullMessage `json:",omitzero,inline"`
	paramUnion
}

type TestEasyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TestFullMessage struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func init() {
	RegisterUnion[TestMessageUnion](
		"type",
		Discriminator[TestFullMessage]("message"),
	)
}

func TestStructUnionMissingDiscriminator(t *testing.T) {
	// JSON without "type" field - should still work via try-all-variants
	jsonData := []byte(`{"role":"user","content":"hello"}`)

	var result TestMessageUnion
	err := UnmarshalRoot(jsonData, &result)
	if err != nil {
		t.Fatalf("UnmarshalRoot failed: %v", err)
	}

	if result.OfEasyMessage == nil {
		t.Fatal("Expected OfEasyMessage to be set")
	}
	if result.OfEasyMessage.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", result.OfEasyMessage.Role)
	}
}

func TestStructUnionWithDiscriminator(t *testing.T) {
	// JSON with "type" field - should use discriminator matching
	jsonData := []byte(`{"type":"message","role":"user","content":"hello"}`)

	var result TestMessageUnion
	err := UnmarshalRoot(jsonData, &result)
	if err != nil {
		t.Fatalf("UnmarshalRoot failed: %v", err)
	}

	if result.OfFullMessage == nil {
		t.Fatal("Expected OfFullMessage to be set")
	}
	if result.OfFullMessage.Role != "user" {
		t.Errorf("Expected role 'user', got '%s'", result.OfFullMessage.Role)
	}
}

func TestStructUnionUnknownDiscriminator(t *testing.T) {
	// JSON with unknown "type" value - should fall through to try-all-variants
	jsonData := []byte(`{"type":"unknown","role":"user","content":"hello"}`)

	var result TestMessageUnion
	err := UnmarshalRoot(jsonData, &result)
	// This should either succeed (by matching a variant) or fail with "coerce" error
	// It should NOT fail with "discriminated union variant" error
	if err != nil && err.Error() == "apijson: was not able to find discriminated union variant" {
		t.Fatal("Should not get discriminated union error for unknown type")
	}
}
