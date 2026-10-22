# Wave 0 — Threat Model: Encoder TOML do destino Codex

**ML-4.0 | REQ-2026-09-26 | Agente: hades-tf | 2026-10-22**

---

## 1. Enumeration completeness

Os quatro campos do bloco `custom-agent-toml` são `name`, `description`, `model` e
`developer_instructions`. O roadmap lista as quatro superfícies de entrada. A
verificação abaixo confirma que a lista está fechada.

### Mapeamento entrada -> campo

| Campo TOML | Origem do valor | Caminho de código | Validação antes do encoder |
|---|---|---|---|
| `name` | Asset frontmatter `name:` (padrao `"trackfw-agent"`) | `markdownParts` -> `strings.ReplaceAll` -> `strconv.Quote` | Nenhuma |
| `name` (com identidade) | `identity.AgentName(id.Slug)` = `slug + "-tf"` | `identity.Load` (sem Validate) -> `AgentName` | `slugPattern ^[a-z0-9]+(-[a-z0-9]+)*$` so corre em `identity.Validate`; Load nao chama Validate |
| `description` | Asset frontmatter `description:` | `markdownParts` -> `strconv.Quote` | Nenhuma |
| `description` (com identidade) | `id.DisplayName + " - " + description` | `identity.Load` -> `containsControlCharIdent` | Rejeita U+0000-U+001F e U+2028/U+2029. **U+007F (DEL) nao e rejeitado** |
| `model` | `mapModelCodex(model)` -> constantes hardcoded | `render.go:220` | Constantes `"gpt-5.4"` e `"gpt-5.4-mini"` -- sem chars de controle por construcao |
| `developer_instructions` | Corpo do asset markdown | `markdownParts` -> `strconv.Quote` | Nenhuma |
| `developer_instructions` (com identidade) | `greetingLine(id, cfg.UserNickname) + "\n\n" + body` | `identity.Load` -> `containsControlCharIdent` | Mesma lacuna: U+007F nao rejeitado em DisplayName e UserNickname |

### Callers de BuildPlans -- nenhum chama Validate antes de BuildPlans

| Caller | Validate antes de BuildPlans? | Evidencia |
|---|---|---|
| `update.go:169` | Nao | `identity.Load` direto, sem Validate |
| `integrations_flags.go:224` | Nao | `identity.Load` direto, sem Validate |
| `integrations_flags.go:374` | Nao | Validate em L343 e de `applyIdentityPresetFlag`, path separado |
| `integrations_thirdparty.go:321` | Nao | `identity.Load` direto, sem Validate |
| `init.go:415` | Nao | Validate em L78 guarda o save do preset, nao o render em L415 |

### Assets embutidos (compile-time)

Doze arquivos em `internal/integrations/assets/agents/`. Verificados:
todos os 12 sem char de controle fora de TAB/LF/CR, todos UTF-8 valido.
Os bytes >= 0x80 sao sequencias UTF-8 multi-byte validas (ex.: EM DASH U+2014 em
descricoes, `Artemis` no catalogo). Lista fechada.

### Presets (`internal/identity/preset.go`)

DisplayName e Slug hardcoded. Slugs verificados contra `^[a-z0-9]+(-[a-z0-9]+)*$`:
todos os 12 presets (zeus, apolo, afrodite, artemis, ares, hades, poseidon, atena,
hefesto, metis, dedalo, prometeu) e todos os grupos Norse e HP passam. DisplayNames
limpos: ASCII imprimivel ou UTF-8 multi-byte printable (Artemis, Metis, Dedalo).

### Conteudo de terceiros

Nao alcanca `strconv.Quote`. `catalog.ReadAsset` em `plan.go:65` le exclusivamente
assets embutidos. Skills de terceiro (representacao `skill`) passam por
`normalizeMarkdown`, nao pelo ramo `custom-agent-toml`. Lista fechada.

### UTF-8 invalido e `json.Unmarshal`

`json.Unmarshal` substitui bytes UTF-8 invalidos por U+FFFD ao desserializar
`identity.json`. `json.Marshal(string([]byte{0xFF}))` produz `"�"`. Portanto,
bytes invalidos de UTF-8 lidos de `identity.json` chegam ao encoder como U+FFFD --
valido em UTF-8 e imprimivel, produzindo output TOML 1.0 valido sem intervencao do
encoder. Medido com `jsontest.go` no scratchpad.

