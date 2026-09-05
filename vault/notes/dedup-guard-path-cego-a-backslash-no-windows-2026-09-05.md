---
title: normalizeGuardPath colapsa "//" mas nunca canoniza "\" — dedup falha no Windows por separador misto, não pela barra dupla
date: 2026-09-05
---

# `normalizeGuardPath` (Go/Node/Python) é cego a `\` — o nome do defeito engana

**Sintoma:** `TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand` (Go) e os
pares Node/Python (grupo **G12** da re-triagem por mecanismo) falham no Windows, passam em POSIX.

**Causa raiz, medida (não hipótese):** `normalizeGuardPath`/`_normalize_guard_path`
(`internal/generators/agentfiles.go:1621`, `npm/src/generators/hooks.js:1295`,
`pypi/trackfw/generators/hooks.py:113`, os três byte-a-byte idênticos) só colapsa runs do caractere
`/`; nunca converte `\`→`/` nem o inverso. No Windows, `filepath.Join`/`path.win32.join`/`ntpath.join`
sempre emitem o separador **nativo** (`\`) em cada fronteira de segmento — confirmado por leitura do
fonte Go instalado (`internal/filepathlite/path.go:65-138`, `out.append(Separator)` + `FromSlash()`
final) e por **execução real** de `path.win32.join` e `ntpath.join` nesta máquina macOS (são módulos
lexicais puros, cross-platform por desenho — não precisam de Windows real para rodar). O comando
armazenado num config hand-edited (ou fixture) que usa `/` continua com `/` depois do colapso; o
comparando computado via `Join` fica todo `\`. As duas strings normalizadas nunca batem no Windows.

**Discriminante que separa isto de "JSON malformado" (G3) ou "resolução de $HOME" (item 2/6, já
corrigido):** o teste **passa em POSIX** (medido, `go test ... PASS` nesta máquina) — se a causa fosse
parsing de JSON ou `$HOME` vs `%USERPROFILE%`, falharia igual em qualquer SO. Só falha onde o
separador nativo diverge do separador do fixture.

**Produto, não teste — e o gatilho mais forte não é hipotético.** O comentário de
`normalizeGuardPath` promete tolerar dois cenários: "`$HOME` resolving with a trailing slash" e
"hand-edited config file". Medido: **nenhum dos dois é coberto no Windows.** O primeiro produz `\\`
duplicado na costura (`home\` + `\.trackfw\...`) e a função só testa `r == '/'`, nunca `r == '\\'` —
não colapsa, demonstrado com a mesma cópia literal do algoritmo. Esse gatilho **não depende de edição
manual** — qualquer perfil Windows que resolva com separador final dispara. O segundo (hand-edited com
`/`) segue plausível (JSON com `\` exige escapar `\\`) mas não observado. Hoje nenhum sítio de
produção escreve o comando global por concatenação crua (todos usam `Join`) — o defeito só se
manifesta com um gatilho externo, mas ao menos um dos dois gatilhos citados pela própria doc-comment é
inteiramente plausível sem intervenção humana.

**Discriminante de contagem, gratuito:** só a variante `//` (concatenação crua) do teste de dedup
falha; as variantes-irmãs (`...SkipsProjectEntryWhenGlobalInstalled`, mesma home isolada, mas os dois
lados da comparação vêm de `Join`) passam nos 3 runtimes. Se a causa fosse `$HOME`/`%USERPROFILE%`
quebrado, todas as variantes-irmãs falhariam também — G12 teria ~15 falhas, não 3. Isso isola o
separador como único fator, sem depender de confiar na leitura do código de resolução de home —
$HOME foi verificado nos 3 runtimes mesmo assim (`npm/src/homedir.js`, `pypi/trackfw/homedir.py`
preferem `$HOME` no Windows, igual ao Go).

**Armadilha para quem for medir isto:** `path.win32`/`ntpath` são módulos que rodam em qualquer SO —
dá para reproduzir o comportamento exato do `Join` no Windows a partir de macOS/Linux sem precisar de
runner Windows, só chamando a variante explícita (`path.win32.join`, `ntpath.join`). Go não tem uma
variante explícita equivalente compilável sem `GOOS=windows`, mas o algoritmo de `Clean`/`Join` é puro
(lê `internal/filepathlite/path.go` do SDK instalado) — dá para confirmar por leitura de fonte +
reprodução da string-algoritmo isolada (cópia literal de `normalizeGuardPath`, sem chamadas de SO).

**Cuidado para quem for corrigir:** a correção óbvia (`strings.ReplaceAll(p, "\\", "/")` antes do
colapso) funciona no caso feliz mas **destrói o prefixo UNC** — `\\server\share\...` vira
`//server/share/...`, que o colapso de barra dupla já existente reduz para `/server/share/...`,
perdendo a marca de duas barras que distingue UNC de caminho relativo. Não há sítio hoje que grave
UNC nesse fluxo, mas qualquer implementação real precisa tratar isso como caso especial — mesmo tipo
de cuidado que `ML-3B` (Wave 3 do roadmap de Windows) já teve que aplicar num predicado vizinho.

Relatório completo: `docs/portabilidade/2026-09-05-mecanismo-do-dedup-barra-dupla.md`.
