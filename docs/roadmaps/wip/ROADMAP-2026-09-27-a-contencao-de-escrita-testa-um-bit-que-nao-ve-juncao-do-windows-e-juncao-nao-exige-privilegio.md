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

⚠️ **Esta wave nasceu sem MLs, de propósito, e só os ganhou depois da Wave 0.** Escrevê-los antes
fixaria a solução antes da medição que a escolhe — e a medição de fato mudou a base (o achado do
`winsymlink`). O placeholder vazio que o `roadmap new` gerou foi consolidado aqui: 🔴 **o gerador
produz uma segunda seção com o mesmo rótulo** quando se acrescenta uma wave própria, e o `barrier`
passa a ler a primeira — que está vazia. Aconteceu duas vezes neste roadmap (Wave 0 e Wave 1).

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
- [x] Junção no caminho → **recusa**, medido **dentro do pacote** — o braço saiu de `t.Logf` e virou
      `t.Fatalf`; ⚠️ o veredito no Windows é do CI, os testes têm `//go:build windows`
- [x] 🔴 **Contra-braço (AC4):** `TestRejectSymlinks_CleanPath` e `_ExistingFile` verdes; e
      `TestJunction_IsInvisibleToBothInstruments` afirma `ModeIrregular == 0` para diretório comum —
      é a falsificação direta se o predicado novo quebrasse objeto seguro
- [x] 🔴 **AC5:** o braço **C2** é o **primeiro passo** do teste da junção — se ele falhar, o teste
      aborta com *"it is not armed"*. O escape visível continua recusado
- [x] A frase da Regra Dura de Reconciliação, por teste alterado
- [x] `make quality` `exit=0` (**1367** `^OK `, **0** `: FALHA`) · `validate` 172 warnings, 0 violations

#### Auditoria do ML-1A — e o que eu fiz além dele

🔴 **Emendei a `ADR-2026-09-18`, e isso não estava no ML.** Aquela ADR **adota** o predicado antigo
(*"o predicado adotado é o que o projeto já tem e já provou"*) e exibe o código com `ModeSymlink`.
Mudar o predicado sem emendar deixaria o código divergindo de decisão registrada, e o próximo leitor
copiaria o bloco da ADR como referência atual.

Descobri isso indo verificar outra coisa: se a mensagem `"refusing symlink path"` estava **pinada**
em algum gate. **Não estava** — só em documentos históricos, que ficam como estão. Mas no caminho
apareceu a ADR que decidia o predicado.

**Verificações minhas, além do relatório:**
- a cláusula-tripwire `junction reported ModeSymlink` foi **preservada**;
- a asserção é *"o guard recusa"*, **não** *"`ModeIrregular` acendeu"* — a segunda testaria o Go;
- 6 arquivos de teste atualizaram a asserção de mensagem; todos testavam **comportamento de recusa**,
  não a string como invariante.

### ML-1B — **AC6** — o contrato declara o que a guarda NÃO cobre
**Owner:** `artemis-tf`
**Status:** ✅ Concluído — auditado em 2026-09-28
**Ações:** registrar em `docs/cli-parity.md` o que o guard cobre e o que **não**:
`IO_REPARSE_TAG_DEDUP` (carve-out intencional do Go, não é risco de travessia) e **diretório
*cloud-only* do OneDrive** (falso-positivo possível, não medido).

🔴 **Limitação declarada é honesta; implícita é o defeito que esta REQ corrige** — foi um gate de
escopo não declarado que deixou o #444 passar.

**Critérios de aceite:**
- [x] As duas limitações escritas, **com a distinção preservada** — subseções separadas, `gap reason=`
      distintos: DEDUP é *"não vemos, e está tudo bem"*; OneDrive é *"podemos barrar indevidamente, e
      não medimos"*
- [x] O predicado novo nomeado no contrato, no formato que o arquivo já usa
- [x] `check-parity-contract-coverage.sh` → **OK**, anotação válida nos **6** níveis de cabeçalho

#### A lacuna que o contrato declara, e que não dá para fechar por gate

> *"não há gate que leia o `go.mod` e valide que a linha `go` permanece ≥ 1.23"*

É honesto: não temos como garantir por gate que alguém não regrida a diretiva e **reabra o buraco**.
O que temos é o contrato avisando **e** a cláusula-tripwire no teste. 🔴 Registrar a impossibilidade
é diferente de não ter pensado nela.

⚠️ **Custo de orquestração meu:** o primeiro executor deste ML **travou** lendo o `cli-parity.md` —
**7718 linhas** — porque o handoff dizia *"encontre a seção e siga o formato"*. Refiz levantando eu o
formato obrigatório (`<!-- trackfw-contract: … -->`, com gate próprio), o molde (`sed -n '5387,5410p'`)
e o local. Virou *"leia 24 linhas e siga"*. **Delegar a tarefa sem delegar a busca** é o que custou
uma rodada.