### Geracao de `.codex/config.toml`

O trackfw nunca escreve `.codex/config.toml`. Apenas `validator_guard_wiring.go:676`
o le. Fora do escopo.

**A lista de superficies esta fechada.**

---

## 2. Threat model

O adversario e o **implementador apressado** e o **arquiteto otimista** -- quem
assume que "funciona hoje" e suficiente ou que o consumidor atual (Codex 0.163.0,
parser TOML 1.1) e o unico consumidor possivel.

### T-1 -- `strconv.Quote` nao e um encoder TOML

`strconv.Quote` usa a tabela de escapes do Go, nao a do TOML.

**Medido** (`scratchpad/toml-w0/probe.go`):

```
0x07 (BEL):   strconv.Quote -> "\a"
0x0B (VT):    strconv.Quote -> "\v"
0x01 (SOH):   strconv.Quote -> "\x01"
0x7F (DEL):   strconv.Quote -> "\x7f"
0x1B (ESC):   strconv.Quote -> "\x1b"
```

`\a` e `\v` sao escapes Go sem equivalente em qualquer versao do TOML -- invalidos
em TOML 1.0 e 1.1. `\xHH` e invalido em TOML 1.0 (valido apenas em 1.1 com
`spec-1.1.0`).

**Verificado com Python 3.12 tomllib (oracle TOML 1.0):**

```
aceita:  \n, \r, \t, \\, \", \uXXXX, \UXXXXXXXX
rejeita: \a, \v, \x01, \x7f, \e  (todos: "Unescaped '\' in a string")
```

**Codex 0.163.0 usa TOML 1.1** (declarado por metadados de crates no binario;
comportamento do loader de agentes nao medido diretamente):

```
toml-0.9.11+spec-1.1.0
toml_edit-0.24.0+spec-1.1.0
toml_parser-1.1.3+spec-1.1.0
```

**Consequencia:** TOML 1.1 aceita `\xHH` mas ainda rejeita `\a` e `\v`.

**Atencao sobre corrupcao silenciosa:** Python 3.15 (`tomllib` TOML 1.1) aceita
`\xff` e le de volta como U+00FF (y-grave), nao como o byte 0xFF. Testar com 3.15
e concluir "funciona" e ver corrupcao, nao correcao. Usar sempre 3.12 como oracle
TOML 1.0.

**O TOML 1.0 como piso e uma decisao de portabilidade** -- nao urgencia para o
Codex atual.

### T-2 -- Lacuna de validacao: U+007F chega via identidade ao campo `description`

**Medido separadamente no caminho real** (`TestZZT2Only`, worktree copiado):

Input: `{"schema_version":1,"agents":{"backend":{"display_name":"Zeus\u007fTF","slug":"zeus"}}}`

```
zz_t2_test.go:35: T-2-only: identity.Load PASSED slug="zeus" display_name="Zeus\x7fTF"
zz_t2_test.go:55: T-2-only output (first 300):
    name = "zeus_tf"
    description = "Zeus\x7fTF -- Senior backend specialist for APIs, domain logic, integrations and data access."
    model = "gpt-5.4-mini"
    developer_instructions = "You are Zeus\x7fTF.\n\n# Backend\n\n## Mode lock...
```

Python 3.12:
```
TOML 1.0 REJECTED: Unescaped '\' in a string (at line 2, column 22)
```

`containsControlCharIdent` (`identity.go:44-79`) rejeita `r < 0x20` mas nao 0x7F.
O valor passa Load, entra em `description` na linha 2, e `strconv.Quote` emite `\x7f`
-- invalido em TOML 1.0. Aceito pelo Codex 0.163.0 (TOML 1.1, metadata de crates)
-- portabilidade, nao crash imediato. Apos o fix, `\u007F` e emitido.

### T-3 -- Slug nao validado em Load: BEL (0x07) quebra o Codex hoje

**Medido separadamente no caminho real** (`TestZZT3Only`, worktree copiado):

Input: `{"schema_version":1,"agents":{"backend":{"display_name":"Apolo","slug":"a\u0007b"}}}`

