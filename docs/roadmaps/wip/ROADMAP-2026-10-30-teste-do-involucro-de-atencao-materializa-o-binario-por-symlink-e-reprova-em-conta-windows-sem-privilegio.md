---
status: wip
date: 2026-10-30
req: "docs/req/REQ-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md"
squad: "hades-tf, artemis-tf"
---

# Roadmap: teste do invólucro de atencao materializa o binario por symlink e reprova em conta Windows sem privilegio

> Created: 2026-10-30 | Status: wip

## Context

REQ: docs/req/REQ-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink-e-reprova-em-conta-windows-sem-privilegio.md
Fecha o issue #634.

`TestAttentionScripts_FallbackWithoutJQ` põe o binário compilado no PATH do filho por `os.Symlink` e
segue em frente quando falta privilégio; o invólucro sai 0 sem escrever. Correção só no teste.
Único sítio da causa: `git_branch_guard_test.go:932` já cai para cópia.

Corre em paralelo com o roadmap do #633 (`internal/commands/`), que não compartilha arquivo.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do PATH do filho no teste de atenção (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. P-1: o PATH do filho passa a ser só o diretório de `compiledGuardBinary` (contém só o `trackfw`, com `.exe` no Windows); P-2: sonda `command -v trackfw` no setup, com contra-braço. Reprodução do arquiteto na VM (conta `tfnopriv`, sem `SeCreateSymbolicLinkPrivilege`, Modo de Desenvolvedor desligado): `origin/main` `45ed483` reprova em `scaffold_test.go:432`, como no issue.
**Files affected:** `docs/seguranca/2026-10-30-wave0-teste-atencao-sem-symlink.md`
**Actions:**
1. O diretório de `compiledGuardBinary` (`internal/generators/guard_parity_helper_test.go`) contém só
   o `trackfw`? Se contiver `jq` ou outro binário, pô-lo no PATH muda o que o teste afirma.
2. Comparar as duas correções (diretório do binário no PATH × cópia com `filepath.Base`): qual
   preserva o modo sem jq e o `.exe` no Windows.
3. Confirmar a enumeração da REQ (só dois testes materializam binário por symlink).
4. Alvos de falsificação para AC1–AC3 da REQ, e o que medir na VM para o AC4.
5. Prescrições numeradas para o ML-1A, e residual.
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-wave0-teste-atencao-sem-symlink.md
grep -q 'Prescrições' docs/seguranca/2026-10-30-wave0-teste-atencao-sem-symlink.md
```

---

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Teste de atenção sem symlink (artemis-tf) — AC1–AC3
**Status:** ✅ Concluído — auditado em 2026-10-30. O PATH do filho é só o diretório do binário compilado; a sonda `command -v trackfw` reprova no setup com o diretório vazio (contra-braço).
**Files affected:** `internal/generators/scaffold_test.go`
**Actions:**
1. Trocar o laço de `os.Symlink` pela correção prescrita na Wave 0 (P-1 a P-4 de `docs/seguranca/2026-10-30-wave0-teste-atencao-sem-symlink.md`).
2. Falta do binário no PATH do filho reprova no setup, com mensagem que nomeia a causa.
3. Contra-braço: com o binário fora do PATH do filho, o teste reprova no setup.
**Acceptance criteria:**
- [x] AC1, AC2 e AC3 da REQ
- [x] contra-braço medido

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/generators/ -count=1 -run TestAttentionScripts
```

### ML-1B — Medição na conta Windows sem privilégio (zeus-tf) — AC4
**Status:** ✅ Concluído — medido em 2026-10-30 como `tfnopriv`: `main` FAIL em `scaffold_test.go:432`, com o teste do ML-1A PASS.
**Files affected:** `docs/portabilidade/2026-10-30-teste-atencao-sem-symlink-medido-na-vm.md`
**Actions:**
1. Na VM `Windows-Lab`, conta padrão `tfnopriv` (sem `SeCreateSymbolicLinkPrivilege`; a conta `Lab` tem o privilégio), via tarefa agendada.
2. Rodar o teste em `origin/main` e na branch; registrar os dois resultados.
**Acceptance criteria:**
- [x] AC4 da REQ

---

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team do teste de atenção (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-30-redteam-teste-atencao-sem-symlink.md`
**Actions:**
1. Tentar fazer o teste passar com o invólucro quebrado ou com `jq` alcançável.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-teste-atencao-sem-symlink.md
```
