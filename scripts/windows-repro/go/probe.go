//go:build ignore

// probe.go — sonda sob demanda (ADR-2026-08-30, decisão 3 / AC5 / AC6 / AC9),
// ML-3A. Responde perguntas pontuais sobre os primitivos de filesystem do
// Windows que os.Stat/os.Lstat do Go enxergam, imprimindo o resultado BRUTO
// — sem veredito, sem comparação contra "esperado". Quem lê decide o que
// significa (mesma régua de scripts/windows-repro/go/checks.go, camada 2).
//
// NÃO É a suíte de reprodução de defeito (checks.go, camada 2, mapeada 1:1
// aos 11 itens da issue #216 e que precisa nascer vermelha). A sonda não
// tem veredito REPRODUCED/ABSENT — só imprime o que o SO devolveu. Sonda
// prova o que alguém lembrou de perguntar; regressão prova que não voltou
// (ADR, decisão 3). Ver .github/workflows/windows-probe.yml para o texto
// completo desta distinção.
//
// Tag `//go:build ignore`, mesmo motivo de checks.go: INVISÍVEL para
// `go build ./...` / `go vet ./...` / `go test ./...` do job `go` em
// ubuntu-latest — só roda via `go run scripts/windows-repro/go/probe.go
// <subcomando>`.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: probe.go <statmode-common|statmode-chmod|lstat-common|lstat-symlink|lstat-junction|lstat-path|rmdir-junction|table> [args...]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "statmode-common":
		cmdStatModeCommon()
	case "statmode-chmod":
		cmdStatModeChmod()
	case "lstat-common":
		cmdLstatCommon()
	case "lstat-symlink":
		cmdLstatSymlink()
	case "lstat-junction":
		cmdLstatJunction()
	case "lstat-path":
		cmdLstatPath()
	case "rmdir-junction":
		cmdRmdirJunction()
	case "table":
		cmdTable()
	default:
		fmt.Fprintf(os.Stderr, "subcomando desconhecido: %s\n", os.Args[1])
		os.Exit(2)
	}
}

// printTempDirInfo imprime o diretório temporário que de fato foi resolvido
// ao lado de $env:RUNNER_TEMP — achado do ML-0A (hades-tf): os.MkdirTemp("",
// ...) resolve via %TEMP%, que NÃO é RUNNER_TEMP, e a alegação "todo link
// fica dentro de RUNNER_TEMP/workspace" não era verificável pelo próprio
// código. Em vez de reescrever a alegação, medimos a diferença aqui.
func printTempDirInfo(tmp string) {
	fmt.Printf("tempdir_resolvido=%s runner_temp=%s\n", tmp, os.Getenv("RUNNER_TEMP"))
}

func printMode(label, path string, info os.FileInfo, err error) {
	if err != nil {
		fmt.Printf("%s path=%s err=%v\n", label, path, err)
		return
	}
	m := info.Mode()
	fmt.Printf(
		"%s path=%s mode=%v bits=%#o ModeSymlink=%v ModeDir=%v ModeIrregular=%v\n",
		label, path, m, uint32(m), m&os.ModeSymlink != 0, m.IsDir(), m&os.ModeIrregular != 0,
	)
}

// Pergunta 1a — modo devolvido por os.Stat num arquivo comum.
func cmdStatModeCommon() {
	f, err := os.CreateTemp("", "trackfw-probe-common-*.txt")
	if err != nil {
		fmt.Printf("statmode-common err_create=%v\n", err)
		os.Exit(1)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)
	info, err := os.Stat(path)
	printMode("statmode-common", path, info, err)
}

// Pergunta 1b — modo devolvido por os.Stat após chmod +x (0755).
func cmdStatModeChmod() {
	f, err := os.CreateTemp("", "trackfw-probe-chmod-*.sh")
	if err != nil {
		fmt.Printf("statmode-chmod err_create=%v\n", err)
		os.Exit(1)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)
	if err := os.Chmod(path, 0o755); err != nil {
		fmt.Printf("statmode-chmod path=%s err_chmod=%v\n", path, err)
		os.Exit(1)
	}
	info, err := os.Stat(path)
	printMode("statmode-chmod", path, info, err)
}

