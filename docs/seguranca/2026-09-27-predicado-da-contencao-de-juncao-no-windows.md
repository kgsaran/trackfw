# Parecer de Segurança — ML-0A: Predicado da Contenção de Junção no Windows

> Emitido: 2026-09-27 | REQ: `REQ-2026-09-27` | Issue: #444 | Agente: `hades-tf`
> VM: `Lab@192.168.64.6` (Go 1.27.0 windows/arm64, Windows 11 ARM64)

---

## 1. Mecanismo do Defeito (confirmado por medição e leitura de fonte)

`internal/pathguard/pathguard.go:88` testa um único bit:

```go
if err == nil && info.Mode()&os.ModeSymlink != 0 {
    return fmt.Errorf("refusing symlink path %q", current)
}
```

**Por que a junção escapa**: o bit `ModeSymlink` nunca acende em `IO_REPARSE_TAG_MOUNT_POINT`
(junção `mklink /J`) quando o módulo usa `go >= 1.23` no `go.mod`. O módulo trackfw usa
`go 1.25.2`, que ativa `winsymlink=1`.

O teste `TestRejectSymlinks_WalksThroughAJunction` no CI de Windows confirma: com junção no
caminho, `RejectSymlinks(...) = nil` — o guard é silencioso e a escrita alcança fora da raiz.

---

## 2. O Mecanismo `winsymlink` — por que a tabela é uma propriedade do módulo

Go 1.23 introduziu `GODEBUG=winsymlink` e mudou o padrão:

| `go` no `go.mod` | comportamento padrão | `IO_REPARSE_TAG_MOUNT_POINT` (`mklink /J`) |
|---|---|---|
| `< 1.23` | `winsymlink=0` (`modePreGo1_23()`) | → `ModeSymlink` |
| `>= 1.23` (trackfw: `1.25.2`) | `winsymlink=1` (`mode()`) | → `ModeIrregular` |

**Fonte primária (Go 1.27.0, `src/os/types_windows.go`, linhas 163–231):**

```go
// mode() — usado quando winsymlink=1
switch fs.ReparseTag {
case syscall.IO_REPARSE_TAG_SYMLINK:   m |= ModeSymlink
case windows.IO_REPARSE_TAG_AF_UNIX:   m |= ModeSocket
case windows.IO_REPARSE_TAG_DEDUP:     // tratado como arquivo regular (intencional)
default:                               m |= ModeIrregular   // ← MOUNT_POINT cai aqui
}

// modePreGo1_23() — usado quando winsymlink=0
if fs.ReparseTag == syscall.IO_REPARSE_TAG_SYMLINK ||
    fs.ReparseTag == windows.IO_REPARSE_TAG_MOUNT_POINT {
    return m | ModeSymlink   // ← junção = symlink no comportamento antigo
}
```

**A/B verificado**: mesmo binário Go 1.27.0, mesmo código de medição, mesma VM ARM64:

| `go` no `go.mod` | junção (`mklink /J`) | Mode (hex) |
|---|---|---|
| `go 1.21` | `ModeSymlink=true, ModeIrregular=false` | `0x80001b6` (`L...`) |
| `go 1.25` | `ModeSymlink=false, ModeIrregular=true` | `0x801b6` (`?...`) |

🔴 **A tabela KG/reporter reflete go.mod `>= 1.23` (winsymlink=1), não ARM64 nem uma versão
específica do Go.** Um agente que compilar uma sonda com `go 1.21` no go.mod vai medir
`ModeSymlink=true` e concluir erroneamente que o guard já funciona.

---

## 3. Tabela Completa: Objeto × Tag × Modo (no contexto do módulo trackfw)

Todos os `os.Lstat` abaixo medidos dentro do pacote `pathguard` (go.mod: `go 1.25.2`,
`winsymlink=1`). Tags via `fsutil reparsepoint query`.

