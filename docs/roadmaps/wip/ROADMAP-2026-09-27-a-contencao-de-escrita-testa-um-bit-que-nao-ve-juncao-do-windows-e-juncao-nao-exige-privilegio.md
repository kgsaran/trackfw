---
status: wip
date: 2026-09-27
req: "docs/req/REQ-2026-09-27-a-contencao-de-escrita-testa-um-bit-que-nao-ve-juncao-do-windows-e-juncao-nao-exige-privilegio.md"
squad: ""
---

# Roadmap: a contencao de escrita testa um bit que nao ve juncao do Windows, e juncao nao exige privilegio

> Created: 2026-09-27 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-27-a-contencao-de-escrita-testa-um-bit-que-nao-ve-juncao-do-windows-e-juncao-nao-exige-privilegio.md -->
REQ: docs/req/REQ-2026-09-27-a-contencao-de-escrita-testa-um-bit-que-nao-ve-juncao-do-windows-e-juncao-nao-exige-privilegio.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Implementação
> 🔴 Dependências: **Wave 0 auditada**. O predicado sai de lá, não daqui.

Os MLs desta wave só são escritos **depois** que o `ML-0A` decidir o predicado — escrevê-los agora
seria fixar a solução antes da medição que a escolhe, que é o erro que esta REQ existe para não
repetir.

O que já está decidido, e independe do predicado:

- o braço da junção em `containment_junction_windows_test.go` passa de `t.Logf` a **expectativa**
  (AC3);
- o braço **C2** (escape visível no caminho) continua recusando (AC5);
- o que a guarda **não** cobre entra no contrato (AC6).


---

## Wave 0 — Threat model: fechar a população de falso-positivo antes de escolher o predicado
> 🔴 **Bloqueia toda implementação.** A decisão do predicado depende desta medição.

### O que já está medido, e não precisa ser refeito

Dez objetos na VM de Windows, `os.Lstat` em Go (arquiteto, 2026-09-27): **só a junção acende
`ModeIrregular`**. Diretório comum, arquivo comum, raiz do perfil, `Documents`/`Desktop`/`Downloads`,
`AppData`, raiz do OneDrive e o próprio repositório — todos `false`.

**Gates da wave:**

```bash
test -f internal/pathguard/containment_junction_windows_test.go && echo "Gate W0: o corpus de juncao do #455 esta presente" || { echo "GATE FALHOU: o teste de juncao sumiu" >&2; exit 1; }
```

### ML-0A — a lacuna que eu não consegui fechar
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-27 · 🔴 **e refutou a base da minha tabela**

🔴 **O OneDrive da VM está VAZIO** — não havia arquivo *cloud-only* para medir. Placeholders do
OneDrive **são reparse points** e são o candidato mais forte a falso-positivo real. Sem essa
medição, recusar `ModeIrregular` está apoiado em população incompleta.

**Ações:**
1. Medir `ModeIrregular` em: arquivo **cloud-only** do OneDrive, arquivo **baixado** do OneDrive,
   **Dev Drive** (ReFS) se disponível, e pasta de perfil **redirecionada**.
2. 🔴 Se algum não for mensurável na VM, **declarar** — não presumir em nenhuma das duas direções.
   *"Não medi"* é resposta legítima; *"provavelmente não acende"* não é.
3. Decidir o predicado: `ModeIrregular`, inspeção do **reparse tag** específico
   (`IO_REPARSE_TAG_MOUNT_POINT`), ou outra via — com o custo de cada uma.

**Critérios de aceite:**
- [x] Tabela de objetos × `ModeSymlink` × `ModeIrregular` → **23 medidos**, 2 declarados por fonte
- [x] Predicado **decidido e justificado** → **(a) `ModeSymlink | ModeIrregular`**
- [x] O não-mensurável **declarado** → OneDrive *cloud-only* e Dev Drive, com o motivo

#### 🔴 O achado que refuta a base da minha medição

O comportamento da junção **não depende da arquitetura nem do toolchain instalado — depende do
`go` directive do `go.mod`**, via o GODEBUG `winsymlink`:

| `go` no `go.mod` | `winsymlink` | junção (`mklink /J`) |
|---|---|---|
| `< 1.23` | `0` | `ModeSymlink=true`, `ModeIrregular=false` — **o guard já pegaria** |
| `>= 1.23` | `1` | `ModeSymlink=false`, `ModeIrregular=true` — **o buraco** |

**Verifiquei nas fontes primárias**, não no relatório:

```
$GOROOT/src/internal/godebugs/table.go:71   {Name: "winsymlink", Package: "os", Changed: 23, Old: "0"}
$GOROOT/src/os/types_windows.go:208-227     case IO_REPARSE_TAG_SYMLINK → ModeSymlink
                                            case AF_UNIX → ModeSocket · case DEDUP → regular
                                            default: m |= ModeIrregular
go.mod do trackfw                           go 1.25.2   ⇒ winsymlink=1 ⇒ o buraco EXISTE
```

🔴 **A armadilha que isso arma para a Wave 1, e que o executor nomeou:** uma sonda *standalone* com
`go.mod` antigo mede `ModeSymlink=true` para junção e conclui que **o guard já funciona**. Toda
medição tem de rodar **dentro do pacote do produto** (`go test ./internal/pathguard/...`).

