---
name: vm-investiga-ci-mede
description: VM de Windows é para investigar; medição oficial de Windows sai do windows-latest do CI — a VM é ARM64 e o runner é x64
metadata:
  type: project
---

**A VM de Windows serve para investigar; a medição que vira afirmação sai do `windows-latest`.**

**Why:** a VM é **ARM64**, o runner é **x64**. Em 2026-09-09, metade do ML-R2a foi gasta provando
que o que a VM media valia para o runner — a ressalva "pode ser artefato desta VM" bloqueou uma
triagem inteira de 512 falhas. Medindo no runner, essa dúvida não nasce. E a VM morreu no mesmo dia
(disco `.qcow2` em volume externo), o que tornou a dependência óbvia.

**How to apply:**

- Investigação interativa (sondar, instrumentar, iterar em minutos) → VM.
- Número que entra em roadmap, REQ, issue ou PR → `.github/workflows/windows-census.yml`
  (`workflow_dispatch`, sonda sem veredito, 8 shards + falsificação + apuração).
- 🔴 **Meça as duas pernas no mesmo runner** quando o número for uma *diferença*: dispare o censo em
  `main` (sem a correção) e na branch (com), em vez de comparar contra base antiga de outra
  plataforma. Isso elimina confundidores de plataforma e de env var por construção, em vez de
  deixá-los como ressalva escrita.
- `workflow_dispatch` **só é acionável se o arquivo existir na branch padrão** — instrumento novo
  precisa de PR próprio antes de poder rodar (foi o PR #303).
- Recriar a VM: `docs/portabilidade/2026-09-09-vm-de-windows-para-medicao-instalacao-e-ssh.md`.
  ⚠️ **Disco em volume externo é decisão TOMADA do KG (2026-09-26)** — ele consultou e a movimentação
  foi recomendada sem ressalvas. A VM atual roda de `/Volumes/External/`. 🔴 **Não levante isso como
  risco de novo**; a doc foi corrigida e o alarme já foi dado uma vez. O que sobrevive da lição
  antiga é **snapshot depois de configurar**, que não depende de onde o disco está.
- Acesso medido em 2026-09-26: `Lab@192.168.64.6`, chave já aceita, repo em `C:\Users\Lab\trackfw`,
  `go1.27.0 windows/arm64`. O shell do SSH é **`cmd.exe`** — `head`/`tail` não existem lá. Caminho que
  funciona: `scp` de um script e
  `ssh Lab@… '"C:\Program Files\Git\bin\bash.exe" -c "bash /c/Users/Lab/script.sh"'`.
  🔴 A árvore da VM costuma ter **trabalho não commitado** — use `git worktree add --detach` em vez de
  `checkout`, que aborta (e, se não abortasse, descartaria).

Relacionado: [[medir-com-a-regra-nao-com-grep]] — mesma família: o instrumento tem que medir o
objetivo, não um proxy conveniente.
