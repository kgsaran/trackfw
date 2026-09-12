---
status: Done
date: 2026-09-11
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md"
---

# REQ: install.sh extrai o tarball sem conferir o checksums.txt que o goreleaser publica

> Date: 2026-09-11 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md

## Motivation

Achado **H-02** da auditoria `hades-tf` do Codex
(`docs/qualidade/2026-09-11-auditoria-hades-codex-dos-fontes.md`).

```
.goreleaser.yaml:28-30    gera checksums.txt (SHA-256)   ✅ produzido
scripts/install.sh:115-123  curl/wget → tar -xzf          🔴 nao baixa, nao confere
```

O checksum **existe no release e não participa da instalação**. HTTPS autentica o transporte enquanto
a conexão e o endpoint permanecerem confiáveis; ele **não** confirma que o conteúdo recebido é o
artefato publicado.

🔴 **É a mesma forma das outras:** o controle foi **construído** e **nada o consome** — a classe que
este projeto fechou três vezes em 2026-09-10 e que motivou o `check-orphan-gates.sh`. Aqui o artefato
órfão não é um gate, é um `checksums.txt`.

## Acceptance Criteria

- [ ] **AC1** — o instalador baixa o `checksums.txt` **da mesma tag**, valida o nome esperado e confere
      o hash **antes** do `tar -xzf`.
- [ ] **AC2** — 🔴 **falha fechado** quando o checksum está **ausente**, **duplicado** ou **divergente**.
      Os três estados têm mensagem própria: "não consegui verificar" não é "verificado".
- [ ] **AC3** — 🔴 **Falsificação:** stub de download que troca **um byte** do tarball ⇒ **nenhum
      binário é instalado**. Prova por ausência do arquivo no destino, não pelo exit code.
- [ ] **AC4** — 🔴 **Contra-braço:** tarball íntegro instala normalmente. Verificador que só recusa é
      indistinguível de verificador quebrado.
- [ ] **AC5** — funciona onde não há `sha256sum` (macOS usa `shasum -a 256`). 🔴 **Ausência da
      ferramenta é falha fechada**, nunca pulo silencioso — senão o AC2 vira decorativo na plataforma
      mais comum entre nós.
- [ ] **AC6** — gate que reprova se o `install.sh` voltar a extrair sem conferir.

## Fora desta REQ

Assinatura (cosign/minisign) e verificação de proveniência. É desenho novo de cadeia de distribuição e
merece ADR própria — esta REQ fecha a lacuna entre **o que já publicamos** e **o que já poderíamos
conferir**.
