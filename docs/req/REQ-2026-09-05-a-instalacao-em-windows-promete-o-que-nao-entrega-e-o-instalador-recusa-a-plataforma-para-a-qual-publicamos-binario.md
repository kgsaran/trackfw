---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
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
Roadmap:
