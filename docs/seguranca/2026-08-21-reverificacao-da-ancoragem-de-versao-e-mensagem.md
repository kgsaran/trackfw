---
status: reviewed
date: 2026-08-21
reviewer: "hades-tf"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-21-release-tag-ancora-versao-e-mensagem-no-forge.md"
adr: "docs/adr/ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md"
ml: "ML-4B"
verdict: "BLOQUEIO LEVANTADO — exploit refs/replace/ fechado nos 3 CLIs, medido via gate; nenhuma terceira camada de indireção identificada; os.Environ() bruto sustenta argumento do ML-4A"
---

# ML-4B — Reverificação: `--no-replace-objects` nos 3 CLIs

## Veredito

**BLOQUEIO LEVANTADO.**

O exploit `refs/replace/` está fechado. Medido via gate (`check-release-tag-parity.sh` Scenario 17)
nos 3 runtimes (go, node, py). Nenhuma terceira camada de indireção do git abre o caminho de
forjaria dentro do escopo desta série. O argumento do ML-4A sobre `os.Environ()` bruto sustenta: o
pior caso com `GIT_DIR` redirecionado é recusa, não forjaria.

Há uma assimetria de profundidade de garantia declarada no final como dívida nomeada (não
bloqueante).

---

## 1. O exploit do ML-3A está fechado?

### 1.1 Fix confirmado por leitura dos 3 CLIs

| CLI | Arquivo | Linha | Invocação atual |
|-----|---------|-------|-----------------|
| Go | `internal/commands/release.go` | 224 | `exec.Command("git", "--no-replace-objects", "show", sha+":"+path)` |
| Node.js | `npm/src/release/runner.js` | 161 | `spawnSync('git', ['--no-replace-objects', 'show', ...])` |
| Python | `pypi/trackfw/release/runner.py` | 239 | `subprocess.run(["git", "--no-replace-objects", "show", ...])` |

A flag está na posição correta (primeiro argumento após `git`) nos três.

### 1.2 Gate executado — todos os 3 runtimes, medido

```
bash scripts/check-release-tag-parity.sh
```

Resultado completo (21 cenários, exit 0):

```
OK   [release-tag-parity/dirty-tree]
OK   [release-tag-parity/main-stale]
OK   [release-tag-parity/version-mismatch-go]
OK   [release-tag-parity/version-mismatch-npm]
OK   [release-tag-parity/version-mismatch-pyproject]
OK   [release-tag-parity/version-mismatch-init-try]
OK   [release-tag-parity/version-mismatch-init-except]
OK   [release-tag-parity/changelog-missing]
OK   [release-tag-parity/tag-exists-local]
OK   [release-tag-parity/tag-exists-remote]
OK   [release-tag-parity/no-forge-cli]
OK   [release-tag-parity/unsupported-forge]
OK   [release-tag-parity/git-identity-missing]
OK   [release-tag-parity/success]
OK   [release-tag-parity/forge-symref-repoint-neutralized]
OK   [release-tag-parity/forge-commit-diverges-update-ref]
OK   [release-tag-parity/forge-commit-diverges-narrowed-fetch]
OK   [release-tag-parity/forge-local-ref-absent-success]
OK   [release-tag-parity/object-absent]
OK   [release-tag-parity/content-from-commit-provenance]
OK   [release-tag-parity/refs-replace-bypass]

All check-release-tag-parity.sh scenarios passed.
```

`refs-replace-bypass` (Scenario 17) passou em go, node e py — a linha `for runtime in go node py`
em `check-release-tag-parity.sh:1438` confirma que os 3 runtimes foram exercidos. O `assert_three_way`
emite uma única linha de OK apenas quando os 3 passam byte a byte. Isso converte Node.js e Python de
"inferido por leitura" para medido.

O gate usa:
- `HOME` e `GIT_CONFIG_GLOBAL` sobrepostos em diretório temporário isolado
- Remote bare local — nenhuma rede, nenhum repositório real tocado
- Guard de não-vacuidade (linhas 126–131) que rejeita o cenário se `gh` vazar para o PATH

