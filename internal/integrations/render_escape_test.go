package integrations

// render_escape_test.go — ML-4A (TOML encoder) and ML-4B (YAML escape).
//
// TestTOMLBasicString: asserts CONCLUSION of ML-4A:
//   tomlBasicString produces TOML 1.0 basic strings that a strict decoder
//   accepts and round-trips, while strconv.Quote produces escapes that the
//   same decoder rejects for BEL/VT; UTF-8 invalid input is refused.
//
// TestYAMLEscapeDoubleQuotedNoRawControl: asserts CONCLUSION of ML-4B:
//   yamlEscapeDoubleQuoted emits no raw C0/DEL/C1 bytes; every value round-
//   trips through a YAML double-quoted decoder; the pre-fix implementation
//   leaves raw control bytes that the verifier catches.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kgsaran/trackfw/internal/identity"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Inline TOML 1.0 basic-string decoder (no external dependency).
//
// Accepts only: \" \\ \b \t \n \f \r \uXXXX \UXXXXXXXX
// Rejects: any '\' followed by a character not in the above set,
//          raw bytes 0x00–0x1F (except TAB 0x09) and 0x7F,
//          surrogate code points (\uD800–\uDFFF).
// ---------------------------------------------------------------------------
func decodeTomlBasicString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("toml decoder: not a double-quoted string: %q", s)
	}
	inner := s[1 : len(s)-1]
	var sb strings.Builder
	i := 0
	for i < len(inner) {
		b := inner[i]
		if b == '\\' {
			if i+1 >= len(inner) {
				return "", fmt.Errorf("toml decoder: trailing backslash at offset %d", i)
			}
			switch inner[i+1] {
			case '"':
				sb.WriteByte('"')
				i += 2
			case '\\':
				sb.WriteByte('\\')
				i += 2
			case 'b':
				sb.WriteByte('\b')
				i += 2
			case 't':
				sb.WriteByte('\t')
				i += 2
			case 'n':
				sb.WriteByte('\n')
				i += 2
			case 'f':
				sb.WriteByte('\f')
				i += 2
			case 'r':
				sb.WriteByte('\r')
				i += 2
			case 'u':
				if i+6 > len(inner) {
					return "", fmt.Errorf("toml decoder: \\u escape too short at offset %d", i)
				}
				hex := inner[i+2 : i+6]
				cp64, parseErr := strconv.ParseUint(hex, 16, 32)
				if parseErr != nil {
					return "", fmt.Errorf("toml decoder: invalid \\u%s at offset %d: %v", hex, i, parseErr)
				}
				cp := rune(cp64)
				if cp >= 0xD800 && cp <= 0xDFFF {
					return "", fmt.Errorf("toml decoder: surrogate \\u%04x not allowed", cp)
				}
				sb.WriteRune(cp)
				i += 6
			case 'U':
				if i+10 > len(inner) {
					return "", fmt.Errorf("toml decoder: \\U escape too short at offset %d", i)
				}
				hex := inner[i+2 : i+10]
				cp64, parseErr := strconv.ParseUint(hex, 16, 32)
				if parseErr != nil {
					return "", fmt.Errorf("toml decoder: invalid \\U%s at offset %d: %v", hex, i, parseErr)
				}
				cp := rune(cp64)
				if cp > 0x10FFFF {
					return "", fmt.Errorf("toml decoder: \\U%08x out of Unicode range", cp64)
				}
				if cp >= 0xD800 && cp <= 0xDFFF {
					return "", fmt.Errorf("toml decoder: surrogate \\U%08x not allowed", cp64)
				}
				sb.WriteRune(cp)
				i += 10
			default:
				return "", fmt.Errorf("toml decoder: unrecognized escape \\%c (0x%02x) at offset %d",
					inner[i+1], inner[i+1], i)
			}
		} else if b == 0x7F || (b < 0x20 && b != 0x09) {
			// Raw control byte not allowed in TOML 1.0 basic strings
			return "", fmt.Errorf("toml decoder: raw control byte 0x%02x at offset %d", b, i)
		} else {
			// Valid UTF-8 character — decode the full rune
			r, size := utf8.DecodeRuneInString(inner[i:])
			if r == utf8.RuneError && size == 1 {
				return "", fmt.Errorf("toml decoder: invalid UTF-8 at offset %d", i)
			}
			sb.WriteRune(r)
			i += size
		}
	}
	return sb.String(), nil
}

