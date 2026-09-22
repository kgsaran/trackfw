---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-09-22-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md"
---

# REQ: a instalacao em Windows promete o que nao entrega e o instalador recusa a plataforma para a qual publicamos binario

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado da auditoria externa de 2026-09-05 (item 14 do inventário em
`docs/portabilidade/2026-09-05-auditoria-externa-astra-achados-e-verificacao.md`). **Verifiquei os
cinco pontos abaixo diretamente no repositório.**

### 1. O README promete o que o produto não entrega

```
README.md:96 — "It works wherever Node.js ≥ 18 is installed"
```

Sem qualificação. Mas os hooks de guard são scripts `.sh` executados por **bash**. Num Windows sem
Git Bash, o `trackfw` instala, o `init` roda, os hooks são escritos — **e não executam**.

🔴 **É o pior modo de falha possível para um produto de governança:** o usuário recebe um guard que
parece instalado e não protege nada. O `credential_guard` e o `git_branch_guard` existem para impedir
`git push` bruto por subagente; um hook que nunca dispara reporta saúde onde não olhou. É a mesma
família de defeito que a campanha de Windows inteira vem caçando — agora na fronteira de instalação.

### 2. O README não tem seção de Windows. Nenhuma.

`grep` por `Windows`/`PowerShell`: **zero** ocorrências de instalação. Só macOS/Linux via `curl` e
Homebrew.

### 3. O instalador recusa a plataforma para a qual publicamos binário

```sh
# scripts/install.sh:12-17
*) echo "Sistema operacional nao suportado: $RAW_OS" >&2
   echo "Plataformas suportadas: macOS (Darwin), Linux" >&2
   exit 1
```

E o release publica **`trackfw_7.3.0_windows_amd64.tar.gz`**. Publicamos o binário e o instalador o
recusa.

### 4. Windows ARM64 está fora por configuração, e isso não está escrito

`.goreleaser.yaml:18-19` ignora `goos: windows / goarch: arm64`. **É decisão legítima** — mas não
declarada em nenhum lugar que o usuário leia antes de tentar.

### 5. Não existe jornada verificada

Temos CI que roda suítes em Windows. **Não temos nada que exercite o caminho de um usuário real:**
instalar → `PATH` → `init` → hooks escritos → hook **dispara** → `barrier` roda.

Os três CLIs passando testes não prova que a instalação funciona: são superfícies diferentes, e a
campanha inteira mostrou que teste verde não implica comportamento entregue.

## Acceptance Criteria

- [ ] **AC1** — O README declara a dependência de **shell** para as funcionalidades que dependem
      dela, no mesmo lugar onde promete "wherever Node.js ≥ 18". 🔴 A promessa e a ressalva no mesmo
      parágrafo — ressalva em outra seção não é lida.
- [ ] **AC2** — Existe seção de instalação para Windows, com o caminho real (qual artefato, onde
      colocar, como pôr no `PATH`).
- [ ] **AC3** — A contradição entre `install.sh` e o release é resolvida **numa direção declarada**:
      ou o instalador passa a suportar Windows, ou ele **diz o que usar** em vez de "não suportado"
      seco. 🔴 Publicar binário que o instalador recusa é o defeito; qualquer das duas saídas o
      fecha, mas a escolha tem de ser escrita.
- [ ] **AC4** — O limite de ARM64 fica **declarado** onde o usuário lê, não só no `.goreleaser.yaml`.
- [ ] **AC5** — 🔴 **Jornada verificada de ponta a ponta em Windows**, com o hook **disparando** —
      não apenas escrito. Escrever o arquivo e verificar que ele existe é o mesmo erro dos testes que
      passavam sem exercer nada.
- [ ] **AC6** — 🔴 **Falsificação da AC5:** num ambiente **sem** Git Bash, a jornada tem de **falhar
      de forma visível e nomeada** — não silenciosamente. Uma jornada que "passa" nos dois ambientes
      não mediu o que importa.
- [ ] **AC7** — Caminhos com **espaço** e **acento** exercitados (`C:\Program Files\...`,
      `C:\Users\José\...`). O espaço já apareceu nesta campanha: o bash provado do harness é
      `C:\Program Files\Git\bin\bash.exe`.

## Negative Scope

- ❌ **Não** decidir a release aqui. Publicar é decisão do usuário — mas registre-se que **a última
  release é a v7.3.0, de 28/08**, anterior a toda a campanha: enquanto não houver release, nenhuma
  destas correções chega a quem instala.
- ❌ **Não** transformar isto em suporte a WSL como caminho primário. A ADR de shell já decidiu Git
  Bash/WSL para execução de hook; esta REQ é sobre **declarar e verificar**, não sobre re-decidir.
- ❌ **Não** implementar ARM64. Declarar o limite é o entregável; construir é outra decisão.
- ❌ **Não** misturar com o CRLF em renderizadores (item 13 do inventário) — superfície própria.

