# Wave 0 — Threat Model: Guards do Primeiro Usuário Externo

> Hades (hades-tf) · 2026-11-05  
> Roadmap: `ROADMAP-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md`  
> REQ: `REQ-2026-11-05-guards-do-primeiro-usuario-externo-mode-block-ignorado-com-crlf-padroes-de-credencial-e-involucro-de-shell-no-guard-de-git.md`  
> ADR: `ADR-2026-11-05-guards-cumprem-o-que-o-readme-promete-mode-lido-com-crlf-padroes-de-credencial-ampliados-e-involucro-de-shell-analisado.md`

---

## 1. Completude da Enumeração

A lista de superfícies no roadmap cobre os pontos ativos. Esta seção inventaria cada leitor de mode, cada sítio de padrão de credencial e cada ponto de decisão do guard de git, com o status ativo/morto e o caminho da chamada atual.

### 1.1 Leitores de `credential_guard.mode`

| Sítio | Função | Escopo | Status | Caminho de chamada ativo |
|---|---|---|---|---|
| `internal/guard/credential.go` L978 | `credReadModeWithDefault` default=`"warn"` | projeto | **ATIVO** | `.claude/settings.json` → `trackfw guard credential` |
| `internal/guard/credential.go` L978 | `credReadModeWithDefault` default=`"block"` | global | **ATIVO** | `.claude/settings.json` → `trackfw guard credential --global` |
| `internal/validator/validator_credential_guard_integrity.go` L93 | `extractCredentialGuardMode` | validação | **ATIVO** | `trackfw validate` |
| `internal/generators/scaffold.go` L1507 | `credentialGuardModeResolution` (constante shell) | — | **MORTO** | nunca chamado na geração; `.sh` é thin shim |
| `scripts/trackfw-credential-guard.sh` | thin shim | projeto (Git hook) | ATIVO (sem leitura de mode) | `exec trackfw guard credential` |
| `~/.trackfw/scripts/trackfw-credential-guard.sh` | thin shim | global (Git hook) | ATIVO (sem leitura de mode) | `exec trackfw guard credential --global` |

**Achado 1 — CRLF (D1):** `credReadModeWithDefault` não remove `\r`. O pipeline `tr -d "\"'"` (reproduzido em Go) não inclui `\r`. O `switch` sobre `"block\r"` não casa com "block" → cai no default.

Medição (2026-11-05):
```
# LF trackfw.yaml, mode: block → rc=2 (bloqueado — esperado)
# CRLF trackfw.yaml, mode: block\r → rc=0 (aviso — defeito confirmado)
# CRLF trackfw.yaml, mode: warn\r (escopo global, default=block) → rc=2 (bloqueado — defeito inverso)
```

**Achado 2 — Linha comentada (D1, novo):** `credReadModeWithDefault` detecta `# mode: block` como modo ativo porque `Contains("mode:")` casa a posição do "mode:" dentro do comentário. `idx = strings.Index(line, "mode:")` aponta para a posição de "mode:" dentro de `# mode: block`; `line[idx+5:]` = ` block`; `TrimLeft → "block"`. Não há verificação de que "mode:" ocorre no início da chave YAML.

Medição:
```
# credential_guard:\n  # mode: block\n → rc=2 (guard bloqueia — falso positivo confirmado)
# credential_guard:\n  mode: block  # trailing comment\n → rc=2 (guard bloqueia — correto)
```

**Achado 3 — Chave de sufixo `ci_mode: block` (D1, novo):** `Contains("mode:")` casa com `ci_mode: block` → `idx` aponta para a posição de "mode:" dentro de "ci_mode:" → extrai ` block` → retorna `"block"`. Guard bloqueia (rc=2 confirmado empiricamente); validator retorna `("ci_mode: block", true)` (valor não reconhecido como "block" ou "warn"). Divergência.

Medição:
```
# trackfw.yaml: credential_guard:\n  ci_mode: block\n
# printf JWT payload | trackfw guard credential → rc=2 (guard bloqueia via infix match)
```

**O `extractCredentialGuardMode` do validator não tem os achados 2 e 3:** usa `TrimPrefix(trimmed, "mode:")` sobre a linha já `TrimSpace`-ada; para `# mode: block`, `trimmed` = `"# mode: block"`, `TrimPrefix` não casa → retorna `("", true)`. Para `ci_mode: block`, `trimmed` = `"ci_mode: block"`, `TrimPrefix` não casa → retorna `("ci_mode: block", true)`.

**Divergência guard/validator:** três vetores de entrada produzem conclusões opostas entre guard e validator para o mesmo arquivo. Entra em AC1.

**Achado 4 — Janela de busca:** o validator usa `lines[start : start+1+5]` (linha do header + 5 seguintes = 6 linhas). O guard usa `lines[i+1 : i+6]` (5 linhas APÓS o header). Como o header `credential_guard:` não contém "mode:", ambos escaneiam efetivamente as mesmas 5 linhas candidatas. Divergência de janela nula na prática.

**Achado 11 — Cross-block boundary (D1, novo):** `credReadModeWithDefault` toma as 5 linhas após o header `credential_guard:` (L984–996) sem parar ao encontrar uma nova chave YAML de topo. Para:
```yaml
credential_guard:
  enabled: true
other_tool:
  mode: block
```
`other_tool:` está em i+2 e `  mode: block` está em i+3 — dentro da janela de 5. `Contains("mode:")` casa com `  mode: block` → extrai "block" → `credReadModeWithDefault` retorna "block" (valor da seção irmã, não de `credential_guard`). Confirmado por leitura de código L984–1003 e por medição ao vivo (2026-11-05, binário commit b80bab34, HOME isolado em dir vazio para casos globais):

```
# CASE A — projeto, credential_guard sem mode, sibling mode:block
# trackfw.yaml: credential_guard:\n  enabled: true\nother_tool:\n  mode: block\n
# printf '{"tool_name":"Bash","tool_input":{"command":"echo AWS_KEY"}}' | trackfw guard credential
#   rc=2  CROSS-BLOCK BUG: guard leu mode=block da seção irmã (default=warn deveria dar rc=0)

# CASE B — global, credential_guard sem mode, sibling mode:warn (HOME isolado)
# trackfw.yaml: credential_guard:\n  enabled: true\nother_tool:\n  mode: warn\n
# printf '{"tool_name":"Bash","tool_input":{"command":"echo AWS_KEY"}}' | HOME=/emptyhome trackfw guard credential --global
#   rc=0  CROSS-BLOCK BUG: guard leu mode=warn da seção irmã (default=block deveria dar rc=2)

# CASE E — contra-braço: projeto, sem seção irmã -> esperar rc=0 (padrão=warn)
# trackfw.yaml: credential_guard:\n  enabled: true\n
# printf '{"tool_name":"Bash","tool_input":{"command":"echo AWS_KEY"}}' | trackfw guard credential
#   rc=0  CONFIRMA: o rc=2 do CASE A veio da seção irmã, não de outra fonte

# CASE F — contra-braço: global, sem seção irmã (HOME isolado) -> esperar rc=2 (padrão=block)
# trackfw.yaml: credential_guard:\n  enabled: true\n
# printf '{"tool_name":"Bash","tool_input":{"command":"echo AWS_KEY"}}' | HOME=/emptyhome trackfw guard credential --global
#   rc=2  CONFIRMA: o rc=0 do CASE B veio da seção irmã, não de outra fonte

# CASE G — contra-braço: --global lê trackfw.yaml no cwd (HOME isolado, mode:block)
# trackfw.yaml: credential_guard:\n  mode: block\n
# printf '{"tool_name":"Bash","tool_input":{"command":"echo AWS_KEY"}}' | HOME=/emptyhome trackfw guard credential --global
#   rc=2  CONFIRMA: --global lê trackfw.yaml no cwd
```

FP (projeto): `credential_guard` sem `mode` explícito, seção irmã com `mode: block` nos primeiros 5 linhas → guard usa mode=block; default correto seria "warn" → rc=2 (over-block confirmado CASE A).
FN (global): default=block, seção irmã com `mode: warn` nos primeiros 5 linhas → guard usa mode=warn; default correto seria "block" → rc=0 (under-block confirmado CASE B).

A spec original (Prescrições 5.1) tinha este bug: `HasPrefix(trimmed, "mode:")` onde `trimmed = TrimSpace("  mode: block") = "mode: block"` cassava com linha de seção irmã. A spec step 2 foi corrigida: para ao encontrar linha não-vazia (definida como `TrimSpace(line) != ""`) cuja primeira coluna não seja espaço nem `#`. Entra em AC1.

**Achado sobre `.sh` gerado:** os scripts `.sh` são thin shims (confirmado em `credentialGuardScript` L1602 e `globalCredentialGuardScript` L1623). Não leem mode nem padrões — apenas chamam `exec trackfw guard credential [--global]`. O `credentialGuardModeResolution` (L1507) é código morto: nunca compõe nenhum script gerado. Os scripts de referência em `internal/generators/testdata/guard-sh-reference/` são fixtures congeladas que o teste de paridade usa para verificar os bytes do shim — não precisam mudar quando os padrões Go mudam.

**Contradição com o ADR:** o contexto do ADR afirma "o script gerado `scripts/trackfw-credential-guard.sh` (`credentialGuardDetectionCore` e `credentialGuardModeResolution`, em `internal/generators/scaffold.go`) traz os mesmos dois padrões e o mesmo leitor de `mode`". Isso descreve a arquitetura anterior. O script atual é um thin shim; `credentialGuardDetectionCore` e `credentialGuardModeResolution` são código morto. O arquiteto deve considerar corrigir o contexto do ADR.

### 1.2 Sítios de padrões de credencial

| Sítio | Padrões | Status | Observação |
|---|---|---|---|
| `internal/guard/credential.go` L23–26 | `credJWTRe`, `credAWSRe` | **ATIVO** | único sítio que o guard lê |
| `internal/guard/credential.go` L150, L163, L247–248 | `match == "JWT"` / `"AWS access key"` | **ATIVO** | segunda camada (isenção efêmera); precisa atualizar se novos tipos forem adicionados |
| `internal/generators/scaffold.go` L1384–1385 | JWT_PATTERN, AWS_KEY_PATTERN (shell) | **MORTO** | parte de `credentialGuardDetectionCore`, nunca chamado |
| `internal/generators/testdata/guard-sh-reference/credential-guard-*.sh` L7–11 | JWT_PATTERN, AWS_KEY_PATTERN | fixture congelada | usada pelo teste de paridade de shim; não muda com D2 |

**Achado 5 — Segunda camada é isenção efêmera:** a condição F2 em L150–154 retorna 0 (isenta) somente quando `cmdHasMatch=true` AND o comando é efêmero (padrão `echo TOKEN > /dev/null`).

Medição (JWT baseline, 2026-11-05):
```
# echo JWT > /dev/null   → rc=0  (second layer isenta — correto; JWT é tipo conhecido)
# echo JWT               → rc=2  (not ephemeral — correto)
# echo JWT > token.txt   → rc=2  (not /dev/null — correto)
# printf JWT > /dev/null → rc=2  (printf não é o padrão ephemeral; só echo TOKEN > /dev/null)
```

`cmdHasMatch=false` NÃO isenta — a credencial é bloqueada. Para novos tipos adicionados em D2 sem atualizar a segunda camada: `echo ghp_... > /dev/null` → `cmdHasMatch=false` → não isento → rc=2. Isso é over-block para cenários de teste com comandos efêmeros. Entra em AC2 (vetor de isenção efêmera por pelo menos um novo tipo).

**Achado 6 — Constantes mortas em scaffold.go:** `credentialGuardDetectionCore`, `credentialGuardModeResolution`, `credentialGuardProjectTail`, `credentialGuardGlobalTail`, `credentialGuardHeader` e `credentialGuardProjectGuardBlock` são constantes Go nunca chamadas na geração de scripts. O comentário em `scaffold_literal_copy_parity_test.go` L81–82 descreve a composição antiga e é obsoleto.

### 1.3 Pontos de decisão do guard de git

| Função | Arquivo | Papel |
|---|---|---|
| `MatchSubcommand` L342 | `internal/guard/gitbranch.go` | entrada principal; percorre trechos separados por `;`, `&&`, `\|\|`, `\|` |
| `quoteAwareSplit` L1136 | `internal/guard/gitbranch.go` | divide em trechos pelos separadores acima, respeitando aspas; POSIX-aware dentro de aspas duplas (L1157–1172): `\"` é escape, não fecha a aspa |
| `shellTokenize` L1081 | `internal/guard/gitbranch.go` | tokeniza um trecho; backslash NÃO é escape dentro nem fora de aspas (ver Achado 7) |
| `stripEnvCommandPrefix` L400 | `internal/guard/gitbranch.go` | remove `env [KEY=VAL...]`, `command`, `sudo [flags]` antes do primeiro token |
| `applyRule` L477 | `internal/guard/gitbranch.go` | tabela de subcomandos bloqueados |

**`quoteAwareSplit` é POSIX-aware — preocupação encerrada:** inspeção de L1157–1172 e medição direta confirmam que `\"` dentro de aspas duplas é tratado como escape (mantém estado "dentro de aspas"), não como fechamento da aspa. Vetor testado: `echo "\"" ; git push` (JSON: `{"command":"echo \"\\\"\" ; git push"}`) → rc=2. Guard bloqueou `git push` como segundo segmento. A hipótese de bypass por backslash-cego em `quoteAwareSplit` está encerrada; não entra em AC4.

**Achado 7 — Backslash bypass (pré-existente a D4):** `shellTokenize` não implementa POSIX backslash escaping. `\x` fora de aspas → `\x` literal no token (default case: `cur.WriteRune(c)`).

Medições (ambos confirmados, 2026-11-05):
```
\git push  → rc=0  (bypass: JSON "\\git push"; token '\git' ≠ 'git')
g\it push  → rc=0  (bypass: JSON "g\\it push"; token 'g\it' ≠ 'git' — mid-token também escapa)
git push   → rc=2  (bloqueado — controle positivo)
```

A solução TrimLeft(token, "\\") não fecha o vetor de mid-token (`g\it push` → `g\it` sem leading backslash → não removido → bypass). A correção necessária é POSIX backslash handling, mas requer DUAS mudanças coordenadas:

1. `quoteAwareSplit` (L1162–1163): atualmente emite `cur.WriteRune(next)` para `\"` dentro de aspas duplas, descartando o `\`. Deve ser alterado para `cur.WriteRune('\\'); cur.WriteRune(next)` — emitindo o par verbatim. Sem esta mudança, `shellTokenize` recebe conteúdo parcialmente desescapado e sofre double-unescaping ou tokenização incorreta.

2. `shellTokenize`: implementar POSIX unescaping. Com `quoteAwareSplit` emitindo verbatim, `shellTokenize` recebe o `\"` completo e faz a única passagem de unescaping correta.

Com AMBAS as correções, o vetor `bash -c "bash -c \"git push\""` depth=2 funciona corretamente: outer → inner command string `bash -c "git push"` (os `\"` são decodificados corretamente por `shellTokenize`) → inner → `git push` → bloqueado. Entra em ML-1B.

**Achado 8 — `stripEnvCommandPrefix` lacunas:**

```
env -i git push     → rc=0  (flag -i → break imediato → primeiro token resultante é -i, não git)
env -- git push     → rc=0  (-- começa com - → break)
env -u X git push   → rc=0  (mesmo mecanismo)
command -p git push → rc=0  (command case remove só "command", deixa -p como primeiro token)
FOO=1 git push      → rc=0  (atribuição nua: FOO=1 não é env/command/sudo → não removido)
sudo -u x git push  → rc=2  (CORRETAMENTE TRATADO — L419-443 enumera -u, -g etc.; controle positivo)
```

Mesma causa dos bypasses originais do #663: `stripEnvCommandPrefix` não cobre todos os formatos. Entra em ML-1B.

**Achado 9 — `bash -c -e "git push"` é bypass (não FP):**

Medição:
```
bash -c -e 'echo REACHED' → imprime REACHED; EXIT 0
```

Bash coleta `-e` como flag de shell ANTES de consumir o argumento de `-c`. O command string é o primeiro argumento não-opção. `bash -c -e "git push"` → `-e` ativa exit-on-error, `"git push"` é o command string → git push É executado. Confirmado como bypass.

**Achado 10 — Wrappers transparentes:**

```
nohup git push       → rc=0  (bypass confirmado)
time git push        → rc=0  (bypass confirmado)
exec git push        → rc=0  (bypass confirmado)
timeout 10 git push  → rc=0  (bypass confirmado)
nice git push        → rc=0  (bypass confirmado, 2026-11-05)
doas git push        → rc=0  (bypass confirmado, 2026-11-05)
stdbuf -oL git push  → rc=0  (bypass confirmado, 2026-11-05)
```

Mesmo mecanismo: o primeiro token é o wrapper, não `git`. Este é um **denylist aberto**: nice, doas, stdbuf, ionice, taskset, nsenter e outros wrappers de sistema compartilham o mecanismo; nenhuma lista enumerada pode ser declarada fechada. ML-1B prescreve os tokens conhecidos e deve declarar o denylist como aberto na documentação.

**Sem segundo guard de git:** o `.sh` do git-branch também é thin shim (`exec trackfw guard git-branch`). O validator não verifica comandos git. O único sítio de decisão é `MatchSubcommand` em Go.

---

## 2. Modelo de Ameaça

O adversário relevante aqui é o implementador sob pressão. Os vetores abaixo são formas como a implementação pode passar nos testes sem fechar as vulnerabilidades.

**A.1 — Correção parcial do CRLF (strip `\r` sem corrigir comentário e chave de sufixo):**
O implementador adiciona `strings.TrimRight(val, "\r")` antes do switch, mas não corrige `# mode: block` nem `ci_mode: block`. O teste de AC1 que cobre apenas LF/CRLF passa, mas os vetores de comentário e sufixo não estão cobertos. **Mitigação:** a tabela unificada de AC1 deve incluir `# mode: block → rc=0`, `ci_mode: block → rc=0` e o vetor de indentação; o parser compartilhado (Prescrição 5.1) fecha todos os três na mesma mudança.

**A.2 — Parser não compartilhado cria divergência nova:**
O implementador corrige `credReadModeWithDefault` para pular linhas comentadas mas usa lógica diferente da do validator. Para `credential_guard:\n  # mode: warn\n  mode: block`, o guard (após fix: skip `# mode: warn`, encontra `mode: block`) retorna "block". O validator (encontra a primeira linha com "mode:" que é `# mode: warn`, extrai `""` via TrimPrefix que não casa, retorna `("", true)` e para) retorna "" (default). Nova divergência. **Mitigação:** compartilhar um parser único entre guard e validator (ver Prescrições 5.1).

**A.3 — D2 sem atualizar segunda camada (isenção efêmera):**
O implementador adiciona novos regexes mas não atualiza `match == "JWT" / "AWS access key"` em L150, L163, L247, L248. Consequência: novos tipos nunca ganham isenção efêmera → `echo ghp_XXXX > /dev/null` com mode=block → rc=2 (bloqueado, enquanto JWT seria rc=0). Este é falso positivo para cenários de teste. **Mitigação:** AC2 deve ter vetor de isenção efêmera por pelo menos um novo tipo; prescrita refatoração para table lookup.

**A.4 — `sk-` infix match em texto com hifens:**
Medição confirma FP:
```
# input: task- + 48 A's (len=53)
# grep -E 'sk-[A-Za-z0-9]{48}' → 1 match (sk- em posição 2 de 'task-', 48 A's após; confirmado)
# \bsk-[A-Za-z0-9]{48,} → 0 matches (\b evita porque 'a' é word char antes de 's')
# (?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-[...]{48,} → 0 matches (mesmo comportamento; 'a' ∉ [^A-Za-z0-9_])
```
Prescrita boundary explicita (ver nota de boundary em 5.1). FP medido em internal/: F=0 L=0.

**A.5 — `sk-ant-` não casa com formato `api03-`:**
`sk-ant-api03-ABCDE...` não casa com `sk-ant-[A-Za-z0-9_]{20,}` (hifens no body). Medição Go RE2:
```
sk-ant-[A-Za-z0-9_]{20,}  vs sk-ant-api03-ABCDE...  → false (NO MATCH — bug confirmado)
sk-ant-[A-Za-z0-9_-]{20,} vs sk-ant-api03-ABCDE...  → true  (MATCH — fix)
```
Prescrita boundary explicita `(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-ant-[A-Za-z0-9_-]{20,}` (ver nota de boundary em 5.1).

**A.6 — `AIza` positive example curto:**
`AIzaSyD1234567890123456789012345678901` tem 34 chars após `AIza` → não casa com `{35,}`. Medição Go RE2: FAIL confirmado. O exemplo correto é `AIzaSyD12345678901234567890123456789012` (35 chars: `SyD` + 32 chars). Prescrita `AIza[0-9A-Za-z_-]{35,}` com exemplo corrigido.

**A.7 — D4 flags-após-`-c` e flags-combinadas não cobertas:**
Medições confirmadas como bypasses:
```
bash -c -e "git push"  → rc=0  (bash coleta -e, cmd string é "git push"; confirmado)
bash -lc "git push"    → rc=0  (flags combinadas)
bash -ec "git push"    → rc=0  (flags combinadas)
bash -xc "git push"    → rc=0  (flags combinadas)
```
**Mitigação:** D4 deve: (a) detectar flags combinadas via `^-[a-zA-Z]*c[a-zA-Z]*$`; (b) para flags não combinadas, coletar todos os flags e pegar o primeiro argumento não-flag após eles como command string.

**A.8 — Falso positivo D4 não testado:**
Implementador bloqueia `bash -c "git status"`. **Mitigação:** AC4 deve ter vetor negativo explícito `bash -c "git status"` → rc=0.

**A.9 — Backslash bypass não coberto por TrimLeft:**
`\git push` → rc=0 e `g\it push` → rc=0 confirmados (2026-11-05). TrimLeft apenas remove backslashes INICIAIS do token; `g\it` não é afetado. A prescrição é POSIX backslash handling em DUAS funções: (1) `quoteAwareSplit` deve emitir `\` + next verbatim (atualmente descarta o `\`, ver L1162-1163); (2) `shellTokenize` implementa a única passagem de POSIX unescaping. Sem a correção em `quoteAwareSplit`, double-unescaping ocorre para o vetor depth=2 `bash -c "bash -c \"git push\""`. Com ambas as correções, depth=2 fica coberto. Entra em AC4 e ML-1B.

**A.10 — Comentário obsoleto `scaffold_literal_copy_parity_test.go` L81–82 não atualizado:**
Descreve `credentialGuardScript` como composição de quatro constantes mortas. Um auditor futuro conclui que os padrões estão no `.sh`. **Mitigação:** ML-1A atualiza o comentário.

**A.11 — Constantes mortas não deletadas:**
Se ML-1A não deleta `credentialGuardDetectionCore` e afins, permanecem como artefatos com padrões antigos. **Mitigação:** ML-1A deleta as constantes mortas.

---

## 3. Alvos de Falsificação — AC1 a AC5

### AC1 — `credential_guard.mode` lido igualmente em guard e validator

**Sentido falso negativo (bypass permanece):**

| Vetor | Problema atual | Gate após fix |
|---|---|---|
| CRLF `mode: block\r` (projeto) | rc=0 (cai no warn default) | rc=2 |
| CRLF `mode: warn\r` (global, default=block) | rc=2 (bloqueia indevidamente) | rc=0 |
| `# mode: block` (comentário) | rc=2 (guard interpreta como block via infix match) | rc=0 |
| `ci_mode: block` (chave de sufixo) | rc=2 (guard extrai "block" do infix; confirmado) | rc=0 |
| `  mode: block` (indentado) | rc=2 (correto — verificar mantido) | rc=2 |