// ---------------------------------------------------------------------------
// Inline YAML 1.2 double-quoted string decoder (no external dependency).
//
// Accepts: \" \\ \xHH \uXXXX \UXXXXXXXX (and raw non-control chars).
// ---------------------------------------------------------------------------
func decodeYAMLDoubleQuotedString(s string) (string, error) {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("yaml decoder: not double-quoted: %q", s)
	}
	inner := s[1 : len(s)-1]
	var sb strings.Builder
	i := 0
	for i < len(inner) {
		b := inner[i]
		if b == '\\' {
			if i+1 >= len(inner) {
				return "", fmt.Errorf("yaml decoder: trailing backslash")
			}
			switch inner[i+1] {
			case '"':
				sb.WriteByte('"')
				i += 2
			case '\\':
				sb.WriteByte('\\')
				i += 2
			case 'x':
				if i+4 > len(inner) {
					return "", fmt.Errorf("yaml decoder: \\x too short at %d", i)
				}
				hex := inner[i+2 : i+4]
				var cp uint32
				if n, _ := fmt.Sscanf(hex, "%02x", &cp); n != 1 {
					return "", fmt.Errorf("yaml decoder: invalid \\x%s at offset %d", hex, i)
				}
				sb.WriteRune(rune(cp))
				i += 4
			case 'u':
				if i+6 > len(inner) {
					return "", fmt.Errorf("yaml decoder: \\u too short at %d", i)
				}
				hex := inner[i+2 : i+6]
				var cp uint32
				if n, _ := fmt.Sscanf(hex, "%04x", &cp); n != 1 {
					return "", fmt.Errorf("yaml decoder: invalid \\u%s", hex)
				}
				sb.WriteRune(rune(cp))
				i += 6
			case 'U':
				if i+10 > len(inner) {
					return "", fmt.Errorf("yaml decoder: \\U too short at %d", i)
				}
				hex := inner[i+2 : i+10]
				var cp uint32
				if n, _ := fmt.Sscanf(hex, "%08x", &cp); n != 1 {
					return "", fmt.Errorf("yaml decoder: invalid \\U%s", hex)
				}
				sb.WriteRune(rune(cp))
				i += 10
			default:
				return "", fmt.Errorf("yaml decoder: unknown escape \\%c", inner[i+1])
			}
		} else {
			r, size := utf8.DecodeRuneInString(inner[i:])
			if r == utf8.RuneError && size == 1 {
				return "", fmt.Errorf("yaml decoder: invalid UTF-8 at offset %d", i)
			}
			sb.WriteRune(r)
			i += size
		}
	}
	return sb.String(), nil
}

// hasRawControlRuneInDoubleQuoted checks whether a double-quoted scalar
// (including the surrounding double-quote characters) contains any raw rune
// from {0x00–0x1F except 0x09, 0x7F, 0x80–0x9F}.
// It skips over '\' + next byte to avoid inspecting the interior of escape
// sequences as if they were literal characters.
func hasRawControlRuneInDoubleQuoted(s string) bool {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return false
	}
	inner := s[1 : len(s)-1]
	i := 0
	for i < len(inner) {
		// Skip escape sequences without interpreting them.
		if inner[i] == '\\' {
			i += 2
			continue
		}
		r, size := utf8.DecodeRuneInString(inner[i:])
		if (r < 0x20 && r != 0x09) || r == 0x7F || (r >= 0x80 && r <= 0x9F) || r == 0x2028 || r == 0x2029 {
			return true
		}
		i += size
	}
	return false
}