## Linked ADR
<!-- Decisão de shell já existe: ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-...
     Esta REQ NÃO a re-decide; ela declara e verifica o que aquela decisão implica na instalação. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-09-22-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md`

---

## Medição em Windows ARM64 real — 2026-09-16 (v8.0.0-rc2)

**Instrumento:** VM `Windows-Lab` (UTM/QEMU `aarch64`, `-accel hvf`), **Windows 11 ARM64 build
26200**, acesso por SSH. Artefatos **publicados** da `v8.0.0-rc2` — não build local.

🔴 **Por que a VM vale como medição aqui, contra a regra usual "VM investiga, CI mede":** o runner do
CI é **x64**. Os artefatos `@trackfw-bin/win32-arm64` (npm) e a wheel `win_arm64` (PyPI) **nunca
tinham sido executados em lugar nenhum** — nem no CI, nem antes. Para eles a VM não é um proxy pior
que o runner: é a única máquina que existe. A ressalva de transferência (`n=1`, ARM64) vale para
qualquer generalização a x64, **não** para a afirmação sobre arm64, que é direta.

### Resultado por canal

| Canal | Comando | Resultado |
|---|---|---|
| npm | `npm install -g trackfw@rc` | ✅ shim + `@trackfw-bin/win32-arm64` com `bin/trackfw.exe`; `--version` → `8.0.0-rc2`; exit 0 |
| PyPI | `pip install trackfw==8.0.0rc2` | ✅ wheel `win_arm64`; `pip show` → `8.0.0rc2`; `--version` → `8.0.0-rc2`; exit 0 |
| `install.sh` | `TRACKFW_VERSION=v8.0.0-rc2 sh install.sh` | 🔴 **recusa**, exit **1** |

Saída exata do terceiro:

```
Sistema operacional nao suportado: MINGW64_NT-10.0-26200-ARM64
Plataformas suportadas: macOS (Darwin), Linux
```

### O que isto faz com os ACs

- **AC3 — confirmado em hardware real, e continua aberto.** A contradição medida em 05/09 por
  leitura do repositório agora está medida por **execução**: publicamos `windows_arm64`, o pacote
  instala e roda pelos outros dois canais, e o instalador recusa a mesma plataforma. O `exit 1` está
  correto (recusa não é silenciosa); o defeito é **recusar plataforma para a qual publicamos
  binário**, exatamente como a AC descreve.
- **AC4 — o limite mudou de natureza.** A AC pedia *declarar* o limite de ARM64. Medido hoje: **não
  há limite de ARM64 nos canais npm e PyPI** — os dois entregam e executam em Windows ARM64. O que
  resta declarar é o limite do **`install.sh`**, que é outro eixo (sistema operacional, não
  arquitetura).
- 🔴 **O escopo negativo *"❌ Não implementar ARM64"* está obsoleto e deve ser removido.** Ele foi
  escrito em 05/09, quando o `.goreleaser.yaml` tinha bloco de `ignore` para `windows/arm64`. O ML-1A
  da v8 removeu esse bloco; a `v8.0.0-rc2` publica arm64 nos três canais e **dois deles foram
  executados agora**. Manter a linha faria um leitor futuro recusar trabalho já entregue.
- **AC1 e AC2 — parcialmente atendidos pelo ML-3D da v8**, que reescreveu o README: há aviso de
  plataforma no topo (linha 21), seção `Windows support (partial)` (linha 118) e a contradição do
  instalador está **declarada** na linha 137 (*"`scripts/install.sh` refuses Windows, even though we
  publish a Windows binary"*). Declarar não fecha a AC3 — o defeito continua —, mas o usuário deixou
  de ser surpreendido.
- **AC5, AC6, AC7 — não medidos aqui.** Esta sessão exercitou **instalação e execução de
  `--version`**, não a jornada com hook disparando, nem a falsificação sem Git Bash, nem caminhos com
  espaço e acento. Registrado para não ser lido como cobertura maior do que foi.

### Armadilhas do instrumento, registradas para a próxima medição

- Um `trackfw.exe` **7.5.1 do pip**, de sessão anterior, sombreava o `PATH`
  (`...\Python312-arm64\Scripts\` antes de `...\Roaming\npm\`). O primeiro `trackfw --version`
  devolveu `7.5.1` e quase virou "a rc2 não instalou". Sempre resolver por `where trackfw` antes de
  concluir.
- `npm.ps1` é bloqueado pela *execution policy* do PowerShell; usar `cmd /c "npm ..."`.
- Medir exit code de script **sem** canalizar para `tail`: `sh x.sh | tail` devolve o `$?` do `tail`.
  A primeira medição deu `RC=0` por esse motivo e foi refeita — o valor real é `1`.