```
zz_t3_test.go:35: T-3-only: identity.Load PASSED slug="a\ab" display_name="Apolo"
zz_t3_test.go:55: T-3-only output (first 200):
    name = "a\ab_tf"
    description = "Apolo -- Senior backend specialist for APIs, domain logic, integrations and data access."
    model = "gpt-5.4-mini"
    developer_instructions = "You are Apolo.\n\n# Backend\...
```

Python 3.12:
```
TOML 1.0 REJECTED: Unescaped '\' in a string (at line 1, column 12)
```

0x07 (BEL) no slug passa `identity.Load` (a funcao `containsControlCharIdent`
valida `DisplayName` e `UserNickname`, nao `slug`). `AgentName("a\u0007b")` =
`"a\u0007b-tf"`. `strconv.Quote` = `"a\ab_tf"` -- **invalido em TOML 1.0 e 1.1**,
rejeitado por qualquer parser TOML incluindo o Codex 0.163.0. **Esta e a rota live
que quebra o Codex com identity.json editado a mao.**

### T-4 -- `ApplyThirdPartyReferences` injeta HTML em TOML para todos os KindAgents

**Medido no caminho BuildPlans** (`TestZZProbe`, T-4, worktree copiado):

```
T-4: HTML comment in TOML output: true
```

Tail do output (`codex_t4_output.toml`):
```
<!-- trackfw:thirdparty-skills:start -->
- Third-party skill "test-skill": .agents/skills/thirdparty/test-skill.md (source: https://example.com/skill.md)
<!-- trackfw:thirdparty-skills:end -->
```

`plan.go:79` chama `ApplyThirdPartyReferences` para todo `KindAgents` sem verificar
`capability.Representation`. Para destino Codex, o output ja e TOML quando a funcao
e chamada. HTML em TOML e invalido. Medido via `BuildPlans` + `UpsertThirdPartyReference`
diretamente -- caminho de CLI `--apply-to codex` nao executado.

Mecanismo diferente de T-1/T-2/T-3 (injection, nao encoding). Mesmo sintoma: TOML
invalido entregue ao Codex. Recomendo ML adicional nesta REQ (Regra Dura). Decisao
de placement final fica com o arquiteto.

---

## 3. Falsificacao nas duas direcoes

### Especificacao do encoder TOML 1.0 basic string

Um unico helper `tomlBasicString(s string) (string, error)` para substituir as 4
chamadas de `strconv.Quote` (linhas 217, 218, 221, 223 de `render.go`).

**Regra: identico a `strconv.Quote`, exceto:**

| Input | strconv.Quote (atual) | Novo encoder |
|---|---|---|
| 0x07 (BEL) | `\a` -- invalido em qualquer TOML | `\u0007` |
| 0x0b (VT) | `\v` -- invalido em qualquer TOML | `\u000b` |
| 0x00-0x06, 0x0e-0x1f (outros C0) | `\xhh` -- invalido TOML 1.0 | `\u00hh` |
| 0x7f (DEL) | `\x7f` -- invalido TOML 1.0 | `\u007f` |
| 0xff / UTF-8 invalido | `\xff` -- invalido TOML 1.0 | `error` (veja nota AC7) |
| Todo o resto | correto | identico |

"Identico ao resto" inclui: `\uXXXX` para `!strconv.IsPrint` runes (NBSP, U+200B,
bidi overrides, U+FEFF). Goldens estaveis por construcao: os 12 assets tem apenas
chars que o novo encoder trata igual a `strconv.Quote`.

**Nota sobre AC7 e UTF-8 invalido:** AC7 diz "a saida para ... UTF-8 invalido e TOML
1.0 valido". Um `error` produz nenhuma saida, conflitando com AC7. Porem, a rota e
atualmente unreachable: `json.Unmarshal` ja substitui bytes invalidos por U+FFFD
antes de chegar ao encoder (medido em `jsontest.go`). O arquiteto deve decidir:

- **Rejeitar com `error` (recomendado por hades-tf):** alinha com o comentario
  `"callers must reject, not sanitize"` em `render.go:665-667`; requer amender AC7
  para "UTF-8 invalido e recusado com erro e nenhum arquivo e escrito".
- **Substituir por U+FFFD:** atende AC7 literalmente; alinha com `json.Unmarshal`.

