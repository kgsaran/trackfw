---
title: Parecer — a issue #216 pode ser fechada?
date: 2026-09-05
author: artemis-tf (QA — investigação, nenhuma correção aplicada)
status: investigação concluída
---

# Parecer sobre o fechamento da issue #216

> 🔴 Documento de investigação pura. Nenhum arquivo de produto, teste, fixture ou gate foi alterado
> para produzi-lo. Nenhuma operação de git foi executada por este agente. Os arquivos que o ML-6E
> está editando agora (`npm/tests/git_branch_guard_hook_integrity.test.js`,
> `internal/integrations/manifest_origin_test.go`) não foram tocados.

## Estado do repositório no momento da medição

Branch `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`, cujo HEAD (`b254d2b5`) é
descendente direto e sem divergência de `main` (`87aded6`, merge-base == tip de `main`). O run de CI
mais recente da `main`, `33931363032` (job `windows-full-suites`, `windows-defect-reproduction`,
`parity`), corresponde exatamente a este `main` HEAD — as medições abaixo são atuais, não históricas.

## Tabela item a item

| # | item (texto da issue) | veredito | evidência |
|---|---|---|---|
| 1 | `UnicodeEncodeError` cp1252 no `--help`/`status`/`validate` | **CORRIGIDO** | `_force_utf8_output()` chamada em `cli.py::main()`. Sonda em **Windows real**, run `33931363032`: `VERDICT=ABSENT`. `TestCliEmConsoleCp1252` reproduz via `PYTHONIOENCODING=cp1252` em qualquer SO. |
| 2 e 6 | Os 3 runtimes ignoram `$HOME` no Windows; isolação de teste vácua | **CORRIGIDO no produto** (instrumento de CI tem lacuna — ver §3) | Helper `homedir.Dir()`/`homedir()`/`home_dir()` nos 3 runtimes; **zero** call site cru fora do helper (`grep` confirmado); 21/23/28 sítios de produção (Go/Node/Python). `scripts/check-homedir-parity.sh` roda de ponta a ponta contra os 3 binários compilados — medido agora, `rc=0`. |
| 3 | `credential_guard_hook_resolvable` sempre reprova em NTFS | **CORRIGIDO no produto** (instrumento tem a mesma lacuna do item 2 — ver §3) | `CurrentGOOS != "windows" && info.Mode()&0111 == 0` presente em `validator_credential_guard.go:460` e `validator_git_branch_guard.go:200`; equivalentes confirmados em `npm/src/validator/index.js:1752,2861` (`_platform !== 'win32'`) e `pypi/trackfw/validator.py:2174,3379` (`_current_platform != "win32"`). Falsificação portátil presente e passando: `TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao` simula `CurrentGOOS="windows"` com um arquivo genuinamente `0644` — não depende de NTFS real. Decisão de segurança já auditada: `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md`. |
| 4 | `check-parity-contract-coverage.sh` crasha em cp1252 | **CORRIGIDO — com ressalva sobre o instrumento (ver §3)** | O `.sh` exporta `PYTHONIOENCODING=utf-8` (linha 54), adicionado em 2026-09-02 (`ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`), **depois** de o roadmap de 31/08 ter declarado o item fora de escopo. Medido agora: `PYTHONIOENCODING=cp1252 bash scripts/check-parity-contract-coverage.sh` → `rc=0`, sem crash. Gate ligado em `Makefile:22`, dentro de `parity:`, que roda em CI (job `parity`, `ubuntu-latest`). |
| 5 | Geradores Python escrevem CRLF (38 sítios) | **CORRIGIDO** | Sonda em **Windows real**, run `33931363032`: `VERDICT=ABSENT` — mas isso cobre só os **5** `.sh` de `init`, não os 18 `.md`/YAML dos comandos `new` que o reporter também contou (38 sítios/68 com `write_text`). Fechei essa lacuna por leitura direta: **todo** `open(..., "w"/"a", ...)` e `.write_text(...)` texto (não-binário) em `pypi/trackfw/` — generators, `commands/update_harness.py` (12 sítios de `write_text`), `commands/{update,sync,metrics,configure,discover,context}.py` — tem `newline="\n"` explícito; zero exceção. `scripts/check-python-writes-lf.sh` varre **todo** `pypi/trackfw/**/*.py` por `open(`/`.write_text(` (não só os 5 scripts do `init`), com guarda de vacuidade por `find`; rodei agora, `rc=0`, "nenhuma chamada sem newline explícito". **É o item com a evidência mais forte de todos: medido de ponta a ponta em Windows real para os `.sh`, e por varredura completa do corpus (não amostragem) para o restante do universo que o reporter contou.** |
| 7 | `sys.stdin.isatty()` mente `True` para `NUL` | **CORRIGIDO** | `pypi/trackfw/tty.py` usa `GetConsoleMode` (mesma primitiva do Go); `stdin_is_interactive()` wired em `init.py:118` e no portão de confiança de `thirdparty install --yes-i-trust-this-source`. Sonda em **Windows real**, run `33931363032`: `VERDICT=ABSENT`, `init` completou sem entrar no wizard sob `stdin=NUL`. |

