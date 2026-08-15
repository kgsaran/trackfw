---
status: Superseded
date: 2026-08-15
author: "Zeus (Arquiteto)"
---

# ADR: Gate de plugins binários — detecção de adulteração sem detecção de instalação, e `chmod` após aprovação

> Date: 2026-08-15 | Status: **Superseded**
> **Superseded por:** `docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md`
>
> Este ADR decidiu **cercar** o download de binário de terceiro com um gate de duas fases. KG propôs
> a alternativa que não estava na mesa — **remover o subsistema** — e ela ganha: o gate exigiria uma
> superfície nova inteira para entregar garantia parcial (o ramo (i) da detecção é impossível com
> escopo global, o `chmod` tardio é redução de janela e não controle, e o revisor não lê binário).
> **O conteúdo abaixo permanece como registro do porquê:** foi a análise deste ADR, e o parecer que
> o embasou, que tornaram a remoção obviamente preferível. O débito D9 (fallback de argumento
> desconhecido executando plugin) é **fechado** pela remoção, não mais adiado.

REQ: `docs/req/REQ-2026-08-15-gate-de-seguranca-para-trackfw-plugins-install-download-de-binario-de-terceiro-sem-parecer-previo.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-gate-de-seguranca-para-trackfw-plugins-add-binario-de-terceiro.md`
Parecer (ML-0A, `hades-tf`): `docs/seguranca/2026-08-15-gate-de-plugins-binario.md`
ADR do gate markdown, cujo padrão é reusado: `docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-...md`

## Context

`trackfw plugins add` baixa um **binário** de terceiro do GitHub Releases e o torna executável, sem
quarentena, sem parecer e sem proveniência. O débito foi criado conscientemente pelo D8(e) do ADR do
gate markdown, que classificou este vetor como de **severidade maior**: markdown influencia, binário
executa.

Este ADR estende o handshake de duas fases ao caminho binário — **e declara, sem contornar, o que
esse handshake não consegue entregar aqui.**

Três fatos verificados no código condicionam tudo abaixo:

1. **`~/.trackfw/plugins` é por-máquina, não por-projeto.** `plugins.Dir()`
   (`internal/plugins/plugins.go:146`) e `pluginsDir()` (`npm/src/commands/plugins.js:61`) sempre
   resolvem para o home. Não há `--scope`.
2. **O binário nasce executável antes de qualquer aprovação.** Go faz `os.Chmod(tmpPath, 0755)`
   **antes** do rename (`plugins.go:246`); Node grava com `mode: 0o755`
   (`plugins.js:108`).
3. 🔴 **Qualquer subcomando desconhecido vira execução de plugin.** `internal/commands/root.go:71-74`:
   `root.RunE` chama `RunPlugin(args[0], args[1:])` para qualquer primeiro argumento não reconhecido.
   Um erro de digitação (`trackfw vaildate`) executa `trackfw-vaildate` se ele existir. Verificado
   por leitura direta. Isto **amplifica** o impacto de um plugin malicioso e não estava no radar da REQ.

## Decision

### D1 — Handshake de duas fases, reusando `internal/thirdparty/` onde couber

`plugins add` **para em quarentena**; um segundo comando consuma a instalação exigindo aprovação
vinculada por checksum (`VerifyApproval`, o mesmo mecanismo que fecha o TOCTOU no gate markdown).

### D2 — 🔴 Detecção: **só o ramo (ii)**. O ramo (i) é declarado **estruturalmente ausente**

Esta é a decisão mais importante deste ADR, e ela **corrige o AC6 da REQ**, que era inimplementável
como escrito.

- **Ramo (ii) — adulteração pós-aprovação: EXISTE.** Um índice versionado **no projeto**, chaveado
  por nome de plugin e guardando o checksum aprovado, permite detectar que o binário em
  `~/.trackfw/plugins/<nome>` **mudou** desde a aprovação.
- **Ramo (i) — instalação sem aprovação: NÃO EXISTE, por desenho.** `~/.trackfw/plugins` é
  compartilhado entre **todos** os projetos da máquina. Um plugin presente no home pode ter sido
  legitimamente aprovado em outro projeto, ou instalado fora do trackfw. Um índice por-projeto não
  tem como distinguir esses casos de uma instalação clandestina — e uma regra que tentasse
  dispararia **falso-positivo entre projetos não relacionados**, o que a tornaria ruído e, na
  prática, desligada.

**Consequência aceita e declarada:** para plugins, o gate entrega a barreira do **momento da
instalação** (nada instala sem parecer) e a detecção de **adulteração posterior** — mas **não** a
detecção de "apareceu um plugin que ninguém aprovou". Isso é diferente do que o gate markdown
entrega, e a diferença é do escopo global, não de esforço.

