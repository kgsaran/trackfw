# credential-guard: extração de comando na segunda camada de detecção usa o campo JSON "command", não o primeiro token de $RAW — 2026-08-08

## Contexto

ML-1A (ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md)
implementou a segunda camada de detecção de `credentialGuardDetectionCore`
(`internal/generators/scaffold.go`), item (b): quando o payload cru não contém o padrão (ex.:
`head -c 50 /tmp/token.txt`), o script deveria inspecionar o conteúdo de arquivos passados como
argumento a `cat`/`head`/`tail`/`jq`/`grep`.

O texto do roadmap/ADR (ADR-2026-08-06 emenda 8) descreve isso como "extrair o primeiro token
não-vazio de RAW (nome do comando)". Isso é **literalmente inexecutável**: `$RAW` no script é o
payload JSON inteiro do hook, ex.:
`{"tool_name":"Bash","tool_input":{"command":"head -c 50 /tmp/x"}}` — o primeiro token
word-splitted por espaço é o prefixo JSON inteiro até "head" (`{"tool_name":"Bash",...,"command":"head`),
não `head`.

## Decisão

A extração do nome do comando usa o campo JSON `"command"` diretamente, via sed:

```sh
CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
```

Isso captura o valor do campo até a primeira aspa não escapada (`[^"]*` não cruza `"`). Suficiente
para o payload plano típico de hook (mesmo espírito do resto do script — sem parser JSON completo,
mesmo padrão de `REDIRECTS`/`is_ephemeral_target`). Limitação conhecida e aceita: um comando com
aspas internas no argumento (ex.: `echo JWT > "$TMPFILE"`) trunca a captura na primeira aspa —
não é um problema na prática porque esse caso já é coberto pela camada 1 (payload cru), que roda
antes e teria encontrado o match diretamente no `$INPUT`.

## Por que importa para ML-1B/ML-1C

Node (`npm/src/generators/hooks.js`, `CG_DETECTION_CORE`) e Python
(`pypi/trackfw/generators/init_gen.py`, `_CG_DETECTION_CORE`) devem portar **esta mesma extração
via campo JSON**, não uma tradução literal de "primeiro token de RAW" — senão a paridade
byte-a-byte entre os 3 stacks (testada em `TestCredentialGuardScript_ParityAcrossStacks` e
`TestGlobalCredentialGuardScript_ParityAcrossStacks`) fica impossível de fechar, e o comportamento
funcional dos 3 CLIs diverge de verdade (não é só string diferente — um deles vai realmente falhar
em detectar `head -c 50 arquivo.txt`).

## Risco remanescente aceito, não corrigido neste ML

Default block + a nova camada de leitura de conteúdo de arquivo significa que rodar
`cat`/`grep`/`head` sobre **qualquer arquivo que contenha um JWT sintético de teste** agora
bloqueia (exit 2) quando o hook global está instalado — por exemplo,
`internal/generators/credential_guard_test.go` no próprio repo contém `syntheticJWT` como
constante Go. O ADR (emenda 8) só previu o teto de tamanho (1MB) como guarda de custo, não uma
exceção para fixtures de teste — não expandido aqui por estar fora do escopo do ML-1A. Se isso
gerar fricção real (ex.: um dev com o hook global instalado rodando `cat` neste arquivo de teste),
é candidato a REQ futura, não fix silencioso.

## Referências

- `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md` (itens 6 e 8, emenda 2026-08-08)
- `docs/roadmaps/wip/ROADMAP-2026-08-08-credential-guard-modo-block-por-padrao-cobertura-de-read-write-e-resolucao-de-arquivo-referenciado.md` (ML-1A)
- `internal/generators/scaffold.go` (`credentialGuardDetectionCore`, `credentialGuardModeResolution`)
- `internal/generators/credential_guard_test.go`
