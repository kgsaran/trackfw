---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: Regra `thirdparty_artifact_has_provenance` existe em Go e Python, mas não no validator do Node

> Date: 2026-09-01 | Status: Open

## Motivation

Achado do ML-1C da `REQ-2026-08-30-caminho-portavel-...`, e é resultado de **uma hipótese minha que
estava errada**: eu supus que a regra existia **só no Go**. Medido:

| arquivo | ocorrências |
|---|---|
| `internal/validator/validator_thirdparty_provenance.go` | 7 |
| `pypi/trackfw/validator.py` | 10 |
| `npm/src/validator/index.js` | **0** |

**Falta no Node**, não no Go. E o erro da hipótese foi útil: ao checar o Python, descobri que ele
tinha **o mesmo bug de separador** do Go (`os.path.relpath` devolve nativo igual ao `filepath.Rel`) —
corrigido no PR #231. Se eu tivesse acertado a hipótese, teria corrigido só o Go e **introduzido
quebra de paridade dentro da REQ que existe para corrigir divergência**.

## Por que importa

`thirdparty_artifact_has_provenance` é a regra que garante que **artefato de terceiro instalado tem
aprovação de proveniência registrada**. É controle de cadeia de suprimentos.

**Um usuário que roda `trackfw validate` pelo CLI Node não tem esse controle** — e não recebe aviso
de que não tem. O `validate` reporta limpo, porque a regra não existe ali para reprovar.

Mesma família que já nos custou sete ocorrências: **o mecanismo dá sinal verde enquanto o controle
está ausente.** Aqui a ausência é por runtime, o que é pior — o mesmo comando, no mesmo repositório,
dá respostas diferentes conforme quem o executa.

## Acceptance Criteria

- [ ] **AC1** — A regra existe no validator do Node, com **mesma mensagem e mesma severidade** dos
      outros dois.
- [ ] **AC2** — A chave de busca é montada com `/`, não com separador nativo — o mesmo defeito
      corrigido em Go e Python no PR #231. **Nascer já correta**, não repetir o ciclo.
- [ ] **AC3** — 🔴 **Falsificação nos 3 runtimes**: artefato de terceiro **sem** proveniência é
      recusado pelos três, com saída equivalente; **e o controle**, artefato **com** proveniência
      passa nos três. Sem o controle, trocaríamos ausência de regra por regra que recusa tudo.
- [ ] **AC4** — Gate de paridade que reprove se a regra existir em N runtimes e faltar em outro.
      **Esta lacuna sobreviveu porque nenhum gate compara o conjunto de regras entre os 3
      validators** — é a causa raiz, e sem a AC4 ela reaparece na próxima regra nova.

## Negative Scope

- **Não** alterar a semântica da regra em Go ou Python; o Node se alinha aos existentes.
- **Não** tratar o achado de separador — já corrigido no PR #231.

## Linked ADR

ADR: <!-- nenhum. Fechamento de lacuna de paridade sobre regra já decidida. -->

## Linked Roadmap

Roadmap:
