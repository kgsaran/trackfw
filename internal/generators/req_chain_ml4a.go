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
// aponta para reqPath, ou "" quando nenhum aponta (ou quando há ambiguidade).
//
// A comparação usa BASENAME como forma estável para o modo flat: req move atualiza o campo `req:`
// do roadmap para o novo caminho (sincroniza via syncRoadmapREQReference), então o ref rastreado
// é o caminho corrente da REQ — mas req new sempre cria no nível superior (cfg.REQDir), então o
// input para este guard de idempotência é sempre o caminho de criação, e o basename elimina a
// divergência de separador de plataforma.
//
// 🔴 ML-1C: em modo by_agent, a comparação por basename pura deixa o guard de idempotência cruzar
// namespaces de agente: ao criar a REQ de `apolo` com mesmo título da REQ de `hades`, a busca
// retornava o roadmap de `hades` (mesmo basename) e vinculava a REQ de `apolo` a ele. O corretor:
//
//   1. Namespace guard (by_agent): o componente de agente extraído de `reqPath` (ancorado no
//      req_dir) deve ser igual ao componente extraído do ref armazenado no roadmap. Isso previne
//      cruzamento de agentes sem afetar o modo flat, onde o guard nunca é ativado.
//
//   2. Recusa por ambiguidade: se dois ou mais roadmaps sobreviverem ao filtro de namespace com
//      mesmo basename, retorna "" em vez de escolher às cegas — "recusar e nomear é melhor que
//      escolher errado em silêncio" (handoff ML-1C).
//
// 🔴 Por que BASENAME é mantido para o modo flat e como base inicial no by_agent:
//
//   Mensurado na Wave 0: req move atualiza o campo req: do roadmap (syncRoadmapREQReference).
//   O comentário anterior afirmava o contrário — está corrigido aqui. A razão de manter basename
//   em flat é que os refs armazenados são sempre o caminho de criação, e req new cria sempre no
//   topo do req_dir; a divergência que basename resolve é de separador de plataforma (\ vs /),
//   não de localização de arquivo.
//
// 🔴 Por que o guard não usa filepath.Dir diretamente:
//
//   Em by_agent, req move pode mover a REQ para agent/state/REQ.md; o roadmap é atualizado para
//   esse caminho. Se uma nova REQ é criada com mesmo basename, o input é agent/REQ.md mas o ref
//   armazenado pode ser agent/wip/REQ.md. filepath.Dir diferiria. reqAgentOf resolve nos dois
//   layouts ancorado no basename do req_dir, não no parent imediato.
func FindRoadmapLinkingREQ(reqPath string) string {
	if strings.TrimSpace(reqPath) == "" {
		return ""
	}
	normalReqPath := normalizeRefSeparator(reqPath)
	want := filepath.Base(normalReqPath)
	if want == "" || want == "." {
		return ""
	}
	cfg := config.Load()
	byAgent := cfg.RoadmapNamespacing == config.NamespacingByAgent
	wantAgent := reqAgentOf(normalReqPath, cfg.REQDir, cfg.Agents)

	var matches []string
	for _, candidate := range roadmapCandidateFiles(cfg) {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		ref := extractFrontmatterReq(string(content))
		if ref == "" {
			continue
		}
		normalRef := normalizeRefSeparator(ref)
		if filepath.Base(normalRef) != want {
			continue
		}
		// 🔴 ML-1C: namespace guard — applies only in by_agent mode. In flat mode the
		// byAgent flag is false and this block is never entered, so the existing
		// basename-only behaviour is byte-identical to the pre-ML-1C code.
		if byAgent && reqAgentOf(normalRef, cfg.REQDir, cfg.Agents) != wantAgent {
			continue
		}
		matches = append(matches, normalizeRefSeparator(candidate))
	}
	if len(matches) == 1 {
		return matches[0]
	}
	// 0 matches: no linked roadmap (normal for a brand-new REQ).
	// >1 matches: ambiguous — refuse rather than guess.
	return ""
}

// reqAgentOf extracts the agent component from a REQ path, anchored on the basename
// of reqDir. Returns "" when the path is in flat mode (directly under reqDir or under
// a state subdir that is not an agent name).
//
// It handles both the canonical by_agent layout (req_dir/agent/REQ.md) and the
// state-subdir variant (req_dir/agent/state/REQ.md), because it looks at the component
// immediately after the LAST occurrence of reqDir's basename — not at the immediate
// parent of the filename. This keeps absolute paths (e.g., /private/tmp/…/docs/req/…)
// safe against a coincidentally-named ancestor directory hijacking the match.
//
// Contrast: if we used filepath.Base(filepath.Dir(reqPath)) directly, the layout
// req_dir/agent/wip/REQ.md would return "wip" instead of "agent" — breaking
// idempotency for stale refs after req move in by_agent mode.
func reqAgentOf(reqPath, reqDir string, agents []string) string {
	if len(agents) == 0 {
		return ""
	}
	reqDirBase := filepath.Base(filepath.ToSlash(reqDir))
	parts := strings.Split(filepath.ToSlash(reqPath), "/")
	// Walk backwards to find the LAST component equal to reqDirBase; the next
	// component is the agent candidate.
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != reqDirBase {
			continue
		}
		if i+1 >= len(parts) {
			break
		}
		candidate := parts[i+1]
		for _, a := range agents {
			if candidate == a {
				return a
			}
		}
		// reqDirBase found but the next component is not an agent name (e.g. it is
		// "REQ-X.md" for flat, or "wip" for a flat state subdir). Stop here — no
		// point searching earlier occurrences of reqDirBase in the path.
		break
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
