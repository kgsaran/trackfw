package pathanchor

import "testing"

// TestIsAnchored_Ancorado — falsificação direção 1 (ADR-2026-09-04, "Verificação exigida"):
// formas POSIX e Windows que o predicado deve classificar como ANCORADO, sem depender de GOOS.
// É o caso que ERA classificado errado no Windows via filepath.IsAbs em
// internal/validator (ML-3A, hook config) e, medido depois, também em
// internal/integrations.Manager.resolve (ROADMAP-2026-09-03 Wave reaberta ML-R1, 2026-09-08).
// Movida de internal/validator/validator_credential_guard_test.go quando o predicado foi
// extraído (mesma tabela, mesmo veredito — a extração é comportamento-preservante).
func TestIsAnchored_Ancorado(t *testing.T) {
	cases := []string{
		"/opt/foo/guard.sh", // POSIX absoluto — o caso do ADR
		"/absolute/path/to/guard.sh",
		"/",
		`C:\Users\kg\scripts\guard.sh`, // letra de unidade, barra invertida
		`C:/Users/kg/scripts/guard.sh`, // letra de unidade, barra normal
		`z:\scripts\guard.sh`,          // minúscula
		`\\servidor\share\guard.sh`,    // UNC
		`\\srv\y.sh`,
		`//servidor/share/guard.sh`, // UNC em forma POSIX (2 barras normais) — coberto pelo braço raw[0]=='/', não pelo braço UNC de barra invertida; veredito não pode mudar com a correção da ressalva
	}
	for _, raw := range cases {
		if !IsAnchored(raw) {
			t.Errorf("IsAnchored(%q) = false, quero true (ancorado)", raw)
		}
	}
}

// TestIsAnchored_NaoAfrouxamento — falsificação direção 2 e controle de não-afrouxamento
// (ADR-2026-09-04): NENHUMA forma relativa deve entrar no conjunto "ancorado". É o critério
// principal do ML-3A original — afrouxar aqui enfraqueceria a detecção de guard ausente em
// internal/validator E abriria a mesma classe de aceitação indevida em
// internal/integrations.Manager.resolve, os dois consumidores deste predicado.
func TestIsAnchored_NaoAfrouxamento(t *testing.T) {
	cases := []string{
		"scripts/guard.sh", // relativo puro — o caso do ADR que DEVE continuar classe 2
		"./scripts/guard.sh",
		"../scripts/guard.sh",
		"guard.sh",
		"",
		"~/scripts/guard.sh", // til não é reconhecido por este predicado — tratado à parte
		"$PWD/scripts/guard.sh",
		"C",                     // letra sozinha, sem ":" — não é forma de unidade
		"C:",                    // sem separador após ":" — não é forma de unidade completa
		"C:foo",                 // sem separador — caminho relativo à unidade corrente no Windows, não ancorado por este predicado
		`\scripts\guard.sh`,     // uma única barra invertida — não é UNC (precisa de duas)
		"1:\\scripts\\guard.sh", // dígito antes de ":" não é letra de unidade válida
		`\\`,                    // UNC degenerado: só o prefixo, sem servidor nem share (ressalva hades ML-3A)
		`\\x`,                   // UNC degenerado: servidor sem separador de share
		`\\..\evil`,             // UNC degenerado (leitura 3-barras): servidor "..", não é hostname válido
		`\\..\\evil`,            // UNC degenerado (leitura 4-barras): servidor "..", e share vazio (barra dupla no meio) — notação do parecer é ambígua entre as duas, ambas cobertas
	}
	for _, raw := range cases {
		if IsAnchored(raw) {
			t.Errorf("IsAnchored(%q) = true, quero false — não deve afrouxar para forma relativa", raw)
		}
	}
}

// TestIsAnchored_ControlePOSIX — controle POSIX exigido pela ADR-2026-09-04: os casos abaixo
// devem continuar classificados como ancorado/não-ancorado em QUALQUER host, inclusive Windows.
// Os valores esperados são pinados LITERALMENTE, e não derivados de filepath.IsAbs — a própria
// ADR determina que IsAnchored("/foo") seja true em Windows mesmo que filepath.IsAbs("/foo") seja
// false lá (a divergência é a correção da Wave 3, não um defeito).
//
// A asserção de que filepath.IsAbs continua governando os sítios de TRAVESSIA REAL (o Clean/Join/
// Rel que roda DEPOIS de um caller já ter decidido, via este predicado + filepath.IsAbs
// combinados, que a string é anchored no host corrente — ver internal/integrations.Manager.resolve
// e o doc comment do pacote) está em:
//   - TestManagerRejectsTraversalAbsoluteMismatchAndNUL (internal/integrations/manager_test.go) —
//     comportamento pré-existente preservado para o caso host-consistente.
//   - TestManagerRejectsAnchoredDestinationHostMismatch (internal/integrations/manager_test.go) —
//     o caso NOVO desta extração: forma anchored-por-predicado mas não filepath.IsAbs no host
//     corrente é rejeitada, não silenciosamente tratada como relativa (ROADMAP-2026-09-03 Wave
//     reaberta ML-R1).
//
// Não duplicado aqui: este arquivo testa só a classificação da string, não a decisão de
// aceitar/rejeitar um destino real, que é responsabilidade do caller.
func TestIsAnchored_ControlePOSIX(t *testing.T) {
	anchoredCases := []string{
		"/opt/foo/guard.sh",
		"/absolute/path/to/guard.sh",
		"/",
		"/a",
	}
	for _, raw := range anchoredCases {
		if !IsAnchored(raw) {
			t.Errorf("IsAnchored(%q) = false, quero true — ancorado em qualquer host (ADR-2026-09-04)", raw)
		}
	}
	relativeCases := []string{
		"scripts/guard.sh",
		"./scripts/guard.sh",
		"../scripts/guard.sh",
		"guard.sh",
	}
	for _, raw := range relativeCases {
		if IsAnchored(raw) {
			t.Errorf("IsAnchored(%q) = true, quero false — não deve afrouxar para forma relativa", raw)
		}
	}
}
