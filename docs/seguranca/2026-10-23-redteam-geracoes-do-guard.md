# ML-2B — Red-team da remoção de gerações do guard

**Data:** 2026-10-23  
**Agente:** hades-tf  
**ML:** ML-2B (barreira independente)  
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard...  
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard  

---

## 1. Modelo independente

Derivado da leitura do ADR (adendos D12/D13/D14 revisada, 2026-10-23) e da REQ, sem consultar o diff.

**D12:** O registro em `internal/guardforms/guard_generations.go` é a única lista autorizada de formas legadas. Migração e validação consomem este registro. Comparação sempre por igualdade exata (nunca `strings.Contains`).

**D13:** Após migração: exatamente 1 entrada por par (evento, matcher). Ordem: migrar → deduplicar → mesclar. Entradas fora do registro (hooks do usuário) nunca removidas.

**D14 revisada:** Decisão de remoção/skip é por par. Par coberto pelo global harness → remover entrada de projeto. Par não coberto → sempre emitido (com ou sem global). Fail-open em qualquer erro de leitura.

**Consequências do modelo:**
- Cursor `preToolUse×Read` e `preToolUse×Write` não são cobertos pelo global de Cursor (que só cobre `beforeShellExecution`) → devem estar sempre presentes.
- Claude `Read`, `Write|Edit` não são cobertos pelo global de Claude → devem estar sempre presentes.
- Copilot global harness cobre apenas `bash` → `view` e `create|edit` devem estar sempre presentes.
- AmazonQ não tem harness global → nenhuma remoção D14.

---

## 2. Vetores atacados e resultados

Arquivo de teste: `internal/generators/hades_ml2b_redteam_test.go`  
Comando: `go test ./internal/generators/ -run TestRedteam -count=1 -v`  
Status final do arquivo: **deletado** (teste descartável por design)  
`git status --short internal/`: **vazio**

| ID | Vetor | Veredicto | Resultado |
|----|-------|-----------|-----------|
| A1 | User hook com substring de forma legada em Cursor preToolUse×Read | PASS | Survives byte-for-byte; legado removido; canônico presente |
| A2 | LegacyD2CredentialCmdExe ("trackfw guard credential") em Cursor preToolUse×Read | PASS | Sobrevive — é família cmd.exe, não está na lista PSPOSIX do Cursor |
| A3 | LegacyD2CredentialCmdExe em AmazonQ execute_bash (R3 documentado) | PASS | Migrado para canônico cmd.exe; residual R3 confirmado como documentado |
| A4 | D14: Cursor preToolUse×Read presente mesmo com global instalado | PASS | Canônico presente; legado removido; D14 revisada respeitada |
| A5 | T5: 5 formas legadas em Cursor preToolUse×Read → exatamente 1 canônico | PASS | Deduplicação correta: 1 entrada canônica |
| A6 | T2: global corrompido → fail-open → guard emitido | PASS | beforeShellExecution presente; fail-open funciona |
| A7 | Idempotência: 2 execuções de InjectCursorHooks produzem arquivo idêntico | PASS | Byte-for-byte igual |
| A8 | User hook em envelope Claude aninhado sobrevive ao lado do guard | PASS | User hook preservado; legado removido; canônico presente |
| A9 | Campos extras (timeout) preservados na migração flat Cursor | PASS | Campo timeout=30 preservado (migração in-place) |
| A10 | **B: Copilot D14 — pares view e create\|edit ausentes com global instalado** | **FAIL** | **Bug confirmado — agentfiles.go:1149** |
| A11 | R: Envelope Claude sem chave "hooks" dropped silenciosamente | PASS (R) | Envelope removido; edge case sem impacto de segurança |
| A12 | R: Global Claude com forma legada dispara remoção da entrada de projeto | PASS (R) | Comportamento pré-existente; aceito pelo ADR ("pior caso = comportamento de hoje") |
| A13 | Entrada com espaço ao final não está no registro → sobrevive | PASS | Igualdade exata; espaço fora do registro preserva a entrada |
| A14 | Forma PSPOSIX em AmazonQ execute_bash migrada para canônico cmd.exe (não PSPOSIX) | PASS | Família correta; cmd.exe canônico presente; PSPOSIX canônico ausente |
| A15 | User hook em envelope AmazonQ execute_bash sobrevive | PASS | User hook preservado; legado PSPOSIX removido |

**Resumo:** 14 PASS · 1 FAIL  

---

## 3. Achados

### B1 — BLOQUEANTE — Copilot D14: pares `view` e `create|edit` ausentes quando global está instalado

**Arquivo:** `internal/generators/agentfiles.go:1149`  
**Confirmado por:** teste A10, reproduzível.

```
if !globalCredentialGuardInstalledCopilot() {
    // ... adiciona bash, view, create|edit
}
```

