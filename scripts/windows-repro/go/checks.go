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
// ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2 (ML-2A/ML-2D): os
// subcomandos "shc" e "gatequote" foram REMOVIDOS daqui. O item 2 do run.ps1
// agora invoca o binário real do trackfw (`trackfw agents models`), não
// `checks.go home` — mas `cmdHome` abaixo PERMANECE: `.github/workflows/
// quality.yml:280` reusa `go run scripts/windows-repro/go/checks.go home`
// como precondição AC12 (confirma que a isolação de HOME/USERPROFINE do job
// colou), um consumidor externo a este roadmap que não pode ser quebrado por
// esta ML — ver docs/portabilidade/2026-09-05-enumeracao-dos-checks-do-
// harness-de-windows.md, seção "Consumidor externo ao harness". O item 7
// agora invoca `trackfw barrier` de verdade (ver run.ps1); `cmdShC` e
// `cmdGateQuote`, que replicavam `exec.Command("sh","-c",...)` fora do
// `barrier`, não têm mais consumidor e foram removidos — mantê-los mortos
// convidaria a reintrodução do mesmo defeito (substituto medindo a si
// mesmo) que este roadmap corrigiu.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: checks.go <home|execbit>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "home":
		cmdHome()
	case "execbit":
		cmdExecBit()
	default:
		fmt.Fprintf(os.Stderr, "subcomando desconhecido: %s\n", os.Args[1])
		os.Exit(2)
	}
}

// cmdHome — NÃO é mais usado pelo item 2 do run.ps1 (ver ML-2A: o item 2
// agora invoca `trackfw agents models`, o binário real, via
// internal/homedir/homedir.go). Mantido apenas porque
// `.github/workflows/quality.yml:280` o reusa como precondição AC12 — ver
// comentário do pacote acima. Não remover sem atualizar quality.yml.
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
// validator_git_branch_guard.go:193: info.Mode()&0111==0). CONFIRMATÓRIO
// (ML-2B, decisão registrada em run.ps1): a evidência primária é a camada 1
// (go test ./... já roda TestCredentialGuardHookResolvable_
// WindowsNaoDisparaBitDeExecucao / TestGitBranchGuardHookResolvable_
// WindowsNaoDisparaBitDeExecucao, que exercitam o guard real via
// validator.CurrentGOOS). Replicado aqui em isolamento só para ficar citável
// sem depender do resultado bruto de `go test` — o veredito NUNCA entra no
// contador de REPRODUCED/INCONCLUSIVE do run.ps1 (ver Add-Result do item 3).
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
