# Carga de CPU na orquestração: a suíte de falsificação × agentes paralelos

> 2026-09-22 · medido durante a campanha da REQ-2026-08-31 (contenção de escrita)

## O sintoma

Durante um dia de campanha com múltiplos microlotes, a máquina fica saturada por longos períodos,
com muitos processos `git` e `go` efêmeros. Ao investigar **depois**, não se encontra nada vivo — o
que leva a suspeitar de processo órfão ou vazamento, e a procurar no lugar errado.

## Os dois suspeitos naturais estão ERRADOS — medidos

Antes de acusar o ambiente, meça. Foram descartados:

| suspeito | medição | veredito |
|---|---|---|
| `gitstatusd` (daemon do powerlevel10k) | **144 KB** de RSS, **2s** de CPU em 1 dia | inerte |
| `redirect_listener` (cache do go-build) | **0,37s** de CPU em **4 dias** | inerte |

O daemon de prompt é o palpite mais tentador — ele varre o repositório a cada mudança de árvore, e
uma sessão de agentes muda a árvore o tempo todo. **Os números dizem que não é ele.**

## A causa real, com os multiplicadores

```
scripts/run-gates-falsify-parallel.sh  -> min(nproc, 8) = 8 chunks EM PARALELO
scripts/check-gates-falsify.sh         -> 161 invocações de git
                                          22 de go build/test/run
                                          binários isolados compilados POR CENÁRIO (Txx_BIN)
```

Some a isso:
- `make quality` leva ~13 min e foi executado **4 vezes num único dia** (cada auditoria de wave exige
  uma barreira nova, com a árvore parada);
- **subagentes em paralelo**, cada um rodando `go build ./...` e `go test ./...` — o `go test`
  paraleliza por pacote, e o módulo tem 21 pacotes com teste.

Numa máquina de **10 CPUs**, 8 chunks de falsificação **ou** 3 agentes compilando Go saturam
sozinhos. Os dois juntos, muito mais.

## 🔴 O erro de orquestração que produziu o pico

Eu proibi `make quality` nos três subagentes paralelos — exatamente para evitar barreira concorrente,
que já abortou runs em 188/630 e 276/630 nesta campanha.

**Mas não proibi `go test ./...`.** Três agentes compilando e testando o mesmo módulo ao mesmo tempo,
competindo pelo mesmo `GOCACHE`. A proibição estava certa no motivo e **incompleta no alcance**.

## O que fazer

**Na barreira local:**
```bash
TRACKFW_FALSIFY_JOBS=4 make quality
```
O driver aceita o override e **denuncia em stderr** que ele foi usado — não há risco de mascarar
resultado sem deixar rastro. Deixa 6 núcleos livres.

**No handoff de microlote paralelo**, exigir o teste do **pacote tocado**, não da árvore:
```
go test ./internal/generators/     # não  go test ./...
```
A suíte completa fica para a barreira única do arquiteto, que roda sozinha de qualquer forma.

**Teto de 2 subagentes simultâneos** quando cada um compila Go. Acima disso, o ganho de paralelismo
é comido pela contenção de CPU e de cache.

## O que esta nota NÃO diz

Não diz que a suíte de falsificação é cara demais — ela é o que sustenta a afirmação de que os gates
reprovam de verdade, e o custo é justificado. Diz que **o paralelismo dela e o dos agentes se
multiplicam**, e que quem orquestra precisa contar os dois juntos antes de despachar.

Relacionado: `processos-orfaos-de-subagente` (loops de polling que seguram o agente vivo) — sintoma
parecido, causa diferente. Ali o processo **fica**; aqui ele termina e some.
