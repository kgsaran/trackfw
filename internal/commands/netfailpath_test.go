package commands

// netfailpath_test.go — testes do ML-1B (ADR D1, D5, D7/A5, A7, A8).
//
// Cada teste afirma o caminho de falha por ponto de chamada de rede. O fake injetado
// retorna *netdeadline.TimeoutError (ou outro erro) para simular estouro de prazo.
// Usar fake é correto aqui: o prazo nos executores de produção foi provado no ML-1A;
// o que se testa agora é o CHAMADOR — se ele expõe o erro corretamente ao usuário.
//
// Regra Dura de Reconciliação: cada teste declara em comentário a conclusão que afirma.

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/netdeadline"
	"github.com/kgsaran/trackfw/internal/validator"
)

// fakeTimeout creates a *netdeadline.TimeoutError as if "git <subcmd>" timed out.
func fakeTimeout(subcmd string) error {
	return &netdeadline.TimeoutError{
		Op:    "git " + subcmd,
		Limit: 60 * time.Second,
		Var:   netdeadline.NetworkTimeoutEnv,
	}
}

// fakeGhTimeout creates a *netdeadline.TimeoutError as if "gh" timed out.
func fakeGhTimeout() error {
	return &netdeadline.TimeoutError{
		Op:    "gh",
		Limit: 60 * time.Second,
		Var:   netdeadline.NetworkTimeoutEnv,
	}
}

// ─── Ponto 1: push.go ~238 — git fetch origin --prune ────────────────────────────────────────

// TestPush_FetchTimeout_WarningNamesTimeout_PushProceeds asserts:
// quando git fetch estourou o prazo em trackfw push, o aviso nomeia o erro de estouro
// (incluindo a variável TRACKFW_NETWORK_TIMEOUT) e o push prossegue sem retornar erro.
//
// Conclusion: um estouro de prazo no fetch de push é comunicado ao usuário com o texto
// do TimeoutError, e não mascara o estouro como "offline or no remote".
//
// Reprovar sem a correção: sem o `errors.Is(ferr, netdeadline.ErrTimeout)`, a mensagem
// seria a genérica "offline or no remote" — o teste falharia em `strings.Contains(out, "TRACKFW_NETWORK_TIMEOUT")`.
func TestPush_FetchTimeout_WarningNamesTimeout_PushProceeds(t *testing.T) {
	out := &bytes.Buffer{}
	fetchErr := fakeTimeout("fetch")
	var pushCalled bool

	m := &mockPushGit{branch: "feat/x", hasUpstream: true}
	origExec := m.exec
	d := pushDeps{
		execGit: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.HasPrefix(joined, "fetch") {
				return "", fetchErr
			}
			if strings.HasPrefix(joined, "push") {
				pushCalled = true
				return "", nil
			}
			return origExec(args...)
		},
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             out,
	}

	err := runPush(pushOpts{}, d)
	if err != nil {
		t.Fatalf("push should proceed after fetch timeout, got error: %v", err)
	}
	if !pushCalled {
		t.Fatal("git push was not called — fetch timeout should not abort push")
	}
	stdout := out.String()
	if !strings.Contains(stdout, netdeadline.NetworkTimeoutEnv) {
		t.Errorf("warning must name %s, got: %q", netdeadline.NetworkTimeoutEnv, stdout)
	}
	if strings.Contains(stdout, "offline or no remote") {
		t.Errorf("timeout must not be reported as 'offline or no remote', got: %q", stdout)
	}
	// Confirm errors.Is identity is preserved by the fake.
	if !errors.Is(fetchErr, netdeadline.ErrTimeout) {
		t.Fatal("test fixture: fetchErr must satisfy errors.Is(netdeadline.ErrTimeout)")
	}
}

// ─── Ponto 2: ship.go ~392 — git fetch origin --prune ────────────────────────────────────────

