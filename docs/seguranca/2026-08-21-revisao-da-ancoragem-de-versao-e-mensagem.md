---
status: reviewed
date: 2026-08-21
reviewer: "hades-tf"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-21-release-tag-ancora-versao-e-mensagem-no-forge.md"
adr: "docs/adr/ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md"
verdict: "BLOQUEAR — refs/replace/ contorna a ancoragem nos 3 CLIs por escrita direta de arquivo sem comando git"
---

# Revisão de segurança: âncora de versão e mensagem (`release tag`)

## Veredito

**BLOQUEAR.**

A ancoragem de conteúdo via `git show <sha>:<path>` não é inviolável: o mecanismo `refs/replace/`
de git permite que um atacante com acesso local de escrita substitua o conteúdo servido para
qualquer sha, incluindo o sha resolvido do forge. O ataque não requer nenhum comando git — uma
escrita direta de arquivo em `.git/refs/replace/<forge-sha>` é suficiente, tornando o guard de
branch irrelevante para este caminho. **Medido ao vivo.** O defeito é idêntico nos 3 CLIs. O fix
é barato: um flag por CLI.

---

## O que foi medido (não inferido)

### 1. Reprodução do vetor `refs/replace/`

Fixture isolada em
`scratchpad/ml3a-fixture/` (HOME e GIT_CONFIG_GLOBAL sobrepostos, remote bare local, nenhum
comando toca o repositório real do projeto):

```
REAL_SHA:   ba1719bbccab32ddfa443de8be82c9e3872713cd   (tip da main, empurrado ao remote)
FORGED_SHA: da8a1d21f0cb2c708d0a7abd3e8d2da1faa62007   (commit local, nunca empurrado)

$ mkdir -p .git/refs/replace
$ echo "$FORGED_SHA" > .git/refs/replace/$REAL_SHA    # escrita de arquivo, sem git

$ git show "$REAL_SHA:CHANGELOG.md"
# Changelog
## [1.0.0] - 2026-08-21
### FORGED-ATTACKER-CONTROLLED-CONTENT          ← conteúdo do commit forjado
```

`git --no-replace-objects show "$REAL_SHA:CHANGELOG.md"` retorna o conteúdo legítimo — o flag
neutraliza o mecanismo de substituição.

### 2. Impacto nos 3 CLIs

Lido linha a linha em cada implementação:

| CLI | Função | Invocação |
|-----|--------|-----------|
| Go | `defaultReleaseReadCommittedFile` (`release.go:223`) | `exec.Command("git", "show", sha+":"+path)` |
| Node.js | `defaultReadAtCommit` (`runner.js:161`) | `spawnSync('git', ['show', ...])`  |
| Python | `default_read_at_commit` (`runner.py:238`) | `subprocess.run(["git", "show", ...])` |

Nenhum dos três passa `--no-replace-objects`. O comportamento é idêntico nos três: `git show`
honra `refs/replace/` por padrão, servindo o conteúdo do commit de substituição em vez do commit
apontado pelo sha do forge.

### 3. O fix é uma linha por CLI

Substituir `"git", "show", sha+":"+path` por `"git", "--no-replace-objects", "show", sha+":"+path`
em cada implementação. A flag pré-existe ao git 2.13 (2017) e é portável. Alternativa equivalente:
`GIT_NO_REPLACE_OBJECTS=1` no ambiente do subprocess. Ambas foram verificadas no fixture: ambas
retornam o conteúdo legítimo para REAL_SHA mesmo com o replace ref presente.

### 4. O guard de branch não cobre este caminho

`git replace` não está listado nos blocos do `trackfw-git-branch-guard.sh`. Mais relevante: o
ataque não requer o comando `git replace` — uma escrita direta a
`.git/refs/replace/<forge-sha>` é suficiente. O guard intercepta comandos git; não intercepta
operações `mkdir`/`printf` no diretório `.git/`.

### 5. Objeto ausente continua recusando

O caminho de ausência (`releaseTagObjectAbsentFmt`) foi verificado por leitura — nenhum fallback
para o working tree existe (campo `readFile` foi removido do struct em Go; Node.js e Python não
têm um `readFile` alternativo acessível por P3/P4). A recusa por objeto ausente é correta e o
ML-2A a nomeou explicitamente. Não remedi nesta rodada porque o vetor acima é mais urgente e esta
propriedade não é contestada.

---

## O vetor que levantei está fechado?

**Não completamente.** O argumento original era: "corrigir o commit-alvo tornou a mensagem forjada
mais crível." A ADR-2026-08-21 propôs que ler versão e CHANGELOG *do sha do forge* fecharia o
vetor — o sha é criptograficamente determinado. Esse argumento é correto para o object store bruto.
Mas `git show` não lê o object store bruto: passa pela camada `refs/replace/`, que permite que o
conteúdo associado a um sha seja redirecionado localmente, sem alterar o sha em si.

**O que mudou:** antes da ancoragem, o atacante editava arquivos locais e publicava conteúdo
forjado. Depois da ancoragem, o atacante cria um replace ref apontando o sha do forge para um
commit local forjado, e publica conteúdo forjado. A superfície atacada mudou — passou de edição
de working tree para escrita em `.git/refs/replace/` — mas o resultado é o mesmo: versão e texto
da tag são controlados localmente.

