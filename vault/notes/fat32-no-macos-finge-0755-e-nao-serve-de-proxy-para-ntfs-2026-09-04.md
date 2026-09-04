# FAT32 montado no macOS finge `0755` — não é proxy do NTFS para o bit de execução

**Contexto:** ML-4A de `ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz`
trocou os asserts de bit de execução por uma **guarda medida** (sonda `chmod 0o755` → `stat`), e a
tentação óbvia foi validar o ramo suprimido localmente montando um volume não-POSIX, do mesmo jeito
que `[[dedup-lexical-nao-ve-caixa-diferente-em-apfs-e-o-conserto-obvio-suprime-em-linux-2026-09-03]]`
usou `hdiutil create -fs "Case-sensitive APFS"` para reproduzir a diferença de caixa.

**Medido, e o resultado é o inverso do esperado:**

```
hdiutil create -size 200m -fs "MS-DOS FAT32" -volname EXECBIT -o execbit
hdiutil attach execbit.dmg -mountpoint fatmnt -nobrowse   ->  /dev/disk5s1  DOS_FAT_32

exec_bit_representavel(fatmnt)  =  True     <- FAT32
exec_bit_representavel()        =  True     <- APFS
```

O VFS `msdos` do macOS **sintetiza** modo POSIX para um filesystem que não tem nenhum: todo arquivo
aparece como `0755` (ou o que a opção de mount disser), e `chmod` não falha — só não muda nada que
importe. A sonda devolve `True`, então o ramo suprimido **nunca é atingido**.

🔴 **A consequência é a que engana:** um volume FAT montado no macOS **não** reproduz o
comportamento do NTFS sob Go/Node/CPython no Windows, onde `Mode()&0111` / `mode & 0o111` /
`st_mode & 0o111` é **0** para todo arquivo. Quem usar FAT32 como stand-in vai concluir "o ramo de
NTFS está exercitado" tendo exercitado **o mesmo ramo POSIX de sempre** — a mesma classe de erro que
`[[job-de-windows-largo-so-reproduz-2-dos-11-defeitos-2026-08-30]]` registra: o instrumento
responde, mas não sobre o que se quer medir.

**O que sobra como verificação local, e o que ela vale:** forçar o retorno da sonda para `false` nos
3 runtimes exercita o **caminho de código** da supressão — provou que os testes seguem passando, que
nenhum some, e que a mensagem nomeada é emitida uma vez por sítio (Go 13, Node 11, Python 8). Isso é
prova de que a guarda **não deixa o teste vacuoso nem quebrado**; **não** é prova de que o NTFS cai
nela. Essa parte só o CI de Windows fecha.

**Regra prática:** para o bit de execução, o discriminante local não existe no macOS. Sondas de
filesystem exóticas resolvem **caixa** (`hdiutil` case-sensitive funciona de verdade), não **modo**.
Não gastar tempo procurando outra imagem — exFAT e MS-DOS FAT16 têm o mesmo VFS.

**Decisão que governa o remendo:**
`[[goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01]]` — o bit nunca foi
discriminante em NTFS, e o WSL (kernel Linux, ext4) continua coberto porque ali o bit é representável.