// ---------------------------------------------------------------------------
// TestTOMLBasicString
// ---------------------------------------------------------------------------

// TestTOMLBasicString asserts CONCLUSION of ML-4A: tomlBasicString produces
// TOML 1.0–valid basic strings that round-trip through a strict decoder, while
// strconv.Quote produces sequences the same decoder rejects for BEL/VT; UTF-8
// invalid input is refused.
func TestTOMLBasicString(t *testing.T) {
	t.Run("arm_a_round_trip", func(t *testing.T) {
		// Build the round-trip cases: the decoder must accept the encoder's
		// output and recover the original string.
		cases := []struct {
			name  string
			input string
		}{
			// All C0 bytes individually
			{"nul", "\x00"},
			{"soh", "\x01"},
			{"stx", "\x02"},
			{"etx", "\x03"},
			{"eot", "\x04"},
			{"enq", "\x05"},
			{"ack", "\x06"},
			{"bel", "\x07"}, // 0x07 — strconv.Quote emits \a (invalid TOML)
			{"bs", "\x08"},
			{"ht", "\t"},
			{"lf", "\n"},
			{"vt", "\x0b"}, // 0x0B — strconv.Quote emits \v (invalid TOML)
			{"ff", "\f"},
			{"cr", "\r"},
			{"so", "\x0e"},
			{"si", "\x0f"},
			{"dle", "\x10"},
			{"dc1", "\x11"},
			{"dc2", "\x12"},
			{"dc3", "\x13"},
			{"dc4", "\x14"},
			{"nak", "\x15"},
			{"syn", "\x16"},
			{"etb", "\x17"},
			{"can", "\x18"},
			{"em", "\x19"},
			{"sub", "\x1a"},
			{"esc", "\x1b"},
			{"fs", "\x1c"},
			{"gs", "\x1d"},
			{"rs", "\x1e"},
			{"us", "\x1f"},
			{"del", "\x7f"}, // 0x7F DEL — strconv.Quote emits \x7f (invalid TOML 1.0)
			// C1 range (U+0080–U+009F)
			{"nel", string(rune(0x85))},  // U+0085 NEL
			{"nbsp_like", string(rune(0x80))}, // U+0080
			{"c1_9f", string(rune(0x9F))},     // U+009F
			{"c1_80_to_9f", func() string {
				var b strings.Builder
				for i := rune(0x80); i <= 0x9F; i++ {
					b.WriteRune(i)
				}
				return b.String()
			}()},
			// NBSP (U+00A0) — IsPrint=false → should be \u00a0
			{"nbsp", string(rune(0x00A0))},
			// U+2028 LINE SEPARATOR — IsPrint=false → should be \u2028
			{"line_sep", string(rune(0x2028))},
			// Printable UTF-8 — should pass through unchanged
			{"action_emdash_u", "ação—ü"},
			// String with quotes and backslash
			{"quotes_and_backslash", `He said "hello\" \world`},
			// U+FFFD replacement char — VALID UTF-8, must NOT error
			{"fffd_valid", "\uFFFD"},
			// Mixed: printable + control
			{"mixed", "abc\x07def\x7fghi"},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				encoded, err := tomlBasicString(tc.input)
				if err != nil {
					t.Fatalf("tomlBasicString(%q): unexpected error: %v", tc.input, err)
				}
				decoded, err := decodeTomlBasicString(encoded)
				if err != nil {
					t.Fatalf("decodeTomlBasicString(%s): error: %v\n(input was %q)", encoded, err, tc.input)
				}
				if decoded != tc.input {
					t.Fatalf("round-trip mismatch:\n  input:   %q\n  encoded: %s\n  decoded: %q", tc.input, encoded, decoded)
				}
			})
		}
	})

	t.Run("arm_b_contra_arm_strconv_quote_rejected", func(t *testing.T) {
		// Verify that strconv.Quote output is rejected by our TOML 1.0 decoder for
		// inputs that differ from tomlBasicString. This validates the DECODER
		// (decodeTomlBasicString), not the encoder — a decoder that never rejects
		// anything cannot falsify arm_a round-trip claims.
		biteInputs := []struct {
			name  string
			input string
		}{
			{"bel_0x07", "a\x07b"}, // strconv.Quote emits \a — invalid in any TOML
			{"vt_0x0b", "a\x0bb"},  // strconv.Quote emits \v — invalid in any TOML
		}
		for _, tc := range biteInputs {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				goQuoted := strconv.Quote(tc.input)
				_, err := decodeTomlBasicString(goQuoted)
				if err == nil {
					t.Fatalf("decodeTomlBasicString(%s) accepted strconv.Quote output — decoder is not biting", goQuoted)
				}
			})
		}

		// Also prove the decoder accepts raw tab (to confirm it is not over-strict).
		t.Run("raw_tab_accepted", func(t *testing.T) {
			// Raw tab is valid in TOML 1.0 basic strings.
			if _, err := decodeTomlBasicString(`"a	b"`); err != nil {
				t.Fatalf("decodeTomlBasicString accepted raw tab unexpectedly failed: %v", err)
			}
		})

		// Prove the decoder rejects a raw 0x07 byte embedded literally.
		t.Run("raw_bel_rejected", func(t *testing.T) {
			raw := "\"a\x07b\""
			if _, err := decodeTomlBasicString(raw); err == nil {
				t.Fatal("decodeTomlBasicString should reject raw 0x07 byte but accepted it")
			}
		})

		// Decoder rejects partial-hex, out-of-range, and surrogate escapes.
		// These validate the ML-4F fixes to decodeTomlBasicString (strconv.ParseUint
		// replaces fmt.Sscanf which accepted partial hex like \u00zz as \u0000).
		t.Run("partial_hex_u_rejected", func(t *testing.T) {
			if _, err := decodeTomlBasicString(`"\u00zz"`); err == nil {
				t.Fatal("decodeTomlBasicString should reject \\u00zz (partial hex) but accepted it")
			}
		})
		t.Run("out_of_range_U_rejected", func(t *testing.T) {
			if _, err := decodeTomlBasicString(`"\U00110000"`); err == nil {
				t.Fatal("decodeTomlBasicString should reject \\U00110000 (> 0x10FFFF) but accepted it")
			}
		})
		t.Run("surrogate_U_rejected", func(t *testing.T) {
			if _, err := decodeTomlBasicString(`"\U0000D800"`); err == nil {
				t.Fatal("decodeTomlBasicString should reject surrogate \\U0000D800 but accepted it")
			}
		})
	})

	t.Run("arm_c_invalid_utf8_error", func(t *testing.T) {
		// Direct call: invalid UTF-8 byte returns error.
		t.Run("direct_encode_errors", func(t *testing.T) {
			_, err := tomlBasicString("a\xffb")
			if err == nil {
				t.Fatal("tomlBasicString(\"a\\xffb\"): expected error for invalid UTF-8, got nil")
			}
		})

		// Render path: \xff in description (via identity DisplayName) propagates error.
		t.Run("render_with_invalid_utf8_errors", func(t *testing.T) {
			catalog, err := LoadCatalog()
			if err != nil {
				t.Fatalf("LoadCatalog: %v", err)
			}
			item, ok := catalog.Item(KindAgents, "backend")
			if !ok {
				t.Fatal("catalog: backend item not found")
			}
			source, err := catalog.ReadAsset(item)
			if err != nil {
				t.Fatalf("ReadAsset backend: %v", err)
			}

			// Config with \xff in DisplayName — bypasses json.Unmarshal sanitization.
			cfg := identity.Config{
				Agents: map[string]identity.AgentIdentity{
					"backend": {
						DisplayName: "Zeus\xffTF", // invalid UTF-8 byte 0xFF
						Slug:        "zeus",
					},
				},
			}
			_, err = Render(item, KindAgents, Capability{Representation: "custom-agent-toml"}, source, cfg, "codex", nil)
			if err == nil {
				t.Fatal("Render with \\xff in description: expected error, got nil")
			}
			if !strings.Contains(err.Error(), "as TOML") {
				t.Fatalf("Render error should contain \"as TOML\": %v", err)
			}

			// Control: clean DisplayName renders without error.
			cleanCfg := identity.Config{
				Agents: map[string]identity.AgentIdentity{
					"backend": {
						DisplayName: "Zeus",
						Slug:        "zeus",
					},
				},
			}
			_, err = Render(item, KindAgents, Capability{Representation: "custom-agent-toml"}, source, cleanCfg, "codex", nil)
			if err != nil {
				t.Fatalf("Render with clean DisplayName: unexpected error: %v", err)
			}
		})
	})

	t.Run("arm_d_goldens_zero_churn", func(t *testing.T) {
		// For the fields of every golden asset (no identity), tomlBasicString
		// and strconv.Quote must produce identical output — goldens must not change.
		catalog, err := LoadCatalog()
		if err != nil {
			t.Fatalf("LoadCatalog: %v", err)
		}
		allItems := catalog.Items(KindAgents)
		for _, item := range allItems {
			item := item
			source, err := catalog.ReadAsset(item)
			if err != nil {
				t.Fatalf("ReadAsset %s: %v", item.ID, err)
			}
			name, description, model, body := markdownParts(source)
			fields := map[string]string{
				"name":                  strings.ReplaceAll(name, "-", "_"),
				"description":           description,
				"developer_instructions": body,
			}
			if mapped, ok := mapModelCodex(model); ok {
				fields["model"] = mapped
			}
			for fieldName, v := range fields {
				fieldName, v := fieldName, v
				t.Run(item.ID+"/"+fieldName, func(t *testing.T) {
					enc, err := tomlBasicString(v)
					if err != nil {
						t.Fatalf("tomlBasicString(%s/%s): unexpected error: %v", item.ID, fieldName, err)
					}
					want := strconv.Quote(v)
					if enc != want {
						t.Fatalf("tomlBasicString diverges from strconv.Quote for %s/%s:\n  tomlBasicString: %s\n  strconv.Quote:   %s",
							item.ID, fieldName, enc, want)
					}
				})
			}
		}
	})
}

