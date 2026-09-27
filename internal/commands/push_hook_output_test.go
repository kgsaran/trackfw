package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #449 — a saída do hook pre-push chegava a ser capturada e era descartada.
//
// Reconciliação exigida pela Regra Dura: cada teste declara, em uma frase, qual
// conclusão da medição ele afirma.
// ────────────────────────────────────────────────────────────────────────────

// hookRejection is what a pre-push hook writes to ITS stdout, which git forwards to
// its own stdout and defaultGitExec captures with cmd.Output(). It is the only text
// that says WHY the push was refused.
const hookRejection = "Fast CI gates (pre-push)...Failed\n- hook id: fast-gates\n- exit code: 1\n\nlarge file detected: assets/dump.bin (18 MB)"

// pushFailingGit builds an execGit that behaves like a repo whose pre-push hook
// rejects: `git push` exits non-zero, stderr carries the symptom, stdout carries the
// hook's reason.
func pushFailingGit(branch, hookStdout string) func(args ...string) (string, error) {
	return func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.HasPrefix(joined, "symbolic-ref --short"):
			return branch, nil
		case strings.HasPrefix(joined, "rev-parse --abbrev-ref --symbolic-full-name @{u}"):
			return "", errors.New("no upstream")
		case strings.HasPrefix(joined, "fetch"):
			return "", errors.New("could not connect")
		case strings.HasPrefix(joined, "push"):
			// Exactly the defaultGitExec contract: stdout as the first value, stderr
			// folded into the error.
			return hookStdout, errors.New("error: failed to push some refs to 'github.com:owner/repo.git'")
		}
		return "", nil
	}
}

func pushDepsFailing(branch, hookStdout string) pushDeps {
	return pushDeps{
		execGit:         pushFailingGit(branch, hookStdout),
		checkGovernance: func() []string { return nil },
		out:             &bytes.Buffer{},
	}
}

// AFIRMA: a razão da recusa estava sendo produzida pelo processo e jogada fora no
// `_` do sítio de chamada — a correção a devolve ao usuário.
func TestPush_HookReasonReachesTheUser(t *testing.T) {
	err := runPush(pushOpts{}, pushDepsFailing("feat/x", hookRejection))
	if err == nil {
		t.Fatal("expected the push to fail")
	}
	if !strings.Contains(err.Error(), "large file detected: assets/dump.bin (18 MB)") {
		t.Fatalf("the hook's reason did not reach the error; got:\n%s", err.Error())
	}
	// O sintoma continua presente: a correção acrescenta, não substitui.
	if !strings.Contains(err.Error(), "failed to push some refs") {
		t.Fatalf("the git symptom was lost; got:\n%s", err.Error())
	}
}

// AFIRMA: quem falha SEM saída de hook continua lendo exatamente a mensagem de
// antes — o controle que impede a correção de mudar o contrato de quem já estava
// certo. Sem ele, "a mensagem melhorou" e "a mensagem mudou para todo mundo" seriam
// indistinguíveis.
func TestPush_WithoutHookOutput_MessageIsUnchanged(t *testing.T) {
	err := runPush(pushOpts{}, pushDepsFailing("feat/x", ""))
	if err == nil {
		t.Fatal("expected the push to fail")
	}
	const antes = "git push failed: error: failed to push some refs to 'github.com:owner/repo.git'"
	if err.Error() != antes {
		t.Fatalf("expected the previous wording byte for byte\n want: %q\n  got: %q", antes, err.Error())
	}
}

// AFIRMA: a causa tem DOIS sítios com o mesmo descarte — push.go e ship.go —, e o
// ponto único os cobre. Testar só um deixaria metade do defeito vivo com a aparência
// de corrigido, que é o padrão que a Regra Dura de Causa Raiz existe para impedir.
func TestPushFailure_IsTheSinglePointForBothSites(t *testing.T) {
	base := errors.New("error: failed to push some refs")

	comSaida := pushFailure(hookRejection, base)
	if !strings.Contains(comSaida.Error(), "large file detected") {
		t.Fatalf("hook reason missing:\n%s", comSaida.Error())
	}

	semSaida := pushFailure("", base)
	if semSaida.Error() != "git push failed: error: failed to push some refs" {
		t.Fatalf("empty case changed wording: %q", semSaida.Error())
	}

	// Espaço em branco puro é o mesmo que ausência: um hook que só imprime "\n" não
	// deve produzir uma linha vazia pendurada na mensagem.
	soBranco := pushFailure("\n \t\n", base)
	if soBranco.Error() != semSaida.Error() {
		t.Fatalf("whitespace-only stdout should read as absent:\n%s", soBranco.Error())
	}
}