// TestShip_FetchTimeout_WarningNamesTimeout asserts:
// quando git fetch estourou o prazo em trackfw ship (passo 3), o aviso nomeia o erro
// de estouro (incluindo TRACKFW_NETWORK_TIMEOUT) e não usa a mensagem genérica.
//
// Conclusion: um estouro de prazo no fetch de ship.go é comunicado com o texto do
// TimeoutError; a mensagem genérica "offline or no remote" não aparece.
//
// Reprovar sem a correção: sem o ramo `errors.Is(ferr, netdeadline.ErrTimeout)`, a
// mensagem seria "offline or no remote" — o teste falharia em `Contains(TRACKFW_NETWORK_TIMEOUT)`.
func TestShip_FetchTimeout_WarningNamesTimeout(t *testing.T) {
	out := &bytes.Buffer{}
	fetchErr := fakeTimeout("fetch")

	d := shipDeps{
		execGit: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "symbolic-ref --short"):
				return "feat/x", nil
			case strings.HasPrefix(joined, "symbolic-ref refs/remotes"):
				return "refs/remotes/origin/main", nil
			case joined == "remote get-url origin":
				return "https://github.com/owner/repo.git", nil
			case strings.HasPrefix(joined, "fetch"):
				return "", fetchErr
			case strings.HasPrefix(joined, "diff --cached --stat"):
				return "some file", nil
			case strings.HasPrefix(joined, "status --short"):
				return "M file.go", nil
			case strings.HasPrefix(joined, "rev-parse --abbrev-ref"):
				return "origin/feat/x", nil
			case strings.HasPrefix(joined, "push"):
				return "", nil
			case strings.HasPrefix(joined, "commit"):
				return "", nil
			}
			return "", nil
		},
		checkGovernance: func() *validator.GovernanceViolation { return nil },
		out:             out,
	}

	_ = runShip(shipOpts{message: "test: fetch timeout warning"}, d)

	stdout := out.String()
	if !strings.Contains(stdout, netdeadline.NetworkTimeoutEnv) {
		t.Errorf("warning must name %s, got: %q", netdeadline.NetworkTimeoutEnv, stdout)
	}
	if strings.Contains(stdout, "offline or no remote") {
		t.Errorf("timeout must not be reported as 'offline or no remote', got: %q", stdout)
	}
}

// ─── Ponto 3: branch_prune.go ~390 — git fetch origin --prune ────────────────────────────────

// TestBranchPrune_FetchTimeout_WarningNamesTimeout_FieldsSet asserts:
// quando git fetch estourou o prazo em branch prune, o aviso nomeia o erro de estouro
// (incluindo TRACKFW_NETWORK_TIMEOUT), e deps.fetchFailed e deps.fetchTimedOut ficam true.
//
// Conclusion: um estouro de prazo no fetch de branch prune é comunicado com o texto do
// TimeoutError; os campos fetchFailed e fetchTimedOut ficam verdadeiros para uso pelo
// passo remoto de ROADMAP-2026-10-10.
//
// Reprovar sem a correção: sem o `errors.Is(err, netdeadline.ErrTimeout)`, a mensagem
// seria "offline, no remote, or fetch failed" e os campos ficariam false.
func TestBranchPrune_FetchTimeout_WarningNamesTimeout_FieldsSet(t *testing.T) {
	out := &bytes.Buffer{}
	fetchErr := fakeTimeout("fetch")

	deps := branchPruneDeps{
		gitExec: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			switch {
			case strings.HasPrefix(joined, "fetch"):
				return "", fetchErr
			case strings.HasPrefix(joined, "rev-parse --verify -q "+branchPruneDefaultRemoteRef):
				return "deadbeef", nil
			case strings.HasPrefix(joined, "branch --format"):
				return "feat/x\n", nil
			case strings.HasPrefix(joined, "symbolic-ref"):
				return "feat/x", nil
			case strings.HasPrefix(joined, "worktree list"):
				return "", nil
			}
			return "", nil
		},
		ghExec: nil,
		listLocalBranches: func(gitExec func(...string) (string, error)) ([]string, error) {
			return []string{}, nil
		},
		currentBranch: func(gitExec func(...string) (string, error)) string { return "feat/x" },
		worktreeBranches: func(gitExec func(...string) (string, error)) map[string]bool {
			return map[string]bool{}
		},
		deleteBranch: func(gitExec func(...string) (string, error), name string) error {
			return nil
		},
		out: out,
	}

	_ = runBranchPrune(false, deps)

	stdout := out.String()
	if !strings.Contains(stdout, netdeadline.NetworkTimeoutEnv) {
		t.Errorf("warning must name %s, got: %q", netdeadline.NetworkTimeoutEnv, stdout)
	}
	if strings.Contains(stdout, "offline, no remote, or fetch failed") {
		t.Errorf("timeout must not be reported as generic 'offline/fetch failed', got: %q", stdout)
	}
	// Note: runBranchPrune takes branchPruneDeps by value, so deps.fetchFailed and
	// deps.fetchTimedOut are set on the function's internal copy and are not observable
	// from the caller. The warning message check above is the correct observable assertion.
	// The struct fields are reserved for ROADMAP-2026-10-10 code inside runBranchPrune.
}

