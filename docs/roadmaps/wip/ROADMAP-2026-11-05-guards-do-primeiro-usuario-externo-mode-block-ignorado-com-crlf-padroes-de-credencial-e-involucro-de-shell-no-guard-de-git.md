---
status: wip
date: 2026-11-05
req: "docs/req/REQ-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Guards do primeiro usuário externo: mode block ignorado com CRLF, padrões de credencial e invólucro de shell no guard de git

> Created: 2026-11-05 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md
ADR: docs/adr/ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-mode-lido-com-crlf-padroes-de-credencial-ampliados-e-involucro-de-shell-analisado.md
Issues: #662, #663

A varredura de issues abertos em 2026-11-05 achou #662, #663 e #665. O #665 (prazo do PowerShell 5.1 no
teste) e a Discussion #664 são de outra causa, a do instrumento de CI, e vão para a REQ de CI.

🔴 **Medição de guard:** alimentar os payloads por stdin a partir de um script (Write + `bash arquivo`). O
guard de git desta sessão bloqueia um comando inline que contenha o texto de um git que escreve. E o guard de
credencial bloqueia `grep` com glob que alcance arquivos de teste com JWT de exemplo.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md`
**Actions:**
1. Completude da enumeração, por causa:
   - todos os leitores de `credential_guard.mode`: guard de projeto, `--global`, validator, script gerado
     (`credentialGuardModeResolution`) e qualquer outro. Diga qual é o ativo na linha de hook atual e se o
     `.sh` gerado ainda é executado por algum hook ou fluxo;
   - todos os sítios com os padrões de credencial: `credJWTRe` e `credAWSRe`, as comparações
     `match == "JWT"`/`"AWS access key"` da segunda camada, `credentialGuardDetectionCore` e os testes de
     paridade do script;
   - todos os pontos em que o guard de git decide pelo primeiro token (`MatchSubcommand`,
     `stripEnvCommandPrefix`, `quoteAwareSplit`, `shellTokenize`) e se existe um segundo guard de git (script
     ou validator) que precise convergir;
   - os testes e documentos que afirmam o comportamento atual: README, `docs/cli-parity.md`, os comentários
     de "resíduo CRLF" e os pins.
2. Modelo de ameaça:
   - contornos do D4: aspas aninhadas, `bash -c` com `$'…'`, `-c` depois de outras flags, `--` e `bash -s`
     com stdin, `exec`, `xargs`, `nohup`, `time`, `timeout`, `env bash -c`, `sudo bash -c`, `cmd /c` no
     Windows e `powershell -EncodedCommand`. Para cada um, diga se entra, se fica como residual, e por quê;
   - o limite de profundidade: qual valor, e o que acontece acima dele;
   - falso positivo do D4: `bash -c "git status"`, mensagem de commit com o texto `bash -c`, `git log` com
     `--grep`;
   - os padrões do D2: proponha a expressão exata de cada um, com o comprimento mínimo, e meça o falso
     positivo contra este repositório. Para cada expressão, informe quantos casamentos ela tem em `docs/`,
     `internal/` e `vault/`;
   - o CRLF: há outras leituras linha a linha do `trackfw.yaml` no guard (`credReadRoadmapDir` e outras)
     com o mesmo defeito?
3. Alvos de falsificação nos dois sentidos, do AC1 ao AC5.
4. Residual declarado.
5. `## Prescrições`, com a lista de arquivos de cada ML. O ML-1A (credencial) e o ML-1B (git) não podem
   ter arquivo em comum, ou a Wave 0 diz que ficam em sequência.
**Acceptance criteria:**
- [ ] As cinco seções respondidas com evidência (comando e resultado)
- [ ] Prescrições em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md
grep -q '^## Prescrições' docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada. Os MLs e a lista exata de arquivos saem das prescrições da Wave 0, e o
> arquiteto atualiza esta seção antes do despacho.

### ML-1A — Guard de credencial: mode com CRLF e padrões ampliados (apolo-tf) — AC1, AC2, AC3
**Status:** ⬜ Pendente

### ML-1B — Guard de git: invólucro de shell analisado (apolo-tf) — AC4, AC5
**Status:** ⬜ Pendente

## Wave 2 — Red team e release
> Dependencies: Wave 1 auditada.

### ML-2A — Red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md`
