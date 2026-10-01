package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// Issue #490: `roadmap move <x> blocked` tirava o roadmap de wip/, e o gate do `commit` exigia
// wip/ ou done/ — então o commit da PRÓPRIA transição era recusado, e o bloqueio acabava declarado
// só na prosa, com a pasta mentindo.
//
// Cada teste declara, na primeira linha, qual conclusão da medição ele afirma.

// acervoBlocked monta um roadmap_dir plano com os arquivos pedidos por estado e devolve o cfg.
//
// 🔴 Faz chdir para a fixture, como o linkFixture já existente: o pathguard ancora a escrita do
// arquivo de links na raiz do PROCESSO, então um roadmap_dir absoluto em t.TempDir() faz
// writeBranchLink recusar com "escapes root" — medido ao escrever este arquivo.
func acervoBlocked(t *testing.T, porEstado map[string][]string) config.ProjectConfig {
	t.Helper()
	raiz := t.TempDir()
	rel := "docs/roadmaps"
	for _, estado := range []string{"backlog", "wip", "blocked", "done", "abandoned"} {
		if err := os.MkdirAll(filepath.Join(raiz, rel, estado), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", estado, err)
		}
	}
	for estado, nomes := range porEstado {
		for _, n := range nomes {
			if err := os.WriteFile(filepath.Join(raiz, rel, estado, n), []byte("# roadmap\n"), 0o644); err != nil {
				t.Fatalf("write %s/%s: %v", estado, n, err)
			}
		}
	}
	writeFile(t, raiz, "trackfw.yaml", "roadmap_dir: "+rel+"\n")
	config.Reset()
	chdir(t, raiz)
	t.Cleanup(config.Reset)
	return config.Load()
}

const rmBloqueio = "ROADMAP-2026-09-30-cerca-nao-terminada-apaga-o-ml-pendente.md"

// Afirma o achado central: um roadmap em blocked/ volta a GOVERNAR a branch. Medido por efeito antes
// da correção, com o estado como única variável: em wip/ o commit saía rc=0, em blocked/ rc=1, e de
// volta a wip/ rc=0.
func TestResolveSettledDirs_BlockedGovernaComoDone(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"blocked": {rmBloqueio}})

	slug := NormalizeBranchSlug("cerca-nao-terminada-apaga-o-ml-pendente")
	matched, candidates := BranchSlugMatchesRoadmap(slug, ResolveWIPDirs(cfg), ResolveSettledDirs(cfg))
	if !matched {
		t.Errorf("roadmap em blocked/ deveria casar: candidatos=%v", candidates)
	}

	// Controle: com APENAS done/ no conjunto, o mesmo acervo não casa — é o estado anterior à
	// correção, e sem ele este teste passaria mesmo que ResolveSettledDirs devolvesse só done/.
	soDone, _ := BranchSlugMatchesRoadmap(slug, ResolveWIPDirs(cfg), ResolveDoneDirs(cfg))
	if soDone {
		t.Error("controle falhou: com só done/ não deveria casar — o acervo tem o roadmap em blocked/")
	}
}

// Afirma que a correção não confunde os estados: wip/ continua separado de done/+blocked/, porque
// quem precisa distinguir "em curso" de "assentado" — o `branch new`, via #494 — recebe as duas
// listas. Se ResolveSettledDirs passasse a incluir wip/, aquela distinção morreria em silêncio.
func TestResolveSettledDirs_NaoIncluiWip(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"wip": {rmBloqueio}})

	for _, d := range ResolveSettledDirs(cfg) {
		if strings.HasSuffix(filepath.ToSlash(d), "/wip") {
			t.Fatalf("ResolveSettledDirs não pode incluir wip/: %v", ResolveSettledDirs(cfg))
		}
	}
	// E o roadmap em wip/ não é alcançado por esse conjunto.
	slug := NormalizeBranchSlug("cerca-nao-terminada-apaga-o-ml-pendente")
	if m, _ := BranchSlugMatchesRoadmap(slug, nil, ResolveSettledDirs(cfg)); m {
		t.Error("roadmap em wip/ não deveria ser alcançado por ResolveSettledDirs")
	}
}

// Afirma que done/ continua no conjunto — a correção acrescenta blocked/, não substitui. Controle de
// remédio excessivo: trocar uma lista pela outra passaria nos outros testes.
func TestResolveSettledDirs_DonePermanece(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"done": {rmBloqueio}})

	slug := NormalizeBranchSlug("cerca-nao-terminada-apaga-o-ml-pendente")
	if m, c := BranchSlugMatchesRoadmap(slug, ResolveWIPDirs(cfg), ResolveSettledDirs(cfg)); !m {
		t.Errorf("roadmap em done/ deveria continuar casando: candidatos=%v", c)
	}
}