Nenhuma das duas opcoes bloqueia ML-4A: a rota e unreachable hoje. O prometeu-tf
nao deve implementar substituicao sem instrucao explicita.

Checar com `r == utf8.RuneError && size == 1` para distinguir de U+FFFD legitimo
no input (evita falso positivo).

### Tabela de falsificacao

| Superficie | FN -- codigo antigo reprova | Arma que captura o FN | FP -- novo codigo rejeita valido | Arma que captura o FP |
|---|---|---|---|---|
| **T-1: strconv.Quote -> `\a`/`\v`** | Decoder inline TOML 1.0 rejeita `\a`, `\v` | Braco 1: decoder inline rejeit `"\a"`, `"\v"` | N/A: `\a`/`\v` nunca ocorrem em chars printable; encoder correto produz `\u0007`/`\u000b` | -- |
| **T-2: DEL via identidade** | `Render` com `DisplayName="Zeus\u007f"` + decoder inline rejeita `\x7f` no output | Braco 3: via `Render`, decoder rejeita | Fix adiciona `r >= 0x7F` a `containsControlCharIdent` -> rejeita Artemis, Metis, Dedalo (bytes UTF-8 > 0x7F). Fix correto: apenas `r == 0x7F` | Braco 2 (FP): `Render` com `DisplayName="Artemis"` -> decoder aceita |
| **T-3: slug BEL via identidade** | `Render` com `slug="a\u0007b"` + decoder inline rejeita `\a` | Braco 3 | Fix adiciona `Validate` a `Load` -> `slugPattern` rejeita slugs com maiusculas ou `_` que funcionam hoje (ex.: `"my_agent"`). Presets aprovados (verificado). | Braco 2 (FP): Load com `slug="a_b"` -> deve PASSAR |
| **Goldens -- zero churn** | N/A: assets sao limpos; encoder novo produz saida identica | Braco 4: `tomlBasicString(x) == strconv.Quote(x)` para todos os 12 assets + presets | Encoder novo diverge de `strconv.Quote` para chars printable legitimos | Braco 4 detecta divergencia desnecessaria |
| **Encoder: RuneError** | UTF-8 invalido atinge encoder -> `error` nao retornado | (unreachable hoje) | `RuneError && !size==1` rejeita U+FFFD legitimo no input | Teste com `"�"` literal -- deve passar sem erro |

### Alvos do teste de ML-4A

**Nome:** `TestRenderCodexTOMLEncoder` em `internal/integrations/render_test.go`.

**Bracos obrigatorios:**

1. **Decoder inline -- contra-check:** provar que o decoder rejeita `"\a"`, `"\v"`,
   `"\x7f"`, e raw 0x07 / 0x7F literais. Este braco e o que da valor ao braco 3:
   sem ele, braco 3 pode passar com decoder permissivo.

2. **FP -- encoder aprova input limpo + round-trip:**
   Tabela de (input, expected literal, decoded value). Inclui: ASCII imprimivel,
   `\"`, `\\`, U+2014 EM DASH, `\n`, U+FFFD, `"Artemis"`, `"Metis"`. Cada input:
   encoder -> literal esperado -> decoder aceita e devolve valor original.

3. **Caminho real via `Render`:**
   `identity.Config` com `slug="a\x07b"` e `DisplayName="Zeus\x7fTF"`.
   `Render(item_backend, KindAgents, capability_custom-agent-toml, source, cfg, "codex", nil)`.
   Com codigo antigo: decoder inline rejeita `\a` ou `\x7f` no output.
   Com codigo novo: decoder inline aceita.

4. **Goldens -- zero churn:** para todos os campos dos 12 assets com todos os presets,
   `tomlBasicString(x) == strconv.Quote(x)`. Prova por igualdade, nao recomputando golden.

