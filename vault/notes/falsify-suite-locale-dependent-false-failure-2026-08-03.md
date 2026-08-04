# `scripts/check-gates-falsify.sh` reprova em máquina com `LANG` não-inglês — Cenário 29 (e potencialmente outros com mensagem pinada) é sensível ao ambiente, não ao código

> Data: 2026-08-03 | Autor: Ártemis (ML-3A, ROADMAP-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis) | Domínio: gates / i18n / testes

## O achado

Rodando `bash scripts/check-gates-falsify.sh` numa máquina com `LANG=pt_BR.UTF-8`
(sem `LANG=C`/`en_US` explícito), o script aborta em `set -euo pipefail` no
**Cenário 29** (`validate-ok-message/baseline-byte-identical-and-pinned`):

```
FAIL [falsify/validate-ok-message/baseline-byte-identical-and-pinned]: esperava '✓ No violations found.
' nos 3 CLIs
  go:     $'✓ Nenhuma violação encontrada.\n'
  node:   $'✓ Nenhuma violação encontrada.\n'
  python: $'✓ Nenhuma violação encontrada.\n'
```

Não é regressão de código. É o gate: `S29_EXPECTED` (linha ~1947 do script) fixa o
literal em inglês (`"✓ No violations found."`), mas nenhum dos três CLIs invocados
pelo cenário recebe `LANG=`/`LC_ALL=` explícito — herdam o ambiente do shell. Desde
que o projeto ganhou i18n (`internal/i18n/i18n.go`: `DetectLocale()` lê
`LANG`/`LC_ALL`/`LANGUAGE`, mapeia `pt_*` → `pt-BR`, `es_*` → `es-ES`, e só cai em
`en-US` se nenhuma dessas variáveis contiver um desses prefixos), a mensagem `ok` de
`validate` é lida de `internal/i18n/locales/<locale>.json` — em `pt-BR.json` ela é
"Nenhuma violação encontrada.", não a string pinada do gate.

## Como foi confirmado

```
$ echo $LANG
pt_BR.UTF-8
$ bash scripts/check-gates-falsify.sh          # aborta no Cenário 29
$ env -u LANG -u LC_ALL -u LANGUAGE bash scripts/check-gates-falsify.sh   # 92/92 OK
```

Rodar com as três variáveis de locale explicitamente ausentes faz `DetectLocale()`
cair no fallback `en-US` — e a suíte inteira (92 cenários, na revisão pré-ML-3A)
passa limpa.

## Por que isso NÃO é escopo do ML-3A

O Cenário 29 testa a mensagem de sucesso do `validate` (`internal/validator` +
i18n), não os scanners de `update`/`sync` que este ML elimina. Não toquei nele.
Confirmado que é pré-existente à minha mudança (reproduzido antes de qualquer edição
no arquivo, ver protocolo de baseline em
`vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`).

## Como reconhecer / evitar no futuro

- **Sintoma**: `check-gates-falsify.sh` reprova especificamente no Cenário 29 (ou
  qualquer cenário futuro que fixe uma mensagem pinada de i18n) com o texto
  "esperado" em inglês e o "obtido" em outro idioma, byte-idêntico nos 3 CLIs — não
  é divergência entre runtimes, é o ambiente do executor.
- **Diagnóstico rápido**: `echo $LANG` — se não for `C`/vazio/`en_US*`, rode com
  `env -u LANG -u LC_ALL -u LANGUAGE bash scripts/check-gates-falsify.sh` para
  confirmar a hipótese antes de investigar código.
- **Correção correta (fora deste ML)**: o gate deveria fixar `LANG=C` (ou
  `LC_ALL=C`) explicitamente nas invocações que comparam contra uma mensagem pinada
  em inglês — mesmo padrão de determinismo de ambiente já aplicado em outros
  cenários deste arquivo (ver a "Armadilha relacionada — corrupção não
  determinística" em
  `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`).
  Não fiz essa correção aqui — não é o ML-3A e alterar a asserção do Cenário 29
  está fora do "arquivo compartilhado, exclusividade total" que este ML tinha sobre
  `check-gates-falsify.sh`, mas o fix é mecânico (uma linha `env LANG=C` nos comandos
  de validate do Cenário 29) para quem pegar essa dívida.
- Todos os cenários que adicionei neste ML (39/40/41, `update-config-loader/*`)
  usam mensagens de `updateHooksSurgical`/`_update_hooks_surgical`
  ("trackfw-validate injetado"/"trackfw validate injetado") que **não** têm
  variante i18n hoje (grep confirma: essas strings não estão em
  `internal/i18n/locales/*.json`) — não são afetadas por este problema, mas
  qualquer ML futuro que mova essas mensagens para o sistema de i18n deveria
  revisitar este achado.

Relacionado: `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`.

## Resolvido em 2026-08-04

Correção aplicada (não `LANG=C` como sugerido acima, mas `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`
explícito — decisão registrada em
`docs/adr/ADR-2026-08-04-make-quality-forca-locale-fixo-no-gate-de-falsificacao-em-vez-de-pin-em-ingles.md`,
via `docs/req/REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md`
e `docs/roadmaps/wip/ROADMAP-2026-08-04-make-quality-locale-fixo-no-falsify.md`, ML-1A) em
`scripts/check-gates-falsify.sh` Cenário 29: as 4 chamadas que capturam `s29_go_out`, `s29_node_out`,
`s29_python_out` e `s29c_python_out` agora compõem `env LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 ...` (e o
`PYTHONPATH=` já existente é combinado no mesmo `env`, seguindo o padrão do restante do script).

**Auditoria dos demais cenários pinados (30/31/33/34/35/36) confirmou que NÃO precisam da mesma
correção**: `trackfw status` (Go/Node/Python) não passa nenhuma das suas strings de saída
("Inventory", "WIP", "Blocked", "Done" etc.) por `i18n_t`/`i18n.T` — são literais hardcoded
idênticos nos 3 CLIs independente de locale (confirmado por grep de `i18n` nos 3 `status.go` /
`status.js` / `status.py`; o único uso de i18n em `status.js` é `t('status.description')`, que é a
descrição do `--help`, não capturada por nenhum desses cenários). Também não há outro `_EXPECTED=`
no script que pine uma das outras mensagens i18n existentes (`validate.violations`,
`validate.warnings`, `validate.lenient_mode`) — só `validate.ok` (Cenário 29) é exercitado.

Confirmado empiricamente: reprodução do bug ANTES da correção sob `LANG=pt_BR.UTF-8` reproduziu
exatamente a falha documentada acima; após a correção, `make quality` passou 99/99 cenários sob
`LANG=pt_BR.UTF-8` e sob `LANG=en_US.UTF-8`, incluindo a prova de detecção de regressão do Python
(`s29c_python_out` reintroduzindo `"✓ Governance OK"` hardcoded continua reprovando corretamente nos
dois locales) — isso descarta empiricamente sensibilidade a locale em qualquer outro dos 99 cenários,
não só nos citados acima.