// Pergunta 2 (referência) — Lstat num arquivo comum, para comparação com
// symlink e junction abaixo.
func cmdLstatCommon() {
	f, err := os.CreateTemp("", "trackfw-probe-lstat-common-*.txt")
	if err != nil {
		fmt.Printf("lstat-common err_create=%v\n", err)
		os.Exit(1)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)
	info, err := os.Lstat(path)
	printMode("lstat-common", path, info, err)
}

// Pergunta 2 — Lstat diante de um symlink criado por os.Symlink. Em
// windows-latest sem Developer Mode/admin, a criação em si costuma falhar
// com "A required privilege is not held by the client" (WinError 1314) —
// isso é sinal também: imprime o erro bruto de criação, não tenta
// contornar. Se a criação funcionar, faz Lstat no link.
func cmdLstatSymlink() {
	tmp, err := os.MkdirTemp("", "trackfw-probe-symlink-*")
	if err != nil {
		fmt.Printf("lstat-symlink err_mkdtemp=%v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	printTempDirInfo(tmp)

	target := tmp + string(os.PathSeparator) + "target.txt"
	if err := os.WriteFile(target, []byte("trackfw-probe-target\n"), 0o644); err != nil {
		fmt.Printf("lstat-symlink err_write_target=%v\n", err)
		os.Exit(1)
	}
	link := tmp + string(os.PathSeparator) + "link.txt"
	if err := os.Symlink(target, link); err != nil {
		fmt.Printf("lstat-symlink create_err=%v (esperado sem Developer Mode/admin — sinal em si, não falha da sonda)\n", err)
		return
	}
	info, err := os.Lstat(link)
	printMode("lstat-symlink", link, info, err)
}

// Pergunta central desta sonda (ver cabeçalho do arquivo, e comentário do
// ADR) — Lstat diante de uma JUNCTION criada por `mklink /J`, não por
// os.Symlink. Junction é um reparse point de tipo distinto de symlink
// (IO_REPARSE_TAG_MOUNT_POINT vs IO_REPARSE_TAG_SYMLINK) e, ao contrário de
// symlink, `mklink /J` NÃO exige privilégio elevado — por isso é o
// mecanismo que qualquer clone real do Git para Windows com
// core.symlinks=false pode encontrar sem Developer Mode. A pergunta bruta:
// o Lstat do Go marca ModeSymlink para esse reparse point, ou não?
func cmdLstatJunction() {
	tmp, err := os.MkdirTemp("", "trackfw-probe-junction-*")
	if err != nil {
		fmt.Printf("lstat-junction err_mkdtemp=%v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	printTempDirInfo(tmp)

	targetDir := tmp + string(os.PathSeparator) + "targetdir"
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		fmt.Printf("lstat-junction err_mkdir_target=%v\n", err)
		os.Exit(1)
	}
	junction := tmp + string(os.PathSeparator) + "junctionlink"

	// mklink /J não existe como binário — é builtin do cmd.exe.
	out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, targetDir).CombinedOutput()
	fmt.Printf("lstat-junction mklink_output=%q mklink_err=%v\n", string(out), err)
	if err != nil {
		fmt.Println("lstat-junction create_failed — sonda não pode medir Lstat sobre a junction nesta execução")
		return
	}

	info, err := os.Lstat(junction)
	printMode("lstat-junction", junction, info, err)

	// Comparação direta: Stat (segue o link) sobre o mesmo caminho, para
	// deixar explícito que a junction FUNCIONA como redirecionamento —
	// só a pergunta é se Lstat a marca como symlink.
	statInfo, statErr := os.Stat(junction)
	printMode("stat-junction(segue o link)", junction, statInfo, statErr)
}

// lstat-path <path> — variante genérica de lstat-common para inspecionar um
// caminho arbitrário passado pelo chamador (usado pela Pergunta 7 do
// workflow, sobre o symlink que `git checkout` materializou via plumbing —
// não criado por este programa).
func cmdLstatPath() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: probe.go lstat-path <caminho>")
		os.Exit(2)
	}
	path := os.Args[2]
	info, err := os.Lstat(path)
	printMode("lstat-path", path, info, err)
}