**Decoder inline TOML 1.0** (sem nova dependencia):
Aceita apenas `\b \t \n \f \r \" \\` e `\uXXXX`/`\UXXXXXXXX`. Rejeita qualquer `\`
seguido de outro char, bytes 0x00-0x1F e 0x7F literais, surrogates `\uD800-\uDFFF`.
Falha ruidosamente.

---

## 4. Residuo declarado

1. **Codex 0.163.0 + TOML 1.1:** T-2 (`\x7f`) e aceito pelo Codex atual -- portabilidade.
   T-3 (`\a`) rejeita qualquer TOML. ML-4A fecha ambos. TOML 1.0 como piso cobre
   versoes anteriores sem medicao adicional.

2. **Versoes de Codex != 0.163.0:** apenas esta versao foi inspecionada. Os crates
   TOML 1.1 sao os mesmos para versoes contemporaneas; loader nao foi testado
   diretamente contra arquivos de agente com esses inputs.

3. **T-4 (HTML em TOML via ApplyThirdPartyReferences):** nao coberto por ML-4A.
   Medido via `BuildPlans` direto; CLI `--apply-to` nao executado. Recomendo ML
   adicional nesta REQ. Nao bloqueia ML-4A.

4. **YAML -- mesma causa, surfaces distintas:** `containsControlChar` em `render.go:679`
   (YAML) tambem nao cobre 0x7F, e `yamlEscapeDoubleQuoted` emite bytes raw C0 para
   slugs com 0x07, que e invalido em YAML double-quoted strings. C1 (U+0080-U+0084,
   U+0086-U+009F): `containsControlChar` nao cobre; para TOML nao e problema porque
   `strconv.Quote` emite `\uXXXX` para eles (IsPrint=false); para YAML esses bytes
   caem no mesmo path de C0 (raw byte invalido em `c-printable`). Mesma causa raiz:
   encoders de formato nao validam/transformam a gramatica do target. Recomendo ML
   adicional nesta REQ. Nao bloqueia ML-4A.

5. **`model` unreachable:** constantes ASCII hardcoded em `mapModelCodex`. Encoder
   aplicado por uniformidade e seguranca futura.

6. **AC7 UTF-8:** rota unreachable hoje (`json.Unmarshal` substitui antes). O encoder
   deve rejeitar com `error` para cobrir a extensao futura. O arquiteto deve amender
   AC7 ao auditar.

**Veredito: ML-4A pode seguir sem pre-condicao.**

**Arquivos afetados (ML-4A):**
- `internal/integrations/render.go`: substituir as 4 chamadas de `strconv.Quote`
  (linhas 217, 218, 221, 223) por `tomlBasicString(s string) (string, error)`.
  Callers propagam o `error`.
- `internal/integrations/render_test.go`: `TestRenderCodexTOMLEncoder` com os 4
  bracos acima.

---

## Anexo A -- Evidencia tecnica

### Probe source verbatim

Arquivo: `scratchpad/toml-w0/wt/internal/integrations/zz_probe_test.go`
(copia descartavel do worktree; NAO esta no worktree real)

```go
package integrations

// ZZProbe: throwaway ML-4.0 probe. NOT for real tests. In scratchpad copy only.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/identity"
)

