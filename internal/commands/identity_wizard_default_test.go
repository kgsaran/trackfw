package commands

import (
	"testing"

	"github.com/charmbracelet/huh"
)

// TestDefaultIdentitySelectionIsNeutral asserts that defaultIdentitySelection
// returns "neutral" — the value required so the huh cursor opens on the
// "(padrão)" option instead of the first option ("greek") (AC4b).
func TestDefaultIdentitySelectionIsNeutral(t *testing.T) {
	if got := defaultIdentitySelection(); got != "neutral" {
		t.Fatalf("defaultIdentitySelection() = %q, want %q", got, "neutral")
	}
}

// TestIdentitySelectDefaultCursorIsNeutral builds the preset selector exactly
// as runIdentityWizard does — identitySelect pre-set via defaultIdentitySelection,
// Options first then Value — and confirms via Hovered() that the cursor lands
// on "neutral" without any user input (AC4b).
//
// This test does not require a TTY: Hovered() reads s.filteredOptions[s.selected]
// directly from the struct, set during Value() via selectValue(). No form.Run()
// is called.
//
// Reconciliation (Regra Dura): this test asserts that pre-filling identitySelect
// with "neutral" causes the huh Select cursor to open at the neutral option.
// That is exactly the invariant AC4b requires. Counter-arm: replacing
// defaultIdentitySelection() to return "" makes this test fail because
// selectValue("") finds no match and s.selected stays at 0 ("greek").
func TestIdentitySelectDefaultCursorIsNeutral(t *testing.T) {
	identitySelect := defaultIdentitySelection()
	sel := huh.NewSelect[string]().
		Options(identityPresetOptions()...).
		Value(&identitySelect)
	got, ok := sel.Hovered()
	if !ok {
		t.Fatal("Hovered() returned no option — identityPresetOptions() may be empty")
	}
	if got != "neutral" {
		t.Fatalf("selector default cursor = %q, want %q (the huh cursor opened on the wrong option)", got, "neutral")
	}
}
