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

## Wave 1 — Implementação
> 🔴 Dependências: **Wave 0 auditada**. O predicado sai de lá, não daqui.

Os MLs desta wave só são escritos **depois** que o `ML-0A` decidir o predicado — escrevê-los agora
seria fixar a solução antes da medição que a escolhe, que é o erro que esta REQ existe para não
repetir.

O que já está decidido, e independe do predicado:

- o braço da junção em `containment_junction_windows_test.go` passa de `t.Logf` a **expectativa**
  (AC3);
- o braço **C2** (escape visível no caminho) continua recusando (AC5);
- o que a guarda **não** cobre entra no contrato (AC6).


---

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
**Status:** ⬜ Pendente

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
- [ ] Tabela de objetos × `ModeSymlink` × `ModeIrregular`, incluindo os do item 1
- [ ] Predicado **decidido e justificado**, com o que ele recusa e o que deixa passar
- [ ] O não-mensurável **declarado**, com o motivo

⚠️ **Custo do erro, nas duas direções, para calibrar a decisão:**
**falso-negativo** = escrita fora do projeto sem aviso (o defeito de hoje);
**falso-positivo** = o produto **paralisa** numa máquina Windows legítima — e o AC13 da
`REQ-2026-09-09` nomeia isso: *"falso-positivo aqui paralisa, não irrita"*.

🔴 **A VM investiga; o `windows-latest` mede.** A VM é ARM64 e o runner é x64: número que vira
afirmação em REQ, PR ou changelog sai do CI, não da VM.
