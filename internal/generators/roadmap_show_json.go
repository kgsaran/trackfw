package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/roadmapdoc"
)

// ────────────────────────────────────────────────────────────────────────────
// `trackfw roadmap show <name> --json` — parte 2 da #407.
//
// O problema que isto fecha: o `**Status:**` por ML só existia como texto do
// roadmap, então quem precisava dele reimplementava o dialeto — e o dialeto tem
// regras que não são óbvias (marcador pelo primeiro token, vocabulário fechado,
// U+FE0F tolerado, marca combinante recusada, cerca de código ignorada, CRLF
// normalizado). O resultado era um SEGUNDO leitor do mesmo formato, que só
// descobria divergência quando o número saía errado.
//
// 🔴 Por que aqui, e não num comando novo: `roadmap show` já resolve o nome
// parcial, já recusa nome vazio e já recusa ambiguidade. Um subcomando novo
// duplicaria as três coisas — e duplicar resolução foi exatamente a classe de
// defeito que a ML-1C do próprio ShowRoadmap fechou.
//
// 🔴 Por que NÃO existe campo `title`: o título de um ML vive na linha do
// heading, e separá-lo do id exigiria adivinhar o separador (— , -, :, nenhum).
// Isso seria inventar um dialeto para descrever um dialeto — o defeito que esta
// saída existe para eliminar. O campo `heading` carrega a linha como ela é, sem
// o `### `, e quem quiser o título decide a própria régua com o dado à vista.
// ────────────────────────────────────────────────────────────────────────────

// roadmapShowDoc é o documento raiz emitido por --json.
type roadmapShowDoc struct {
	Roadmap        string              `json:"roadmap"`
	Path           string              `json:"path"`
	State          string              `json:"state"`
	MalformedWaves []malformedWaveJSON `json:"malformed_waves"`
	Waves          []waveJSON          `json:"waves"`
}

type malformedWaveJSON struct {
	Line  int    `json:"line"`
	Token string `json:"token"`
}

type waveJSON struct {
	Label string   `json:"label"`
	Line  int      `json:"line"`
	MLs   []mlJSON `json:"mls"`
}

type mlJSON struct {
	ID string `json:"id"`
	// Heading é a linha do `### ` inteira, sem o marcador — deliberadamente crua.
	Heading string `json:"heading"`
	Line    int    `json:"line"`
	// StatusMarker é o texto do `**Status:**` como está no arquivo. O acervo do
	// upstream tem 8 grafias medidas (#407), e nenhuma se perde aqui.
	StatusMarker string `json:"status_marker"`
	// StatusFound distingue "ML sem linha de Status" de "Status vazio" — os dois
	// dariam marcador "" e são situações diferentes para quem audita.
	StatusFound bool `json:"status_found"`
	// Status é a categoria do produto: complete | pending | terminated. É ela que
	// dispensa o consumidor de reimplementar o dialeto.
	Status     string         `json:"status"`
	Acceptance acceptanceJSON `json:"acceptance"`
}

type acceptanceJSON struct {
	Met      int  `json:"met"`
	Unmet    int  `json:"unmet"`
	HasBlock bool `json:"has_block"`
}

// statusCatName traduz a categoria do produto para o nome estável do JSON.
//
// 🔴 São exatamente as três do roadmapdoc.StatusCat, nem uma a mais. Acrescentar
// categoria aqui (por exemplo separar "bloqueado" de "pendente") criaria uma
// classificação que o produto não tem — que é o segundo leitor de novo, só que
// desta vez dentro do próprio produto.
func statusCatName(c roadmapdoc.StatusCat) string {
	switch c {
	case roadmapdoc.StatusComplete:
		return "complete"
	case roadmapdoc.StatusTerminated:
		return "terminated"
	default:
		return "pending"
	}
}

// ShowRoadmapJSON emite o documento de um roadmap em JSON, sem rodar gate nenhum
// e sem chamar o validate: é leitura.
func ShowRoadmapJSON(name string) error {
	matches, err := resolveRoadmapMatches(name)
	if err != nil {
		return err
	}
	if len(matches) > 1 {
		// Sem imprimir a lista no stdout: com --json, o stdout é o documento ou
		// nada. Os candidatos vão na mensagem de erro, que o cobra manda ao stderr.
		return fmt.Errorf("ambiguous match for %q — candidates: %s", name, strings.Join(matches, ", "))
	}

	path := matches[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	doc := buildRoadmapShowDoc(path, string(data))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	// SetEscapeHTML(false): os marcadores são emoji e texto em português; escapar
	// HTML transformaria o documento em algo que não casa com o arquivo lido.
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

// buildRoadmapShowDoc é a montagem pura — separada da E/S para poder ser medida
// sem tocar em disco nem em stdout.
func buildRoadmapShowDoc(path, data string) roadmapShowDoc {
	lines := roadmapdoc.SplitRoadmapLines(data)
	fenced := roadmapdoc.FenceMask(lines)
	waves, malformed := roadmapdoc.ParseWaves(lines, fenced)

	doc := roadmapShowDoc{
		Roadmap:        filepath.Base(path),
		Path:           filepath.ToSlash(path),
		State:          filepath.Base(filepath.Dir(path)),
		MalformedWaves: []malformedWaveJSON{},
		Waves:          []waveJSON{},
	}

	// Waves malformadas entram no documento em vez de sumirem: o ParseWaves as
	// ISOLA de propósito, e um consumidor que só visse `waves` concluiria que o
	// documento está íntegro. Fail-safe do lado de quem lê.
	for _, mw := range malformed {
		doc.MalformedWaves = append(doc.MalformedWaves, malformedWaveJSON{Line: mw.Line, Token: mw.Token})
	}

	for _, w := range waves {
		wj := waveJSON{Label: w.Label, Line: w.Start + 1, MLs: []mlJSON{}}
		for _, ml := range roadmapdoc.ParseMLs(lines, fenced, w.Start, w.End) {
			marker, found := roadmapdoc.MLStatusMarker(lines, fenced, ml)
			met, unmet, hasBlock := roadmapdoc.AcceptanceEvaluate(lines, fenced, ml)
			wj.MLs = append(wj.MLs, mlJSON{
				ID:           ml.ID,
				Heading:      headingText(lines, ml.Start),
				Line:         ml.Start + 1,
				StatusMarker: marker,
				StatusFound:  found,
				Status:       statusCatName(roadmapdoc.StatusCategory(marker)),
				Acceptance:   acceptanceJSON{Met: met, Unmet: unmet, HasBlock: hasBlock},
			})
		}
		doc.Waves = append(doc.Waves, wj)
	}
	return doc
}

// headingText devolve a linha do heading sem os `#` e sem o espaço que os segue.
// Não tenta separar id de título — ver a nota do topo do arquivo.
func headingText(lines []string, idx int) string {
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	return strings.TrimSpace(strings.TrimLeft(lines[idx], "#"))
}
