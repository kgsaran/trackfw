---
title: "Recusa de root em função void: o analisador exige que a recusa seja ACIONADA, e `_ =` reprova"
tags: [pathguard, containment, analyzer, go, ml-9a]
date: 2026-09-25
related: [analisador-de-contencao-a-populacao-e-a-decisao-cara-nao-o-predicado-2026-09-25, marcador-de-contencao-pode-ser-falso-guarda-de-dir-nao-cobre-a-folha-2026-09-21, chdir-removeall-nao-forca-getwd-falhar-no-windows-2026-09-17]
---

# Recusa de root em função void: o analisador exige que a recusa seja ACIONADA

## Problem

No ML-9A (REQ-2026-08-31) os 5 sítios fail-open tinham a guarda dentro de
`if root, err := projectRoot(); err == nil { … }` e a escrita fora. A correção é trocar o ramo de
erro por `pathguard.RefuseUnverifiableRoot`. Nos 3 sítios que **retornam erro** (`java.go`,
`note.go`) isso é um `return` e acabou.

Nos 2 sítios **void e não-fatais por decisão registrada** (`appendTransitionLog`,
`appendREQTransitionLog` — *"uma falha de escrita de log não deve abortar o move"*) não há valor para
retornar. A forma óbvia:

```go
if rootErr != nil {
	_ = pathguard.RefuseUnverifiableRoot(lp, rootErr)
	return
}
```

**reprova no analisador de AST do ML-7C**, com achado novo:

```
internal/generators/roadmap.go:833 appendTransitionLog() [guard-not-acted-on] path=lp
  — guard result assigned but no failure branch found in the same block
```

## Root cause

`RefuseUnverifiableRoot` está em `guardFunctions` (`containment_analyzer_test.go`), e **toda** chamada
de guarda passa pelo predicado `failureBranchIsTerminating(stmt)`, que só aceita três formas:
`return guard(…)`, `if err := guard(…); err != nil { <bloco que termina> }`, ou um
`err := guard(…)` com um `if err != nil` posterior no mesmo corpo. `_ = guard(…)` (AssignStmt) e
`guard(…)` nu (ExprStmt) são achados por construção.

🔴 **E isso está certo.** A regra é o único instrumento que pega
`RefuseUnverifiableRoot(p, err)` nu no topo da função seguido de `os.WriteFile(p, …)`: se ela fosse
afrouxada para essa função, a chamada **viraria um guardRecord que domina a escrita** e o sítio seria
lido como contido — tendo apenas **imprimido** uma mensagem.

A única forma que passaria sem extração é
`if refusal := pathguard.RefuseUnverifiableRoot(lp, rootErr); refusal != nil { return }` — uma
condição **sempre verdadeira** sobre uma função documentada como sempre não-nil. O gate aceitaria a
mentira.

## Solution

Extrair o corpo guardado para um irmão que **retorna erro**, e deixar a decisão de não-fatalidade
**só** no invólucro:

```go
func appendTransitionLog(basename, fromState, toState string) {
	// NON-FATAL by decision: a log write failure must not abort the move.
	_ = appendTransitionLogEntry(basename, fromState, toState)
}

func appendTransitionLogEntry(basename, fromState, toState string) error {
	root, rootErr := projectRoot()
	if rootErr != nil {
		return pathguard.RefuseUnverifiableRoot(lp, rootErr)
	}
	…
}
```

Efeito colateral bom: a não-fatalidade passa a ser **uma linha auditável** em vez de uma propriedade
difusa do corpo.

### Onde a prova de não-fatalidade fica vazia depois da extração

Se o teste exercitar só `…Entry` e afirmar que ele retorna erro, provou-se a metade **fatal** e nada
sobre a não-fatal. A asserção tem de ser no **invólucro void**: chamar
`appendTransitionLog(…)` com a falha injetada e afirmar as **três** coisas — retorna normalmente,
stderr carrega a recusa **nomeando o alvo**, e o arquivo de log **não existe**.

🔴 E **não** provar isso pelo chamador real (`MoveRoadmap`): `getwdFn` é global e esses caminhos
chamam `projectRoot()` em outros pontos, então a operação externa falha por motivo alheio e o braço
fica verde pela causa errada.

## Como forçar `projectRoot()` a falhar (portável)

`projectRoot()` (scaffold.go) tem **um único** ramo de erro: `os.Getwd` falhando — `EvalSymlinks`
falhando cai no fallback e devolve `nil`. O seam é `var getwdFn = os.Getwd`, e os testes de
`package generators` injetam direto (não precisa ser exportado).

🔴 **O seam vai em `os.Getwd`, nunca em `projectRoot`.** O analisador casa provenência pelo
**identificador literal** `projectRoot` no sítio de chamada (`approvedResolvers` / `taintSourceCalls`);
transformar `projectRoot` numa variável (`projectRootFn()`) tiraria a provenância de resolvedor de
**toda** raiz de guarda do pacote e P2 passaria a reportá-las como não resolvidas.

O truque de filesystem (`chdir` + `RemoveAll` do cwd) **não** serve: no Windows o SO recusa remover o
cwd — ver `chdir-removeall-nao-forca-getwd-falhar-no-windows-2026-09-17.md`.
