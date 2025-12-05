package memes

import (
	"testing"
)

func TestGetRandomCharacter(t *testing.T) {
	// Run multiple times to ensure randomness works
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		char := GetRandomCharacter()
		if char.Name == "" {
			t.Error("GetRandomCharacter returned character with empty name")
		}
		if char.Image == "" {
			t.Error("GetRandomCharacter returned character with empty image")
		}
		if len(char.Quotes) == 0 {
			t.Error("GetRandomCharacter returned character with no quotes")
		}
		seen[char.Name] = true
	}

	// Should have seen both characters over 100 iterations
	if len(seen) < 2 {
		t.Errorf("Expected to see multiple characters, only saw: %v", seen)
	}
}

func TestGetCharacterByName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"LSP by name", "lsp", "Lumpy Space Princess"},
		{"Sloth by name", "sloth", "Flash Slothmore"},
		{"Unknown returns random", "unknown", ""}, // Will be random
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			char := GetCharacterByName(tt.input)
			if tt.expected != "" && char.Name != tt.expected {
				t.Errorf("GetCharacterByName(%q) = %q, want %q", tt.input, char.Name, tt.expected)
			}
			if char.Name == "" {
				t.Errorf("GetCharacterByName(%q) returned empty character", tt.input)
			}
		})
	}
}

func TestGetRandomQuote(t *testing.T) {
	char := LumpySpacePrincess

	// Run multiple times to ensure randomness works
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		quote := GetRandomQuote(char)
		if quote == "" {
			t.Error("GetRandomQuote returned empty quote")
		}
		seen[quote] = true
	}

	// Should have seen multiple quotes over 100 iterations
	if len(seen) < 2 {
		t.Errorf("Expected to see multiple quotes, only saw %d unique", len(seen))
	}
}

func TestGetMemeData(t *testing.T) {
	tests := []struct {
		name       string
		preference string
	}{
		{"Random preference", "random"},
		{"Empty preference", ""},
		{"LSP preference", "lsp"},
		{"Sloth preference", "sloth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := GetMemeData(tt.preference)

			if data.Character.Name == "" {
				t.Error("GetMemeData returned empty character name")
			}
			if data.Quote == "" {
				t.Error("GetMemeData returned empty quote")
			}
			if data.ColorTheme == "" {
				t.Error("GetMemeData returned empty color theme")
			}
			if data.ColorTheme != data.Character.ColorTheme {
				t.Errorf("ColorTheme mismatch: got %q, want %q", data.ColorTheme, data.Character.ColorTheme)
			}
		})
	}
}

func TestCharacterData(t *testing.T) {
	// Verify the predefined characters have required data
	characters := []Character{LumpySpacePrincess, DMVSloth}

	for _, char := range characters {
		t.Run(char.Name, func(t *testing.T) {
			if char.Name == "" {
				t.Error("Character has empty name")
			}
			if char.Image == "" {
				t.Error("Character has empty image")
			}
			if char.AltText == "" {
				t.Error("Character has empty alt text")
			}
			if char.ColorTheme == "" {
				t.Error("Character has empty color theme")
			}
			if len(char.Quotes) == 0 {
				t.Error("Character has no quotes")
			}

			// Verify quotes are not empty
			for i, quote := range char.Quotes {
				if quote == "" {
					t.Errorf("Quote %d is empty", i)
				}
			}
		})
	}
}

func TestCharactersSlice(t *testing.T) {
	if len(Characters) != 2 {
		t.Errorf("Expected 2 characters, got %d", len(Characters))
	}
}