// rmdir-junction — Pergunta 10 do workflow (ML-1A). os.Remove sobre uma
// junction cujo alvo está VAZIO. Discriminante citado pelo achado do ML-0A
// (hades-tf): pypi/trackfw/integrations/manager.py:589 _remove_empty
// depende SÓ de `except OSError` ao redor do rmdir() para parar de subir
// removendo ancestrais — este é literalmente o dado que decide "Python
// para" vs "Python sobe removendo diretórios do usuário". Imprime o erro
// (ou sucesso) cru, e depois — separadamente — se a JUNCTION sumiu e se o
// ALVO sobreviveu.
func cmdRmdirJunction() {
	tmp, err := os.MkdirTemp("", "trackfw-probe-rmdir-*")
	if err != nil {
		fmt.Printf("rmdir-junction err_mkdtemp=%v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	printTempDirInfo(tmp)

	targetDir := tmp + string(os.PathSeparator) + "targetdir"
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		fmt.Printf("rmdir-junction err_mkdir_target=%v\n", err)
		os.Exit(1)
	}
	junction := tmp + string(os.PathSeparator) + "junctionlink"

	out, mkErr := exec.Command("cmd", "/c", "mklink", "/J", junction, targetDir).CombinedOutput()
	fmt.Printf("rmdir-junction mklink_output=%q mklink_err=%v\n", string(out), mkErr)
	if mkErr != nil {
		fmt.Println("rmdir-junction create_failed — sonda não pode medir rmdir sobre a junction nesta execução")
		return
	}

	removeErr := os.Remove(junction)
	fmt.Printf("rmdir-junction os.Remove(junction)_err=%v\n", removeErr)

	_, statJunctionErr := os.Lstat(junction)
	fmt.Printf("rmdir-junction junction_ainda_existe=%v (lstat_err=%v)\n", statJunctionErr == nil, statJunctionErr)

	_, statTargetErr := os.Lstat(targetDir)
	fmt.Printf("rmdir-junction alvo_ainda_existe=%v (lstat_err=%v)\n", statTargetErr == nil, statTargetErr)
}

// printTableRow — mesmo formato usado por probe.js/probe.py (prefixo
// "TABELA runtime=... target=..."), para que os 3 braços do workflow
// (Pergunta 11) fiquem legíveis lado a lado no mesmo step, sem cruzar logs
// (AC5). Sem veredito (AC6): só os bits crus que este runtime usa para
// decidir "é link?".
func printTableRow(runtime, target string, info os.FileInfo, err error) {
	if err != nil {
		fmt.Printf("TABELA runtime=%s target=%s err=%v\n", runtime, target, err)
		return
	}
	m := info.Mode()
	fmt.Printf(
		"TABELA runtime=%s target=%s ModeSymlink=%v ModeDir=%v ModeIrregular=%v\n",
		runtime, target, m&os.ModeSymlink != 0, m.IsDir(), m&os.ModeIrregular != 0,
	)
}

// table — Pergunta 11 do workflow. Recria arquivo comum, symlink e junction
// do zero (fixture própria, isolada das perguntas anteriores) e imprime uma
// linha TABELA por alvo.
func cmdTable() {
	tmp, err := os.MkdirTemp("", "trackfw-probe-table-*")
	if err != nil {
		fmt.Printf("TABELA runtime=go err_mkdtemp=%v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)
	printTempDirInfo(tmp)

	common := tmp + string(os.PathSeparator) + "common.txt"
	if err := os.WriteFile(common, []byte("x"), 0o644); err != nil {
		fmt.Printf("TABELA runtime=go target=arquivo err=%v\n", err)
	} else {
		info, err := os.Lstat(common)
		printTableRow("go", "arquivo", info, err)
	}

	target := tmp + string(os.PathSeparator) + "target.txt"
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		fmt.Printf("TABELA runtime=go target=symlink err_write_target=%v\n", err)
	} else {
		link := tmp + string(os.PathSeparator) + "link.txt"
		if err := os.Symlink(target, link); err != nil {
			fmt.Printf("TABELA runtime=go target=symlink err_create=%v\n", err)
		} else {
			info, err := os.Lstat(link)
			printTableRow("go", "symlink", info, err)
		}
	}

	targetDir := tmp + string(os.PathSeparator) + "targetdir"
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		fmt.Printf("TABELA runtime=go target=junction err_mkdir_target=%v\n", err)
		return
	}
	junction := tmp + string(os.PathSeparator) + "junctionlink"
	out, mkErr := exec.Command("cmd", "/c", "mklink", "/J", junction, targetDir).CombinedOutput()
	if mkErr != nil {
		fmt.Printf("TABELA runtime=go target=junction err_create=%v output=%q\n", mkErr, string(out))
		return
	}
	info, err := os.Lstat(junction)
	printTableRow("go", "junction", info, err)
}
