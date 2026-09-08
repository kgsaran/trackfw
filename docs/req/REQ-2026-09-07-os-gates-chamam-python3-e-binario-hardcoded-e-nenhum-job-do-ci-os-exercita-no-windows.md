---
status: Done
date: 2026-09-07
author: ""
adr: ""
roadmap: ""
---

# REQ: os gates chamam python3 e binario hardcoded e nenhum job do CI os exercita no Windows

> Date: 2026-09-07 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Origem

Medido pelo arquiteto na **VM Windows 10 Pro ARM64** em 2026-09-07, ao tentar responder a pergunta
que o autor do issue [#288](https://github.com/kgsaran/trackfw/issues/288) deixou em aberto:

> *"O gate aborta na linha 543; não medi o que vem depois dela. **Pode haver mais vermelhos escondidos
> atrás deste.**"*

Resposta: há — e não são da causa que ele viu.

## Motivation

🔴 **`python3` no Windows resolve para o stub da Microsoft Store**, que imprime *"Python was not
found"* **com o Python instalado e funcionando**:

```
python   → /c/Users/Lab/AppData/Local/Programs/Python/Python312-arm64/python   ✅ real
python3  → /c/Users/Lab/AppData/Local/Microsoft/WindowsApps/python3            ❌ stub da Store
py       → /c/Users/Lab/AppData/Local/Programs/Python/Launcher/py              ✅ launcher
```

E os gates chamam, **hardcoded**:

```
python3    396 ocorrências em scripts/*.sh
python     312 ocorrências
```

Só `scripts/smoke-integration-packages.sh` tem `PYTHON_BIN` configurável (e foi pinado no `Makefile`
pelo ML-2E da REQ do `parity`).

### Por que isto nunca apareceu

```
.github/workflows/quality.yml:522-523
  parity:
    runs-on: ubuntu-latest
```

**Nenhum job do CI executa estes scripts no Windows.** Não é regressão: é superfície que nunca foi
exercitada. Foi preciso uma VM para vê-la.

### A mensagem de erro aponta para o lugar errado — duas vezes

Este é o padrão que mais custou tempo nesta investigação, e ele se repetiu:

| sintoma observado | causa real |
|---|---|
| `go: go.mod file not found in current directory or any parent directory` | **binário ausente** em `bin/` — o helper cai no fallback de `go build` a partir de uma fixture sem `go.mod` |
| `Python was not found; run without arguments to install from the Microsoft Store` | Python **está** instalado; `python3` é o alias da Store |

🔴 **Duas hipóteses do arquiteto caíram por medição**, e ficam registradas para não serem
re-derivadas:

1. *"A causa é `$ROOT_DIR/bin/trackfw` hardcoded sem `.exe`, 88 ocorrências."* **Errado** — patchei as
   88 na VM e o resultado foi idêntico. A falha era o binário não existir.
2. *"O `cp -r` do Cenário 8 é o que trava o Windows."* Correto, mas **não é o primeiro** obstáculo —
   o gate morre antes dele.

## Acceptance Criteria

- [ ] **AC1** — a seleção do interpretador Python é **resolvida**, não presumida: um ponto único que
      escolhe um interpretador **funcional** (rejeitando o stub da Store), usado pelos scripts de
      gate. 🔴 Provar que rejeita o stub, não só que acha *algum* `python3`.
- [ ] **AC2** — a seleção do binário do CLI honra o sufixo da plataforma (`.exe`) **e falha alto se o
      binário não existir**, em vez de cair em `go build` a partir de fixture sem `go.mod`. A
      mensagem de erro deve nomear a causa real.
- [ ] **AC3** — o `check-gates-falsify.sh` **roda até o fim** no Windows, e a lista de cenários que
      reprovam ali é **medida e escrita**. 🔴 "N cenários reprovam no Windows" é resultado válido; o
      que não é válido é não saber.
- [ ] **AC4** — a contagem de falhas de Windows do projeto é **reconciliada** com o que este gate
      revelar. As 39 residuais foram medidas **sem** que este gate jamais rodasse lá.
- [ ] **AC5** — decisão explícita e escrita sobre **rodar o `parity` (ou um subconjunto) em Windows no
      CI**. 🔴 "Não vamos rodar" é decisão válida — mas então fica escrito que a superfície é
      declaradamente não coberta, em vez de silenciosamente.

## Negative Scope

- **Não** altera o comportamento dos gates em Linux/macOS — o conjunto de cenários e seus vereditos
  ficam idênticos, verificado por igualdade de **conjunto**.
- **Não** troca `python3` por `python` em massa por `sed`: no Linux `python` pode não existir ou ser
  Python 2. A resolução tem de ser por **detecção**, não por substituição literal.
- **Não** reabre a paralelização do `parity` (REQ própria, já entregue e mergeada no PR #291).
- **Não** promete fechar o issue #288: ele fecha quando o AC3 produzir a medição, não antes.

## Nota de método

A VM reduziu o custo deste diagnóstico de **um ciclo de PR + leitura de log de CI** para **minutos**.
O Go não estava instalado nela por omissão do arquiteto ao montá-la — a lista de pré-requisitos foi
herdada de um objetivo anterior (medir CLIs de agente, que não compilam Go) e não revista quando o
alvo mudou.


## Encerramento — 2026-09-08

O `check-gates-falsify.sh` — o gate que falsifica todos os outros — **nunca havia rodado no Windows**.
Agora roda, e o censo é **440 OK · 512 FAIL**, reproduzível por `TRACKFW_FALSIFY_ENUMERATE=1`.

Entregue: resolução de interpretador (`resolve_py_bin`, que rejeita o stub da Store **validando por
execução**), resolução de binário (`FALSIFY_GO_BIN`, que falha alto nomeando a causa real), shim de
`PATH` para os ~40 sub-scripts, e o modo de enumeração com guarda provada por sabotagem — enumera
**e** sai != 0.

**AC5 respondido por escrito:** o `parity` **não** passa a rodar em Windows no CI agora. A cobertura
fica declaradamente sob demanda, com o pré-requisito nomeado.

🔴 **Sítios conhecidos e não corrigidos, transferidos com endereço — não abandonados:**
- **triagem dos 512 por causa** → `ML-R2` da `REQ-2026-09-03-as-217-falhas-reais-de-windows-...`
  (mesma causa: falhas de Windows agrupadas por mecanismo). É o pré-requisito do ratchet de CI.
- obstáculos medidos e **fora do escopo desta REQ** por serem de outra causa: separador embutido em
  string (`MSYS` não converte caminho não-token — vault), locale (Node ignora `LANG`/`LC_ALL`),
  mojibake **não verificado**, e divergência Go-vs-Python não relacionada a Windows.

Fecha porque **as causas desta REQ — interpretador e binário — não têm sítio remanescente**. As
outras têm dono escrito.