// ---------------------------------------------------------------------------
// TestYAMLEscapeDoubleQuotedNoRawControl
// ---------------------------------------------------------------------------

// oldYAMLEscapeDoubleQuoted is the pre-fix implementation, copied verbatim
// as a local function for the contra-arm. It only escapes " and \.
func oldYAMLEscapeDoubleQuoted(v string) string {
	var sb strings.Builder
	sb.Grow(len(v))
	for _, r := range v {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// TestYAMLEscapeDoubleQuotedNoRawControl asserts CONCLUSION of ML-4B:
// yamlEscapeDoubleQuoted emits no raw C0/DEL/C1 runes in the encoded output,
// and every value round-trips through a YAML double-quoted decoder.
// The pre-fix implementation leaves raw control bytes that the verifier catches.
func TestYAMLEscapeDoubleQuotedNoRawControl(t *testing.T) {
	// Build the set of test characters: C0 (0x00–0x1F), DEL (0x7F), C1 (0x80–0x9F).
	type controlCase struct {
		name  string
		input string
	}
	var cases []controlCase
	for i := rune(0x00); i <= 0x1F; i++ {
		cases = append(cases, controlCase{
			name:  fmt.Sprintf("C0_0x%02x", i),
			input: "a" + string(i) + "b",
		})
	}
	cases = append(cases, controlCase{name: "DEL_0x7f", input: "a" + string(rune(0x7F)) + "b"})
	for i := rune(0x80); i <= 0x9F; i++ {
		cases = append(cases, controlCase{
			name:  fmt.Sprintf("C1_0x%02x", i),
			input: "a" + string(i) + "b",
		})
	}
	// U+2028 (LINE SEPARATOR) and U+2029 (PARAGRAPH SEPARATOR): yaml.v3 folds
	// these to empty/spaces in double-quoted scalars when emitted raw. After
	// ML-4F, yamlEscapeDoubleQuoted emits \u2028/\u2029 which round-trip.
	cases = append(cases,
		controlCase{name: "LS_U+2028", input: "a" + string(rune(0x2028)) + "b"},
		controlCase{name: "PS_U+2029", input: "a" + string(rune(0x2029)) + "b"},
	)

	t.Run("new_encoder_no_raw_control_and_round_trips", func(t *testing.T) {
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				out := yamlSafeScalar(tc.input)

				// Must be double-quoted (containsControlChar returns true for all inputs).
				if !strings.HasPrefix(out, `"`) || !strings.HasSuffix(out, `"`) {
					t.Fatalf("yamlSafeScalar(%q) = %q: not double-quoted", tc.input, out)
				}

				// Must contain no raw control rune.
				if hasRawControlRuneInDoubleQuoted(out) {
					t.Fatalf("yamlSafeScalar(%q) = %q: contains raw control char", tc.input, out)
				}

				// Must round-trip through the YAML decoder.
				decoded, err := decodeYAMLDoubleQuotedString(out)
				if err != nil {
					t.Fatalf("decodeYAMLDoubleQuotedString(%q): error: %v (input %q)", out, err, tc.input)
				}
				if decoded != tc.input {
					t.Fatalf("YAML round-trip mismatch for %q:\n  encoded: %q\n  decoded: %q", tc.input, out, decoded)
				}
			})
		}
	})

	t.Run("contra_arm_old_encoder_leaves_raw_control", func(t *testing.T) {
		// Inputs where the old implementation emits raw control bytes.
		// Tab (0x09) is excluded because YAML double-quoted allows raw tab.
		biteInputs := []struct {
			name  string
			input string
		}{
			{"bel_0x07", "a\x07b"},
			{"lf_0x0a", "a\x0ab"},
			{"cr_0x0d", "a\x0db"},
			{"nel_0x85", "a" + string(rune(0x85)) + "b"},
			{"ls_0x2028", "a" + string(rune(0x2028)) + "b"},
			{"ps_0x2029", "a" + string(rune(0x2029)) + "b"},
		}
		for _, tc := range biteInputs {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				old := `"` + oldYAMLEscapeDoubleQuoted(tc.input) + `"`
				if !hasRawControlRuneInDoubleQuoted(old) {
					t.Fatalf("contra-arm: old encoder did not leave raw control in %q → verifier is not biting", old)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// TestYAMLScalarEnumerationAllRunes — ML-4F
// ---------------------------------------------------------------------------

// TestYAMLScalarEnumerationAllRunes asserts CONCLUSION of ML-4F:
// For every Unicode rune (excluding surrogates), yamlSafeScalar produces a
// scalar that yaml.v3 round-trips faithfully, OR Render returns a non-nil
// error that explicitly rejects the rune before any YAML is emitted. Three
// forms exercise bare (single-rune), embedded, and leading-position contexts.
//
// Bite (HEAD) expectation: fails for at least U+FFFE, U+FFFF, U+2028, U+2029
// (yaml.v3 folds or rejects them when emitted raw) and U+002C comma in
// leading position (yaml.v3 rejects "description: ,").
func TestYAMLScalarEnumerationAllRunes(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatalf("ReadAsset: %v", err)
	}

	type failure struct {
		form string
		r    rune
		msg  string
	}
	var failures []failure

	forms := []struct {
		name string
		make func(rune) string
	}{
		{"bare", func(r rune) string { return string(r) }},
		{"embedded", func(r rune) string { return "a" + string(r) + "b" }},
		{"leading", func(r rune) string { return string(r) + " x" }},
	}

	for r := rune(0); r <= 0x10FFFF; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue // surrogates: not valid Unicode scalar values
		}
		for _, form := range forms {
			v := form.make(r)

			// Condition 2 (cheap): yamlSafeScalar produces a scalar that yaml.v3
			// round-trips faithfully.
			var m map[string]string
			yErr := yaml.Unmarshal([]byte("description: "+yamlSafeScalar(v)), &m)
			if yErr == nil && m["description"] == v {
				continue // condition 2 satisfied
			}

			// Condition 1 (fallback): Render rejects the rune explicitly by
			// returning a non-nil error for a YAML output path (subagent/claude).
			cfg := identity.Config{
				Agents: map[string]identity.AgentIdentity{
					item.ID: {
						DisplayName: v,
						Slug:        "zeus",
					},
				},
			}
			_, rErr := Render(item, KindAgents, Capability{Representation: "subagent"}, source, cfg, "claude", nil)
			if rErr != nil {
				continue // condition 1 satisfied
			}

			// Neither condition met — record failure.
			var msg string
			if yErr != nil {
				msg = fmt.Sprintf("yaml.Unmarshal error: %v", yErr)
			} else {
				msg = fmt.Sprintf("round-trip mismatch: got %q, want %q", m["description"], v)
			}
			failures = append(failures, failure{form: form.name, r: r, msg: msg})
		}
	}

	if len(failures) == 0 {
		return
	}

	// Deduplicate by rune (report each rune once across all forms).
	seen := make(map[rune]bool)
	var distinct []rune
	for _, f := range failures {
		if !seen[f.r] {
			seen[f.r] = true
			distinct = append(distinct, f.r)
		}
	}
	sort.Slice(distinct, func(i, j int) bool { return distinct[i] < distinct[j] })

	const maxReport = 20
	t.Errorf("%d distinct rune(s) fail the YAML round-trip guarantee (first %d):",
		len(distinct), min(len(distinct), maxReport))
	for i, r := range distinct {
		if i >= maxReport {
			break
		}
		for _, f := range failures {
			if f.r == r {
				t.Logf("  U+%04X form=%s: %s", r, f.form, f.msg)
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestRender_NonYAMLChar_Rejected — ML-4F
// ---------------------------------------------------------------------------

// TestRender_NonYAMLChar_Rejected asserts CONCLUSION of ML-4F (policy gate):
// Render returns a non-nil error when DisplayName contains U+FFFE for YAML
// output paths (subagent, agent-directory, opencode-agent). For structured
// representations (custom-agent-toml, agent-json) the same input is accepted
// without error — those encoders handle the rune safely via their own encoding.
func TestRender_NonYAMLChar_Rejected(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	item, ok := catalog.Item(KindAgents, "backend")
	if !ok {
		t.Fatal("catalog: backend item not found")
	}
	source, err := catalog.ReadAsset(item)
	if err != nil {
		t.Fatalf("ReadAsset: %v", err)
	}

	// Config constructed directly (bypasses Load) to inject U+FFFE.
	noncharCfg := identity.Config{
		Agents: map[string]identity.AgentIdentity{
			item.ID: {
				DisplayName: "Zeus￾TF", // U+FFFE non-character
				Slug:        "zeus",
			},
		},
	}

	// YAML output paths must refuse with a non-nil error.
	yamlReps := []string{"subagent", "agent-directory", "opencode-agent"}
	for _, rep := range yamlReps {
		_, err := Render(item, KindAgents, Capability{Representation: rep}, source, noncharCfg, "claude", nil)
		if err == nil {
			t.Errorf("Render(%s) with U+FFFE in DisplayName: expected error, got nil", rep)
		}
	}

	// Structured output paths (TOML, JSON) must accept the same input.
	structuredReps := []string{"custom-agent-toml", "agent-json"}
	for _, rep := range structuredReps {
		_, err := Render(item, KindAgents, Capability{Representation: rep}, source, noncharCfg, "codex", nil)
		if err != nil {
			t.Errorf("Render(%s) with U+FFFE in DisplayName: unexpected error: %v", rep, err)
		}
	}
}