**Sentido falso positivo (over-block após fix):**
O fix "skip linhas começando com #" cria nova divergência: para `credential_guard:\n  # mode: warn\n  mode: block`, guard pula `# mode: warn` e encontra `mode: block` → retorna "block". Validator encontra `# mode: warn` como primeira linha com "mode:", extrai "", retorna e para → guard="block", validator="". **Mitigação:** compartilhar parser único (ver Prescrições 5.1).

**Tabela unificada mínima para AC1 (guard × validator devem convergir):**

```
LF  "mode: block"                   → ambos: "block"
CRLF "mode: block\r"                → ambos: "block"  (após fix)
LF  "# mode: block"                 → ambos: ""        (após fix com parser compartilhado)
LF  "ci_mode: block"                → ambos: ""        (apenas "mode:" como chave exata)
LF  "mode: \"block\""               → ambos: "block"
LF  "mode: 'block'"                 → ambos: "block"
LF  "  mode: block"                 → ambos: "block"   (indentação preservada)
LF  (credential_guard: ausente)     → guard: default; validator: ("", false)
LF  "mode: other"                   → ambos: default
LF  "# mode: warn\n  mode: block"   → ambos: "block"   (após fix com parser compartilhado)
LF  "mode: block  "                 → ambos: "block"   (trailing spaces sem comment; TrimSpace em step 4 resolve)
LF  "mode: block  # comment"        → ambos: "block"   (trailing spaces com comment; TrimRight em step 5 resolve)
multi-section "credential_guard:\n  enabled: true\nother_tool:\n  mode: block\n" → ambos: default  (cross-block boundary; Achado 11; parser para em "other_tool:")
CRLF blank "credential_guard:\r\n  enabled: true\r\n\r\n  mode: block\r\n" → ambos: "block"  (linha blank CRLF = "\r"; TrimSpace("\r")="" → trata como vazia → continua varrendo; sem TrimSpace → para precocemente)
aninhado "other:\n  credential_guard:\n    mode: block\n" → ambos: default  (coluna-0 HasPrefix não casa linha indentada; evita cross-block por TrimSpace)
BOM "\\xEF\\xBB\\xBFcredential_guard:\n  mode: block\n" → ambos: "block"  (BOM stripped do line 0 antes do HasPrefix — companion de CRLF em arquivos Windows)
tab "credential_guard:\n\tmode: block\n" → ambos: default  (tab-indentado: stop rule dispara — TrimSpace != "" e não começa com espaço; YAML proíbe tabs)
```

### AC2 — Padrões de conteúdo ampliados (D2)

**Sentido falso negativo (bypass):**
Padrão não implementado ou regex muito estreita → credencial real passa com rc=0.

Vetores positivos obrigatórios — Go RE2 testado (2026-11-05), todos passam com as expressões prescritas:

| Tipo | Exemplo público | Regex prescrita |
|---|---|---|
| GitHub PAT clássico | `ghp_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` (40 chars) | `ghp_[A-Za-z0-9]{36,}` |
| GitHub OAuth | `gho_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` | `gho_[A-Za-z0-9]{36,}` |
| GitHub user-to-server | `ghu_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` | `ghu_[A-Za-z0-9]{36,}` |
| GitHub server-to-server | `ghs_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` | `ghs_[A-Za-z0-9]{36,}` |
| GitHub refresh | `ghr_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` | `ghr_[A-Za-z0-9]{36,}` |
| GitHub fine-grained PAT | `github_pat_` + 82 A's | `github_pat_[A-Za-z0-9_]{82,}` |
| OpenAI project | `sk-proj-AAAAAAAAAAAAAAAAAAAAAAAAAAAA` (28+ chars after prefix) | boundary explicita sk-proj (ver nota boundary em 5.1) |
| Anthropic | `sk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ01234` | boundary explicita sk-ant (ver nota boundary em 5.1) |
| OpenAI clássico | `sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijkl` (48 chars after sk-) | boundary explicita sk-48 (ver nota boundary em 5.1) |
| Google API | `AIzaSyD12345678901234567890123456789012` (35 chars after AIza) | `AIza[0-9A-Za-z_-]{35,}` |
| Slack bot | `xoxb-ABCDEFGHIJ-ABCDEFGHIJ-ABCDEFGHIJKLMNO` | `xox[abprs]-[0-9A-Za-z-]{20,}` |
| PEM key | `-----BEGIN RSA PRIVATE KEY-----` | `-----BEGIN [A-Z ]*PRIVATE KEY-----` |

Notas de calibração:
- `sk-ant-[A-Za-z0-9_]{20,}` (sem hífen) → NO MATCH para `sk-ant-api03-...` — confirmado em Go RE2.
- `AIzaSyD1234567890123456789012345678901` (38 total, 34 após AIza) → NO MATCH para `{35,}` — confirmado.
- `sk-[A-Za-z0-9]{48}` sem `\b`: `task-` + 48 A's → MATCH (FP confirmado); com `\b` → NO MATCH.

**Sentido falso positivo (deve permanecer rc=0):**

| Tipo | Vetor negativo de FP |
|---|---|
| boundary sk-48 (ver 5.1) | `task-` + 48 A's → NO MATCH; char antes de `s` é `a` ∈ word-class (confirmado Go RE2 e grep) |
| boundary sk-proj (ver 5.1) | `task-proj-` + 20 chars → NO MATCH; char antes de `s` é `a` ∈ word-class |
| boundary sk-ant (ver 5.1) | `task-ant-` + 20 chars → NO MATCH; char antes de `s` é `a` ∈ word-class |
| `sk-` curto | `sk-short` (< 20 chars) → NO MATCH |
| `AIza` | `AIza` + 34 chars → NO MATCH ({35,}) |
| PEM | `BEGIN VERY PRIVATE DATA` (sem "KEY") → NO MATCH |
| JWT | `eyJhello` (sem dois pontos) → NO MATCH |

**FP medidos neste repo (2026-11-05, expressões prescritas):**
Todos os 12 padrões em `internal/`: F=0 (5 sk-*/AIza/PEM medidos via script; 7 gh-*/xox medidos via `grep -Ef`). Em `docs/`: F=0 exceto o próprio arquivo de parecer (strings de exemplo documentais, nao credenciais). As expressoes boundary explicitas `(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-...` foram tambem verificadas com `grep -Ef` contra `internal/` e `docs/`: F=0 e F=1 (so o parecer), respectivamente — mesmo resultado que `\b`. (A extensao `\b`/`\f` nao foi re-medida com vetores proprios; e superset de `\\[nrt]` medido; vetores para controles JSON raros.)

**Vetores Write/Edit — confirmam necessidade de scan em `inputStr` (AC2):**

```
# D2 nao implementado → rc=0 baseline (false negative confirmado)
# Write tool: sk-ant no campo content
printf '{"tool_name":"Write","tool_input":{"file_path":"x","content":"sk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ01234"}}' | trackfw guard credential
# → rc=0 (baseline, 2026-11-05)
# → rc=2 esperado apos D2 com boundary em inputStr

# Edit tool: sk-ant no campo new_string  
printf '{"tool_name":"Edit","tool_input":{"file_path":"x","old_string":"","new_string":"sk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ01234"}}' | trackfw guard credential
# → rc=0 (baseline, 2026-11-05)
# → rc=2 esperado apos D2 com boundary em inputStr

# Se a implementacao aplicasse os novos padroes so a contextStr (em vez de inputStr):
# shellCmd="" para Write/Edit (tool sem campo command) → contextStr="" → rc=0 permanece (false negative)
```

**Vetor de isenção efêmera (AC2, ver Achado 5):**
Baseline confirmado: `echo JWT > /dev/null` → rc=0 (JWT é tipo conhecido, second layer isenta). Para novos tipos sem atualização da segunda camada: `echo ghp_AAAA… > /dev/null` → rc=2 (over-block). Após fix com table lookup: rc=0.

### AC3 — README e `docs/cli-parity.md`

**Sentido falso negativo:**
README ainda contém promessa de detecção sem ressalva → usuário externo lê como cobertura incondicional.
Gate: a seção de credential guard em README e cli-parity.md deve listar os 12 tipos detectados e incluir ressalva sobre o que NÃO é detectado (caminho de arquivo, atribuição genérica, interpretadores de linguagem).

**Sentido falso positivo:**
README afirma detecção de `python -c subprocess` → promessa não implementada.
Gate: a seção de limite em README/cli-parity.md não pode listar interpretadores de linguagem como "detectados".

### AC4 — Invólucro de shell analisado (D4)

**Sentido falso negativo (bypass permanece após fix):**

Todos os itens abaixo têm rc=0 confirmado (pré-fix). Após D4 + fixes ML-1B, devem ser rc=2:

```
bash -c "git push"                    → rc=2
sh -c "git push"                      → rc=2
bash -lc "git push"                   → rc=2  (flags combinadas)
bash -ec "git push"                   → rc=2  (flags combinadas)
bash -xc "git push"                   → rc=2  (flags combinadas)
bash -l -c "git push"                 → rc=2
bash -c -e "git push"                 → rc=2  (flags-após-c confirmado como bypass)
bash --posix -c "git push"            → rc=2
bash -o pipefail -c "git push"        → rc=2  (flag com arg antes de -c)
eval "git push"                       → rc=2
pwsh -c "git push"                    → rc=2
powershell -Command "git push"        → rc=2
cmd /c "git push"                     → rc=2
env bash -c "git push"                → rc=2
sudo bash -c "git push"               → rc=2
bash -c "bash -c 'git push'"          → rc=2  (depth=2 com POSIX tokenize)
bash -c "bash -c \"git push\""        → rc=2  (depth=2; requer POSIX fix em quoteAwareSplit + shellTokenize — ver Achado 7 e ML-1B ação 1)
env -i git push                       → rc=2  (lacuna stripEnvCommandPrefix — ML-1B)
env -- git push                       → rc=2  (lacuna stripEnvCommandPrefix — ML-1B)
command -p git push                   → rc=2  (lacuna stripEnvCommandPrefix — ML-1B)
FOO=1 git push                        → rc=2  (atribuição nua — ML-1B)
\git push                             → rc=2  (POSIX backslash: \g → g → 'git')
g\it push                             → rc=2  (POSIX backslash: g\i → gi → 'git'; confirmado que TrimLeft não resolve)
nohup git push                        → rc=2  (wrapper — ML-1B)
time git push                         → rc=2  (wrapper — ML-1B)
exec git push                         → rc=2  (wrapper — ML-1B)
exec -a x git push                    → rc=2  (wrapper com flag -a NAME; rc=0 confirmado pré-fix — ML-1B)
timeout 10 git push                   → rc=2  (wrapper com arg numérico — ML-1B)
timeout 10s git push                  → rc=2  (wrapper com sufixo 's' na duração; rc=0 confirmado pré-fix — ML-1B)
timeout -k 5 10 git push              → rc=2  (wrapper com flags -k DUR; rc=0 confirmado pré-fix — ML-1B)
nice git push                         → rc=2  (wrapper — ML-1B)
nice -n 10 git push                   → rc=2  (wrapper com flag -n ADJUSTMENT; rc=0 confirmado pré-fix — ML-1B)
doas git push                         → rc=2  (wrapper — ML-1B)
doas -u x git push                    → rc=2  (wrapper com flag -u USER; rc=0 confirmado pré-fix — ML-1B)
stdbuf -oL git push                   → rc=2  (wrapper com flag+arg — ML-1B)
```

**Falso negativo aceito (residual declarado na seção 4):**
```
powershell -EncodedCommand ZwBpAHQAIABwAHUAcwBoAA==  → rc=0  (encoding não decodificado)
bash << 'EOF'\ngit push\nEOF                          → rc=0  (heredoc stdin)
echo git push | sh                                   → rc=0  (pipe stdin)
bash <<< "git push"                                  → rc=0  (herestring)
echo push | xargs git                               → rc=0  (stdin do xargs)
python -c "subprocess.run(['git','push'])"           → rc=0  (interpretador D5)
ionice -c 2 git push                                → rc=0  (wrapper fora do denylist; residual R4)
G=git; $G push                                      → rc=0  (variavel; residual R6)
git $X                                              → rc=0  (argumento como variavel; residual R6)
bash -c 'git "$@"' _ push                           → rc=0  (D4 inner: "$@" nao e subcomando literal; residual R6)
```

**Sentido falso positivo (deve permanecer rc=0 após D4):**

```
bash -c "git status"                    → rc=0
bash -c "git log"                       → rc=0
bash -c "git diff"                      → rc=0
bash -c "echo 'git push done'"          → rc=0  (echo, não git)
git commit -m "fix bash -c issue"       → rc=0  (texto na mensagem, não invólucro)
bash -c "bash -c 'git status'"          → rc=0  (depth=2, inner = "git status" → permitido)
```

