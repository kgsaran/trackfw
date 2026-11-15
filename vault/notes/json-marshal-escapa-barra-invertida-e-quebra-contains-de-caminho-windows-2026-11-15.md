---
title: "json.Marshal escapa a barra invertida e quebra strings.Contains de caminho no Windows"
date: 2026-11-15
tags: [windows, testes, json, caminhos]
---

# `strings.Contains(jsonCru, caminhoWindows)` é sempre falso

`json.Marshal` serializa `\` como `\\`, ou seja, dois caracteres nos bytes crus. Um caminho nativo do Windows,
como `C:\a\b.md`, tem só uma barra. Por isso a busca do caminho dentro do JSON cru nunca casa.

Medido em 2026-11-15 (ML-1B, REQ-2026-11-15):
```
raw: [{"roadmap":"C:\\a\\b.md","wave":"1"}]
strings.Contains(raw, `C:\a\b.md`) → false
json.Unmarshal → decoded[0].Roadmap == `C:\a\b.md` → true
```

- **Sintoma:** o teste passa no macOS e no Linux, onde o caminho não tem `\`, e reprova no Windows. Quando o teste
  estava pulado no Windows, o defeito ficava invisível.
- **Correção:** decodificar o JSON e comparar o campo. Nunca procurar um caminho nos bytes crus.
- **Sítio corrigido:** T3 (AC3) em `internal/commands/barrier_reentry_test.go`. O `grep` por
  `TRACKFW_BARRIER_STACK` e `strings.Contains` em `*_test.go` não achou outro sítio.

Na mesma ML, o `echo` do `/bin/sh` do macOS (xpg_echo) interpreta `\b` dentro do JSON. Use `printf '%s\n'`.
