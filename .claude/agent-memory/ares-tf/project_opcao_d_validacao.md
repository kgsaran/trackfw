---
name: opcao-d-validacao
description: Resultados da validação técnica da opção D (um binário Go, casquinhas npm/pip) antes da decisão v8
metadata:
  type: project
---

Opção D validada (Trilha 1 completa) em 2026-09-12. worktree `trackfw-nul`, branch `fix/validar-um-binario-muitos-canais`.

**Por:** `prototype/` descartável, verdaccio local, wheel manual.

**Provado empiricamente:**
- AC1–AC3: npm com optionalDependencies funciona em registry privado, --ignore-scripts, sem github.com (proxy log tem advertência de ambiguidade — proxy pode ter morrido antes; exit 0 do npm install é evidência primária)
- AC4 (variante corporativa): HOME ro + npm cache gravável passa; npm cache ro bloqueia qualquer install (restrição do npm, não da opção D)
- AC5: wheel PyPI no formato gh-bin (data/scripts/, Root-Is-Purelib: false, zero .py), pip install --no-index OK
- AC7: shim nomeia a plataforma no erro — nunca MODULE_NOT_FOUND
- AC8/AC9: byte-idêntico em darwin/arm64 (6 cenários)

**Não provado empiricamente (ACs em andamento):**
- AC6: lockfile macOS → Windows real; provado localmente via --os/--cpu flag (aproximação); Windows CI seria o fechamento correto via windows-probe.yml
- AC8/AC9: Linux e Windows (OrbStack não ativo, sem QEMU; windows-probe.yml é o caminho)

<<<<<<< HEAD
**Prova VM (win32/arm64, 2026-09-12):**
- AC2: npm ci --ignore-scripts win32/arm64 ✅ (shim resolve dinamicamente, sem platformMap hardcoded)
- AC6: lockfile darwin/arm64 → npm ci win32/arm64 instala win32-arm64 corretamente ✅
- AC8/AC9: byte-idêntico win32/arm64 5/5 + CRLF (LF, 14 bytes) ✅

**Defeito corrigido:** shim tinha platformMap hardcoded sem win32-arm64; corrigido para resolução dinâmica em 0.0.3.

**Observação:** VM é ARM64 (UTM); CI runner win32/x64 não testado empiricamente. Para x64: windows-probe.yml (instrumento disponível).

**AC3 resolvido por Zeus:** cache frio + exit 0 do npm install provam que npm não tentou github.com (a advertência do proxy era inofensiva).
=======
**Caminho para fechar:** npm pack os 5 tarballs + adicionar pergunta ao windows-probe + disparar manualmente. Requer commit/push por Zeus.
>>>>>>> origin/main

**Custo oculto confirmado:** issue #338 piora — 5 sítios de versão viram 11+ (5 platform pkgs + shim + wheels).

**go-to-wheel v0.2:** incompatível com layout `cmd/<nome>/`. Wheel construído manualmente (30 linhas de script).

**Why:** decisão da v8 sobre substituir 53.744 linhas (Node+Python) por casquinhas finas.
**How to apply:** se a v8 for aprovada, o script de build de wheels manuais está em prototype/ e o shim em prototype/packages/trackfw-shim/.