#### O que sustenta a decisão do predicado

Ele mediu que **9 pastas legadas do perfil** (`Meus Documentos`, `Configurações Locais`,
`C:\Documents and Settings`, …) são `MOUNT_POINT` com `ModeIrregular=true`. Isso parecia condenar o
predicado (a) — mas **`RejectSymlinks` para em `current == root` e nunca inspeciona acima da raiz**
(verifiquei: `pathguard.go`, `if current == root { return nil }`). Nenhum projeto declara raiz
dentro dessas pastas.

**Candidatos eliminados, com razão medida:**
- **(b) checar `IO_REPARSE_TAG_MOUNT_POINT` por syscall** — é **mais estreito que a população**:
  `C:\Users\All Users` é `SYMLINK`, não `MOUNT_POINT`. Código específico de plataforma sem ganho.
- **(c) `EvalSymlinks` antes do guard** — sob `winsymlink=1` ele **não desfaz junção**, porque só
  segue `ModeSymlink`.

#### ⚠️ Residual declarado — falso-positivo, não falso-negativo

**Diretório *cloud-only* do OneDrive** (desidratado) tem `ModeIrregular=true` **por fonte**, não
medido — o OneDrive da VM está vazio. Se um ancestral entre o alvo e a raiz estiver desidratado, o
guard **recusa**. É falso-positivo em cenário estreito, e fica escrito.


⚠️ **Custo do erro, nas duas direções, para calibrar a decisão:**
**falso-negativo** = escrita fora do projeto sem aviso (o defeito de hoje);
**falso-positivo** = o produto **paralisa** numa máquina Windows legítima — e o AC13 da
`REQ-2026-09-09` nomeia isso: *"falso-positivo aqui paralisa, não irrita"*.

🔴 **A VM investiga; o `windows-latest` mede.** A VM é ARM64 e o runner é x64: número que vira
afirmação em REQ, PR ou changelog sai do CI, não da VM.


---

## Wave 1 — o predicado decidido vira código
> Dependências: **Wave 0 auditada** ✅

**Gates da wave:**

```bash
grep -qF 'ModeIrregular' internal/pathguard/pathguard.go && echo "Gate W1: o guard testa ModeIrregular alem de ModeSymlink" || { echo "GATE FALHOU: pathguard.go ainda decide por um bit so" >&2; exit 1; }
```

⚠️ Este gate **reprova hoje**, de propósito — afirma o estado que a wave tem de alcançar.

### ML-1A — o guard passa a recusar reparse point, não só symlink
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos:** `internal/pathguard/pathguard.go` · `internal/pathguard/containment_junction_windows_test.go`

**Ações:**
1. Predicado: `info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0`.
2. 🔴 **A mensagem nomeia o modo** — `refusing reparse-point path %q (mode %v)`. É o que converte
   *"paralisa"* em *"irrita"*: quem for barrado precisa saber **por quê**, senão o AC13 da
   `REQ-2026-09-09` se realiza.
3. **AC3** — o braço da junção sai de `t.Logf` e vira expectativa: **o guard recusa**.
   ⚠️ A asserção é *"o guard recusa"*, **não** *"`ModeIrregular` acendeu"* — a segunda testaria o Go,
   não o produto.
4. 🔴 **NÃO remover** a cláusula `junction reported ModeSymlink` de
   `TestJunction_IsInvisibleToBothInstruments`: ela é **tripwire** para uma virada futura de
   `winsymlink`. Se um dia a junção voltar a ser `ModeSymlink`, é ela que avisa.

**Critérios de aceite:**
- [ ] Junção no caminho → **recusa**, medido **dentro do pacote** (`go test ./internal/pathguard/...`),
      nunca por sonda standalone — ver a armadilha do `go.mod` na Wave 0
- [ ] 🔴 **Contra-braço (AC4):** diretório comum, arquivo comum e raiz de projeto **continuam
      passando**. Nenhum dos 12 objetos seguros medidos na Wave 0 passa a ser recusado
- [ ] 🔴 **AC5:** o braço **C2** (escape visível no caminho) **continua recusando** — a correção não
      pode afrouxar o que o #441 já fecha
- [ ] A frase da Regra Dura de Reconciliação, por teste alterado
- [ ] `make quality` `exit=0` · `trackfw validate` sem violation nova

### ML-1B — **AC6** — o contrato declara o que a guarda NÃO cobre
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente — depende do ML-1A
**Ações:** registrar em `docs/cli-parity.md` o que o guard cobre e o que **não**:
`IO_REPARSE_TAG_DEDUP` (carve-out intencional do Go, não é risco de travessia) e **diretório
*cloud-only* do OneDrive** (falso-positivo possível, não medido).

🔴 **Limitação declarada é honesta; implícita é o defeito que esta REQ corrige** — foi um gate de
escopo não declarado que deixou o #444 passar.

**Critérios de aceite:**
- [ ] As duas limitações escritas, com o motivo de cada uma
- [ ] O predicado novo nomeado no contrato, como os demais gates