// Afirma que o link escrito sobrevive à transição para blocked/. Era o SEGUNDO mecanismo, que a issue
// não nomeia: o link conferia "in wip/ nor done/" e virava stale na mesma transição — e a mensagem
// prescrevia `trackfw branch new`, que era recusado pelo mesmo motivo. Instrução circular, medida.
func TestBranchLinkFor_LinkParaBlockedContinuaNoEscopo(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"blocked": {rmBloqueio}})
	if err := writeBranchLink(cfg, "fix/cerca-nao-terminada", rmBloqueio); err != nil {
		t.Fatalf("writeBranchLink: %v", err)
	}

	link := BranchLinkFor(cfg, "fix/cerca-nao-terminada", ResolveWIPDirs(cfg), ResolveSettledDirs(cfg))
	if !link.Present {
		t.Fatal("o link foi escrito, deveria estar presente")
	}
	if !link.InScope {
		t.Error("link para roadmap em blocked/ deveria estar NO ESCOPO — era a causa da instrução circular")
	}

	// Controle: com só done/, o mesmo link sai do escopo. É o estado anterior à correção.
	antes := BranchLinkFor(cfg, "fix/cerca-nao-terminada", ResolveWIPDirs(cfg), ResolveDoneDirs(cfg))
	if antes.InScope {
		t.Error("controle falhou: com só done/ o link deveria estar fora do escopo")
	}
}

// Afirma que a resolução do D1 devolve o roadmap bloqueado pelo LINK e não cai na inferência — o
// aviso de branch_link_stale deixa de ser emitido nessa transição, que é o que remove a
// circularidade sem precisar afrouxar o `branch new`.
func TestResolveBranchRoadmap_BlockedResolvePeloLinkSemAviso(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"blocked": {rmBloqueio}})
	if err := writeBranchLink(cfg, "fix/cerca-nao-terminada", rmBloqueio); err != nil {
		t.Fatalf("writeBranchLink: %v", err)
	}

	res := ResolveBranchRoadmap(cfg, "fix/cerca-nao-terminada", ResolveWIPDirs(cfg), ResolveSettledDirs(cfg))
	if !res.Matched {
		t.Fatal("deveria casar pelo link escrito")
	}
	if res.Source != "written-link" {
		t.Errorf("Source = %q, esperava written-link", res.Source)
	}
	for _, w := range res.Warnings {
		if strings.Contains(w, "branch_link_stale") {
			t.Errorf("não deveria avisar stale — o roadmap está em blocked/, que agora é escopo:\n%s", w)
		}
	}
}

// Afirma que o aviso de stale continua existindo para o caso que ele existe para pegar: roadmap que
// saiu de TODOS os estados que governam. Sem este controle, a correção poderia ter desligado o aviso.
func TestBranchLinkStaleWarning_AindaDisparaForaDeTodosOsEstados(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"backlog": {rmBloqueio}})
	if err := writeBranchLink(cfg, "fix/cerca-nao-terminada", rmBloqueio); err != nil {
		t.Fatalf("writeBranchLink: %v", err)
	}

	link := BranchLinkFor(cfg, "fix/cerca-nao-terminada", ResolveWIPDirs(cfg), ResolveSettledDirs(cfg))
	if !link.Present {
		t.Fatal("o link foi escrito")
	}
	if link.InScope {
		t.Error("roadmap em backlog/ NÃO governa a branch — o link deve sair do escopo")
	}

	msg := BranchLinkStaleWarning(cfg, "fix/cerca-nao-terminada", rmBloqueio)
	if !strings.Contains(msg, "blocked/") {
		t.Errorf("a mensagem deve enumerar blocked/ agora que ele está no escopo:\n%s", msg)
	}
}

// Afirma que RecordBranchLink escreve o link para um roadmap que já está em blocked/ — o link é
// escrito na criação da branch, mas a mesma função é o ponto de re-criação, e sem blocked/ no
// conjunto ela devolvia nil em silêncio.
func TestRecordBranchLink_EscreveParaBlocked(t *testing.T) {
	cfg := acervoBlocked(t, map[string][]string{"blocked": {rmBloqueio}})

	if err := RecordBranchLink(cfg, "fix/cerca-nao-terminada-apaga-o-ml-pendente"); err != nil {
		t.Fatalf("RecordBranchLink: %v", err)
	}
	links := readBranchLinks(cfg)
	if got := links["fix/cerca-nao-terminada-apaga-o-ml-pendente"]; got != rmBloqueio {
		t.Errorf("link gravado = %q, esperava %q", got, rmBloqueio)
	}
}
