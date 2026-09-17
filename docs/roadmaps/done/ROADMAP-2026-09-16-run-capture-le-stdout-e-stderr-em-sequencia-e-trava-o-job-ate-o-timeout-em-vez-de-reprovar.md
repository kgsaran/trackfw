---
status: done
date: 2026-09-16
req: "docs/req/REQ-2026-09-16-run-capture-le-stdout-e-stderr-em-sequencia-e-trava-o-job-ate-o-timeout-em-vez-de-reprovar.md"
squad: ""
---

# Roadmap: run-capture le stdout e stderr em sequencia e trava o job ate o timeout em vez de reprovar

> Created: 2026-09-16 | Status: done

## Resultado — concluído em 2026-09-17 (PR #377, issue #372)

**Causa:** `scripts/windows-repro/run.ps1`, `Run-Capture` redirecionava os dois fluxos e os lia **em
sequência** com `ReadToEnd()` síncrono — o deadlock que a documentação da .NET descreve: o pai
bloqueia lendo stdout até o fim enquanto o filho bloqueia escrevendo num buffer de stderr cheio.

**Efeito, pior que uma falha:** o job saía `cancelled`, não `failure`. O veredito sumia, os itens já
executados não apareciam no sumário, e 20 min de runner iam embora sem diagnóstico. Um verificador
que emudece é pior que o defeito que ele deveria verificar.

**Correção:** as duas tasks `ReadToEndAsync()` são emitidas **antes** de esperar qualquer uma.
Duas armadilhas evitadas, com o motivo escrito no código:
1. o `WaitForExit()` final é **sem prazo** — a sobrecarga com argumento não espera a drenagem dos
   pipes redirecionados e trocaria deadlock por **truncamento silencioso**;
2. timeout interno de 18 min, **abaixo** dos `timeout-minutes: 20` do job — um filho travado por
   outro motivo passa a produzir diagnóstico nomeado em vez de cancelamento mudo.

**Falsificação em Windows real, nas duas direções:** deadlock reproduzido a **8 KB** de stderr com a
versão antiga; depois, **256 KB em stdout e stderr simultâneos** retornam completos com `exit=42`
preservado. Reproduzir antes era a exigência — sem isso não se sabe que corrigiu, sabe-se que mudou.

**Crédito:** relatado por consumidor externo (Lourival), com o gatilho isolado (gate de cobertura
reprovando), a objeção *"é problema do fork"* antecipada e respondida, e o efeito de perda de
veredito nomeado. Os 15 sítios de chamada não mudaram — o objeto retornado é idêntico.

## Contexto

REQ: `docs/req/REQ-2026-09-16-run-capture-le-stdout-e-stderr-em-sequencia-e-trava-o-job-ate-o-timeout-em-vez-de-reprovar.md`
Issue: #372 · PR: #377

## Nota do arquiteto — 2026-09-17

🔴 **Este roadmap continha, até hoje, o scaffold intocado do `trackfw roadmap new`** abaixo do
resultado: um `ML-0A` e um `ML-1A` em `⬜ Pendente`, com o gate placeholder `exit 1`, **dentro de um
roadmap em `done/`**. O trabalho real foi entregue no PR #377 e está descrito na seção *Resultado*
acima; o scaffold nunca foi preenchido nem executado.

Removido aqui porque um roadmap concluído que carrega microlotes pendentes afirma duas coisas
contraditórias ao mesmo tempo, e a que engana é a que diz "done".

**Terceira ocorrência do mesmo padrão em 2026-09-17** — o scaffold do `roadmap new` sobrevive por
baixo do conteúdo escrito à mão e chega a `done/`. As outras duas foram o roadmap do #376 (scaffold
duplicado, com dois `ML-1A`) e este. Nenhuma regra do `validate` e nenhum `barrier` pega: o `barrier`
só roda sob demanda, por wave. Registrado como issue própria.

## Legenda de status

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado
