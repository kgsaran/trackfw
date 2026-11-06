# Wave 0 — Threat Model: Guard de Credencial — Origem do Achado e Leitura Recursiva

> Hades (hades-tf) · 2026-11-06  
> Roadmap: `ROADMAP-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md`  
> REQ: `REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md`  
> ADR: `ADR-2026-11-06-guard-de-credencial-diz-de-onde-veio-o-achado-e-nao-le-diretorio-recursivamente.md`  
> Binário medido: trackfw 11.5.0

Medições desta sessão utilizam payload `{"tool_name":"Bash","tool_input":{"command":"<cmd>"},"tool_info":{"cwd":"<cwd>"}}` enviado por `python3 -I` ao guard. O campo que contém o cwd é `tool_info.cwd`, não `cwd` de nível raiz (o guard ignora este último).

---

## 1. Completude da Enumeração

A lista de superfícies no roadmap cobre três decisões (D1, D2, D3). Esta seção inventaria cada caminho que chega a `credScanFile` e cada ponto de emissão de mensagem, para verificar que nenhuma superfície foi omitida.

### 1.1 Caminhos que chegam a `credScanFile` (onde D1 precisa agir)

| Número | Caminho | Função chamadora | Escopo | Medido (rc=2 = bloqueado) |
|---|---|---|---|---|
| L2a | Alvo de redirecionamento não efêmero | `credSecondLayer` L726–755 | Projeto + Global | ✓ `echo x > fixture.txt` onde fixture tem JWT → rc=2 (medido; `echo` não está na allowlist de argv0, confirmando que apenas 2a pode ter disparado) |
| L2a-ADS | Fallback NTFS ADS (Windows apenas) | `credSecondLayer` L746–755 | Windows | Não medido nesta plataforma |
| L2b | Argumento de arquivo de cat/head/tail/jq/grep | `credSecondLayer` L758–784 | Projeto + Global | ✓ `cat fixture.txt` → rc=2 |
| L2b-nonJSON | credNonJSONLayerTwoB via credCmdLineRe | `credNonJSONLayerTwoB` L1000–1013 | Projeto + Global | ✓ `printf '{"command":"cat <fixture>",}' \| guard --global` (JSON inválido por vírgula) → rc=2 |
| L4-deep | credDeepScan (schema não-padrão: Cursor, Copilot) | `credDeepScan` L977 | Projeto + Global | ✓ `{"tool_name":"Bash","params":{"command":"cat <fixture>"}}` → rc=2 |

**Achado E1 — Cobertura do D1 deve abranger todos os cinco caminhos.** Um implementador que testa apenas L2b em modo block/global deixa quatro caminhos sem a nova mensagem. AC1 deve listar cada caminho explicitamente.

### 1.2 Pontos de emissão da mensagem (onde a string com o caminho vai parar)

| Ponto | Função | Modo | Destino |
|---|---|---|---|
| Stderr block (projeto) | `RunCredential` L236 | block | stderr |
| Stderr warn (projeto) | `RunCredential` L240 | warn | stderr |
| Stderr block (global) | `RunCredentialGlobal` L332 | block | stderr |
| Stderr warn (global) | `RunCredentialGlobal` L336 | warn | stderr |
| Arquivo de atenção | `credWriteAttention` L1186 | warn (ambos escopos) | `<roadmap_dir>/.trackfw-credential-guard.json` |

### 1.3 Cobertura atual por arquivo (census AC3)

Census executado com payload `{"tool_name":"Bash","tool_input":{"command":"cat <arquivo>"},"tool_info":{"cwd":"<repo>"}}` para cada arquivo em `internal/guard/*.go` (11 arquivos) e `internal/generators/*_test.go` (77 arquivos), binário 11.5.0:

| Arquivo | Resultado | Tipo detectado |
|---|---|---|
| `internal/guard/credential_patterns_test.go` | **MATCH** | PEM private key (último padrão da lista — confirma ausência de matches anteriores) |
| `internal/generators/credential_guard_test.go` | **MATCH** | JWT (primeiro padrão — pode ocultar outros matches no mesmo arquivo) |
| Todos os outros (87 arquivos) | PASS | — |

Dois arquivos com literais, ambos escopo de ML-1B.

**Achado E2 — `.trackfw-credential-guard.json` está no `.gitignore` DESTE repositório (``.gitignore:31`), mas `trackfw init` NÃO grava esta entrada para projetos consumidores.** Medição: `trackfw init` em diretório temporário gerou `.gitignore` com três blocos (branch-links, leases, agent-memory) sem qualquer menção a `credential-guard.json`. Em projetos consumidores com `mode: warn`, o arquivo de atenção escrito em `docs/roadmaps/.trackfw-credential-guard.json` é commitável. Isso transforma R4 (path leakage efêmero) em um commit persistente do caminho do arquivo com credencial. Ver R4 revisado.

**Achado E3 — `credential_test.go` atualmente PASSA o census.** ML-1A adiciona testes a este arquivo. Novos testes com literais de token fariam o arquivo falhar. Os novos testes de D1 em `credential_test.go` devem montar tokens em tempo de execução (mesma regra de D3).

---

## 2. Modelo de Ameaça

O adversário aqui é o implementador apressado e o arquiteto otimista. As três decisões do ADR criam superfícies novas que podem ser implementadas de forma incompleta ou insegura.

### 2.1 D1 — O caminho no stderr e no sinal de atenção

**Superfície nova:** após D1, um caminho de arquivo aparece em dois lugares onde hoje só aparece o nome do tipo de credencial. Hoje nenhum caminho chega a stderr ou ao arquivo de atenção; todos os seis riscos abaixo são criados por D1.

| Risco | Existe hoje | Criado por D1 |
|---|---|---|
| Controle chars (ESC, DEL, C1, bidi) no caminho | Não | Sim |
| `\n` no nome do arquivo quebra linha do stderr | Não | Sim |
| Workflow commands (`::error::`) no log do CI | Não | Sim |
| JSON malformation no arquivo de atenção | Não | Sim |
| Sensitive path leakage efêmero (stderr) | Não | Sim |
| Sensitive path leakage persistente (arquivo de atenção em consumer sem gitignore) | Não | Sim |

**Ameaça 1 — Controle chars e ANSI no caminho (MÉDIA)**

O caminho tem duas origens de natureza distinta:

a) *Tokens da string do comando* (`strings.Fields(shellCmd)[1:]`): controlados pelo agente/usuário.

b) *Nomes de arquivo retornados por `filepath.Glob`* (L2b): controlados pelo **sistema de arquivos**. Quem controla o conteúdo de um diretório clonado ou de um arquivo extraído pode criar arquivos com ESC, DEL, C1, bidi controls no nome. O `filepath.Glob` retorna os nomes literais do SO. Esses nomes sobrevivem ao `strings.Fields` (não são whitespace), chegam a `credScanFile` como path e, se o arquivo tiver credencial, chegam ao novo ponto de impressão de D1.

**Achado T1 — O ADR afirma "o caminho vem do comando e pode ser forjado".** Esta premissa é incompleta: para L2b com glob, o caminho vem de `filepath.Glob` (nomes do SO, não do comando). A sanitização é igualmente necessária, mas a fonte de risco é diferente. O ADR deve ser corrigido.

**Medição:** arquivo com nome `a\x1b[31mred\x1b[0m.txt` criado em diretório temporário com conteúdo JWT montado em tempo de execução. Payload `cat $DIR/*.txt` → guard bloqueia (rc=2, confirmado). O glob expande para incluir o arquivo com ESC no nome.

**Achado T2 — O ADR diz "caractere de controle vira `?`".** `credWriteAttention` atualmente **descarta** chars < 0x20 (`continue` no loop), não os substitui por `?`. Há discrepância entre ADR e implementação. ML-1A deve alinhar: usar substituição por `?`.

**Ameaça 2 — `\n` no nome do arquivo quebra a linha do stderr (ALTA)**

Um arquivo com `\n` no nome pode ser criado no macOS/Linux. `\n` é < 0x20 e é whitespace, então `strings.Fields` o trataria como separador de token — mas o `\n` está no NOME DO ARQUIVO retornado por `filepath.Glob`, não no token do comando. O glob retorna o nome literal com `\n`, e esse nome chega a `credScanFile` e, via D1, ao ponto de impressão.

Se D1 imprimir o nome sem sanitização, o stderr teria:
```
trackfw-credential-guard: blocked - possible JWT detected in file /path/a
::error::x.txt referenced by the command
```

A segunda linha começa com `::error::`, que o GitHub Actions interpreta como anotação de erro. Isso não é um risco de arquivo com `::`  no meio — é a quebra de linha pelo `\n` que posiciona `::error::` no início da segunda linha do stderr.

**Medição:** arquivo com nome `a\n::error::x.txt` criado (`testlVt5jX/`). Payload `cat $DIR/*.txt` → rc=2 (guard bloqueia). Se D1 imprimisse sem sanitização, a segunda linha começaria com `::error::x.txt`.

**Sanitização exata exigida por ML-1A:**
- Substituir por `?` (não descartar): `r < 0x20` (control C0 — inclui `\n` 0x0A, `\r` 0x0D, ESC 0x1B), DEL (0x7F), C1 (0x80–0x9F, cobre CSI 0x9B), bidi controls (`unicode.Is(unicode.Bidi_Control, r)` — inclui U+200E, U+200F, U+061C, U+202A–U+202E, U+2066–U+2069), UTF-8 inválido → `?`.
- Especificar por propriedade: `unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)` ou equivalente.
- Adicionar teste unitário com `"\x9b"` (C1 raw byte) direto no sanitizador (APFS rejeita esse nome de arquivo, então só por teste unitário do sanitizador).
- Aplicar **uma vez, antes** de o caminho ser embutido em qualquer mensagem (stderr e atenção JSON). Sem `filepath.Abs` ou `filepath.EvalSymlinks`.
- Resultado: cada linha do stderr produzida pelo guard começa com `trackfw-credential-guard:`, mesmo com filename malicioso.

**Ameaça 3 — JSON malformation no arquivo de atenção (BAIXA)**

`credWriteAttention` constrói JSON manualmente. O loop de escaping já trata `"` → `\"` e `\` → `\\`. Usando `range` sobre string, inválido UTF-8 → U+FFFD (benign). Se o caminho for embutido em `msg` antes do loop, o JSON resultante é válido.

**Pré-condição obrigatória para ML-1A:** o caminho deve ser embutido em `msg` **antes** do loop de escaping, não depois. Senão, `"` no caminho quebra o JSON. Adicionar um teste de round-trip: `json.Unmarshal` do arquivo escrito deve produzir o campo `message` sem erro.

**Ameaça 4 — Invariante "impresso == aberto" (MÉDIA)**

Se D1 imprimir o token original (antes da limpeza do `credScanFile`) e o arquivo lido for diferente (porque `credScanFile` remove aspas e `},`), o diagnóstico é enganoso.

**Exigência para ML-1A:** imprimir exatamente o caminho que `credScanFile` passou para `os.Stat`, após a limpeza interna (strip de aspas e `},`). Sem `filepath.Abs` ou `filepath.EvalSymlinks`.

**Ameaça 5 — Cobertura parcial de call sites (BLOQUEANTE se não coberto)**

Se ML-1A aplicar D1 apenas em L2b/global/block, os outros quatro caminhos continuam dizendo "in tool payload" para achados de arquivo.

**Exigência:** o diff de ML-1A deve alterar todos os cinco caminhos de chegada (1.1) e os cinco pontos de emissão (1.2) de forma consistente.

**Ameaça 6 — F2 (exemption) recebe tipo errado após refactor (BLOQUEANTE se não coberto)**

O ML-1A refatora `credSecondLayer` (ou adiciona wrapper) para retornar tanto o nome do tipo quanto o caminho. Se a refatoração passar o caminho onde `credCmdHasTypeMatch` espera o nome do tipo, `echo <token> > /dev/null` para de disparar o bloqueio.

**Exigência:** `match` (tipo de credencial) permanece separado de `filePath` em toda a cadeia.

### 2.2 D2 — O que a segunda camada não cobre

A segunda camada não lê diretório recursivamente. A limitação já existe; D2 apenas a documenta. O risco não é novo. As causas de não-cobertura são distintas e devem ser documentadas separadamente:

**Medições (rc=0 = passa, sem bloqueio; rc=2 = bloqueado):**

| Comando | rc | Causa |
|---|---|---|
| `grep -r dir/` | 0 | D2 propriamente dito: diretório não percorrido |
| `grep -R dir/` | 0 | D2 propriamente dito |
| `rg foo dir/` | 0 | argv0 `rg` não está na allowlist |
| `rg foo file.txt` (arquivo nomeado) | 0 | argv0 `rg` não está na allowlist — **mesmo com arquivo explícito** |
| `/bin/cat file.txt` | 0 | argv0 resolve para `/bin/cat`, não `cat` |
| `true && cat file.txt` | 0 | tokens[0] é `true`, não `cat` (posição) |
| `sed -n 1p file.txt` | 0 | argv0 não está na allowlist |
| `awk '{print}' file.txt` | 0 | argv0 não está na allowlist |
| `python3 -c "open('f')"` | 0 | argv0 não está na allowlist |
| `find dir/ -exec cat {} \;` | 0 | argv0 não está na allowlist |
| `tar czf out.tgz dir/` | 0 | argv0 não está na allowlist |
| `zip out.zip file.txt` | 0 | argv0 não está na allowlist |
| `xargs cat` (stdin com filenames) | 0 | argv0 não está na allowlist |
| `cp src dst && cat dst` | 0 | tokens[0] é `cp` (posição) |
| `cat $(ls dir/*)` | 0 | `$()` é metacaractere de shell; `credHasShellMeta` retorna true → command não é simples, mas o scan ocorre em tokens[] — tokens[1] é `$(ls`, que não é arquivo existente |
| `cat $DIR/**/*.txt` (arquivo 2 níveis abaixo) | 0 | `filepath.Glob("**/*.txt")` trata `**` como componente literal; arquivo não é encontrado |
| `cat $DIR/sub/dir/*.txt` (glob 1 nível correto) | 2 | BLOQUEADO — glob expande corretamente |
| `cat fixture.txt` (relativo, `tool_info.cwd`=dir) | 2 | BLOQUEADO — `credResolveArg` usa `tool_info.cwd` |
| `cat *.txt` (relativo glob, `tool_info.cwd`=dir, process cwd≠dir) | 0 | `credGlobToken` usa PROCESS CWD, não `tool_info.cwd` |

**Achado T3 — ADR diz "continua cobrindo arquivo nomeado" mas menciona `grep -r` como exemplo.** O ADR não menciona `rg`. `rg foo file.txt` passa sem bloqueio mesmo com arquivo explícito. O texto documentado não pode implicar que `rg` em arquivo nomeado é coberto.

**Achado T4 — Glob relativo usa process cwd; named relativo usa `tool_info.cwd` (Ameaça ao AC2 contract).** Medição: com `tool_info.cwd`=dir-com-fixture e process cwd=repo: `cat fixture.txt` → rc=2 (BLOQUEADO — resolve via `tool_info.cwd`); `cat *.txt` → rc=0 (PASSA — glob expande no process cwd, não no `tool_info.cwd`). O contrato "one-level glob covered" é falso para globs relativos quando `tool_info.cwd` ≠ process cwd. **Notar ao arquiteto:** se a Regra Dura exigir correção, é novo ML. Este Wave 0 apenas documenta a observação.

**Recomendação para a documentação (AC2):** enunciar o **contrato positivo**, não uma lista de bypass. O risco de listar contornos é o usuário confiar demais na camada:

> A segunda camada escaneia: (1) arquivos nomeados como argumentos de `cat`, `head`, `tail`, `jq`, `grep`, quando esses verbos aparecem como primeira palavra do comando; (2) expansão de glob de um nível (ex.: `dir/*.go`) para os mesmos verbos, usando o cwd do processo; (3) alvos de redirecionamento não efêmeros. Não escaneia: diretórios de forma recursiva (ex.: `grep -r`, `grep -R`); verbos fora do conjunto acima (incluindo `rg`, mesmo com arquivo nomeado); globs com `**`.

Exemplo medido para `--help` e `docs/cli-parity.md`:
```
# Bloqueado — grep com glob de arquivo:
grep -n foo internal/guard/*.go  → rc=2

# Passa — grep recursivo em diretório:
grep -rn foo internal/guard  → rc=0
```

### 2.3 D3 — Montagem de token em tempo de execução

**Achado T5 — Reprodução ao vivo do defeito #3 da REQ (guard bloqueia `grep` nos arquivos de fixture).** Os dois greps que a REQ cita foram bloqueados pelo guard na própria sessão de medição deste Wave 0:
- Payload `cat internal/guard/credential_patterns_test.go` → rc=2 (PEM literal no arquivo)
- Payload `cat internal/generators/credential_guard_test.go` → rc=2 (JWT literal no arquivo)

**A montagem em tempo de execução não enfraquece os testes de detecção.** O guard recebe o token montado como bytes e compara com o regex. O token montado `header + "." + payload + "." + sig` produz a mesma sequência de bytes que o literal.

**Pré-condições que mantêm a afirmação:**

1. Cada fragmento individualmente NÃO deve casar com nenhum padrão em `credPatterns` (contra-braço de fragmento).
2. Os fixtures não devem ser derivados de `credPatterns` — se derivados, uma alteração no padrão não reprovaria o teste.
3. O token montado DEVE casar, E a remoção ou mutação do padrão em `credPatterns` deve reprovar o teste (contra-braço de mutação).
4. AC3 deve ser verificado pelo guard (payload `cat <arquivo>`), não por grep manual do código-fonte. O grep seria bloqueado pelo próprio guard.

---

## 3. Alvos de Falsificação

### 3.1 ML-1A — Mensagem com origem do achado e documentação de D2

**Falso negativo (guard bloqueia mas diz "in tool payload" para achado de arquivo):**
- Enviar payload `cat <arquivo_com_jwt>` → guard bloqueia → mensagem contém "in file `<caminho>` referenced by the command". Se ainda disser "in tool payload", D1 não foi implementado neste caminho.
- Testar todos os cinco caminhos (L2a, L2b, L4-deep, L2b-nonJSON, L2a-ADS[Windows]) separadamente.
- Testar modo warn: a atenção JSON também deve conter o caminho.

**Falso positivo (guard diz "in file X" para achado no payload):**
- Enviar payload JSON com JWT no próprio campo `command` (sem arquivo): mensagem deve continuar "in tool payload", não "in file".
- Verificar que `credCmdHasTypeMatch` recebe o tipo, não o caminho (exemption F2).

**Falsificação por caractere de controle (sanitização incompleta):**
- Criar arquivo com `\n::error::` no nome, contendo JWT. Enviar `cat $DIR/*.txt`. Guard bloqueia. O stderr deve ter exatamente UMA linha, começando com `trackfw-credential-guard:`. Se aparecerem duas linhas, a sanitização de `\n` não foi aplicada.
- Criar arquivo com ESC no nome. Verificar que a mensagem contém `?` onde estaria o ESC.
- Teste unitário do sanitizador com `"\x9b"` (C1 raw byte): deve retornar `"?"`.
- A atenção JSON deve ser válida após `json.Unmarshal`.

**Falsificação por call site parcial:**
- Testar L2a com `echo x > fixture.txt` onde `fixture.txt` tem JWT: mensagem deve indicar o arquivo.
- Testar deep-scan com schema Cursor `{"params":{"command":"cat <arquivo>"}}`: mensagem deve indicar o arquivo.

**Falsificação por exemption (F2) quebrada:**
- `echo eyJ... > /dev/null` (token no payload, redirect para /dev/null): deve continuar passando (rc=0). Se rc=2 após D1, o refactor corrompeu a exemption.

### 3.2 ML-1B — Fixtures montam token em tempo de execução

**Falso negativo (após remover literais, detecção para):**
- Para cada padrão removido: `credScanInput(assembledToken)` deve retornar o nome do tipo esperado (não "").
- Payload `cat <arquivo_com_token_montado>` pelo guard → rc=2 no modo block.

**Falso positivo (fragmento individual aciona detecção — contra-braço de fragmento):**
- Para cada fragmento do token montado: `credScanInput(fragment)` deve retornar "".

**Falso negativo por mutação do padrão (contra-braço de mutação):**
- Remover ou alterar o padrão correspondente em `credPatterns`: o teste deve reprovar (rc=0 onde esperava rc=2). Sem esse braço, o teste não prova que a detecção depende daquele padrão específico.

**Falso positivo por AC3 — ML-1A re-introduz literal:**
- Após ML-1B E ML-1A concluídos: census completo deve ter zero matches. O census é a prova; não usar grep manual.

**Falso negativo por census incompleto:**
- O census cobre `internal/guard/*.go` e `internal/generators/*_test.go`. Se ML-1A adicionar novos arquivos fora desse glob, o census deve ser expandido.

---

## 4. Residual Declarado

**R1 — Verbos fora da allowlist de argv0.** `rg`, `/bin/cat`, `sed`, `awk`, `python3`, `find`, `tar`, `zip`, `xargs` passam sem escaneamento de arquivo, mesmo com arquivo nomeado. Aceito: a segunda camada é uma lombada para o caso óbvio, não uma fronteira contra quem quer contornar.

**R2 — Glob `**` não é recursivo no guard.** O guard trata `**` como componente literal. Aceito: leitura recursiva tem custo ilimitado (ADR D2). Um agente em zsh que usa `cat dir/**/*.txt` tem o shell expandindo os tokens antes do payload JSON; nesse caso, cada arquivo nomeado chega individualmente e é escaneado.

**R3 — `\n` em nome de arquivo: FECHADO pela sanitização.** A substituição `\n` → `?` garante que cada linha de stderr produzida pelo guard começa com `trackfw-credential-guard:`. Nenhum texto injetado pode ocupar o início de uma linha. Este item era candidato a residual; a sanitização mandatória o fecha.

**R4 — Path leakage persistente em projetos consumidores com `mode: warn`.** `trackfw init` não adiciona `.trackfw-credential-guard.json` ao `.gitignore` gerado. Um commit acidental expõe o caminho do arquivo com credencial. O arquivo de atenção do PRÓPRIO repositório trackfw está protegido (`.gitignore:31`), mas consumidores não têm essa proteção. Aceito para este roadmap: a correção (adicionar a entrada em `generateGitIgnore`) pertence a outro defeito. **Notar ao arquiteto como ação futura independente.**

**R5 — L2a-ADS não testado nesta plataforma.** O ADS fallback é Windows-only. Precisa de cobertura de teste nas suítes existentes; este Wave 0 não o mediu ao vivo.

**R6 — Glob relativo usa process cwd; named relativo usa `tool_info.cwd`.** Para globs relativos, `credGlobToken` expande no process cwd; para nomes relativos, `credResolveArg` usa `tool_info.cwd`. Quando os dois diferem (cenário medido), o glob relativo passa sem bloqueio mesmo que `tool_info.cwd` aponte para um diretório com credencial. Aceito para este roadmap: corrigir exigiria mudar a semântica de `credGlobToken` ou introduzir um parâmetro de base de glob. Notar ao arquiteto.

---

## Veredito: APROVA COM RESSALVAS

As decisões D1, D2, D3 são viáveis. Nenhum achado exige reprovar o roadmap. As ressalvas abaixo são requisitos obrigatórios para ML-1A e ML-1B.

---

## Itens Obrigatórios para ML-1A

1. **Sanitização do caminho (T1, T2, Ameaça 2):** aplicar antes de embutir o caminho em qualquer mensagem. Especificação por propriedade: substituir por `?` (não descartar) qualquer char onde `unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)` seja verdadeiro, além de UTF-8 inválido → `?`. Aplicar tanto a tokens do comando quanto a nomes retornados por `filepath.Glob`. Adicionar teste unitário do sanitizador com `"\x9b"` (raw C1 byte). Adicionar teste com `\n::error::` no nome de arquivo: stdout/stderr deve ter exatamente uma linha por chamada ao guard.

2. **Cobertura de todos os call sites (E1, Ameaça 5):** os cinco caminhos de chegada (L2a, L2a-ADS, L2b, L2b-nonJSON, L4-deep) e os cinco pontos de emissão (stderr block/warn em ambos escopos, `credWriteAttention`) devem usar a nova mensagem de forma consistente.

3. **`credWriteAttention`: substituição, não descarte (T2).** Mudar o loop de `continue` para `msgEsc.WriteRune('?')` para chars de controle. Alinha com o ADR. Adicionar teste de round-trip: `json.Unmarshal` do arquivo escrito após detecção em modo warn deve produzir o campo `message` sem erro de parse.

4. **Invariante "impresso == aberto" (Ameaça 4).** Imprimir o caminho exatamente como chegou a `os.Stat` em `credScanFile`, após a limpeza interna (strip de aspas e `},`). Sem `filepath.Abs` ou `filepath.EvalSymlinks`.

5. **Separação tipo/caminho na exemption (Ameaça 6).** `match` (tipo de credencial) permanece separado de `filePath`. Verificar que `echo <token> > /dev/null` continua passando (rc=0) após o refactor.

6. **Montagem de token em `credential_test.go` (E3).** Os novos testes de D1 não devem conter literais de token que caiam no padrão do guard. Montar tokens em tempo de execução.

7. **Testes para ambas as formas de mensagem.** Um caso de L1 (achado no payload) verifica que a mensagem diz "in tool payload"; um caso de L2 (achado em arquivo via `cat`) verifica que diz "in file `<caminho>` referenced by the command". Um caso com `\n::error::` no nome de arquivo verifica exatamente uma linha de stderr.

8. **Documentação (AC2).** `--help` e `docs/cli-parity.md` enunciam o contrato positivo (verbos cobertos, escopo do glob, sem recursão). Incluir exemplo medido. Notar que `rg` não está na allowlist, mesmo com arquivo nomeado. Não usar "bypass list".

---

## Itens Obrigatórios para ML-1B

1. **Census como prova de AC3.** Após ML-1B E ML-1A concluídos: census completo (payload `cat <arquivo>` para cada arquivo em `internal/guard/*.go` e `internal/generators/*_test.go`) deve ter zero matches. O census é a prova; não usar grep manual.

2. **Contra-braço de fragmento.** Para cada literal removido: `credScanInput(fragment)` → `""` para cada fragmento individualmente.

3. **Contra-braço de mutação.** Para cada literal removido: remover ou alterar o padrão correspondente em `credPatterns` deve reprovar o teste. Os fixtures não devem ser derivados de `credPatterns`.

4. **Afirmação explícita.** Cada teste positivo declara qual conclusão afirma: "o token montado em tempo de execução é detectado pelo padrão `<tipo>`; o contra-braço de mutação confirma que a detecção depende do padrão, não de outra fonte."

---

## Notas para o arquiteto (ADR e ações futuras — não altere o ADR você mesmo)

- **ADR D1 "o caminho vem do comando e pode ser forjado":** incompleto — via `filepath.Glob`, o caminho vem do sistema de arquivos (nomes de arquivo no SO), não do comando.
- **ADR D2 "continua cobrindo arquivo nomeado":** falso para `rg` e para globs relativos quando `tool_info.cwd` ≠ process cwd.
- **Ação futura — `generateGitIgnore`:** adicionar entrada para `<roadmap_dir>/.trackfw-credential-guard.json` no bloco gerado por `trackfw init`, para proteger projetos consumidores em modo warn.
- **Candidatos a nota de vault (não escrito nesta sessão):** (a) "glob relativo usa process cwd; named relativo usa tool_info.cwd — gap no contrato de cobertura da segunda camada"; (b) "`trackfw init` não ignora .trackfw-credential-guard.json — leak persistente em consumer com mode:warn".
