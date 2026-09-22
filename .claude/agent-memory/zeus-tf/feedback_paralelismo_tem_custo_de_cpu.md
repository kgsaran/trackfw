---
name: paralelismo-tem-custo-de-cpu
description: Ao despachar agentes em paralelo neste repo, contar o custo de CPU junto — a suíte de falsificação já usa 8 chunks, e cada agente com go test ./... satura os 10 núcleos
metadata:
  type: feedback
---

**Ao despachar microlotes em paralelo, restringir o teste ao pacote tocado e limitar a 2 agentes
simultâneos quando cada um compila Go.**

**Why:** medido em 2026-09-22, na campanha da REQ-2026-08-31. Eu proibi `make quality` nos três
subagentes paralelos — corretamente, para evitar barreira concorrente, que já abortou runs em
188/630 e 276/630 — **mas não proibi `go test ./...`**. Três agentes compilando e testando o mesmo
módulo (21 pacotes com teste), competindo pelo mesmo `GOCACHE`, numa máquina de **10 CPUs**.

Somado a isso, o que a barreira já consome sozinha:

| | |
|---|---|
| `run-gates-falsify-parallel.sh` | `min(nproc, 8)` = **8 chunks em paralelo** |
| `check-gates-falsify.sh` | **161** invocações de `git`, **22** de `go build/test`, binário isolado **por cenário** |
| `make quality` | ~13 min, e cada auditoria de wave exige uma barreira nova |

🔴 **A proibição estava certa no motivo e incompleta no alcance.** É o padrão a vigiar: proibir o
comando óbvio (`make quality`) e esquecer o que tem o mesmo efeito por outro caminho.

**How to apply:**

- No handoff paralelo: `go test ./internal/<pacote>/`, **nunca** `go test ./...`. A suíte completa é
  da barreira, que roda sozinha de qualquer forma.
- Teto de **2 subagentes** simultâneos quando cada um compila Go. Acima disso o ganho é comido pela
  contenção de CPU e de cache.
- Barreira local pesada: `TRACKFW_FALSIFY_JOBS=4 make quality` — o driver **denuncia o override em
  stderr**, então não mascara resultado sem rastro.
- 🔴 **Antes de culpar o ambiente, meça.** Os dois suspeitos naturais são inocentes: `gitstatusd` do
  powerlevel10k (144 KB RSS, 2s de CPU em 1 dia) e `redirect_listener` do go-build (0,37s em 4
  dias). Acusar o daemon de prompt é o palpite tentador e errado.

Nota de vault com o detalhe: `vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`

Relacionado: [[processos-orfaos-de-subagente]] — sintoma parecido, causa oposta (lá o processo
**fica**; aqui ele termina e some, e por isso a investigação tardia não acha nada).