**Regressão obrigatória para a mudança em `quoteAwareSplit`:** a mudança altera o contrato de segmento (emite `\` + next em vez de next). Ambas as direções devem ser verificadas após a mudança:

```
echo "\"" ; git push   → rc=2  (regressão: ; ainda é separador de segmento após o fechamento correto da aspa)
git commit -m "say \"hi\""  → rc=0  (FP: backslash dentro de aspa não é subcomando git bloqueado)
```

Nota: as mudanças em `quoteAwareSplit` (emit verbatim) e `shellTokenize` (POSIX unescaping) devem ser entregues NO MESMO COMMIT. Se apenas `quoteAwareSplit` for corrigido, o tokenizer ainda velho fecha aspa no `\` e produz tokenização incorreta. Callers de `quoteAwareSplit`: apenas L344 (`MatchSubcommand`).

**Profundidade D4 recomendada: 2. Acima de 2: fail-closed (rc=2).**

Justificativa:
- depth=1 causaria FP: `bash -c "bash -c 'git status'"` → fail-closed → rc=2 indevidamente.
- depth=2: outer → inner `bash -c 'git status'` → `git status` → rc=0 ✓; outer → inner `bash -c 'git push'` → `git push` → rc=2 ✓.
- depth=3+: fail-closed → rc=2 (conservador, aceito como residual para casos extremamente raros).

### AC5 — Limite declarado (interpretadores de linguagem)

**Sentido falso negativo:**
README/cli-parity.md não menciona o limite → usuário externo não sabe que `python -c` passa.
Gate: README e cli-parity.md declaram explicitamente que interpretadores de linguagem não são analisados.

**Sentido falso positivo:**
O teste de AC5 "corrige" `python -c subprocess` para rc=2 em vez de fixar rc=0 como limite declarado.
Gate: o teste afirma `python -c "import subprocess; subprocess.run(['git','push'])"` → `rc=0` — a asserção é sobre o LIMITE, não sobre uma falha a corrigir.

---

## 4. Residual Declarado

**R1 — Stdin-driven shell (heredoc, herestring, pipe, xargs git push):**
`bash << 'EOF'\ngit push\nEOF`, `bash <<< "git push"`, `echo git push | sh`, `echo push | xargs git` → rc=0. A detecção por flag `-c` não alcança comandos passados via stdin.

**R2 — Interpretadores de linguagem (D5):**
`python -c "import subprocess; subprocess.run(['git','push'])"`, `node -e "require('child_process').exec('git push')"` → rc=0. REQ negative scope: "Não analisa interpretadores de linguagem".

**R3 — `powershell -EncodedCommand`:**
`powershell -EncodedCommand ZwBpAHQAIABwAHUAcwBoAA==` (base64 UTF-16LE de "git push") → rc=0 confirmado. Requer passo de decodificação ausente em D4.

**R4 — Wrapper denylist é aberto:**
O ML-1B enumera wrappers conhecidos (nohup, time, exec, timeout, nice, doas, stdbuf). Outros wrappers (`ionice`, `taskset`, `nsenter`, `chroot`, `unshare` etc.) seguem o mesmo mecanismo e não são prescritos. Declarado como residual; futuros MLs podem estendê-lo por mecanismo, não por sintoma.

**R5 — Constantes mortas até a deleção:**
Enquanto `credentialGuardDetectionCore` e afins não forem deletados em ML-1A, existe código com padrões antigos que parece ser parte do sistema. Este residual fecha com ML-1A.

**R7 — `inputStr` nao decodifica escapes JSON `\uXXXX` (design commitment de boundary explicita):**
Ao optar por boundary explícita em `inputStr` (opção a), o scan fica cego a credenciais cujo primeiro caractere é codificado como escape JSON `\uXXXX`. Exemplo: o payload produzido por `printf '{"command":"echo \\u0073k-ant-..."}'` contém nos bytes crus o JSON escape `\uNNNN` (6 chars literais: backslash + letra `u` + 4 dígitos hex), NÃO o caractere decodificado. A layer 1 (`inputStr = string(data)`) varre esses bytes sem decodificar e procura `sk-ant-`; encontra backslash + `u` + dígitos → não casa. Mecanismo medido (Case E, test-block2.sh) com JWT cujo `e` inicial foi escapado como `\uNNNN` → rc=0 (a mesma cegueira afeta todos os padrões da layer 1):
```
printf '{"tool_name":"Bash","tool_input":{"command":"echo \\u0073k-ant-..."}}' | trackfw guard credential
# → rc=0 (layer 1 cega a \uXXXX; D2 não implementado — baseline)
```
Declarado como residual aceitável com dois vetores:
- **R7a** (prefixo codificado): primeiro char da credencial escapado como `\uNNNN` → layer 1 vê `\uNNNNk-...` → não casa. O serializador do harness não escapa ASCII imprimível em operação normal.
- **R7b** (separador codificado): sequência `\uNNNN` imediatamente antes do prefixo — o último dígito hex é word char (e.g. ampersand `&` = U+0026 → os 6 bytes são `\`, `u`, `0`, `0`, `2`, `6`; char antes de `s` é `6` ∈ word-class) → nem `[^A-Za-z0-9_]` nem `\\[bfnrt]` disparam. Este vetor é realístico apenas para harnesses que serializam com Go's `json.Marshal` (HTMLEscape=true por default), que escapa `&`, `<` e `>` como `\uNNNN`; um payload `...&token=sk-ant-...` serializado pelo marshaller Go produziria `&sk-ant-...` → false negative. Claude Code constrói o stdin do hook em Node.js — `JSON.stringify` **não** escapa `&`, `<` nem `>` — logo R7b não é realístico para o harness atual de Claude Code. Medido em Go RE2: `x` + backslash + `u0026sk-ant-...` (char antes de `s` = `6` ∈ word-class) → NOMATCH; faz parte dos 19/19 em boundary.go. Ambos os vetores estão fora do escopo do modelo de ameaça "implementador sob pressão".

**R6 — Expansão de variável de shell (guard não avalia variáveis):**
O guard compara tokens literais. Formas que resolvem para `git push` em tempo de execução mas não são `git push` literalmente passam com rc=0:
```
G=git; $G push                     → rc=0  (confirmado; $G não é 'git' literalmente)
git $X                             → rc=0  (confirmado; $X não é 'push' literalmente)
bash -c 'git "$@"' _ push          → rc=0  (D4 inner tokens: 'git' + '"$@"'; "$@" não é subcomando bloqueado)
```
Escopo negativo declarado: "O guard não avalia variáveis de shell." ML-1B deve afirmar este limite na documentação.

---

## Prescrições

### 5.1 ML-1A — Guard de credencial (apolo-tf) — AC1, AC2, AC3

**Arquivos afetados:**

| Arquivo | Mudança |
|---|---|
| `internal/guard/credential.go` | (1) Substituir lógica de `credReadModeWithDefault` por parser compartilhado (spec abaixo); (2) adicionar regexes D2 (ver expressões nesta seção); (3) refatorar segunda camada (L150, L163, L247, L248) para table lookup |
| `internal/validator/validator_credential_guard_integrity.go` | (1) Adaptar `extractCredentialGuardMode` para usar mesma spec de parsing; (2) atualizar comentários em L79 e L154 (referenciam `credentialGuardModeResolution`, a deletar) |
| `internal/generators/scaffold.go` | Deletar constantes mortas: `credentialGuardHeader`, `credentialGuardProjectGuardBlock`, `credentialGuardDetectionCore`, `credentialGuardModeResolution`, `credentialGuardProjectTail`, `credentialGuardGlobalTail` |
| `internal/generators/scaffold_literal_copy_parity_test.go` | Atualizar comentário L81–82 para descrever thin shim atual (não a composição das constantes mortas) |
| `README.md` | D3: credential section — listar os 12 padrões detectados, ressalvar "real credentials", declarar limite D5 |
| `docs/cli-parity.md` | D3: credential section — mesma informação |
| Testes em `internal/guard/` | Tabela unificada AC1 (vetores: LF/CRLF/comentário/sufixo-ci_mode/aspas/indentação/ausente/guard-validator-convergem); vetores positivos e negativos AC2 por padrão (Go RE2 medido); vetor de isenção efêmera por pelo menos um novo tipo |

**Parser compartilhado para AC1 (spec):**

Spec implementada igualmente em guard e validator:
1. Encontrar linha onde `strings.HasPrefix(line, "credential_guard:")` — coluna 0, **NÃO** `TrimSpace` (uma `credential_guard:` aninhada sob outra chave seria indentada e não casaria com a coluna-0 HasPrefix; usar TrimSpace abriria a mesma classe de cross-block do Achado 11). Antes do HasPrefix, strip BOM U+FEFF (3 bytes: 0xEF 0xBB 0xBF) do primeiro caractere do line 0 — arquivos Windows podem ter BOM; `TrimSpace` não strip BOM; sem strip, o check falha para a primeira linha de um arquivo BOMED.
2. Escanear as próximas N linhas (N=5 para compatibilidade), **parando antes** quando `TrimSpace(line) != ""` e a linha não começa com espaço nem `#` (nova seção YAML de topo — ver Achado 11); "vazia" é definida como `TrimSpace(line) == ""` para tolerar linhas `"\r"` em arquivos CRLF. Linha tab-indentada (e.g. `\tmode: block`): `TrimSpace(line) != ""` e não começa com espaço → stop rule dispara, não lida. YAML proíbe indentação com tab (spec §6.1); não ler `mode:` tab-indentado é correto.
3. Para cada linha: `trimmed = TrimSpace(line)`; skip se vazia ou começa com `#`; prosseguir apenas se `HasPrefix(trimmed, "mode:")` (exata chave de YAML, não infix via Contains)
4. Extrair valor: `val = TrimSpace(TrimPrefix(trimmed, "mode:"))`
5. Remover comentário trailing: se `#` em `val`, truncar antes **e** aplicar `TrimRight(resultado, " \t")` ← sem isso, `"mode: block  # comment"` → pós-truncar `"block  "` → switch não casa
6. Remover aspas: `Trim(val, "\"'")`
7. Remover `\r`: `TrimRight(val, "\r")`
8. Retornar val

Verificar ciclo de importação antes de compartilhar via pacote (guard e validator são pacotes distintos; uma opção é mover o parser para `internal/guardconfig`).

**Expressões D2 exatas (todas testadas em Go RE2, 2026-11-05):**

```go
// GitHub
ghp_[A-Za-z0-9]{36,}
gho_[A-Za-z0-9]{36,}
ghu_[A-Za-z0-9]{36,}
ghs_[A-Za-z0-9]{36,}
ghr_[A-Za-z0-9]{36,}
github_pat_[A-Za-z0-9_]{82,}

// OpenAI / Anthropic — boundary explicita em vez de \b (ver nota de boundary abaixo)
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-proj-[A-Za-z0-9_-]{20,}   // OpenAI project
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-ant-[A-Za-z0-9_-]{20,}    // Anthropic (hifens no body: api03-)
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-[A-Za-z0-9]{48,}          // OpenAI classico (evita infix FP "task-...")

// Google
AIza[0-9A-Za-z_-]{35,}           // {35,} nao {35}: unanchored regex sempre >=35

// Slack
xox[abprs]-[0-9A-Za-z-]{20,}

// PEM
-----BEGIN [A-Z ]*PRIVATE KEY-----
```

Expressao NAO recomendada: `sk-[A-Za-z0-9_-]{20,}` (generica com hifen, sem `\b`) — FP em "task-" + 20+ chars confirmado.

**Como aplicar os novos padroes D2 — boundary sem `\b` puro em `inputStr`:**

A camada 1 (`inputStr = string(data)`, L91) varre os bytes crus do JSON e não pode ser substituída por `contextStr` para os novos padrões: quando `tool_name` é "Write" (ou qualquer ferramenta sem campo "command"), `credExtractCmdAndCwd` retorna `shellCmd=""` → `contextStr=""` → padrões aplicados só a `contextStr` não detectariam credencial no conteúdo de um Write tool payload (confirmado por leitura do código em L105-109 e L576-578).

Apenas os três padrões `sk-*` usam `\b`. O problema: em `inputStr` (bytes crus JSON), `\n` é representado como dois chars `\` e `n`; `n` é word char → `\b` antes de `s` em `sk-ant-` não dispara após JSON `\n`. A correção não é mover para `contextStr` — é substituir `\b` por uma asserção de boundary explícita que entende escapes JSON:

```go
// Nao usar: \bsk-ant-[A-Za-z0-9_-]{20,}
// Usar:
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-ant-[A-Za-z0-9_-]{20,}
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-proj-[A-Za-z0-9_-]{20,}
(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-[A-Za-z0-9]{48,}
```

`[^A-Za-z0-9_]` cobre espaço, `=`, `"`, `,` e outros não-word chars reais. `\\[bfnrt]` cobre as sequências de dois chars `\b`, `\f`, `\n`, `\r`, `\t` como aparecem nos bytes JSON. **NÃO coberto**: a sequência `\uNNNN` imediatamente antes do prefixo — o char imediatamente antes de `s` é o último dígito hex (e.g. `&` = U+0026 → `6` antes de `s`, dígito ∈ word-class → nem `[^A-Za-z0-9_]` nem `\\[bfnrt]` disparam). Go's `json.Marshal` escapa `&`, `<` e `>` como `\uNNNN` por default (HTMLEscape=true); Claude Code usa `JSON.stringify` (Node.js) que **não** escapa esses caracteres — logo este false negative só ocorre com harnesses que usam o marshaller Go. Este caso entra em R7. RE2 aceita este grupo não-capturante.

Padrões sem boundary (ghp_, gho_, ghu_, ghs_, ghr_, github_pat_, xox, AIza, PEM) podem ser aplicados a `inputStr` sem mudança de spec; o prefixo é distintivo o suficiente.

Os padrões existentes JWT (`eyJ`) e AWS (`AKIA`) não usam `\b` e permanecem na camada 1 sem mudança.

**Expressoes boundary exactas (medidas em Go RE2, 2026-11-05 — 19/19 PASS com `\\[bfnrt]`; inclui vetores `\b`/`\f` MATCH e ampersand-encoded NOMATCH — ver R7b):**

```go
// Usar (backtick raw string — ver nota abaixo):
var skAntRe  = regexp.MustCompile(`(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-ant-[A-Za-z0-9_-]{20,}`)
var skProjRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-proj-[A-Za-z0-9_-]{20,}`)
var skRe     = regexp.MustCompile(`(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-[A-Za-z0-9]{48,}`)
```

**ATENÇÃO — backtick raw string obrigatório:** em Go double-quoted string, `\\[bfnrt]` vira `\[bfnrt]` (class literal). O regex compilado resultante é diferente e silenciosamente incorreto. Usar backtick para garantir `\\[bfnrt]` chegue ao compilador RE2 como dois chars `\` e `[bfnrt]`.

**Vetores Go RE2 medidos (19/19 PASS, 2026-11-05, binário commit b80bab34; programa de teste em package main com `regexp.MustCompile` + 19 `bvector` structs, backtick raw strings):**

```
# MATCH esperado:
{"command":"export FOO=bar\nsk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ01234"}  → MATCH  (\n como dois chars raw, confirmado)
"content":"sk-ant-api03-..."    → MATCH  (" e nao-word-char)
=sk-ant-api03-...               → MATCH  (= e nao-word-char)
sk-ant-api03-...                → MATCH  (inicio de string — ^)

# NOMATCH esperado (FP prevenido):
task-ant-api03-...              → NOMATCH  (char antes de s e 'a', word-class)
task-proj-...                   → NOMATCH
task-(5) + 48 A's (len=53)      → NOMATCH

# MATCH esperado novos: \b e \f (cobertos por \\[bfnrt]):
field\bsk-ant-...               → MATCH  (\b como dois chars raw backslash+b)
field\fsk-ant-...               → MATCH  (\f como dois chars raw backslash+f)

# NOMATCH esperado (residual R7b — JSON Unicode escape antes do prefixo):
# Input Go: "x" + string('\\') + "u0026sk-ant-..."
# Bytes: x, backslash, u, 0, 0, 2, 6, s, k, ...
# char imediatamente antes de 's' = '6' (word char) => NOMATCH
"x" + backslash + "u0026sk-ant-..."  → NOMATCH  ('6' e word char; residual declarado R7b)
```

**Vetor de prescricao (AC2 — distingue boundary JSON de boundary palavra):**

```
# payload: Bash com export FOO=bar\n (JSON) seguido de sk-ant-...
# \b nao dispara (n e word char); boundary explicita dispara (\n como dois chars)
printf '{"tool_name":"Bash","tool_input":{"command":"export FOO=bar\\nsk-ant-api03-ABCDEFGHIJKLMNOPQRSTUVWXYZ01234"}}' | trackfw guard credential
# → rc=2 com boundary explicita aplicada a inputStr (apos D2)
# → rc=0 com \b aplicado a inputStr (false negative — NAO usar)

# Write tool payload: confirma que inputStr deve cobrir (shellCmd="" para Write)
# (usar AWS como proxy — D2 sk-ant nao implementado; leitura do codigo confirma o mecanismo)
# credExtractCmdAndCwd(Write payload) → shellCmd=""; contextStr=""; Layer 1 (inputStr) detecta → rc=2
```

**Refatoracao segunda camada para table lookup:**
Substituir comparacoes `match == "JWT"` etc. por `map[string]*regexp.Regexp` indexado pelo nome do tipo. Assim, qualquer novo tipo adicionado fica automaticamente disponivel na segunda camada.

### 5.2 ML-1B — Guard de git: involucro de shell e backslash (apolo-tf) — AC4, AC5

**Arquivos afetados:**

| Arquivo | Mudança |
|---|---|
| `internal/guard/gitbranch.go` | Ver ações detalhadas abaixo |
| `README.md` | D4/D5: git-guard section — listar invólucros tratados, declarar wrappers como denylist aberto, declarar stdin e encodedCommand como residual |
| `docs/cli-parity.md` | D4/D5: git-guard section — mesma informação |
| Testes em `internal/guard/` | Vetores AC4 completos; vetor AC5 (`python -c subprocess` → rc=0, afirmando o LIMITE) |

Nota sobre README e cli-parity.md: ML-1A escreve a credential section; ML-1B escreve a git-guard/D5 section nos mesmos arquivos. Os dois MLs não escrevem nas mesmas linhas; a sequência ML-1A → ML-1B garante que ambas as seções existam antes do PR.

**Acoes em `gitbranch.go`:**

1. **POSIX backslash handling — duas mudancas coordenadas:**

   a. **`quoteAwareSplit`** (L1162–1163): alterar `cur.WriteRune(next)` para `cur.WriteRune('\\'); cur.WriteRune(next)` dentro do bloco `q == '"' && c == '\\'`. Atualmente `quoteAwareSplit` descarta o `\` antes de emitir o proximo char, causando decodificacao parcial. Com a mudanca, `quoteAwareSplit` apenas rastreia fronteiras de aspas e emite conteudo verbatim; `shellTokenize` faz a unica passagem de unescaping POSIX.

   b. **`shellTokenize`:** implementar backslash escape conforme POSIX:
   - Fora de aspas: `\x` → `x` (backslash remove-se, char seguinte e literal)
   - Dentro de aspas duplas: `\"` → `"`; `\\` → `\`; `\$` → `$`; `` \` `` → `` ` ``; outros `\x` → `x`
   - Dentro de aspas simples: backslash e literal (nao e escape)

   Esta correcao fecha Achado 7 (`\git push`, `g\it push` → ambos bloqueados) e habilita D4 depth=2 com `\"` correto (outer descodifica inner command string; inner descodifica subcomando).

2. **D4 — recursao de involucro em `MatchSubcommand`:** quando o primeiro token (apos stripEnvCommandPrefix e normalizacao) e um interpretador conhecido com flag de comando, aplicar `MatchSubcommand` recursivamente ao argumento do comando, com limite de profundidade=2 e fail-closed acima. Interpretadores e suas flags:
   - `bash`, `sh`, `zsh`, `dash`, `ksh`: flags `-c` separada OU flags combinadas (`^-[a-zA-Z]*c[a-zA-Z]*$`)
   - `pwsh`, `powershell`: `-c`, `-Command`, `-command`
   - `cmd`: `/c`, `/k`
   - `eval`: resto do trecho como argumento direto

3. **D4 — command string = primeiro argumento nao-opcao:** apos encontrar o shell interpreter, coletar tokens que sao flags (comecam com `-` ou `/`). Para flags combinadas (`-ec`, `-lc`), o command string e o proximo token. Para flags separadas, pular cada flag que nao e `-c` (e pular seus args se forem arg-consuming como `-o`). O primeiro token nao-flag apos todos os flags e o command string. Se nenhum token nao-flag existir, fail-closed.

4. **`stripEnvCommandPrefix` — fix de env, command, wrappers:**
   - `env`: apos remover "env", tambem remover `-i`, `--` antes de parar em outros flags ou `KEY=val`
   - `command`: apos remover "command", tambem remover `-p`, `-v`, `-V`, `--`
   - Adicionar: `nohup`, `time` (1 token, sem args)
   - Adicionar: `exec`: pular "exec" mais flags `-a NAME`, `-c`, `-l`, `--`; parar ao primeiro token que nao seja flag nem valor de flag
   - Adicionar: `nice`: pular "nice" mais flag opcional `-n ADJUSTMENT` (e forma longa `--adjustment=N`); pular o valor numerico que segue `-n`; parar ao primeiro token que nao seja flag nem valor de flag
   - Adicionar: `doas`: pular "doas" mais flags `-u USER`, `-C CONFIG`, `-n`, `--`; pular o valor de string que segue `-u`/`-C`; parar ao primeiro token que nao seja flag nem valor de flag
   - Adicionar: `timeout`: pular "timeout", pular flags com args (`-k DUR`, `-s SIG`) e flags sem args (`--foreground`, `--preserve-status`); pular o argumento de duracao (casando com `^[0-9]+(\.[0-9]+)?[smhd]?$`; nota: a expressao anterior `^[0-9]+\.?[0-9]*$` nao casa com sufixo de unidade como "10s")
   - Adicionar: `stdbuf` (1 token + flags com args: pular stdbuf e todos os flags com args ate o proximo nao-flag)
   - Atribuicoes nuas: antes do loop principal, pular tokens que casem com `^[A-Za-z_][A-Za-z0-9_]*=`
   - **Declarar na documentacao:** esta e uma denylist aberta. `ionice`, `taskset`, `nsenter`, `chroot`, `unshare` e outros wrappers de sistema seguem o mesmo mecanismo e nao estao cobertos.

**Profundidade D4: 2. Acima de 2: fail-closed (rc=2).**

### 5.3 Intersecao e sequencia

**Sequencia obrigatoria: ML-1A → ML-1B.**

ML-1A cobre toda a logica de credencial e a credential section da documentacao. ML-1B e Go-only para `gitbranch.go` mais a git-guard section da documentacao.

Arquivos exclusivos de ML-1A: `internal/guard/credential.go`, `internal/validator/validator_credential_guard_integrity.go`, `internal/generators/scaffold.go`, `internal/generators/scaffold_literal_copy_parity_test.go`.

Arquivos exclusivos de ML-1B: `internal/guard/gitbranch.go`.

Arquivos compartilhados (escritos em sequencia, sem conflito): `README.md`, `docs/cli-parity.md` — ML-1A escreve credential section, ML-1B escreve git-guard/D5 section.

Arquivos de testes: ML-1A escreve em `internal/guard/credential_*_test.go`; ML-1B escreve em `internal/guard/gitbranch_*_test.go`. Nenhum arquivo de teste e compartilhado entre os dois MLs.

O `.sh` gerado nao muda em nenhum ML. A thin shim e final. Os checksums em `guard_parity_helper_test.go` L425–426 nao precisam ser atualizados.

### 5.4 Contradicoes a reportar ao arquiteto

1. O contexto do ADR descreve `credentialGuardDetectionCore` e `credentialGuardModeResolution` como parte do script gerado. Falso: sao codigo morto; o script e thin shim.
2. No escopo global, o defeito CRLF e inverso: `mode: warn\r` cai no default "block" e bloqueia indevidamente.
3. As divergencias guard/validator para linha comentada (Achado 2) e chave de sufixo (Achado 3) nao estao na REQ atual como ACs — cobertas implicitamente por AC1, mas devem constar explicitamente na tabela do teste.
4. Os same-cause bypasses `env -i`, `env --`, `command -p`, `FOO=1` (Achado 8) e os wrappers `nohup`, `time`, `exec`, `timeout`, `nice`, `doas`, `stdbuf` (Achado 10) nao estao explicitamente na REQ; entram em ML-1B pela Regra Dura de Causa Raiz. O arquiteto deve confirmar a inclusao antes do despacho.
5. O bypass de backslash (Achado 7, `\git push` e `g\it push`) e pre-existente ao D4 e nao esta na REQ. Entra em ML-1B.
6. O fix POSIX de backslash requer mudança em DUAS funções: `quoteAwareSplit` (L1162–1163) deve emitir `\` + next verbatim (atualmente descarta `\`); `shellTokenize` faz o unescaping. Um fix em apenas uma das funções não fecha o vetor depth=2. As duas mudanças devem chegar no MESMO COMMIT. O arquiteto deve incluir esta instrução no handoff de ML-1B.
7. As expressões boundary explícitas `(?:^|[^A-Za-z0-9_]|\\[bfnrt])sk-...` devem ser compiladas via backtick raw string em Go. Em double-quoted string Go, `\\[bfnrt]` vira `\[bfnrt]` (class literal) e o regex é incorreto silenciosamente. Incluir esta restrição no handoff de ML-1A.
8. O parser compartilhado (spec 5.1) deve parar ao encontrar linha não-vazia cuja primeira coluna não seja espaço nem `#` (Achado 11 — cross-block boundary). Sem esta regra, `mode:` de seção irmã nos primeiros 5 linhas é lida como `credential_guard.mode`. Incluir vector de cross-block na tabela de AC1.
9. O step 1 do parser deve usar `strings.HasPrefix(line, "credential_guard:")` (coluna-0), **não** `HasPrefix(TrimSpace(line), ...)`. TrimSpace faria uma `credential_guard:` aninhada (indentada) ser reconhecida como header — a mesma classe de cross-block do Achado 11. Strip BOM U+FEFF do line 0 antes do HasPrefix — arquivos Windows podem ter BOM; `TrimSpace` não o remove. Incluir vetores de aninhado/BOM/tab na tabela de AC1.
10. R7b (separador `\uNNNN` antes do prefixo sk-) é realístico apenas para harnesses com Go `json.Marshal` (HTMLEscape=true). Claude Code usa `JSON.stringify` (Node.js) que não escapa `&`, `<` nem `>`. Este residual deve ser declarado com a ressalva de serializer no AC2 e na documentação de limites.