O ataque do Scenario 17 instala `.git/refs/replace/<forge-sha>` como escrita direta de arquivo
(sem comando git, idêntico ao vetor do ML-3A). O teste confirma que os 3 CLIs:
- Leem "forge-only" do commit forge (não do conteúdo do atacante)
- Publicam a tag com `tagMessage` contendo "forge-only" (não "refs-replace-forged")

### 1.3 Não há fallback para working tree

Confirmado por leitura dos 3 CLIs: os callers de `readCommittedFile` / `readAtCommit` /
`read_at_commit` propagam o erro imediatamente para `releaseTagObjectAbsentFmt` sem alternativa de
leitura local. Isso é adicionalmente coberto pelo `object-absent` scenario (linha 19 do gate).

---

## 2. Há outra camada de indireção do git no mesmo caminho?

### 2.1 Metodologia

Fixture isolada em scratchpad (HOME/GIT_CONFIG_GLOBAL/GIT_CONFIG_SYSTEM sobrepostos, sem hooks de
projeto). Dois repositórios:
- `legit/`: commit com `CHANGELOG: REAL-CONTENT`, sha `baad5d12...`
- `decoy/`: commit com `CHANGELOG: ATTACKER-CONTROLLED`, sha `ab9799f6...`

Cada teste executa `git --no-replace-objects show <legit-sha>:CHANGELOG.md` com a variável de
ataque ativa e observa se o output contém "ATTACKER" (forjaria) ou não.

### 2.2 Resultados medidos

**Teste A — `GIT_DIR` redirecionado para o repo do atacante:**

```bash
GIT_DIR="$DECOY/.git" git --no-replace-objects show "$LEGIT_SHA:CHANGELOG.md"
# Saída: fatal: path 'CHANGELOG.md' exists on disk, but not in 'baad5d12...'
# Exit: não-zero
```

Resultado: recusa (exit não-zero, sem conteúdo do atacante). O sha legítimo não existe no decoy; git
retorna erro. Go transforma isso em `releaseTagObjectAbsentFmt` — comportamento de recusa, lado
seguro.

**Teste B — `GIT_ALTERNATE_OBJECT_DIRECTORIES` apontando para objects do atacante:**

```bash
GIT_ALTERNATE_OBJECT_DIRECTORIES="$DECOY/.git/objects" git --no-replace-objects show "$LEGIT_SHA:CHANGELOG.md"
# Saída: CHANGELOG: REAL-CONTENT
```

Resultado: conteúdo legítimo. Alternates adicionam fontes de objetos mas não podem forjar: git é
content-addressed, o sha `baad5d12...` só pode ser satisfeito pelo objeto cujo conteúdo o produz.
O conteúdo do atacante tem sha diferente (`ab9799f6...`) e não é servido para o sha legítimo.

**Teste C — `GIT_REPLACE_REF_BASE` apontando para diretório controlado pelo atacante:**

```bash
# Arquivo criado: $WORK/fake-replace/<legit-sha> → conteúdo: <decoy-sha>
GIT_REPLACE_REF_BASE="$WORK/fake-replace" git --no-replace-objects show "$LEGIT_SHA:CHANGELOG.md"
# Saída: CHANGELOG: REAL-CONTENT
```

Resultado: `--no-replace-objects` neutraliza completamente `GIT_REPLACE_REF_BASE`. A flag desabilita
o mecanismo de substituição independentemente de onde os refs de substituição residem.

**Teste D — `GIT_CONFIG_COUNT` injetando `core.useReplaceRefs=true` com replace ref ativo:**

```bash
# Replace ref instalado: .git/refs/replace/<legit-sha> → <decoy-sha>
GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="core.useReplaceRefs" GIT_CONFIG_VALUE_0="true" \
  git --no-replace-objects show "$LEGIT_SHA:CHANGELOG.md"
# Saída: CHANGELOG: REAL-CONTENT
```

Resultado: a flag de linha de comando `--no-replace-objects` sobrepõe a injeção de config via
variáveis de ambiente. Configurações de ambiente não podem re-ativar o mecanismo que a flag desabilita.

**Teste E — `core.hooksPath` apontando para diretório controlado pelo atacante:**

```bash
git -c "core.hooksPath=$EVIL_HOOKS" --no-replace-objects show "$LEGIT_SHA:CHANGELOG.md"
# Saída: CHANGELOG: REAL-CONTENT
```

Resultado: hooks não afetam stdout de `git show`. `git show` não executa hooks de repositório.

### 2.3 Variáveis não testadas diretamente (inferidas)

**`objects/info/alternates` (arquivo):** equivalente funcional de `GIT_ALTERNATE_OBJECT_DIRECTORIES`
— adiciona fontes de objetos. Mesma conclusão do Teste B: objetos são content-addressed, nenhum
alternate pode fornecer um objeto forjado para um sha determinado. Inferido; não medido.

**Promisor/partial clone:** em clones parciais, objetos ausentes são buscados do promisor remoto
sob demanda. O promisor é configurado em `.git/config` e não é acessível via variável de ambiente
pelo processo atacante no mesmo padrão local assumido. Mais importante: objetos buscados do promisor
devem ainda ter hash correspondente ao sha requisitado — content-addressing garante que o promisor
não pode servir conteúdo forjado para um sha determinado. Mesmo se o promisor fosse comprometido, o
resultado seria objeto ausente ou hash incorreto (rejeitado pelo git). Inferido por mecanismo; não
medido.

### 2.4 Conclusão

Nenhuma terceira camada de indireção identificada. A taxonomia:

| Mecanismo | Camada afetada | Afeta `git show <sha>:<path>`? | Status |
|-----------|----------------|-------------------------------|--------|
| `refs/replace/` | Identidade do objeto | Sim (bloqueado por `--no-replace-objects`) | Medido — fechado |
| `.git/info/grafts` | Grafo de pais | Não (não toca a árvore) | Medido em ML-4A — não é vetor |
| `GIT_ALTERNATE_OBJECT_DIRECTORIES` | Fontes de objetos | Não (content-addressed) | Medido — não é vetor |
| `GIT_REPLACE_REF_BASE` | Localização dos replace refs | Não (bloqueado por `--no-replace-objects`) | Medido — não é vetor |
| `GIT_CONFIG_*` env vars | Configuração | Não (flag de CLI sobrepõe config) | Medido — não é vetor |
| `core.hooksPath` | Execução de hooks | Não (git show não executa hooks) | Medido — não é vetor |
| `objects/info/alternates` | Fontes de objetos | Não (content-addressed) | Inferido — não é vetor |
| Promisor/partial clone | Busca de objetos remotos | Não (hash validation) | Inferido — não é vetor |

---

## 3. O argumento sobre `os.Environ()` bruto se sustenta?

### 3.1 Contexto

`defaultReleaseReadCommittedFile` em `internal/commands/release.go` herda `os.Environ()` bruto —
sem passar por `cleanGitEnv()` (que existe em `internal/validator/validator_git_exec.go` e remove
variáveis `GIT_*`). O argumento do ML-4A: para leitura por sha, `GIT_DIR` redirecionado torna o
objeto ausente (recusa), não forjado — lado seguro.

### 3.2 Medição

O Teste A acima mediu diretamente: `GIT_DIR` redirecionado para o decoy produz exit não-zero sem
conteúdo do atacante. O Go transforma isso em `releaseTagObjectAbsentFmt` — comportamento de
recusa.

Os Testes C e D mostraram que o vetor mais perigoso (`GIT_REPLACE_REF_BASE` + `GIT_CONFIG_*` para
re-ativar replace) é anulado pela flag de linha de comando.

### 3.3 Conclusão

O argumento sustenta. Com `--no-replace-objects` como argumento de linha de comando:

