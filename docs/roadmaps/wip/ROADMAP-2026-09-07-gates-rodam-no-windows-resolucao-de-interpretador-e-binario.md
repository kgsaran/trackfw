---
status: wip
date: 2026-09-07
squad: ares-tf
req: "docs/req/REQ-2026-09-07-os-gates-chamam-python3-e-binario-hardcoded-e-nenhum-job-do-ci-os-exercita-no-windows.md"
---

# Roadmap: Os gates rodam no Windows — resolução de interpretador e de binário

> Criado em: 2026-09-07 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-07-os-gates-chamam-python3-e-binario-hardcoded-e-nenhum-job-do-ci-os-exercita-no-windows.md`

## Diagnóstico — medido na VM Windows 10 Pro ARM64, 2026-09-07

```
python   → .../Programs/Python/Python312-arm64/python    ✅ real
python3  → .../Microsoft/WindowsApps/python3             ❌ stub da Microsoft Store
py       → .../Programs/Python/Launcher/py               ✅ launcher

chamadas hardcoded nos gates:  python3 → 396   ·   python → 312
configurável:                  só smoke-integration-packages.sh (PYTHON_BIN)
```

O stub imprime **"Python was not found"** com o Python instalado e funcionando.

**Por que nunca apareceu:** `quality.yml:522-523` — o job `parity` roda em `ubuntu-latest`. Nenhum
job do CI executa estes scripts no Windows. Não é regressão; é **superfície nunca exercitada**.

🔴 **Duas hipóteses do arquiteto caíram por medição** — não re-derivar:
1. *"88 ocorrências de `$ROOT_DIR/bin/trackfw` sem `.exe`"* — patchei as 88 na VM, **resultado
   idêntico**. A falha era o binário **não existir**.
2. *"O `cp -r` do Cenário 8 trava o Windows"* — correto, mas **não é o primeiro** obstáculo.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Fazer o gate chegar ao fim no Windows

### ML-1A — Resolução do interpretador Python
**Status:** ✅ Concluído · **Agente:** `ares-tf`

Ponto único que escolhe um interpretador **funcional**, rejeitando o stub da Store.

🔴 **Não** trocar `python3` por `python` em massa por `sed`: no Linux `python` pode não existir ou ser
Python 2. Tem de ser **detecção**, não substituição literal.

**Critérios:**
- [ ] rejeita o stub da Store — provado, não só "acha algum python3"
- [ ] conjunto de cenários em Linux/macOS **idêntico** (igualdade de conjunto, não contagem)
- [ ] `make quality` para arquivo, `grep -c '^FAIL'` sobre a saída inteira = 0

### ML-1B — Resolução do binário do CLI
**Status:** ✅ Concluído · **Agente:** `ares-tf`

Honrar o sufixo da plataforma **e falhar alto se o binário não existir**, em vez de cair em
`go build` a partir de fixture sem `go.mod`.

🔴 **A mensagem de erro tem de nomear a causa real.** O sintoma observado foi
`go: go.mod file not found` — que não tem nada a ver com a causa e me levou a duas hipóteses erradas.

**Critérios:**
- [ ] binário ausente ⇒ erro que diz *"binário ausente"*, não erro de módulo
- [ ] sufixo de plataforma honrado
- [ ] comportamento em Linux/macOS inalterado

## Wave 2 — Medir o que existe atrás
> Dependências: Wave 1.

### ML-2A — Rodar até o fim no Windows e escrever a lista
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

🔴 **"N cenários reprovam no Windows" é resultado válido.** O que não é válido é não saber.

Reconciliar com as **39 falhas residuais** da campanha — elas foram medidas **sem** que este gate
jamais rodasse no Windows.

**Critérios:**
- [ ] lista escrita dos cenários que reprovam, com o erro de cada um
- [ ] reconciliação com as 39 residuais: quantas são destas, quantas são novas
- [ ] fecha (ou não) o issue #288, com a medição que o autor pediu

## Wave 3 — Decisão de cobertura
> Dependências: Wave 2.

### ML-3A — Rodar o `parity` (ou subconjunto) em Windows no CI?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`

🔴 **"Não vamos rodar" é decisão válida** — mas então fica **escrito** que a superfície é
declaradamente não coberta, em vez de silenciosamente.

## Fora deste roadmap
- **Paralelização do `parity`** → REQ própria, entregue no PR #291.
- **Instalação em Windows** → REQ própria já existente.

## Critérios de Aceite

- [ ] `check-gates-falsify.sh` roda até o fim no Windows
- [ ] conjunto de cenários em Linux/macOS idêntico ao de hoje
- [ ] lista dos que reprovam no Windows, escrita e reconciliada com as 39 residuais
- [ ] decisão de cobertura de CI registrada por escrito
