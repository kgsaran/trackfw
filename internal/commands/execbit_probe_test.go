package commands

// ML-4A de ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz:
// guarda de plataforma MEDIDA para os asserts de bit de execucao.
//
// ## Por que existe
//
// Em NTFS o bit de execucao nao existe: os.Stat(p).Mode().Perm()&0111 devolve 0 para
// TODO arquivo, inclusive imediatamente depois de os.Chmod(p, 0o755). Um assert
// "o artefato gerado e executavel" nao mede o gerador ali -- mede uma propriedade
// que o sistema de arquivos nao tem, e reprova sempre.
//
// A decisao de suprimir a checagem apenas onde o bit nao e representavel esta tomada em
// vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md:
// o bit NUNCA foi discriminante em NTFS, e o WSL (kernel Linux, ext4) continua coberto
// porque ali o bit e representavel de verdade.
//
// ## Detecao pela CONDICAO, nao por runtime.GOOS
//
// A sonda MEDE o filesystem em vez de inferir da plataforma -- mesmo idioma ja usado por
// _exec_bit_representavel em pypi/tests/test_validator.py. Consequencias:
//   - um Windows que um dia represente o bit (ou um ext4 montado sob Windows) volta a ser
//     verificado sozinho, sem editar teste;
//   - um volume POSIX exotico que nao represente o bit nao produz vermelho enganoso;
//   - a sonda recebe o DIRETORIO onde o teste escreve, porque e esse o filesystem sob
//     medicao -- um probe no tmpdir global mediria outro volume (este repositorio ja roda
//     testes sobre volume 'hdiutil' case-sensitive).
//
// ## Nao e skip, de proposito
//
// O teste inteiro continua rodando; so o assert do bit e suprimido, e a supressao NOMEIA
// a garantia que deixou de ser verificada. Um teste pulado nao mede mais que um teste que
// nao existe.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// execBitRepresentavel responde: neste sistema de arquivos, um arquivo criado em dir e
// levado a 0o755 por os.Chmod passa a ter Mode().Perm()&0111 != 0?
//
// Falha o teste (t.Fatalf) se a propria sonda nao puder ser executada -- "nao consegui
// medir" nao pode virar supressao silenciosa dos asserts.
func execBitRepresentavel(t *testing.T, dir string) bool {
	t.Helper()

	f, err := os.CreateTemp(dir, "trackfw-execbit-probe-*.sh")
	if err != nil {
		t.Fatalf("sonda do bit de execucao nao pode ser criada em %s: %v", dir, err)
	}
	p := f.Name()
	if err := f.Close(); err != nil {
		t.Fatalf("sonda do bit de execucao: close %s: %v", p, err)
	}
	defer func() { _ = os.Remove(p) }()

	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatalf("sonda do bit de execucao: chmod 0755 em %s: %v", p, err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("sonda do bit de execucao: stat %s: %v", p, err)
	}
	return info.Mode().Perm()&0o111 != 0
}

// execBitRepresentavelPara e a forma usada nos call sites: mede o filesystem do arquivo
// que esta sendo verificado, sondando o diretorio que o contem.
func execBitRepresentavelPara(t *testing.T, artefato string) bool {
	t.Helper()
	return execBitRepresentavel(t, filepath.Dir(artefato))
}

// execBitNaoExercitado registra, com tag grepavel, QUAL garantia deixou de ser verificada
// e por que. Quem ler o log do CI de Windows daqui a seis meses precisa saber o nome do
// artefato -- nao um "<script>" generico.
//
// 🔴 LIMITE MEDIDO, e ele nao esta fechado no Go. O job de Windows roda
// `go test -p 1 -parallel 1 -timeout 20m ./...` (.github/workflows/quality.yml:384) SEM `-v`, e
// nesse modo o `go test` BUFERIZA e DESCARTA toda a saida de um pacote que PASSA -- t.Logf e
// escrita direta em os.Stderr, indistintamente (medido: as duas formas dao 0 ocorrencias sob
// `go test -count=1 -p 1 -parallel 1`, e 13 sob `-v`). Como estes testes PASSAM no Windows
// justamente por causa da supressao, a mensagem nao chega ao log de la.
//
// os.Stderr e ainda assim a escolha melhor, por uma diferenca real: quando QUALQUER teste do
// pacote reprova, o `go test` despeja a saida do pacote inteiro, e a mensagem aparece junto --
// t.Logf so apareceria se fosse o proprio teste a reprovar.
//
// O fechamento e de UMA palavra e mora fora de um arquivo de teste (`-v` na linha 384 do
// workflow), entao ficou como lacuna reportada, nao como remendo silencioso. Node e Python ja
// estao fechados: `npm test` propaga o console.error, e o Python usa `warnings.warn`, que o
// pytest mostra no warnings summary mesmo sob `-q` e com a suite verde.
func execBitNaoExercitado(t *testing.T, artefato string) {
	t.Helper()
	fmt.Fprintf(os.Stderr,
		"EXEC-BIT-NAO-EXERCITADO: %s [%s] -- garantia NAO verificada: \"o artefato foi criado com o bit de execucao (0755)\". "+
			"Este sistema de arquivos devolve Mode().Perm()&0111 == 0 mesmo apos os.Chmod(0o755) (NTFS nao representa o bit). "+
			"O restante do teste continua medindo. "+
			"Decisao: vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md\n",
		artefato, t.Name())
}