| Objeto | Reparse Tag | ModeSymlink | ModeIrregular | Observação |
|---|---|---|---|---|
| **junção `mklink /J`** (user-created) | `0xa0000003` MOUNT_POINT | `false` | **true** | VETOR DE ATAQUE |
| `C:\Users\Lab\Meus Documentos` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Users\Lab\Configurações Locais` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Users\Lab\Dados de Aplicativos` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Users\Lab\Menu Iniciar` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Users\Lab\Recent` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Users\Lab\SendTo` | `0xa0000003` MOUNT_POINT | `false` | **true** | compat profile |
| `C:\Documents and Settings` | `0xa0000003` MOUNT_POINT | `false` | **true** | system compat |
| `C:\ProgramData\Desktop` | `0xa0000003` MOUNT_POINT | `false` | **true** | system compat |
| `C:\Users\All Users` | `0xa000000c` SYMLINK | `true` | `false` | symlink real |
| symlink `mklink /D` | `0xa000000c` SYMLINK | `true` | `false` | já capturado pelo guard atual |
| diretório comum | — | `false` | `false` | seguro |
| arquivo comum | — | `false` | `false` | seguro |
| `C:\Users\Lab` (raiz de perfil) | — | `false` | `false` | seguro |
| `C:\Users\Lab\Documents` | — | `false` | `false` | seguro |
| `C:\Users\Lab\Desktop` | — | `false` | `false` | seguro |
| `C:\Users\Lab\Downloads` | — | `false` | `false` | seguro |
| `C:\Users\Lab\AppData` | — | `false` | `false` | seguro |
| `C:\Users\Lab\AppData\Local` | — | `false` | `false` | seguro |
| `C:\Users\Lab\OneDrive` (raiz) | — | `false` | `false` | seguro |
| `C:\Users\Lab\OneDrive\desktop.ini` | — | `false` | `false` | seguro |
| o próprio repositório e `docs/` | — | `false` | `false` | seguro |
| `E:\` (mídia de instalação UTM) | — | `false` | `false` | não é Dev Drive |

**Derivados de fonte (não medidos — VM sem esses objetos):**

| Objeto | Reparse Tag | ModeIrregular (winsymlink=1) | Base da afirmação |
|---|---|---|---|
| OneDrive cloud-only (placeholder) | `0x9000001a` CLOUD_FILES | **provável true** | `default: m |= ModeIrregular` em Go 1.27.0 `src/os/types_windows.go:226` |
| `IO_REPARSE_TAG_DEDUP` (dedup server) | `0x80000013` | `false` | carve-out explícito no código Go (linha 212) |

---

## 4. Assimetria de Privilégio

O reportante (#444) mediu que `mklink /J` não exige privilégio e `mklink /D` exige, em
Windows 11 sem Developer Mode.

🔴 **Esta VM tem Developer Mode ativo** (`mklink /D` teve sucesso sem admin durante a medição).
A assimetria de privilégio é citada do reporter; não foi re-confirmada nesta VM. Em máquinas com
Developer Mode ativo, symlinks também são criáveis sem admin — o que torna o check `ModeSymlink`
existente ainda mais relevante, não menos.

---

## 5. EvalSymlinks como Candidato (c) — Eliminado

Sob `winsymlink=1` (trackfw `go 1.25.2`), `filepath.EvalSymlinks` usa `os.Lstat` internamente para
detectar symlinks. Como `IO_REPARSE_TAG_MOUNT_POINT` reporta `ModeIrregular` (não `ModeSymlink`),
`EvalSymlinks` **não desfaz juncões**. O teste `TestJunction_IsInvisibleToBothInstruments` confirma:
`EvalSymlinks(junction)` devolve o caminho da junção sem resolução (no NOTE não disparou).

Sob `winsymlink=0`, junção = `ModeSymlink` → `EvalSymlinks` resolve → mas nesse caso o guard já
veria o ModeSymlink e recusaria sem precisar de EvalSymlinks.

**"Resolver a raiz antes do guard" não fecha o defeito em nenhum dos dois contextos. Candidato (c)
eliminado.**

---

## 6. O Predicado Decidido — Opção (a): `ModeSymlink | ModeIrregular`

```go
if err == nil && (info.Mode()&(os.ModeSymlink|os.ModeIrregular)) != 0 {
    return fmt.Errorf("refusing reparse-point path %q (mode %v)", current, info.Mode())
}
```

**Justificativa:**

| Situação | Comportamento |
|---|---|
| `winsymlink=1` (trackfw hoje) | junction → `ModeIrregular` → **recusado** ✓ |
| `winsymlink=0` (módulo `go < 1.23`) | junction → `ModeSymlink` → **recusado** ✓ |
| Symlink real (`mklink /D`) | → `ModeSymlink` → **recusado** ✓ (já era) |
| Um bump de Go sem alterar go.mod | sem comportamento novo (go.mod fixa o winsymlink) |
| Um bump de go.mod para `< 1.23` | junção volta a ModeSymlink, ainda recusado pelo OR |

A opção é correta sob ambas as configurações de `winsymlink`. **Um bump de go.mod ou de toolchain
não reabre o buraco.**

**Por que não a opção (b) (checar `IO_REPARSE_TAG_MOUNT_POINT` via syscall):**
- O guard na nota do vault sobre fallback permissivo diz que "quando um descasamento de caminho
  escolhe qual controle usar, o fallback tem de ser o mais estrito". Opção (b) é mais estreita que
  (a): perderia qualquer tag que não seja MOUNT_POINT mas que permita travessia (risco futuro).
- Acrescenta dependência de `golang.org/x/sys/windows` e código de plataforma, sem vantagem
  mensurável na população atual.
- Seria desnecessária sob `winsymlink=0`, onde MOUNT_POINT já aparece como ModeSymlink.

---

## 7. Análise de Falso-Positivo

🔴 **"Falso-positivo paralisa, não irrita"** (AC13 da `REQ-2026-09-09`).

### 7.1 Objetos com `ModeIrregular=true` medidos

Os objetos legacy compat na tabela (juncões de perfil do Windows: "Meus Documentos",
"Configurações Locais", etc.) têm `ModeIrregular=true`. A pergunta é: algum projeto legítimo do
trackfw teria seu caminho atravessando um desses?

`RejectSymlinks` inicia em `filename` e caminha para cima até `root`. Retorna `nil` em
`current == root`. Nunca inspeciona **acima** da raiz. Portanto:
- `C:\Documents and Settings` → raiz do sistema, nenhum projeto vive dentro dela.
- `C:\ProgramData\Desktop` → idem.
- `C:\Users\Lab\Meus Documentos` → legado, não é onde projetos vivem (`C:\Users\Lab\Documents`
  é o nome correto e não tem reparse).

**Falso-positivo dos objetos medidos: improvável em projeto típico.** O traversal para cima da
raiz do projeto não alcança esses caminhos.

### 7.2 OneDrive cloud-only — NÃO MEDIDO, declarado por construção

O OneDrive da VM está vazio. O objeto `IO_REPARSE_TAG_CLOUD_FILES` **não pôde ser medido**.

Com base na leitura do fonte (`default: m |= ModeIrregular`): se um diretório **ancestral** do
projeto dentro de uma pasta sincronizada pelo OneDrive estiver no estado cloud-only (desidratado),
`os.Lstat` retornaria `ModeIrregular=true` e o guard recusaria.

Cenário concreto: `C:\Users\Lab\OneDrive\projects\myproject` onde `projects\` está desidratado.
A raiz do projeto seria `myproject` (accesível), mas `projects\` estaria no caminho de travessia
**apenas se** a raiz declarada ao guard for mais alta (ex: `C:\Users\Lab\OneDrive`). Se a raiz for
`myproject`, o guard não sobe além dela.

**O falso-positivo do OneDrive é restrito a: raiz do projeto declarada ACIMA do diretório
desidratado.** Não é o caso típico. Declarado, não resolvido.

### 7.3 Dev Drive (ReFS) — NÃO MENSURÁVEL

A VM não tem Dev Drive (E:\ é mídia UTM). Dev Drive é baseado em ReFS, que não suporta juncões
(`IO_REPARSE_TAG_MOUNT_POINT`). Reparse points existem no ReFS (ex: symlinks), mas o ataque
específico de junção não se aplica. Declarado como não mensurável; não é risco de falso-positivo
para a contenção de junção.

### 7.4 Redirecionamento de pasta

Dois mecanismos distintos:

- **Group Policy folder redirection**: usa path de rede (UNC), sem reparse point. O guard não
  inspeciona reparse tag em caminhos UNC (o `os.Lstat` pode falhar antes). Não é fonte de
  `ModeIrregular`. Não afeta o falso-positivo.
- **Shell "move folder" (ex: mover `Documents` para outro volume)**: deixa junção MOUNT_POINT no
  lugar antigo. Se o projeto estiver dentro, o caminho correto (target) não tem reparse; a junção
  não está no caminho de travessia de baixo para cima. Não é falso-positivo.

Os objetos medidos ("Meus Documentos", "Configurações Locais") são desse segundo tipo — MOUNT_POINT
com `ModeIrregular=true`. Confirmados por `fsutil reparsepoint query`.

---

## 8. Declaração de Residual

O design aceita explicitamente:

1. **`IO_REPARSE_TAG_DEDUP`**: tratado como arquivo regular pelo próprio Go (carve-out intencional
   no Go 1.27.0). Não representa risco de travessia e não será capturado. Declarado.

2. **OneDrive cloud-only directories (desidratados)**: se um diretório ancestral entre `filename`
   e `root` estiver desidratado, o guard recusará (falso-positivo, não falso-negativo). Cenário
   não mensurável por falta do objeto na VM; população esperada é pequena (diretórios ativos não
   são desidratados pelo OneDrive). Declarado.

3. **Reparse tags futuras**: tags introduzidas após a presente medição que não sejam SYMLINK,
   AF_UNIX ou DEDUP cairão no `default: ModeIrregular` do Go e serão recusadas pela opção (a).
   Isso é comportamento conservador: falha-fechado em vez de falha-aberto.

---

## 9. Constraint para Wave 1

- **AC3**: o braço da junção em `containment_junction_windows_test.go` passa de `t.Logf` a
  `t.Fatal`. A expectativa é: `RejectSymlinks` retorna **erro** (não nil).
- **Não deletar** a cláusula `junction reported ModeSymlink` em `TestJunction_IsInvisibleToBothInstruments`.
  Ela é tripwire para um flip de `winsymlink`. A expectativa de recusa deve ser escrita em termos de
  *o guard recusa*, não de *ModeIrregular acendeu* — para que seja válida sob ambos os settings.
- **AC5**: o braço C2 (escape visível no caminho) continua recusando.
- **AC6**: o contrato do guard nomeia o que não cobre (DEDUP, OneDrive desidratado).
- **Mensagem de erro**: deve nomear o componente e o mode, para converter "paralisa" em "irrita"
  quando houver falso-positivo. Ex: `"refusing reparse-point path %q (mode %v)"`.

---

## 10. Resumo — O que a Medição da VM é, e o que não é

| Afirmação | Origem | Confiança |
|---|---|---|
| junção MOUNT_POINT → `ModeIrregular` no trackfw | medição (probe_test.go no pacote) | direta |
| guard passa silenciosamente pela junção | `TestRejectSymlinks_WalksThroughAJunction` | direta |
| A/B: go.mod `1.21` vs `1.25` muda o modo | medição (measure3.exe com dois go.mod) | direta |
| winsymlink=1 para go.mod >= 1.23 | leitura de fonte Go 1.27.0 tipos_windows.go | fonte primária |
| OneDrive CLOUD_FILES → `ModeIrregular` | `default: m |= ModeIrregular` no fonte | fonte (não medido) |
| DEDUP tratado como regular | carve-out explícito no fonte | fonte primária |
| Privilege asymmetry (sem Developer Mode) | medição do reporter (#444) | terceiro (não re-medido) |
| Dev Drive (ReFS) — comportamento | não medido (VM sem Dev Drive) | declarado ausente |

---

*Parecer de Segurança emitido por `hades-tf`. Não modifica código de produto.*