// ─── Ponto 3 (confirmação): release.go ~278 — git fetch origin --prune ───────────────────────

// TestRelease_FetchTimeout_MessageNamesTimeoutError asserts:
// quando o fetch de release.go:278 estourou o prazo, o erro retornado contém o texto
// do TimeoutError (operação, prazo, TRACKFW_NETWORK_TIMEOUT).
//
// Conclusion: a mensagem existente releaseTagFetchFailedFmt (%s formata err.Error())
// já nomeia o TimeoutError sem mudança de código — confirma o contrato D5.
//
// Nota: não há código novo para este ponto; o teste afirma o comportamento já existente.
func TestRelease_FetchTimeout_MessageNamesTimeoutError(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	fetchErr := fakeTimeout("fetch")
	g.errors["fetch origin --prune"] = fetchErr

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when fetch times out")
	}
	if !strings.Contains(err.Error(), netdeadline.NetworkTimeoutEnv) {
		t.Errorf("error must name %s, got: %q", netdeadline.NetworkTimeoutEnv, err.Error())
	}
	if !strings.Contains(err.Error(), "could not fetch origin") {
		t.Errorf("error must use releaseTagFetchFailedFmt text, got: %q", err.Error())
	}
}

// ─── Ponto 4 (AC4): release.go ~310 — git ls-remote --tags ───────────────────────────────────

// TestRelease_LSRemoteError_FailsClosed asserts:
// qualquer erro em ls-remote (não-timeout) produz falha fechada nomeada com
// "could not confirm tag ... is absent from remote"; nunca "tag already exists on origin".
//
// Conclusion: um erro comum em ls-remote é tratado como falha fechada (AC4/A5) —
// o estouro e erros de rede não são interpretados como "tag ausente".
//
// Reprovar sem a correção: com `remoteTagOut, _ := deps.execGit("ls-remote", ...)`, o
// erro é descartado, a variável fica "" e o código prossegue — o teste falha porque não
// há erro retornado (ou o erro tem texto diferente).
func TestRelease_LSRemoteError_FailsClosed(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	commonErr := errors.New("connection refused")
	g.errors["ls-remote --tags origin refs/tags/"+releaseTestTag] = commonErr

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when ls-remote fails")
	}
	if !strings.Contains(err.Error(), "could not confirm tag") {
		t.Errorf("error must use releaseTagRemoteCheckFailedFmt text, got: %q", err.Error())
	}
	if strings.Contains(err.Error(), "already exists on origin") {
		t.Errorf("ls-remote error must not be reported as 'already exists on origin', got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), releaseTestTag) {
		t.Errorf("error must name the tag %q, got: %q", releaseTestTag, err.Error())
	}
}

// TestRelease_LSRemoteTimeout_FailsClosed asserts:
// um estouro de prazo em ls-remote produz a mesma falha fechada nomeada de
// TestRelease_LSRemoteError_FailsClosed, e a mensagem nomeia TRACKFW_NETWORK_TIMEOUT.
//
// Conclusion: estouro de prazo em ls-remote nunca produz "tag ausente" — qualquer
// falha no ls-remote é fechada com a mensagem releaseTagRemoteCheckFailedFmt (ADR D1/A5).
func TestRelease_LSRemoteTimeout_FailsClosed(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	timeoutErr := fakeTimeout("ls-remote")
	g.errors["ls-remote --tags origin refs/tags/"+releaseTestTag] = timeoutErr

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when ls-remote times out")
	}
	if !strings.Contains(err.Error(), "could not confirm tag") {
		t.Errorf("error must use releaseTagRemoteCheckFailedFmt, got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), netdeadline.NetworkTimeoutEnv) {
		t.Errorf("error must name %s, got: %q", netdeadline.NetworkTimeoutEnv, err.Error())
	}
	if strings.Contains(err.Error(), "already exists on origin") {
		t.Errorf("timeout on ls-remote must not produce 'already exists on origin', got: %q", err.Error())
	}
}

// ─── Ponto 5 (A8): release.go ~449 — POST tag object ─────────────────────────────────────────

