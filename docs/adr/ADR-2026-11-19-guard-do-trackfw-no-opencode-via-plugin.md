---
status: Accepted
date: 2026-11-19
author: "zeus-tf (decisões do KG em 2026-11-19)"
---

# ADR: guard do trackfw no OpenCode via plugin

> Date: 2026-11-19 | Status: Accepted

## Context

O OpenCode é o único alvo do catálogo sem guard (`internal/generators/hooks.go:16`; issue #691). Por isso, sob o
OpenCode, `git commit` na `main` passa e um segredo no comando não é barrado. A REQ-2026-11-17 absorveu o caso na
Wave 5, aplicando a Regra Dura de Causa Raiz (decisão do KG em 2026-11-19).

O mecanismo é diferente dos outros CLIs. O OpenCode não tem hook JSON + shell: ele carrega plugins JS/TS que rodam no
Bun. Fontes:
- contrato: `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md` (§1–§7 por fonte primária, §8 medido);
- threat model: `docs/seguranca/2026-11-19-wave5-opencode-threat-model.md`.

Medido no opencode **2.3.2**, no macOS e no Windows 11 ARM64, com o resultado igual nas duas máquinas (contrato §8):
- `throw` no `tool.execute.before` bloqueia a ferramenta;
- o plugin de projeto (`.opencode/plugins/`) e o global (`~/.config/opencode/plugins/`, no Windows
  `%USERPROFILE%\.config\opencode\plugins\`) são carregados sem nenhuma config;
- com `trackfw` fora do PATH, `spawnSync` dá `ENOENT`, e com `throw` nesse caso a ferramenta fica bloqueada;
- `read` de um arquivo com segredo passa, porque os args da ferramenta só trazem o caminho;
- `OPENCODE_CONFIG_PROJECT_DISABLE=1` desliga o plugin de projeto, e o global continua valendo;
- o payload no formato `{tool_name, tool_input, cwd}` já é reconhecido pelos dois guards: rc=2 para chave fake,
  `git commit`, `git add -A` e `git push`, e rc=0 para `ls` (medido sem modelo).

**Medido depois (ML-5H, 2026-11-20):** o plugin do produto bloqueia `shell` com segredo, `git commit`, `read` de arquivo
com segredo, `write` na pasta de plugins e `webfetch` com segredo, no projeto e no global (contrato §8.1).

## Decision

**D1 — Um arquivo de plugin, gerado pelo Go.**
- O conteúdo é `trackfw-guard.js`: CommonJS, sem dependência npm, embutido no binário com `go:embed`.
- O arquivo começa com o marcador `// trackfw:opencode-guard`, que o `validate` e o `doctor` usam para reconhecer e
  auditar. A única diferença entre projeto e global é a constante `SCOPE` (`"project"` ou `"global"`); fora isso, o
  conteúdo é igual.
- **A API suportada é a V2** (`setup`), que é a medida. O export V1 (`server`) continua no arquivo porque custa uma
  função, mas é declarado **não medido** (decisão do KG, opção a). A doc não diz "suporta 1.x".

**D2 — Como o plugin chama o guard.**
- O plugin roda `spawnSync("trackfw", ["guard", <guard>, ...])` com argv, **nunca por shell** (aviso D18 do threat
  model). O payload vai em JSON no stdin, com timeout de 30 s.
- Ele bloqueia com `throw` se `result.error || result.status !== 0`, cobrindo `ENOENT`, timeout e sinal: falha fecha.
  A mensagem do erro é o stderr do guard.
- Com `SCOPE="global"`, o guard de credencial recebe `--global`.
- O plugin não escreve log. O `TRACKFW_PROBE_LOG` era só da sonda.

**D3 — Payload e cobertura.**
- O payload é `{"hook_source":"opencode","tool_name":<nome>,"tool_input":<args, com "file_path" absoluto quando houver
  caminho>,"cwd":<raiz absoluta do projeto>}`.

| Ferramenta (V2 e V1) | Guard |
|---|---|
| `shell`, `bash` | credencial e git-branch |
| `write`, `edit`, `patch`, `apply_patch` | credencial |
| `read` | credencial |

- No guard (Go), o `read` funciona como o `view_file` do Antigravity (ADR-2026-11-18 D5). A camada 2 lê
  `tool_input.file_path` **somente** quando `hook_source == "opencode"` e `tool_name == "read"`. É checagem de schema
  explícita, não prioridade de campo (lição do F1). O arquivo precisa ser regular e vale o mesmo limite de tamanho.
  Isso fecha o gap medido no C4.

**D4 — O agente não desliga o próprio guard (T7).**
- O plugin nega `write`/`edit`/`patch`/`apply_patch` cujo alvo resolvido seja:
  - o próprio arquivo de plugin (projeto ou global);
  - `opencode.json` ou `opencode.jsonc` do projeto ou do global.
- `.opencode/plugins/` e `opencode.json`/`opencode.jsonc` entram em `writeScopeSubagentFrozenRelPaths`, para que
  subagentes do Claude não os editem. O write-scope continua só no Claude. Por isso o T13 (nomes em minúsculas) não
  abre brecha aqui, já que o OpenCode não passa pelo write-scope.
- Apagar o plugin por `shell` (`rm`) não é barrado: residual.

**D5 — Geração e auditoria, no mesmo caminho.**
- **Projeto:**
  - `cliDetectors["opencode"]` detecta o diretório `.opencode/`;
  - `InjectOpenCodePlugin(rootDir)` grava `.opencode/plugins/trackfw-guard.js` com `pathguard`;
  - o teste `contra_opencode_still_skips` é invertido nesse mesmo ML.
- **Global:**
  - o alvo `opencode-guard` do `update harness` grava `<config>/plugins/trackfw-guard.js`;
  - `<config>` é `OPENCODE_CONFIG_DIR` se definido, senão `XDG_CONFIG_HOME/opencode` se definido, senão
    `<home>/.config/opencode`;
  - `<home>` é `USERPROFILE` no Windows (medido) e `HOME` nos demais (`agentconfig.CLIOpenCode`);
  - quando o arquivo falta, o `update harness` reporta **`missing`**. É o sinal que o #691 mostrou não existir.
- **Auditoria:** `validate` e `doctor` auditam os mesmos dois caminhos. Arquivo ausente, sem o marcador ou diferente do
  canônico gera a mesma severidade dos outros guards ausentes ou desatualizados.
- `auditsurface.canonicalRuntimes` inclui o OpenCode.

**D6 — O plugin de projeto pode ser desligado (C5).** O `doctor` informa quando há plugin de projeto e não há o global,
e recomenda `trackfw update harness --targets opencode-guard`. Não entra regra no `validate`, porque ela dispararia
para todo projeto sem global e viraria ruído.

**D7 — Correções do ML-5F e do ML-5G (2026-11-19).** A revisão de qualidade e o red team reprovaram o D3 e o D4 como
estavam implementados. Os ajustes:

- **D4 ampliado:**
  - A autoproteção nega escrita em **qualquer arquivo** sob os diretórios de plugin, não só no `trackfw-guard.js`:
    `.opencode/plugins/`, `.opencode/plugin/`, `<config>/plugins/` e `<config>/plugin/`. Também nega escrita em
    `opencode.json`/`opencode.jsonc` na raiz do projeto, em `.opencode/` e em `<config>`.
  - A comparação passa a ser por identidade de arquivo: `realpath` do alvo, ou do diretório pai quando o alvo ainda não
    existe, e comparação sem caixa em sistemas que não distinguem maiúsculas (F2/F8).
  - Os alvos de `patch`/`apply_patch` são extraídos dos cabeçalhos do texto do patch (`*** Add/Update/Delete File:`,
    `*** Move to:`, `+++ `/`--- `) (A7/F5).
  - A precedência `filePath`/`path` passa a ser a mesma em todo o plugin (F7).
  - `writeScopeSubagentFrozenRelPaths` ganha `.opencode/plugin/`, `.opencode/opencode.json` e
    `.opencode/opencode.jsonc` (F1/F3).
- **Detecção do que a autoproteção não cobre (F4):** escrita por `shell` dentro do diretório de plugin não é barrada,
  porque o plugin não interpreta comando de shell. Em compensação, o `validate`/`doctor` acusam qualquer arquivo
  `.js`/`.ts` nos diretórios de plugin do projeto e do global que não seja o `trackfw-guard.js`, e qualquer chave
  `plugin`/`plugins` em `opencode.json(c)`. A severidade é a mesma do plugin adulterado.
- **D3 ampliado (F6):**
  - `webfetch` passa pelo guard de credencial (a URL vai no `tool_input`).
  - `grep` também passa pelo guard. Em Go, `hook_source=opencode && tool_name=grep` com `tool_input.path` apontando
    para arquivo regular é lido como no `read`. Diretório não é lido: residual.
  - As demais ferramentas nativas da V2 que carregam conteúdo ou caminho de leitura, listadas no contrato, ficam
    mapeadas ou declaradas residual uma a uma.

**D8 — Limite da autoproteção por caminho (2026-11-19, após três rodadas de red team).** A autoproteção do D4/D7
compara caminhos e textos. Ela é uma **camada a mais**, não o controle principal. O agente já consegue escrever na
pasta de plugins por `shell` (residual desde o D4), e nenhuma defesa por texto fecha isso. O controle principal contra
plugin estranho é a **detecção**: o `validate`/`doctor` acusam qualquer arquivo de plugin que não seja o
`trackfw-guard.js` e qualquer chave `plugin(s)` no `opencode.json(c)`, no projeto e no global, com ou sem `.opencode/`
no projeto. Por isso:

- **Entram e são corrigidos** os contornos baratos, em que a mudança é local e tem teste:
  - patch que cria symlink (`new file mode 120000`/`new mode 120000`) é negado;
  - symlink pendurado é resolvido contra o pai real;
  - a comparação normaliza para NFC antes de ignorar caixa (macOS);
  - é negado o alvo que contém um caminho protegido, como apagar ou renomear a pasta `.opencode`;
  - a detecção do global vale mesmo sem `.opencode/` no projeto.
- **Fica declarado como residual, sem nova rodada:** contorno de patch que exige montar o caminho por partes, como
  cadeia intermediária de symlinks ou dois patches que se completam, ou outra técnica que nenhuma comparação de texto
  fecha. O efeito é o mesmo da escrita por `shell`, e o controle é a detecção.
- **Falso positivo aceito:** um patch que só cite `opencode.json` ou `.opencode/plugin` no texto (documentação, nome
  parecido) é negado. O agente pode editar esses arquivos por `write`/`edit` fora dos caminhos protegidos.

## Consequences

- O OpenCode passa a ter guard de credencial e de git-branch, no projeto e no global, com `missing` visível.
- O guard ganha um schema de payload, o `hook_source: opencode`. Os testes de schema das outras CLIs fixam que nada
  muda para elas.

**Residual:**
- OpenCode 1.x (`opencode-ai`, API V1) não foi medido no binário.
- MCP, `task` (subagente) e ferramentas de plugin de terceiro podem não passar pelo `execute.before` (T9, não medido).
- `edit` e `patch` não foram medidos no binário (`write` foi, no P5 do ML-5H).
- `--pure` (1.x) e `OPENCODE_CONFIG_PROJECT_DISABLE=1` desligam o plugin de projeto. A proteção nesse caso é o global.
- O `execute.after` não bloqueia na V2, então não há redação de saída.
- O agente pode apagar o plugin por `shell`.
- O agente pode escrever no diretório de plugin por `shell`. Não é barrado, só detectado pelo `validate`/`doctor` (D7).
- `grep` sobre diretório não tem o conteúdo varrido.
- Um `trackfw` anterior a esta REQ no PATH bloqueia o comando com segredo, mas não lê o arquivo do `read`. A versão sozinha
  não distingue os dois binários (VS do red team).
- O D5 e a auditoria do plugin são warning-only, sem escalada via `rules:` (A9), como o `gemini_cli_legacy`.

## Alternatives Considered

- **Plugin chamando o guard por shell** (`sh -c`/`cmd /c`): rejeitada. A linha poliglota aborta com `errexit`/`nounset`
  herdados e falha aberta (D18).
- **Plugin com `@opencode-ai/plugin` como dependência:** rejeitada, por supply chain (T11). O arquivo solto foi medido
  e funciona.
- **Regra de `validate` para projeto sem plugin global:** rejeitada, porque seria ruído. Fica só no `doctor`.
- **Ligar o write-scope ao OpenCode:** fora desta REQ. Exigiria resolver os cinco pré-requisitos do T13.
