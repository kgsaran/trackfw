---
status: Open
date: 2026-08-17
author: "Zeus (Arquiteto)"
adr: ""
roadmap: ""
---

# REQ: `doctor` detecta artefato em disco ausente do manifesto após janela de gravação parcial

> Date: 2026-08-17 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

Bug reportado por KG em uso real no projeto CMDB: `trackfw agents update --force` falhava com
`unmanaged artifact ".../.codex/agents/trackfw-iac.toml" does not match a trackfw template`.

O comportamento **estava correto** — `preflight` recusa bytes que o trackfw não escreveu, ignorando
`--force` de propósito, porque sobrescrever arquivo desconhecido seria destrutivo. O ML-2C da REQ de
higiene corrigiu a **diagnosticabilidade** (a mensagem agora nomeia o remédio, `install --force`).

Mas o artefato **nunca deveria ter chegado a esse estado**. O disco tinha 12 arquivos com o mesmo
timestamp e o `integrations-manifest.json` registrava 10. Não é legado: `iac`/`tooling` entraram no
catálogo em 2026-07-26 (#72) e o manifesto existe desde 2026-07-19 (#50) — os 12 deveriam ter sido
escritos e registrados na mesma chamada.

### Causa raiz, já investigada

Em `Manager.mutate()` (`internal/integrations/manager.go`), o fluxo por chamada é:

1. `preflight` para todos os itens do lote;
2. snapshot de arquivos e manifestos (para rollback);
3. **Loop 1** — grava os bytes de **todos** os artefatos em disco e atualiza o `Manifest` **em memória**;
4. **Loop 2** — só então persiste cada manifesto em disco, um por escopo;
5. `committed = true`.

**Os bytes de todos os artefatos são gravados antes de qualquer manifesto ser persistido.** Isso abre
uma janela real em que N arquivos existem, corretos, e nenhum registro foi gravado.

O `defer` de rollback cobre erro retornado normalmente, **não** interrupção: `SIGKILL`, crash, queda
de energia ou disco cheio no meio da gravação nunca executam o `defer`. É inerente a gravar múltiplos
arquivos sem WAL/journal cross-file — **não é regressão de um commit específico**, e o mesmo padrão
existe nos 3 CLIs.

📎 `vault/notes/integrations-manifest-write-precedes-persist-janela-de-registro-parcial-2026-08-16.md`

### Por que vira REQ própria, e não item de higiene

Ficou **fora** da REQ de higiene por decisão explícita: aquilo era correção de texto, isto é mudança
de comportamento. Consta da tabela de declaração de não-correção daquele roadmap.

## Escopo

**O problema a resolver é diagnóstico, não prevenção.** Um usuário que caia nessa janela hoje recebe
uma mensagem correta sobre um estado que ele não causou e não sabe interpretar. O produto deve saber
dizer: *"este arquivo bate com o template do catálogo mas não está no manifesto — provável escrita
não registrada; adote com `install --force`"*.

Duas frentes, e a ordem importa:

1. **Detecção (o núcleo desta REQ).** Um comando `trackfw doctor` — ou regra equivalente — que
   compare disco × manifesto e classifique cada divergência. O caso decisivo é **arquivo cujo
   conteúdo bate com o template do catálogo e que está ausente do manifesto**: isso não é
   adulteração, é escrita não registrada, e o remédio é diferente.
2. **Encurtar a janela (a decidir, pode ficar fora).** Persistir o manifesto por item, ou ao menos
   por escopo, reduz o span. **Não elimina** o problema — só o torna menos provável. Precisa de ADR
   antes de mexer: é o caminho de escrita de todos os `install`/`update`, e trocar atomicidade de
   lote por atomicidade por item tem consequência própria.

### Escopo negativo — declarado

- **Não é garantir atomicidade cross-file.** Sem WAL/journal não há como, e construir um WAL para
  isto seria desproporcional. A REQ assume que a janela existe e ataca a consequência.
- **Não é afrouxar o `preflight`.** Ele recusar bytes desconhecidos é o comportamento correto e não
  deve mudar. Coberto por teste nos 3 stacks; qualquer solução que os quebre está errada.
- **Não é mudar a mensagem do `update`** — já feito no ML-2C.

## Acceptance Criteria

- [ ] AC1 — Existe comando/regra que detecta **arquivo em disco ausente do manifesto** e o distingue
      de arquivo modificado à mão. As duas classes têm remédios diferentes e não podem ser fundidas.
- [ ] AC2 — A saída **nomeia o remédio** com o comando pronto para copiar, como o ML-2C fez para o
      `update`.
- [ ] AC3 — Paridade nos 3 CLIs, com gate — **não** por leitura de fonte. A lacuna registrada no
      ML-2C (mensagem idêntica sem gate que compare as três saídas reais) não deve se repetir aqui.
- [ ] AC4 — Cenário de falsificação (P4) com braço baseline e braço de detecção, reproduzindo a
      janela: gravar artefato em disco sem registrar no manifesto e provar que o comando acusa.
- [ ] AC5 — Não-regressão: `update` **continua recusando** bytes unmanaged mesmo com `--force`.
- [ ] AC6 — Decisão sobre encurtar a janela (frente 2) registrada em **ADR** — inclusive se a decisão
      for não mexer, com o motivo.
- [ ] AC7 — `make quality` verde; `trackfw validate` sem novas violações.

## Riscos e armadilhas para quem executar

- **Falso-positivo é o risco dominante.** Um `doctor` que acuse artefato legítimo como suspeito é
  pior que não existir — treina o usuário a ignorar a saída. A comparação tem de ser por conteúdo
  contra o template do catálogo, não por presença de arquivo.
- **Não testar por leitura de código.** O critério é o comportamento com um manifesto de fato
  incompleto, montado no fixture.
- **Cuidado com o binário do `PATH`:** medido em 2026-08-17, o `trackfw` instalado pode estar
  desatualizado e emitir avisos falsos, e `--version` **não** distingue o build. Compilar antes de
  auditar.

## Linked ADR
ADR: <!-- a criar, se a frente 2 for adiante (AC6) -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->

---

## Emenda 1 (2026-08-19) — são **três** classes, não duas

> O corpo acima é o texto original da REQ e não foi reescrito. Esta emenda corrige o número de
> classes, que a barreira do ML-3A mostrou estar errado.

A REQ foi escrita presumindo duas classes. A barreira encontrou um **terceiro estado**
(`!Registered && StateModified`) que nenhuma delas cobria e que o `doctor` silenciava — justamente o
estado que faz o `agents install` recusar com `unmanaged artifact`, ou seja, o sintoma que originou
esta REQ. Fechado no ML-2C com a classe `unknown-content`, cujo remédio **nomeia a recusa** e declara
as duas causas possíveis em vez de acusar adulteração.

Consequência para quem ler o AC1: "distingue de arquivo modificado à mão" continua valendo, mas a
distinção é **ternária**. Ver `docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md` e a
nota de vault correspondente.