**Dois testes que passavam pelo motivo errado:**

| teste | veredito | evidência |
|---|---|---|
| `TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand` (`//` no dedup) | **ABERTO — mecanismo ainda DESCONHECIDO** | Ainda falha no run mais recente (`33931363032`, `git_branch_guard_dedup_test.go:241`). Catalogado como grupo **G12** na re-triagem de 2026-09-04: fixture usa JSON válido (ao contrário do G3), então não é o mesmo defeito de escaping que fechou outros 22 casos. Nenhuma hipótese de causa foi confirmada. **Não desapareceu por acidente junto com os outros fixes — segue vivo, exatamente como o reporter previu.** |
| Fixture só-acento de `check-artifact-parity.sh` (`slugify` deletava em vez de colapsar) | **CORRIGIDO** | Fixture trocada para `TITLE="Autenticação C/C++ v1.2"` (inclui `/` e `+`, como o reporter recomendou). Medido nos **3** runtimes, não só no Python que tinha o bug: `slugify()`/`toSlug()`/`toSlug()` (Python/Go/Node) sobre `"Ação C/C++ & Café"` devolvem **`acao-c-c-cafe"` os três**, byte-idêntico. Rodei o gate real (`scripts/check-artifact-parity.sh`, não só a leitura da fixture): `rc=0`, artefatos gerados pelos 3 CLIs no fixture atual (`Autenticação C/C++ v1.2` → `autenticacao-c-c-v1-2` nos 3) batem. |

## §1 — Nenhuma das 100 falhas residuais de Windows (run `33931363032`, Go 45 / Node 34 / Python 21) é regressão dos 7 itens

Varri o log completo por `homedir|UserHomeDir|expanduser` (item 2) — zero ocorrência entre as
falhas. O grupo mais próximo do tema de permissão (`G5`, bits de modo restritivo NTFS,
`TestSave_WritesAtomicallyWithPermissions`, 0600 vs 0666) é sobre **permissão de arquivo de
identidade**, não sobre o bit de execução do credential guard (item 3) — mecanismo e sítio de código
diferentes. O único item da re-triagem que tem qualquer relação com os 7 é o `G0`
(`TestPathIsAnchoredForHookConfig_ControlePOSIX`), e é um efeito colateral de uma correção
**posterior e não relacionada** (Wave 3, `IsAbs`/ADR-2026-09-04), não um item da issue #216.

## §2 — Correção de premissa do handoff

O handoff citou 21/25/28 sítios de produção para Go/Node/Python. Medido agora: **21/23/28**. A
diferença de 2 no Node é pequena e não muda o veredito, mas registro porque o handoff pedia
conferência, não confiança.

## §3 — O que a auditoria por amostragem do arquiteto não podia ver: o instrumento de CI tem uma lacuna estrutural nos itens 2, 3 e (parcialmente) 4

Este é o achado que mais importa da investigação, e é exatamente o tipo de coisa que "o gate existe"
não revela.

**Os itens 2 e 3, na sonda `windows-defect-reproduction`, são estruturalmente incapazes de ir a
`ABSENT` — por desenho, não por bug.** A sonda mede `os.homedir()`/`os.path.expanduser`/
`os.UserHomeDir()` crus e `info.Mode()&0111` crus — ou seja, mede **a plataforma**, não o `trackfw`.
Isso já está documentado e aceito pelo próprio time (`ROADMAP-2026-08-31-portar-as-correcoes-...`,
seção "MEDIÇÃO NO CI", e `REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-nos-itens-2-e-3-
retarget-dos-checks-de-home-e-bit-de-execucao.md`). **Verifiquei que essa REQ de retarget continua
`status: Open`, sem roadmap associado (`roadmap: ""` no frontmatter, e nenhum roadmap em
`docs/roadmaps/` a referencia).** Ou seja: passados cinco dias, a lacuna que o próprio time identificou
continua sem correção de instrumento.

