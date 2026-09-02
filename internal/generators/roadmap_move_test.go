package generators

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// setupMove prepara um repo temporário em modo flat com um roadmap em wip/.
// Devolve o diretório raiz.
func setupMove(t *testing.T, filename, content string) string {
	t.Helper()
	dir := t.TempDir()
	chdirADR(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	wip := filepath.Join(dir, "docs", "roadmaps", "wip")
	if err := os.MkdirAll(wip, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wip, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func readMoved(t *testing.T, dir, state, filename string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "docs", "roadmaps", state, filename))
	if err != nil {
		t.Fatalf("lendo destino: %v", err)
	}
	return string(data)
}

// TestMoveRoadmap_SincronizaStatusDoFrontmatter cobre o bug em que o move deixava
// o arquivo em done/ ainda declarando status: wip — exatamente a incoerência que a
// regra folder_status do validator reclama.
func TestMoveRoadmap_SincronizaStatusDoFrontmatter(t *testing.T) {
	const name = "x.md"
	src := "---\nname: x\nstatus: wip\ndate: 2026-08-16\n---\n\n# Roadmap: x\n\ncorpo\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	got := readMoved(t, dir, "done", name)
	want := "---\nname: x\nstatus: done\ndate: 2026-08-16\n---\n\n# Roadmap: x\n\ncorpo\n"
	if got != want {
		t.Errorf("conteúdo após move:\n got: %q\nwant: %q", got, want)
	}
}

// TestMoveRoadmap_SemFrontmatterNaoModifica garante que um roadmap sem frontmatter
// sai byte a byte idêntico — inclusive quando o corpo tem uma linha "status:", que
// uma substituição global corromperia.
func TestMoveRoadmap_SemFrontmatterNaoModifica(t *testing.T) {
	const name = "y.md"
	src := "# Roadmap: y\n\n### ML-1\nstatus: pendente\n\ncorpo\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	if got := readMoved(t, dir, "done", name); got != src {
		t.Errorf("arquivo sem frontmatter foi modificado:\n got: %q\nwant: %q", got, src)
	}
}

// TestMoveRoadmap_FrontmatterSemStatusNaoGanhaCampo — não inventamos a chave.
// Mesmo contrato do validator, que ignora quem não declara status.
func TestMoveRoadmap_FrontmatterSemStatusNaoGanhaCampo(t *testing.T) {
	const name = "z.md"
	src := "---\nname: z\ndate: 2026-08-16\n---\n\n# Roadmap: z\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	if got := readMoved(t, dir, "done", name); got != src {
		t.Errorf("frontmatter sem status foi modificado:\n got: %q\nwant: %q", got, src)
	}
}

// TestMoveRoadmap_StatusNoCorpoNaoEhTocado — só a chave dentro do bloco de
// frontmatter é reescrita; ocorrências posteriores ficam intactas.
func TestMoveRoadmap_StatusNoCorpoNaoEhTocado(t *testing.T) {
	const name = "w.md"
	src := "---\nstatus: wip\n---\n\n# Roadmap: w\n\nstatus: isto é corpo\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "blocked"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	got := readMoved(t, dir, "blocked", name)
	want := "---\nstatus: blocked\n---\n\n# Roadmap: w\n\nstatus: isto é corpo\n"
	if got != want {
		t.Errorf("conteúdo após move:\n got: %q\nwant: %q", got, want)
	}
}

// TestMoveRoadmap_SincronizaLinhaHumana — a linha "> … | Status: …" logo abaixo
// do título também acompanha a pasta. Antes disto o arquivo ia para done/
// declarando status: done no frontmatter e Status: wip na linha que o humano lê.
func TestMoveRoadmap_SincronizaLinhaHumana(t *testing.T) {
	const name = "h.md"
	src := "---\nstatus: wip\n---\n\n# Roadmap: h\n\n> Created: 2026-08-16 | Status: wip\n\ncorpo\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	got := readMoved(t, dir, "done", name)
	want := "---\nstatus: done\n---\n\n# Roadmap: h\n\n> Created: 2026-08-16 | Status: done\n\ncorpo\n"
	if got != want {
		t.Errorf("conteúdo após move:\n got: %q\nwant: %q", got, want)
	}
}

// TestMoveRoadmap_LinhaHumanaComEmoji — formato herdado neste repositório. O
// trecho inteiro após o marcador é substituído, então o emoji sai junto.
func TestMoveRoadmap_LinhaHumanaComEmoji(t *testing.T) {
	const name = "e.md"
	src := "---\nstatus: wip\n---\n\n# Roadmap: e\n\n> Criado em: 2026-08-16 | Status: \U0001F504 WIP\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	got := readMoved(t, dir, "done", name)
	want := "---\nstatus: done\n---\n\n# Roadmap: e\n\n> Criado em: 2026-08-16 | Status: done\n"
	if got != want {
		t.Errorf("conteúdo após move:\n got: %q\nwant: %q", got, want)
	}
}

// TestMoveRoadmap_SemLinhaHumanaNaoCria — a linha nunca é inventada.
func TestMoveRoadmap_SemLinhaHumanaNaoCria(t *testing.T) {
	const name = "s.md"
	src := "---\nstatus: wip\n---\n\n# Roadmap: s\n\nsem linha de status aqui\n"
	dir := setupMove(t, name, src)

	if err := MoveRoadmap(name, "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	got := readMoved(t, dir, "done", name)
	want := "---\nstatus: done\n---\n\n# Roadmap: s\n\nsem linha de status aqui\n"
	if got != want {
		t.Errorf("linha foi criada indevidamente:\n got: %q\nwant: %q", got, want)
	}
}
