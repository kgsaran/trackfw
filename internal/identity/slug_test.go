package identity

import (
	"encoding/json"
	"os"
	"testing"
)

type slugVectorCase struct {
	Input  string `json:"input"`
	Expect string `json:"expect"`
	Error  bool   `json:"error"`
}

type slugVectorFile struct {
	Version int              `json:"version"`
	Cases   []slugVectorCase `json:"cases"`
}

func loadSlugVectors(t *testing.T) slugVectorFile {
	t.Helper()
	data, err := os.ReadFile("testdata/slug_vectors.json")
	if err != nil {
		t.Fatalf("falha ao ler fixture de vetores de slug: %v", err)
	}
	var fixture slugVectorFile
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("falha ao decodificar fixture de vetores de slug: %v", err)
	}
	return fixture
}

func TestSlugify_Vectors(t *testing.T) {
	fixture := loadSlugVectors(t)
	if fixture.Version != 1 {
		t.Fatalf("versao de fixture inesperada: %d", fixture.Version)
	}

	for _, tc := range fixture.Cases {
		tc := tc
		t.Run(tc.Input, func(t *testing.T) {
			got, err := Slugify(tc.Input)
			if tc.Error {
				if err == nil {
					t.Fatalf("Slugify(%q) = %q, esperava erro", tc.Input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Slugify(%q) retornou erro inesperado: %v", tc.Input, err)
			}
			if got != tc.Expect {
				t.Fatalf("Slugify(%q) = %q, esperava %q", tc.Input, got, tc.Expect)
			}
		})
	}
}

func TestSlugify_LastVectorHas41Chars(t *testing.T) {
	fixture := loadSlugVectors(t)
	last := fixture.Cases[len(fixture.Cases)-1]
	if len(last.Input) != 41 {
		t.Fatalf("caso final da fixture deveria ter 41 caracteres, tem %d", len(last.Input))
	}
}
