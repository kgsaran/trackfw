//go:build ignore

// checks.go — verificações Go da suíte de reprodução de defeito (camada 2)
// para ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-
// sob-demanda, ML-1A.
//
// Tag `//go:build ignore` de propósito: mantém este arquivo INVISÍVEL para
// `go build ./...`, `go vet ./...` e `go test ./...` (que rodam no job `go`
// existente em ubuntu-latest — AC11 exige que ele siga verde e inalterado).
// Só é executado explicitamente via `go run scripts/windows-repro/go/checks.go
// <subcomando>`, que HONRA a tag ao nomear o arquivo diretamente (verificado:
// `go run arquivo-com-ignore.go` executa; `go build ./...` no mesmo diretório
// não encontra pacote nenhum).
//
// Cada subcomando chama o MESMO primitivo de stdlib que o produto usa em
// produção (grep confirmado antes de escrever este arquivo:
// internal/validator/validator_git_branch_guard.go:133,231 chama
// os.UserHomeDir() diretamente) — não há mock aqui.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: checks.go <home|execbit|shc>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "home":
		cmdHome()
	case "execbit":
		cmdExecBit()
	case "shc":
		cmdShC()
	default:
		fmt.Fprintf(os.Stderr, "subcomando desconhecido: %s\n", os.Args[1])
		os.Exit(2)
	}
}

// item 2 — $HOME ignorado no Windows. Chama exatamente o que a produção
// chama (os.UserHomeDir()) sob o ambiente que o chamador (run.ps1) já
// preparou (HOME=fakeA, USERPROFILE=fakeB, deliberadamente diferentes).
// Imprime só o valor resolvido, sem julgamento — o chamador decide
// REPRODUCED/ABSENT comparando contra HOME e USERPROFILE.
func cmdHome() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(home)
}

// item 3 — bit de execução sempre "presente" no Windows
// (internal/validator/validator_credential_guard.go:377,
// validator_git_branch_guard.go:193: info.Mode()&0111==0). Confirmatório —
// a evidência primária é a camada 1 (go test ./... já roda os testes reais
// do validator que fazem esta mesma asserção). Replicado aqui em isolamento
// para ficar citável sem depender do resultado bruto de `go test`.
func cmdExecBit() {
	f, err := os.CreateTemp("", "trackfw-execbit-*.sh")
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro criando temp file: %v\n", err)
		os.Exit(1)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	if err := os.Chmod(path, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "erro no chmod: %v\n", err)
		os.Exit(1)
	}
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro no stat: %v\n", err)
		os.Exit(1)
	}
	mode := info.Mode()
	fmt.Printf("mode=%v bit0111=%d\n", mode, mode&0o111)
}

// item 7 — `sh -c` hardcodado em internal/commands/barrier.go:729
// (`exec.Command("sh", "-c", command)`). Replica o MESMO primitivo, fora do
// `barrier` (que exigiria fixture de wave completa, fora do escopo desta
// ML) — mede diretamente se `sh` resolve no PATH do runner e o que ele
// devolve, sem mascarar stdout/stderr (ao contrário do caminho de erro real
// do barrier.go, que hoje descarta os dois — achado à parte, não corrigido
// aqui).
func cmdShC() {
	c := exec.Command("sh", "-c", "echo trackfw-sh-check-ok")
	out, err := c.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			fmt.Printf("sh-not-found err=%v\n", err)
			return
		}
		fmt.Printf("sh-ran-nonzero err=%v output=%q\n", err, string(out))
		return
	}
	fmt.Printf("sh-ran-ok output=%q\n", string(out))
}