1. `refs/replace/` está desabilitado — independente de qualquer variável de ambiente.
2. `GIT_DIR` redirecionado → sha ausente no decoy → recusa (não forjaria).
3. `GIT_REPLACE_REF_BASE` → neutralizado pela flag.
4. `GIT_CONFIG_*` injetando `core.useReplaceRefs=true` → sobreposto pela flag.
5. Alternates → content-addressed, não podem forjar.

O pior caso que `os.Environ()` bruto pode produzir é uma **recusa**, não um objeto forjado. Isso é
comportamento seguro. A distinção entre esta função e o validador (que usa `cleanGitEnv()`) é real
mas não é uma diferença de risco para P3/P4: o validador protege leituras `HEAD:path`
(ref-addressed, onde `GIT_DIR` pode fazer o HEAD apontar para outra branch), enquanto P3/P4 usam
sha-addressed (onde `GIT_DIR` só pode tornar o objeto ausente).

A ausência de `cleanGitEnv()` em `defaultReleaseReadCommittedFile` permanece como dívida técnica
de defesa em profundidade, mas não é uma vulnerabilidade ativa. Não bloqueia.

---

## 4. Achados novos

Nenhum vetor novo identificado.

---

## 5. Dívida nomeada (não bloqueante)

### 5.1 Cenário 158 saboteia apenas Go

`check-gates-falsify.sh` Cenário 158 remove `--no-replace-objects` do código Go, reconstrói e
verifica que o Scenario 17 falha. Isso prova que a flag é load-bearing no Go. Não há um braço de
falsificação equivalente para Node.js e Python: o Cenário 158 não remove a flag de `runner.js` ou
`runner.py` e não reconstrói esses runtimes.

O Scenario 17 do gate (que passou nos 3 runtimes nesta sessão) serve como verificação de que a flag
está presente e funcional nos 3 CLIs. Mas se a flag for removida acidentalmente de `runner.js` ou
`runner.py`, o Cenário 158 não detectaria — e o Scenario 17 poderia ser a única barreira, dependendo
de se o conteúdo esperado muda.

Esta é a mesma assimetria de profundidade de garantia notada em ML-3A ("não compila cobre só Go"):
a remoção de compilador não detecta regressão em Node/Python, e o Cenário 158 não detecta remoção
de `--no-replace-objects` em Node/Python. Named debt, não defect.

### 5.2 `os.Environ()` bruto em `defaultReleaseReadCommittedFile`

Descrito na seção 3. A limpeza de env via `cleanGitEnv()` está disponível no codebase mas não é
aplicada aqui. Dívida de defesa em profundidade; não é vulnerabilidade ativa dado o contexto
sha-addressed + `--no-replace-objects`.

---

## 6. Medido vs. Inferido

**Medido, com fixture nova isolada e gate executado:**
- `--no-replace-objects` nos 3 CLIs: leitura direta das funções nas 3 implementações.
- Scenario 17 (`refs-replace-bypass`) verde nos 3 runtimes: `check-release-tag-parity.sh` exit 0,
  confirmado pela linha `for runtime in go node py` em `check-release-tag-parity.sh:1438`.
- `GIT_DIR` redirecionado → recusa (Teste A).
- `GIT_ALTERNATE_OBJECT_DIRECTORIES` → conteúdo legítimo (Teste B).
- `GIT_REPLACE_REF_BASE` → neutralizado por `--no-replace-objects` (Teste C).
- `GIT_CONFIG_COUNT` injetando `core.useReplaceRefs=true` → sobreposto por flag (Teste D).
- `core.hooksPath` → sem efeito em `git show` (Teste E).

**Inferido por mecanismo:**
- `objects/info/alternates` (arquivo): equivalente ao `GIT_ALTERNATE_OBJECT_DIRECTORIES` — mesma
  conclusão (content-addressing impede forjaria).
- Promisor/partial clone: content-addressing aplicado à busca de objetos remotos — promisor não pode
  servir conteúdo forjado para sha determinado.