**Consequência prática:** hoje, a única prova de que os itens 2 e 3 estão corrigidos em Windows real
é indireta — a lógica do fix não depende de SO (prefere `$HOME` sempre que definida, em qualquer
plataforma) e a falsificação portátil (`CurrentGOOS="windows"` + arquivo genuinamente sem bit de
execução) não exige NTFS real para ser válida. Isso é **engenharia defensável**, mas não é o que um
contribuidor externo cético, que mediu com os próprios olhos num Windows real, vai aceitar sem
explicação — ele vai olhar o dashboard do CI, ver `windows-defect-reproduction` **falhando** com
"item 2: REPRODUCED" e "item 3: REPRODUCED", e concluir (razoavelmente, pela leitura da UI) que nada
mudou.

**O item 4 tem uma variante mais sutil do mesmo problema, e é pior porque é silenciosa.** A sonda do
item 4 testa deliberadamente "o mesmo mecanismo do item 1, **sem o wrapper .sh**" — ou seja, ela
nunca invoca `scripts/check-parity-contract-coverage.sh`, o artefato real que foi corrigido em
2026-09-02. Ela testa `print()` cru de um `→` fora do `main()` do CLI, que é genuinamente um
mecanismo diferente e continua existindo em qualquer script standalone que não passe por
`_force_utf8_output()` — mas **não é mais o comportamento do gate que existe hoje**. Medi
diretamente: o gate real, sob `PYTHONIOENCODING=cp1252`, sai `0` sem crash. A sonda continua rotulando
o item como `REPRODUCED` porque **nunca foi atualizada depois do fix de 2026-09-02** — ela mede uma
pergunta que deixou de corresponder ao artefato corrigido. Isso é dívida de instrumento, não defeito
de produto, mas é exatamente o tipo de coisa que o handoff pediu para não aceitar "de graça": o
dashboard mente por omissão de atualização, não por má-fé.

## (a) A issue pode ser fechada hoje, com honestidade?

**Não, ainda não — mas por uma razão diferente de "os defeitos continuam abertos".** Dos 7 itens
originais, **6 estão genuinamente corrigidos no produto** (1, 2/6, 3, 4, 5, 7) e apenas **1 continua
aberto de fato** (o teste de dedup `//`, mecanismo desconhecido). Isso já seria motivo suficiente
para uma resposta de progresso substancial ao reporter.

O que impede fechar **hoje** não é código, é **instrumento**: o dashboard de CI que o reporter (ou
qualquer pessoa) vai olhar continua mostrando "item 2: REPRODUCED" e "item 3: REPRODUCED" — e sem uma
explicação ao lado, ou sem o retarget que o time já decidiu fazer e não fez, fechar a issue citando
"olhe o CI" seria enganoso, mesmo com o código certo.

**Evidência que o reporter aceitaria, no formato que ele mesmo usou:** ele mediu rodando os 3 CLIs
num Windows real com `$HOME` setado e diferente de `%USERPROFILE%`, e leu o resultado. A prova
equivalente e honesta é: portar `scripts/check-homedir-parity.sh` (ou o item 2 da sonda) para rodar
contra os **binários reais compilados** no job `windows-defect-reproduction` (que já roda em
`windows-latest`), comparando o comportamento **antes/depois do fix revertido temporariamente** — a
REQ de retarget já documentada pede exatamente isso e nunca foi implementada. Sem isso, "o código
está certo" fica provado só por leitura e por falsificação portátil, não pela mesma classe de
medição que gerou a issue.

## (b) O que falta, em lista acionável

1. **Implementar `REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-nos-itens-2-e-3-
   retarget-dos-checks-de-home-e-bit-de-execucao.md`** — hoje `Open`, sem roadmap. Sem isso, o
   dashboard de CI vai mostrar os itens 2 e 3 como `REPRODUCED` para sempre, mesmo corrigidos.
2. **Atualizar (ou aposentar) a sonda do item 4** em `scripts/windows-repro/run.ps1` para invocar
   `scripts/check-parity-contract-coverage.sh` de verdade, em vez do mecanismo cru sem wrapper — hoje
   ela testa uma pergunta que o fix de 2026-09-02 já não deixa em aberto.
3. **`TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand`** (Go) e os
   equivalentes Node/Python (grupo G12 da re-triagem) — 3 falhas, mecanismo ainda não identificado.
   Precisa de instrumentação em Windows real da função de dedup
   (`internal/generators/agentfiles.go`, família `hookArrayHasCommand`) com o valor exato
   `home + "//" + ".trackfw/scripts/..."`. Único item dos 7 (+2) genuinamente ainda aberto.