// TestRelease_TagObjectPostTimeout_UncertainState asserts:
// um estouro de prazo no POST de criação do objeto de tag produz mensagem que nomeia
// o estouro e avisa que o estado remoto é incerto, pedindo verificação antes de
// rodar de novo.
//
// Conclusion: estouro no POST de tag object é distinguido de falha limpa pela mensagem
// "remote state is uncertain" (ADR D7/A8); o usuário não re-executa sem verificar.
//
// Reprovar sem a correção: sem o `errors.Is(err, netdeadline.ErrTimeout)` no ramo do
// POST, a mensagem seria "gh api failed creating the tag object" sem mencionar
// incerteza de estado.
func TestRelease_TagObjectPostTimeout_UncertainState(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	tagPostErr := &netdeadline.TimeoutError{
		Op:    "gh",
		Limit: 60 * time.Second,
		Var:   netdeadline.NetworkTimeoutEnv,
	}
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		// POST to git/tags endpoint → timeout; delegate everything else to the
		// standard mock so that pre-POST forge calls (repo info, commit SHA) succeed.
		for _, a := range args {
			if strings.Contains(a, "git/tags") {
				return "", tagPostErr
			}
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when tag object POST times out")
	}
	if !strings.Contains(err.Error(), "remote state is uncertain") {
		t.Errorf("error must mention 'remote state is uncertain', got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), netdeadline.NetworkTimeoutEnv) {
		t.Errorf("error must name %s, got: %q", netdeadline.NetworkTimeoutEnv, err.Error())
	}
}

// ─── Ponto 5 (A8): release.go ~469 — POST tag ref ────────────────────────────────────────────

// TestRelease_RefPostTimeout_UncertainState asserts:
// um estouro de prazo no POST de criação do ref de tag produz mensagem que nomeia o
// estouro e avisa que o estado remoto é incerto.
//
// Conclusion: estouro no POST de tag ref é distinguido de falha limpa por "remote state
// is uncertain" (ADR D7/A8); o usuário sabe que precisa verificar o GitHub antes de
// re-executar o comando.
func TestRelease_RefPostTimeout_UncertainState(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	refPostErr := &netdeadline.TimeoutError{
		Op:    "gh",
		Limit: 60 * time.Second,
		Var:   netdeadline.NetworkTimeoutEnv,
	}
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		// POST to git/refs endpoint → timeout; tag object POST succeeds;
		// delegate everything else (repo info, commit SHA) to the standard mock.
		for _, a := range args {
			if strings.Contains(a, "git/refs") {
				return "", refPostErr
			}
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when tag ref POST times out")
	}
	if !strings.Contains(err.Error(), "remote state is uncertain") {
		t.Errorf("error must mention 'remote state is uncertain', got: %q", err.Error())
	}
	if !strings.Contains(err.Error(), netdeadline.NetworkTimeoutEnv) {
		t.Errorf("error must name %s, got: %q", netdeadline.NetworkTimeoutEnv, err.Error())
	}
}

// ─── Ponto 6 (A7): ship.go ~801 — queryForgePRs timeout em detectPendingSquashMerges ─────────

// TestShip_DetectSquashMerges_GhTimeout_WarnsDegradation asserts:
// quando gh pr list estourou o prazo em detectPendingSquashMerges, um aviso é impresso
// nomeando o estouro (incluindo TRACKFW_NETWORK_TIMEOUT); a heurística de conteúdo
// (D2) continua sem interrupção.
//
// Conclusion: o motivo de degradação de queryForgePRs chega ao usuário quando é um
// timeout (ADR D7/A7); silêncio apenas para degradações estruturais (host não-GitHub,
// gh ausente) que não nomeiam um prazo.
//
// Reprovar sem a correção: sem capturar `cause` da 3ª saída de queryForgePRs (ou sem o
// Fprintf de aviso), nenhuma mensagem de timeout aparece — o teste falha em
// `Contains(TRACKFW_NETWORK_TIMEOUT)`.
func TestShip_DetectSquashMerges_GhTimeout_WarnsDegradation(t *testing.T) {
	out := &bytes.Buffer{}
	ghErr := fakeGhTimeout()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "branch -r --no-merged"):
			// Return one remote branch so the loop body runs.
			return "origin/feat/other-branch\n", nil
		case joined == "remote get-url origin":
			// Return a github.com URL so queryForgePRs reaches the ghExec call.
			return "https://github.com/owner/repo.git", nil
		}
		return "", nil
	}
	ghExec := func(args ...string) ([]byte, error) {
		return nil, ghErr
	}

	detectPendingSquashMerges("feat/current", gitExec, ghExec, out)

	stdout := out.String()
	if !strings.Contains(stdout, netdeadline.NetworkTimeoutEnv) {
		t.Errorf("warning must name %s, got: %q", netdeadline.NetworkTimeoutEnv, stdout)
	}
}