**O que a ancoragem de fato fechou:** a edição silenciosa do working tree sem rastro em nenhum
ref. Com o replace ref, existe um objeto git local com conteúdo hostil — mais rastreável, mas não
visível ao audit de quem olha apenas os commits e a tag publicada.

---

## Consequência de ordem (P3/P4 após resolução do forge)

Mover P3/P4 para depois da resolução do forge muda qual recusa vence quando `gh` está ausente:
o usuário vê "requires the GitHub CLI" antes de qualquer erro de versão — mesmo que o erro de
versão também exista.

**Não abre vetor.** A recusa acontece de qualquer forma — o comando não publica nada sem `gh`. A
mudança é puramente de UX: a mensagem de erro mais informativa (versão incorreta) fica oculta
atrás de uma mais categórica (sem `gh`). É consequência inevitável do ADR, que decidiu resolver o
forge antes de verificar P3/P4.

Avaliação: **aceitável como está.** A alternativa — verificar P3/P4 com conteúdo local antes do
forge — reintroduziria exatamente o vetor que a ADR veio fechar. Não recomendo mitigação de UX
aqui; o cenário de "sem `gh` + versão errada" é borda não crítica em ambiente de release normal.

---

## Garantia estrutural: "não compila" cobre só Go

O ML-2A declarou como ponto forte: "tentei sabotar trocando de volta para leitura do working tree
e não compila — o campo `readFile` foi removido do struct." Isso é correto para Go. Para Node.js e
Python não existe garantia análoga: a remoção do caminho de leitura do working tree é convencional
(não está mais presente nas funções que P3/P4 chamam), não forçada pelo compilador.

Em termos práticos, as três implementações têm o comportamento correto — P3/P4 chamam apenas
`readAtCommit`/`read_at_commit`. Mas uma regressão acidental seria detectada por teste (e pelo
gate), não pelo compilador. Registrado como diferença de profundidade de garantia, não como defeito
ativo.

---

## Achado novo

**Um único achado novo: `refs/replace/` contorna a ancoragem.** Descrito nas seções anteriores.
Não encontrei outros vetores além deste.

Considerei `.git/info/grafts` como mecanismo adicional (análogo a `refs/replace/`, deprecated mas
suportado). Por leitura da documentação do git: `--no-replace-objects` **não** desabilita grafts;
desabilitá-los requereria `-c core.grafts=/dev/null`. Grafts são raros em clones modernos e
ausentes desta fixture, então não medi — registro como superfície adjacente para o ML que
implementar o fix, com recomendação de usar `-c core.grafts=/dev/null` na mesma invocação se a
avaliação de risco justificar.

---

## O que medido vs. inferido

**Medido, com fixture nova, binário recompilado via `make build`:**
- Criação de replace ref por escrita de arquivo e verificação de que `git show <forge-sha>:CHANGELOG.md`
  retorna conteúdo forjado — reproduzido.
- `git --no-replace-objects show <forge-sha>:CHANGELOG.md` retorna conteúdo legítimo — verificado.
- `GIT_NO_REPLACE_OBJECTS=1` produz o mesmo resultado — verificado.
- Guard de branch não menciona `git replace` — verificado por `grep`.

**Inferido por leitura:**
- Impacto em Node.js e Python: estrutura idêntica ao Go, `--no-replace-objects` ausente em ambos.
  Não executei o comando nos outros dois runtimes porque a cópia é literal e o mecanismo de
  invocação (`spawnSync`, `subprocess.run`) é análogo — o flag é passado como argumento ao git,
  não ao runtime.
- Ausência de fallback para working tree em Node.js e Python: confirmado por leitura das funções
  que P3/P4 chamam; não há caminho alternativo de leitura.
- Grafts: inferido por documentação, não medido nesta fixture.

---

## Encaminhamento

O fix é de produto — `internal/commands/release.go`, `npm/src/release/runner.js`,
`pypi/trackfw/release/runner.py`. Não está no meu escopo de implementação. Encaminho ao agente
que implementa código de produto (`apolo-tf` ou equivalente), com as localizações exatas:

- **Go** `internal/commands/release.go:224` (`defaultReleaseReadCommittedFile`): adicionar
  `"--no-replace-objects"` como primeiro argumento após `"git"` na chamada de
  `exec.Command`.
- **Node.js** `npm/src/release/runner.js:161` (`defaultReadAtCommit`): adicionar
  `'--no-replace-objects'` na lista de args do `spawnSync`, após `'show'` ou antes.
- **Python** `pypi/trackfw/release/runner.py:239` (`default_read_at_commit`): adicionar
  `"--no-replace-objects"` no array do `subprocess.run`, após `"show"` ou antes.

O gate existente (`scripts/check-release-tag-parity.sh`) pode ser estendido com um cenário que
crie um replace ref e confirme que a tag é recusada (versão e CHANGELOG continuam vinculados ao
forge). Essa extensão deve ser parte do mesmo ML de correção.