**A alternativa que resolveria — mover plugins para escopo de projeto — foi rejeitada por KG**
(quebraria quem já usa). Registrada aqui como o caminho a reconsiderar se a ausência do ramo (i)
se mostrar cara.

### D3 — `chmod 0755` somente após aprovação; quarentena em `0600`

O binário em quarentena não recebe bit de execução. **O que isso compra, honestamente:** fecha o
caminho de execução padrão durante a janela entre download e aprovação. **O que não compra:** nada
contra um agente com `Bash`, que dá `chmod +x` sozinho, nem contra payload que não precisa de bit
de execução para ser interpretado. É redução de janela, não controle.

### D4 — Artefato de revisão para binário diverge de D8(b) do gate markdown

O revisor **não consegue ler um binário** como lê markdown. Portanto:

- **Bytes brutos em arquivo próprio + JSON sidecar**, não `content_base64` embutido — 50 MiB em
  base64 dentro de JSON é inviável (o gate markdown embutia porque o teto é 2 MiB e o conteúdo é texto).
- O sidecar carrega, no mínimo: **repo resolvido**, **string original digitada pelo usuário**, **URL
  de asset**, **tag efetivamente resolvida** (`latest` resolve para algo — registrar o quê),
  tamanho, checksum SHA-256, plataforma (`GOOS/GOARCH`) e resultado de uma checagem de **assinatura
  de arquivo** (magic bytes: é mesmo um executável da plataforma declarada?).
- **O parecer certifica proveniência aceita, nunca ausência de malícia.** Nenhuma doc ou mensagem
  pode sugerir que o parecer atesta que o binário é seguro.

### D5 — Registro do alvo resolvido **e** da string digitada

`ResolveRepo` (`plugins.go:130`) traduz um nome puro consultando o registry **não-pinado**
(`RegistryURL`, branch `main` de repo externo, parser YAML caseiro). A quarentena registra **os
dois**: o que o usuário digitou e o que aquilo virou. É isso que sustenta manter a confiança no
registry fora de escopo — o gate revisa o **artefato resolvido**, então um comprometimento do
registry aparece na revisão em vez de passar invisível.

### D6 — Limite de origem: gate de **revisão**, não de **supply-chain** (decisão de KG)

Sem checksum publicado pelo autor, sem assinatura, sem pinagem de release. O checksum garante que
**o binário instalado é o que foi revisado** — e **não** que o autor publicou aquilo. Nenhum texto
pode apresentar isso como verificação de origem. O `hades-tf` recomendou reavaliar em REQ futura.

### D7 — Escopo da correção do Node

- **Nesta REQ:** o teto de tamanho aplicado **por stream** em vez de `res.arrayBuffer()` seguido de
  checagem (`plugins.js:103-104`) — AC3 obriga.
- **Nesta REQ, e bloqueante para provar AC3:** os **primeiros testes de plugins do Node** (hoje há
  **zero**).
- **REQ separada:** `fetchRegistry()` sem checagem de status, sem timeout e sem teto
  (`plugins.js:13-21`) — é o caminho de *busca*, não o de instalação.

### D8 — Python é exceção intencional documentada

Não há caminho de instalação em Python (`list` e `run` apenas). O **gate** vale para Go e Node; a
**regra de detecção** do ramo (ii) vai nos **3 CLIs**. Documentar em `docs/cli-parity.md` — incluindo
que a superfície de *execução* do Python é **mais ampla** (varre o `$PATH` inteiro, não só
`~/.trackfw/plugins`).

### D9 — O fallback de subcomando desconhecido fica registrado, e fora desta REQ

`root.go:71-74` faz qualquer argumento não reconhecido virar execução de plugin. Um typo executa um
binário. Isso **amplifica** o impacto de um plugin malicioso e não é resolvido por nenhuma decisão
acima. **Fora de escopo desta REQ** — mudar o comportamento de fallback é mudança de UX do CLI
inteiro. **Registrado aqui para virar REQ própria**, no mesmo espírito do D8(e) que gerou esta.

## Consequences

**Positivas**
- Nada instala sem parecer prévio, também no caminho binário.
- A janela entre download e aprovação deixa de produzir um executável.
- Adulteração pós-aprovação é detectável.
- O alvo resolvido fica auditável, tornando honesta a exclusão do registry do escopo.

**Negativas / riscos aceitos**
- **Não há detecção de instalação clandestina de plugin** (D2). É a consequência direta de manter o
  escopo global.
- O gate é de **revisão**, não de **supply-chain** (D6).
- O `chmod` tardio é redução de janela, não controle (D3).
- `root.go` continua transformando typo em execução de plugin (D9) — débito datado.
