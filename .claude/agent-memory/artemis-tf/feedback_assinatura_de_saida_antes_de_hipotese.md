---
name: assinatura-de-saida-antes-de-hipotese
description: Ao investigar falha de subprocesso sem ter a plataforma, medir a assinatura (rc + stdout + stderr) de cada mecanismo candidato e comparar com a observada, em vez de argumentar
metadata:
  type: feedback
---

Investigação de falha em plataforma que não tenho: **produzir localmente a assinatura de saída de
cada mecanismo candidato** (`rc`, `stdout`, `stderr`) e confrontar com a observada. Mecanismo cuja
assinatura difere está **eliminado**, sem precisar da plataforma. E declarar "não sei ainda" quando
sobra mais de um candidato — hipótese apresentada como causa é o que o gate existe para pegar.

**Why:** no ML-0A do grupo B de Windows (2026-09-04) isso descartou CRLF (rc 2), script ausente
(rc 127), bit de execução (rc 0), `text=True` no stdin e `$HOME` (rc 0) em minutos, no macOS, sem
runner Windows — e isolou a única assinatura compatível (`rc=1` + `stderr` vazio = alguém falou por
um canal que ninguém lê). Argumentar sobre esses mesmos mecanismos teria consumido a sessão inteira
sem eliminar nenhum.

**How to apply:** vale para qualquer falha de subprocesso/CI que eu não consiga reproduzir. Dois
corolários que já custaram correção nessa mesma sessão: (1) se o helper de teste descarta `stdout`,
o dado que explicaria tudo já foi jogado fora — imprimir o canal descartado é o **primeiro** passo
da sonda; (2) **ausência de falha no log não é prova de execução** — `go test` sem `-v` não imprime
`PASS` **nem `SKIP`**, então conferir `t.Skip(`/`GOOS` antes de afirmar que um teste passou.
Relacionado: [[falsificacao-nas-duas-direcoes]], [[diagnostico-herdado-pode-estar-vencido]].
