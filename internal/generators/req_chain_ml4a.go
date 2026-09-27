package generators

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// ML-4A (AC1/AC10 da REQ-2026-09-09) — o lado de IDA do elo: criar a REQ pode criar o roadmap no
// mesmo ato. O ML-1B fechou o lado de VOLTA (o roadmap grava o ponteiro na REQ); o que faltava era
// não depender de um segundo comando que se esquece.
//
// Este arquivo expõe as duas peças que o caminho integrado (internal/commands/req.go) precisa e que
// já existiam aqui em forma não exportada. Nenhuma delas é mecanismo novo:
//
//   - FindRoadmapLinkingREQ — leitura, reusa roadmapCandidateFiles (o mesmo enumerador de roadmaps
//     que findRoadmap e ListRoadmaps usam) e a mesma leitura de frontmatter.
//   - RelinkREQToRoadmap — escrita, delega a linkREQToRoadmap, o ponto único do ML-1B, com a mesma
//     não-fatalidade e as mesmas guardas de contenção.

// FindRoadmapLinkingREQ devolve o caminho do roadmap existente cujo campo `req:` do frontmatter
// aponta para reqPath, ou "" quando nenhum aponta. A comparação é por BASENAME porque é a forma
// estável: o roadmap grava o caminho relativo da REQ, mas a REQ pode ter sido movida entre
// subpastas de estado por `req move` desde então — e nesse caso o elo continua sendo o mesmo.
//
// 🔴 Para que serve: é o guard de idempotência do caminho integrado. Sem ele, um segundo
// `req new "mesmo título"` criaria um SEGUNDO roadmap quando o primeiro já tivesse saído de
// backlog/ (o nome do arquivo deixa de colidir), e a REQ ficaria com um roadmap órfão ao lado do
// que ela de fato referencia — duplicação, não reexecução.
func FindRoadmapLinkingREQ(reqPath string) string {
	if strings.TrimSpace(reqPath) == "" {
		return ""
	}
	want := filepath.Base(normalizeRefSeparator(reqPath))
	if want == "" || want == "." {
		return ""
	}
	cfg := config.Load()
	for _, candidate := range roadmapCandidateFiles(cfg) {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		ref := extractFrontmatterReq(string(content))
		if ref == "" {
			continue
		}
		if filepath.Base(normalizeRefSeparator(ref)) == want {
			return normalizeRefSeparator(candidate)
		}
	}
	return ""
}

// extractFrontmatterReq extrai o valor do campo `req:` do bloco frontmatter de um roadmap.
// É o espelho de extractFrontmatterRoadmap (que lê o campo `roadmap:` de uma REQ) — mesma forma,
// mesmo tratamento de aspas, mesmo corte no fechamento do bloco.
func extractFrontmatterReq(content string) string {
	lines := strings.Split(content, "\n")
	inFM := false
	fmCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "---" {
			fmCount++
			if fmCount == 1 {
				inFM = true
				continue
			}
			break
		}
		if !inFM {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), "req") {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

// RelinkREQToRoadmap grava na REQ o ponteiro para um roadmap que JÁ EXISTE. É o ponto único do
// ML-1B (linkREQToRoadmap) reexposto, não uma segunda escrita: preenche placeholder, não sequestra
// vínculo diferente, é idempotente e nunca é fatal.
//
// Necessária porque `req new` reescreve o arquivo da REQ do zero (comportamento anterior a este ML):
// numa reexecução no mesmo dia, a REQ volta a `roadmap: ""` enquanto o roadmap já existe. Sem esta
// chamada, o guard de idempotência acima deixaria a REQ órfã justamente na segunda rodada.
func RelinkREQToRoadmap(reqPath, roadmapPath string) {
	linkREQToRoadmap(reqPath, normalizeRefSeparator(roadmapPath))
}
