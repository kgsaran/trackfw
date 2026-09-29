//go:build windows

package generators

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// #308 — as três variáveis que o shim do `gh` injeta não tinham guarda nenhuma.
//
// O shim do `check-release-tag-parity.sh` chama `bash.exe <script> "$@"` a partir
// de um processo NATIVO. Nessa fronteira o runtime do MSYS reconstrói o argv a
// partir da linha de comando do Windows e aplica expansão de chaves: o
// `repos/{owner}/{repo}` que o produto passa chega como `repos/owner/repo`, e o
// `case` do stub cai no ramo "unexpected gh call". O `de2dc712` (#311) fechou isso
// injetando MSYS=noglob, MSYS_NO_PATHCONV=1 e MSYS2_ARG_CONV_EXCL=*.
//
// 🔴 Por que este arquivo existe: aquele gate entra em `parity-rest`, e o
// `parity-other-gates` roda em `ubuntu-latest` (quality.yml). O shim só existe no
// Windows. Então as três variáveis não são exercitadas por CI nenhum — remover as
// três não reprovaria nada, e os 48 rótulos do gate só cairiam quando alguém
// rodasse o gate à mão num Windows.
//
// 🔴 E por que AQUI, em internal/generators: é onde já moram os testes que afirmam
// coisas sobre scripts do repositório (git_branch_guard_test.go, os de attention).
// O repositório não tem pacote Go para `scripts/`, e criar um só para isto seria
// superfície nova para uma afirmação pequena.
//
// Reconciliação (Regra Dura): a frase de cada teste está no seu comentário.
// ────────────────────────────────────────────────────────────────────────────

// msysEnvVars são as três que o shim injeta. Lista única: os dois testes a usam.
var msysEnvVars = []string{
	"MSYS=noglob",
	"MSYS_NO_PATHCONV=1",
	"MSYS2_ARG_CONV_EXCL=*",
}

// envSemMSYS devolve o ambiente do processo SEM nenhuma variável MSYS.
//
// 🔴 É o que torna o braço A honesto: se o runner já tivesse MSYS=noglob no
// ambiente, herdar o env faria o braço "sem as variáveis" passar por acidente, e o
// teste inteiro afirmaria o contrário do que pretende.
func envSemMSYS() []string {
	var out []string
	for _, kv := range os.Environ() {
		nome := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			nome = kv[:i]
		}
		if strings.HasPrefix(nome, "MSYS") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// rodaBash escreve um script que ecoa cada argumento numa linha e o invoca via
// bash.exe — a MESMA forma do shim (caminho de script, nunca `-c`).
func rodaBash(t *testing.T, env []string) []string {
	t.Helper()

	if _, err := exec.LookPath("bash.exe"); err != nil {
		t.Skipf("bash.exe não está no PATH: %v", err)
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "eco")
	if err := os.WriteFile(script, []byte("#!/usr/bin/env bash\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Os dois valores medidos na #308: a chave que o produto realmente passa, e o
	// caso que DISCRIMINA expansão de chaves de conversão de caminho — conversão
	// não multiplica argumento.
	cmd := exec.Command("bash.exe", script, "repos/{owner}/{repo}", "a{b,c}d")
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash.exe falhou: %v (saída: %q)", err, string(out))
	}
	linhas := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	for i := range linhas {
		linhas[i] = strings.TrimRight(linhas[i], "\r")
	}
	return linhas
}

// AFIRMA: sem as três variáveis o defeito da #308 reproduz — as chaves somem e
// `a{b,c}d` vira DOIS argumentos —, e com elas não reproduz. O braço sem as
// variáveis é o que prova que a guarda é necessária; sem ele, "com as variáveis
// passa" não distinguiria remédio de placebo.
func TestShimMSYSEnv_IsNecessaryAndSufficient(t *testing.T) {
	semVars := rodaBash(t, envSemMSYS())
	if len(semVars) == 2 && semVars[0] == "repos/{owner}/{repo}" {
		t.Skip("esta máquina não reconstrói o argv nesta fronteira — o defeito da #308 " +
			"não reproduz aqui, então o braço de controle não discrimina e o teste não afirma nada")
	}
	// O discriminante: expansão de chaves MULTIPLICA argumento; conversão de
	// caminho não. Se aqui viessem 2 argumentos com as chaves perdidas, seria outro
	// mecanismo, e a #308 precisaria ser remedida.
	if len(semVars) != 3 {
		t.Fatalf("sem as variáveis esperava 3 argumentos (a{b,c}d expandido em dois), vieram %d: %q", len(semVars), semVars)
	}
	if semVars[0] != "repos/owner/repo" {
		t.Fatalf("sem as variáveis esperava as chaves removidas, veio %q", semVars[0])
	}

	comVars := rodaBash(t, append(envSemMSYS(), msysEnvVars...))
	if len(comVars) != 2 {
		t.Fatalf("com as variáveis esperava 2 argumentos (nada expandido), vieram %d: %q", len(comVars), comVars)
	}
	if comVars[0] != "repos/{owner}/{repo}" {
		t.Fatalf("com as variáveis as chaves deviam sobreviver, veio %q", comVars[0])
	}
	if comVars[1] != "a{b,c}d" {
		t.Fatalf("com as variáveis a{b,c}d devia chegar inteiro, veio %q", comVars[1])
	}
}

// AFIRMA: o shim do gate CONTINUA injetando as três. É este o ratchet — o teste
// acima prova por que elas importam, mas sozinho não pegaria a remoção delas,
// porque ele mesmo as define. Sem esta asserção, apagá-las do gate não reprovaria
// nada em lugar nenhum.
func TestReleaseTagParityShim_StillInjectsTheMSYSEnv(t *testing.T) {
	caminho := filepath.Join("..", "..", "scripts", "check-release-tag-parity.sh")
	dados, err := os.ReadFile(caminho)
	if err != nil {
		t.Skipf("gate não encontrado a partir deste pacote (%v) — nada a afirmar", err)
	}
	texto := string(dados)

	// Guarda de vacuidade: se o shim deixar de existir, as asserções abaixo passariam
	// a falar de um sítio que não existe mais, e o teste viraria ruído.
	if !strings.Contains(texto, "_build_gh_stub_shim_once") {
		t.Fatalf("o construtor do shim sumiu de %s — esta guarda precisa ser remedida, não removida", caminho)
	}

	for _, v := range msysEnvVars {
		// O gate escreve o fonte Go do shim entre aspas, então procuro o literal.
		if !strings.Contains(texto, "\""+v+"\"") {
			t.Errorf("o shim deixou de injetar %q — sem ela o argv é reconstruído na fronteira "+
				"nativo→MSYS e o `case` do stub cai em \"unexpected gh call\" (#308, corrigido no #311)", v)
		}
	}
}
