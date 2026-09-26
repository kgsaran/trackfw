package validator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// ML-4B (REQ-2026-09-09-req-nasce-orfa…, AC2 + AC3) — corte por data da regra
// `req_has_roadmap`, com o grandfathering VISÍVEL.
//
// REQ criada A PARTIR do corte sem roadmap vinculado ⇒ severidade normal da regra
// (default "error"). REQ criada ANTES do corte ⇒ sempre warning, nunca violation —
// e a quantidade de isentas é dita em UMA mensagem no relatório, porque
// *isenção que não se vê vira permanente*.
//
// 🔴 Por que a data é 2026-09-03, e não "hoje" nem a data da REQ:
//
// A curva foi medida nos 231 REQs deste repositório em 2026-09-26 (13 sem roadmap
// vinculado, todos entre 2026-08-16 e 2026-09-02):
//
//	corte        isentas   cobradas
//	2026-08-16      0        13
//	2026-08-21      3        10
//	2026-08-29      5         8
//	2026-08-31      8         5
//	2026-09-02     12         1
//	2026-09-03     13         0     ← satura aqui
//	2026-09-26     13         0
//
// A curva SATURA em 2026-09-03: todo candidato ≥ 09-03 dá (13, 0), então ela
// restringe a escolha a ">= 09-03" mas não decide dentro desse intervalo. O
// desempate é de direção de estritude, não de gosto: o corte concede ANISTIA, e a
// regra já é "error" por default (`req_has_roadmap` está ausente de ruleDefaults).
// Um corte mais TARDE anistiaria, em silêncio, REQs órfãs datadas entre 09-03 e
// hoje — que hoje são erro. Anistia mínima = a data mais antiga que zera o
// deadlock. É por isso, também, que a fixture do Cenário 192 do
// check-gates-falsify (REQ datada 2026-09-06) continua sendo VIOLATION: ela nasceu
// depois do corte.
//
// Régua de data, e por que esta: `date:` do frontmatter primeiro, nome do arquivo
// como fallback — a mesma precedência frontmatter-first que o ML-1A/ML-1D fixou
// para o vínculo. Medido no acervo (231 REQs, 2026-09-26): as duas réguas divergem
// em 13 arquivos (3 com as duas presentes e diferentes, 3 só com data no nome, 7 só
// com `date:` no frontmatter) — e NENHUM desses 13 é uma das 13 REQs sem roadmap,
// logo a escolha de régua não muda veredito nenhum hoje.
//
// 🔴 Sem data legível em NENHUMA das duas réguas ⇒ FAIL CLOSED (tratada como
// pós-corte, portanto cobrada). Motivo: aceitar "data ausente" como anistia abriria
// o bypass de apagar o `date:` e renomear o arquivo. Medido: zero REQs do acervo
// caem nesse ramo (as 3 sem `date:` têm data no nome; as 7 sem data no nome têm
// `date:`), então fail-closed não cobra ninguém hoje.
//
// Residual declarado (não corrigido aqui): `date:` é editável pelo usuário, logo
// retrodatar é um bypass da anistia. Nenhuma das duas réguas é resistente a
// adulteração; o git é, e está fora do escopo deste ML.
// ---------------------------------------------------------------------------

// reqRoadmapCutoff é a data de corte do grandfathering de req_has_roadmap.
// A razão da data está escrita no bloco acima — não a mude sem refazer a curva.
const reqRoadmapCutoff = "2026-09-03"

// reqRoadmapCutoffDate devolve o corte parseado. O literal é constante e validado
// por teste, então o erro de parse é impossível em produção; em caso de defeito de
// edição do literal, devolve o zero de time.Time, que torna TODAS as REQs
// pós-corte (fail closed, coerente com a régua de data ausente).
func reqRoadmapCutoffDate() time.Time {
	d, err := time.Parse("2006-01-02", reqRoadmapCutoff)
	if err != nil {
		return time.Time{}
	}
	return d
}

// reqFilenameDateRe casa a data no INÍCIO do basename (REQ-YYYY-MM-DD-…). A âncora
// `^` é deliberada: o esquema antigo `REQ-<slug>-YYYY-MM-DD.md` (7 arquivos do
// acervo) NÃO casa aqui por construção, e esses são exatamente os que têm `date:`
// no frontmatter — a régua primária.
var reqFilenameDateRe = regexp.MustCompile(`^REQ-(\d{4}-\d{2}-\d{2})`)

// reqCreationDate resolve a data de criação de uma REQ: `date:` do frontmatter
// primeiro, data do nome do arquivo como fallback. O segundo retorno é false quando
// nenhuma das duas é legível — e o chamador trata isso como pós-corte (fail closed).
func reqCreationDate(content, path string) (time.Time, bool) {
	if raw := strings.TrimSpace(extractFrontmatterField(content, "date")); raw != "" {
		v := strings.Trim(raw, "\"'`")
		if len(v) >= 10 {
			if d, err := time.Parse("2006-01-02", v[:10]); err == nil {
				return d, true
			}
		}
	}
	if m := reqFilenameDateRe.FindStringSubmatch(filepath.Base(path)); m != nil {
		if d, err := time.Parse("2006-01-02", m[1]); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

// reqIsGrandfathered informa se a REQ é ISENTA (criada antes do corte). Data
// ilegível ⇒ false (cobrada), pelo argumento de fail-closed do bloco acima.
func reqIsGrandfathered(content, path string) bool {
	d, ok := reqCreationDate(content, path)
	if !ok {
		return false
	}
	return d.Before(reqRoadmapCutoffDate())
}

// reqRoadmapGrandfatherNotice é a mensagem que torna a isenção VISÍVEL no relatório.
//
// 🔴 Contra-braço de vacuidade: a mensagem carrega o DENOMINADOR (quantas REQs foram
// varridas) e as duas contagens, então "0 isentas" nunca aparece sozinha — ela vem
// sempre acompanhada de "N cobradas" e "K varridas", o que distingue *não há
// passivo* de *não medi*.
//
// Deliberadamente NÃO contém a substring "no linked Roadmap": o aviso é sobre a
// política de corte, não sobre um artefato, e confundir os dois faria o `off` da
// regra parecer quebrado nos testes que grepam aquela substring.
func reqRoadmapGrandfatherNotice(exempt, enforced, scanned int) string {
	return fmt.Sprintf(
		"req_has_roadmap grandfathering: %d REQ(s) without a linked Roadmap exempt as created before the cutoff %s, %d enforced as created on/after it, %d REQ(s) scanned (cutoff declared in internal/validator/validator_req_roadmap_cutoff.go)",
		exempt, reqRoadmapCutoff, enforced, scanned,
	)
}