`globalCredentialGuardInstalledCopilot()` verifica apenas o par `bash` no `~/.copilot/settings.json`. O global harness (`mergeCredentialGuardCopilotHooks`) escreve apenas entradas com `matcher: bash`. Com global instalado (bash presente), `globalCredentialGuardInstalledCopilot()` retorna `true` e o `if` inteiro é saltado.

Consequência: quando `trackfw update harness --targets copilot-credential-guard` foi executado (global installed), os pares `view` e `create|edit` não recebem nenhum guard de credencial no arquivo de projeto. Operações de leitura e escrita de arquivo via Copilot executam sem o guard.

**Violação:** D14 revisada ("par não coberto → continua emitido, com ou sem global"). O global não cobre `view` nem `create|edit` — esses pares devem ser sempre emitidos.

**Não introduzido por este diff:** pré-existente. Mas o AC6 da REQ exige que D14 revisada esteja implementada para todos os pares, e este defeito persiste no código atual do branch.

**Evidência no teste A10:**
```
A10 B: Copilot D14 bug confirmed — agentfiles.go:1149 wraps ALL pairs in if
!globalCredentialGuardInstalledCopilot(); with global installed (bash only),
view=false create|edit=false are absent.
```

---

### R1 — Residual documentado — R3: LegacyD2CredentialCmdExe em AmazonQ

`"trackfw guard credential"` (LegacyD2CredentialCmdExe) está em `CredentialLegacyCmdExe` e é indistinguível de uma entrada digitada pelo usuário. AmazonQ migra para `"trackfw guard credential || exit 2"`. Este residual está documentado no Wave 0 (§4, R3). Confirmado por A3.

**Não há path de ataque novo aqui.** A forma cmd.exe canônica é mais segura que a D2 (exit code explícito). O residual é que a distinção "legado de guard" vs "comando do usuário" é impossível neste caso.

---

### R2 — Residual cosmético — Envelope sem chave `hooks` dropped em nested format

Envelope `{"matcher": "Bash|PowerShell"}` sem a chave `hooks` é removido silenciosamente por `migrateDedupGuardInNestedArray` (passes 1+2 produzem array vazio → envelope descartado). Confirmado por A11.

Impacto de segurança: nulo. Tais envelopes não são criados por `trackfw` nem por editores em uso normal. O envelope era semanticamente vazio (sem comandos). Não há guard ausente.

---

### R3 — Residual de campos extras em migração flat

Quando a entrada legada é o primeiro exemplar e é migrada in-place (Pass 1), campos extras como `"timeout"` são preservados. Se a entrada canônica já existia antes do legado (Pass 1 não toca a ordem) e o legado é apenas removido, os campos do legado são perdidos. Confirmado por A9 (timeout preservado neste caso de pass-1-in-place).

Impacto de segurança: nulo. O guard executa independente de `timeout`. Cosmético.

---

### R4 — Residual pré-existente — Global Claude com forma legada remove entrada de projeto

`globalCredentialGuardInstalledClaude()` aceita `LegacyD11RevCredentialGlobalPSPOSIX`. Se o global contém essa forma (quebrada sob zsh, REQ-2026-10-20), o par `claudeShellMatcher` do projeto é removido. Confirmado por A12.

O ADR explicita: "pior caso = comportamento de hoje (fail-open)". Não introduzido por este diff. Aceito como residual pré-existente.

---

## 4. Controles que funcionaram corretamente

- **Igualdade exata vs. substring (D12):** A1, A2, A13 confirmam que entradas com substring de formas legadas ou com delta de espaço sobrevivem.
- **Invariante D13 — hooks do usuário nunca removidos:** A8, A15 confirmam preservação byte-a-byte.
- **Deduplicação (D13 pós-migração):** A5 confirma 5 legadas → 1 canônica.
- **Fail-open D14:** A6 confirma que global corrompido → guard emitido.
- **Idempotência:** A7 confirma.
- **D14 revisada para pares não cobertos — Cursor:** A4 confirma `preToolUse×Read` sempre presente.
- **Família correta em AmazonQ:** A14 confirma PSPOSIX→cmd.exe (não PSPOSIX→PSPOSIX).

---

## 5. Parecer

**APROVA COM BLOQUEANTE.**

O código entregue resolve o defeito central da REQ (acúmulo de formas legadas, falta de deduplicação). Os vetores D12/D13 passam integralmente. O modelo de igualdade exata, a preservação de hooks do usuário, a idempotência e o fail-open estão corretos.

**O defeito B1 (Copilot D14, agentfiles.go:1149) bloqueia o aceite do AC6.**  
AC6 exige que D14 revisada esteja implementada para todos os pares. Com global de Copilot instalado, `view` e `create|edit` ficam sem guard. O fix é mover os dois pares não cobertos pelo global para fora do `if !globalCredentialGuardInstalledCopilot()`.

**Entregar ao apolo-tf para correção antes de marcar ML-1E/AC6 como concluído.**

Residuais aceitos: R1 (documentado no Wave 0), R2 (edge case sem impacto), R3 (cosmético), R4 (pré-existente, ADR-aceito).