func TestZZProbe(t *testing.T) {
	dir := t.TempDir()
	trackfwDir := filepath.Join(dir, ".trackfw")
	if err := os.MkdirAll(trackfwDir, 0700); err != nil {
		t.Fatal(err)
	}

	// identity.json with BEL in slug and DEL in display_name
	identCfg := map[string]interface{}{
		"schema_version": 1,
		"agents": map[string]interface{}{
			"backend": map[string]interface{}{
				"display_name": "Zeus\u007fTF",
				"slug":         "a\u0007b",
			},
		},
	}
	identData, _ := json.Marshal(identCfg)
	if err := os.WriteFile(filepath.Join(trackfwDir, "identity.json"), identData, 0600); err != nil {
		t.Fatal(err)
	}

	// T-2, T-3: identity.Load
	cfg, loadErr := identity.Load(dir)
	if loadErr != nil {
		t.Logf("T-2/T-3: identity.Load REJECTED: %v", loadErr)
	} else {
		agent := cfg.Agents["backend"]
		t.Logf("T-2/T-3: identity.Load PASSED slug=%q display_name=%q", agent.Slug, agent.DisplayName)
	}

	catalog, catErr := LoadCatalog()
	if catErr != nil {
		t.Skipf("LoadCatalog: %v", catErr)
	}

	var ident identity.Config
	if loadErr == nil {
		ident = cfg
	}

	// Render backend for codex with malformed identity
	plans, buildErr := BuildPlans(catalog, PlanRequest{
		Kind:     KindAgents,
		Targets:  []string{"codex"},
		Items:    []string{"backend"},
		Scope:    "project",
		Identity: ident,
	})
	if buildErr != nil {
		t.Logf("BuildPlans codex: error: %v", buildErr)
	} else if len(plans) > 0 {
		content := plans[0].Content
		excerpt := string(content)
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		t.Logf("BuildPlans codex T-2/T-3 output (first 200):\n%s", excerpt)
		scratchPath := "/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/c0992db2-d0d4-47c0-89e6-254582848291/scratchpad/toml-w0/codex_output.toml"
		os.WriteFile(scratchPath, content, 0600)
	}

	// T-4: thirdparty reference injection
	projRoot := t.TempDir()
	ref := ThirdPartyReference{
		Slug:        "test-skill",
		Destination: ".agents/skills/thirdparty/test-skill.md",
		URL:         "https://example.com/skill.md",
	}
	if upsertErr := UpsertThirdPartyReference(projRoot, "codex", "backend", ref); upsertErr != nil {
		t.Logf("T-4: UpsertThirdPartyReference error: %v", upsertErr)
	} else {
		plans4, _ := BuildPlans(catalog, PlanRequest{
			Kind: KindAgents, Targets: []string{"codex"}, Items: []string{"backend"},
			Scope: "project", Identity: ident, ProjectRoot: projRoot,
		})
		if len(plans4) > 0 {
			content4 := plans4[0].Content
			hasHTML := strings.Contains(string(content4), "<!-- trackfw:thirdparty-skills")
			t.Logf("T-4: HTML comment in TOML output: %v", hasHTML)
			os.WriteFile("/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/c0992db2-d0d4-47c0-89e6-254582848291/scratchpad/toml-w0/codex_t4_output.toml", content4, 0600)
		}
	}

	_ = fmt.Sprintf
}
```

### T-2 medido separadamente (slug limpo)

`TestZZT2Only` -- input: `display_name:"Zeus\u007fTF"`, `slug:"zeus"`:
```
zz_t2_test.go:35: T-2-only: identity.Load PASSED slug="zeus" display_name="Zeus\x7fTF"
zz_t2_test.go:55: T-2-only output (first 300):
    name = "zeus_tf"
    description = "Zeus\x7fTF -- Senior backend specialist for APIs, domain logic, integrations and data access."
    model = "gpt-5.4-mini"
    developer_instructions = "You are Zeus\x7fTF.\n\n# Backend\n\n## Mode lock...
```

Python 3.12 no arquivo `codex_t2only.toml`:
```
TOML 1.0 REJECTED: Unescaped '\' in a string (at line 2, column 22)
```

### T-3 medido separadamente (display_name limpo)

`TestZZT3Only` -- input: `display_name:"Apolo"`, `slug:"a\u0007b"`:
```
zz_t3_test.go:35: T-3-only: identity.Load PASSED slug="a\ab" display_name="Apolo"
zz_t3_test.go:55: T-3-only output (first 200):
    name = "a\ab_tf"
    description = "Apolo -- Senior backend specialist for APIs, domain logic, integrations and data access."
    model = "gpt-5.4-mini"
    developer_instructions = "You are Apolo.\n\n# Backend\...
```

Python 3.12 no arquivo `codex_t3only.toml`:
```
TOML 1.0 REJECTED: Unescaped '\' in a string (at line 1, column 12)
```

### Codex 0.163.0 crates TOML 1.1

```
strings /opt/homebrew/Caskroom/codex/0.163.0/bin/codex | \
  grep -oE 'toml[a-z_-]*-[0-9][0-9A-Za-z.+_-]*' | sort -u

toml-0.9.11+spec-1.1.0
toml_datetime-0.7.5+spec-1.1.0
toml_edit-0.24.0+spec-1.1.0
toml_parser-1.1.3+spec-1.1.0
toml_writer-1.0.6+spec-1.1.0
```

### `git status` no worktree real

```
?? docs/seguranca/2026-10-22-wave0-toml-escape.md
```

Nenhum arquivo de produto modificado.

---

*Hades, Security Reviewer -- ML-4.0 Wave 4 -- 2026-10-22*