4. Não bloqueante para o fechamento, mas correlato: as 100 falhas residuais de Windows medidas nesta
   sessão (`docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md`) não são
   regressão de nenhum dos 7 itens — são uma classe de defeitos posterior e distinta (CRLF no parser
   de frontmatter, escaping de mensagem, etc.), sob REQ e roadmap próprios já em andamento.

## (c) Algum item foi "corrigido" escondendo o defeito em vez de curá-lo?

**Não encontrei teste pulado, `skip` silencioso, nem guarda de plataforma que apague asserção** nos
7 itens — o padrão que a campanha já vetou explicitamente (`vault/notes/goos-guard-...`,
`ADR-2026-08-04`) foi seguido: os guards de GOOS/plataforma nos itens 3 e (indiretamente) 2 suprimem
uma checagem que era **sempre-verdadeira travestida de achado** em NTFS, não uma checagem que
discriminava algo real — isso é justificado, auditado por `hades-tf`, e comprovado pela falsificação
portátil que mostrei acima.

**O que encontrei foi mais sutil, e é o achado que o handoff pediu para priorizar:** o item 4 não foi
"escondido" — foi corrigido de verdade em 2026-09-02 — mas o **instrumento que deveria provar isso
em Windows real nunca foi atualizado**, e continua reportando `REPRODUCED` por medir uma versão do
mecanismo que o próprio fix tornou obsoleta. Ninguém mentiu; ninguém escreveu um gate vazio. Mas se
alguém fechar a issue citando "o CI mostra X REPRODUCED, então nada mudou" **sem ler o código do
gate**, chega à conclusão errada — e se alguém fechar a issue citando "6 de 7 corrigidos" sem
mencionar que o dashboard não reflete isso para os itens 2, 3 e 4, entrega uma vitória que o
reporter, olhando o mesmo CI que ele próprio inspirou, não vai conseguir confirmar sozinho. **A
correção mais honesta não é tocar código — é tornar o instrumento capaz de provar o que já é
verdade**, que é exatamente o conteúdo da REQ de retarget já escrita e ainda não implementada.

## Duas notas menores, para completar o quadro

- **Item 4, trade-off explícito no próprio código:** num console genuinamente cp1252 real (não
  simulado por `PYTHONIOENCODING`), a saída vira **mojibake** em vez de crashar — o comentário do
  `.sh` declara essa troca deliberadamente ("acento ilegível com exit code correto vale mais que uma
  reprovação falsa"). O veredito CORRIGIDO é sobre o sintoma que o reporter reportou (o gate
  **morrer**), não sobre legibilidade perfeita de acento no console dele.
- **Itens 2/6, mecanismo do "$HOME-first" não é uniforme entre runtimes, por design documentado:** no
  Python, `home_dir()` só prefere `$HOME` **sob `sys.platform == "win32"`** — em POSIX o
  `expanduser("~")` já lê `$HOME`, e preferir a variável ali **quebraria** a isolação de testes que
  fazem `monkeypatch.setattr("os.path.expanduser", ...)` em vez de var de ambiente (docstring do
  próprio helper cita 3 testes que quebram sem o guard). Go e Node não precisam desse guard porque já
  leem `$HOME` nativamente em POSIX. Os três chegam ao mesmo comportamento final no Windows, por
  caminhos de código diferentes — não é uma divergência de correção, é o guard de plataforma correto
  aplicado só onde a plataforma exige.

## Premissas do handoff que esta medição derrubou ou confirmou

- **Item 4 "fechado no fim de 2026-08 e reaberto"**: o handoff não afirmou isso, mas o roadmap de
  31/08 declarou item 4 fora de escopo e a sonda continua REPRODUCED — uma leitura rápida concluiria
  "ainda aberto". **Derrubado por leitura direta do `.sh` atual e execução local**: está corrigido
  desde 2026-09-02, um ML posterior ao roadmap que o excluiu. A sonda é que ficou para trás.
- **21/25/28 sítios de produção**: Node é **23**, não 25. Diferença pequena, não muda veredito.
- **A conferência por amostragem do arquiteto (homedir/, isatty removido, gates existem)**: confirmada
  como indício correto, e a verificação de uso real (não só existência) também fechou positivo nos
  três runtimes — zero call site cru fora dos helpers, medido por grep, não por nome de arquivo.
- **"Cruzar as 100 falhas residuais com os 7 itens abertos"**: cruzado — nenhuma das 100 é
  regressão de nenhum dos 7. O único item genuinamente aberto (dedup `//`) já estava fora da contagem
  de 100 do resíduo geral porque pertence a um grupo (G12) rastreado separadamente na re-triagem.
