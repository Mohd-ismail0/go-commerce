package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateAttributeValue(t *testing.T) {
	t.Run("number normalizes", func(t *testing.T) {
		v, err := validateAttributeValue("number", " 3.5 ", nil)
		if err != nil || v != "3.5" {
			t.Fatalf("got %q %v", v, err)
		}
	})
	t.Run("boolean yes", func(t *testing.T) {
		v, err := validateAttributeValue("boolean", "YES", nil)
		if err != nil || v != "true" {
			t.Fatalf("got %q %v", v, err)
		}
	})
	t.Run("select member", func(t *testing.T) {
		allowed, _ := json.Marshal([]string{"S", "M", "L"})
		v, err := validateAttributeValue("select", "M", allowed)
		if err != nil || v != "M" {
			t.Fatalf("got %q %v", v, err)
		}
	})
	t.Run("select rejects unknown", func(t *testing.T) {
		allowed, _ := json.Marshal([]string{"S", "M"})
		_, err := validateAttributeValue("select", "XL", allowed)
		if err == nil || !strings.Contains(err.Error(), "allowed") {
			t.Fatalf("expected bad request, got %v", err)
		}
	})
}

func TestValidateCatalogAttributeInput(t *testing.T) {
	if err := validateCatalogAttributeInput("text", nil); err != nil {
		t.Fatal(err)
	}
	if err := validateCatalogAttributeInput("select", []byte(`["a"]`)); err != nil {
		t.Fatal(err)
	}
	if err := validateCatalogAttributeInput("select", []byte(`[]`)); err == nil {
		t.Fatal("expected error for empty select options")
	}
}
