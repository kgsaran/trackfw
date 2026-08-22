---
status: done
date: 2026-08-21
req: "docs/req/REQ-2026-08-19-release-tag-confia-em-conteudo-local-para-versao-e-mensagem-da-tag.md"
adr: "docs/adr/ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `release tag` ancora versão e mensagem no forge

> Created: 2026-08-21 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-19-release-tag-confia-em-conteudo-local-para-versao-e-mensagem-da-tag.md`

Levantado pela reverificação do `hades-tf` ao levantar o bloqueio do commit-alvo: as Pré-condições 3
e 4 do `release tag` seguem lendo **conteúdo local** — os 4 arquivos de versão e o `CHANGELOG.md`
(`internal/commands/release.go:302-329`, via `deps.readFile`).

**O argumento decisivo é do revisor, e é contraintuitivo:** corrigir o commit-alvo tornou a mensagem
forjada **mais crível**. Antes, uma tag suspeita podia apontar para um commit estranho — um sinal.
Agora ela aparece pendurada num commit real do tip da branch padrão, com a mensagem que o atacante
escreveu, assinada pela credencial do usuário.

**A correção de um vetor ampliou a credibilidade do outro.**

## 🔴 O risco dominante, e ele decide o desenho

**O `CHANGELOG.md` local está legitimamente à frente do forge durante o PR de bump.** É o fluxo
normal: o mesmo PR que bumpa a versão acrescenta a seção do `CHANGELOG`, e a tag é criada **depois**
do merge.

Se a verificação for ingênua — "local deve bater com `origin/<default>`" — o comando fica
**inutilizável no próprio fluxo que ele existe para servir**. Pensar nisso **antes** de escrever
código, não depois.

Vale o mesmo princípio do `ADR-2026-08-17`: falso-positivo aqui não irrita, **paralisa**.

## Riscos que valem para todos os MLs

1. **`release tag` publica em repositório público.** Defeito produz tag errada, cara de desfazer.
   Prefira recusar a adivinhar. Fixture com stub de `gh` e remoto bare local — **nunca** rede.
2. **Não afrouxar o gate para caber.** Se o comparador não serve, o comparador muda.
3. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.
4. Ao fechar, a anotação `trackfw-contract` da seção precisa refletir a cobertura nova — o checker é
   bloqueante.

---

## Wave 1 — Decisão

### ML-1A — ADR: o que ancorar, e quando
**Status:** ✅ Concluído · **Agente:** `zeus-tf` (arquiteto — **não delegar**)
`ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md`

**A decisão é mais simples do que o problema sugeria: não comparar local com remoto — não ler o
local.** O comando **já resolve** o sha do commit-alvo no forge; versão e `CHANGELOG` passam a ser
lidos **daquele commit** (`git show <sha>:CHANGELOG.md`).

Objetos git são **endereçados por conteúdo**: dado um sha, o conteúdo é criptograficamente
determinado. A leitura é local, mas **a autoridade é o sha**, e o sha vem do forge. Mesma propriedade
que a Emenda 1 do `ADR-2026-08-19` usou para o commit-alvo, aplicada ao conteúdo.

**E o falso-positivo do PR de bump deixa de existir em vez de precisar de exceção:** o commit-alvo é
o tip **pós-merge**, então a seção do `CHANGELOG` e o bump **já estão nele**. Não há divergência a
tolerar porque não há comparação.

Sem chamada de API nova, e funciona offline depois do fetch.

Decisão material, e o ponto difícil é o **momento** da verificação, não o mecanismo:

- **Versão:** ancorar em quê? Os 4 arquivos são locais por natureza. Comparar com a tag anterior no
  forge? Com o `CHANGELOG` de `origin`? Exigir que o commit-alvo já contenha o bump?
- **Mensagem:** comparar o `CHANGELOG.md` local com o de `origin/<default>` **no commit que está
  sendo taggeado** — que é o commit pós-merge, onde a seção **já existe**. Isso resolve o
  falso-positivo do PR de bump? Verificar, não presumir.
- **Divergência:** recusa ou aviso? O padrão do `ML-4B` é recusar nomeando o quê divergiu.

**Critérios de aceite:**
- [ ] ADR com a decisão, os candidatos descartados e o motivo
- [ ] O caso do PR de bump endereçado explicitamente, com o mecanismo que o preserva

---

## Wave 2 — Implementação

### ML-2A — Ancorar versão e mensagem
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A
**Arquivos (3 stacks):** `internal/commands/release.go`, `npm/src/release/runner.js`,
`pypi/trackfw/release/runner.py`, testes dos 3.

**Critérios de aceite:**
- [ ] Versão não é determinada apenas por conteúdo local editável (AC1 da REQ)
- [ ] Mensagem verificada contra o forge (AC2)
- [ ] Divergência **recusa nomeando o quê** divergiu (AC3)
- [ ] **Fluxo legítimo de release preservado** — provado por cenário que simule o PR de bump
- [ ] `make quality` verde

### Auditoria do ML-2A — aprovada, e a ancoragem é **estrutural**, não convencional

```
release.go:412  deps.readCommittedFile(objectSHA, vf.path)
release.go:427  deps.readCommittedFile(objectSHA, "CHANGELOG.md")
grep readFile em release.go  ->  NENHUMA ocorrencia
make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**Tentei sabotar trocando de volta para leitura do working tree — e não compila.** O campo
`readFile` foi **removido do struct**, não apenas deixado de usar. Isso é melhor que gate: um
fallback silencioso para o working tree passa a ser **impossível de escrever por acidente**, em vez
de detectável depois.

Era exatamente o que eu tinha nomeado como o que não pode acontecer. Ele resolveu removendo a
possibilidade, não vigiando-a.

**As três provas centrais estão nos testes dos 3 stacks**, com discriminante literal
(`- from-commit-object-anchor`) provando que a mensagem vem do commit e não de conteúdo local.

#### Duas lacunas que ele declarou, e ambas são honestas

**1.** A recusa de "objeto ausente" tem cobertura só por teste unitário por stack, sem gate
cross-CLI. Vai para o ML-2B — e é justamente o caminho onde o fallback silencioso seria mais
tentador.

**2. Consequência de ordem que eu não tinha previsto:** mover P3/P4 para depois da resolução do
forge muda **qual recusa vence**. Um usuário sem `gh` agora vê *"requires the GitHub CLI"* antes de
qualquer erro de versão — mesmo que o erro de versão também exista.

É consequência inevitável do ADR, não defeito. Mas é **mudança de experiência**: quem errava a
versão e não tinha `gh` recebia a mensagem útil primeiro. Declarada para veredito do `hades-tf` no
ML-3A, e concordo em levá-la à barreira em vez de decidir sozinho.


### ML-2B — Gate + P4
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-2A
**Estender `scripts/check-release-tag-parity.sh`**, que já tem 18 cenários — não criar paralelo.

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais** nos caminhos novos de recusa
- [ ] Cenário do fluxo legítimo, provando que **não** recusa
- [ ] Cenário P4 com baseline e detecção
- [ ] Anotação `trackfw-contract` atualizada; `cli-parity.md` nomeia o gate
- [ ] `make quality` verde · CI-exata verde

---

### Auditoria do ML-2B — aprovada; o P4 encontrou o único alvo sabotável

Sabotei o sha eu mesmo — `objectSHA` → `"HEAD"`:

```
compila=OK                              <- e o ponto: precisa compilar para ser sabotagem
gate -> EXIT=1
  FAIL [release-tag-parity/content-from-commit-provenance/go]:
    provenance: tag message must contain 'forge-only' (from forge commit CHANGELOG);
    got: ## [9.9.9] - 2026-08-19
restaurado -> EXIT=0
157 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**A escolha do alvo era o problema difícil deste lote, e ele acertou.** Sabotar a leitura em si
**não compila** — o campo foi removido do struct no ML-2A. Um P4 ingênuo teria tentado isso,
falhado, e o lote acabaria sem falsificação real. Ele mirou no que **resta frágil**: passar o sha
errado, que compila e desfaz a ancoragem em silêncio.

**A fixture do cenário 16 prova proveniência, não igualdade.** Dois eixos — HEAD com `9.9.7`
`head-only`, decoy com `9.9.9` `forge-only` — e **ambos os `CHANGELOG` têm `## [9.9.9]`**. Se o
comando lesse o lugar errado, ainda encontraria a seção da versão; só o **conteúdo** denuncia a
origem. Comparar apenas "achou a versão" teria passado com o defeito presente.

É o mesmo padrão do sha sintético que ele usou na REQ do `doctor`: **teste de proveniência em vez de
teste de valor.**

**Wave 2 fechada.** Falta a barreira.

## Wave 3 — Barreira

### ML-3A — `hades-tf`
**Status:** ❌ Bloqueado — veredito BLOQUEAR, novo achado (`refs/replace/`) · **Agente:** `hades-tf`
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-ancoragem-de-versao-e-mensagem.md`

Foi ele quem levantou o achado. Avaliar se a âncora fecha o vetor que ele descreveu, e se a
verificação criou caminho novo de recusa indevida. **Veredito explícito.**

#### Veredito ML-3A — BLOQUEAR

`git show <sha>:<path>` honra `refs/replace/` por padrão. Um atacante com acesso local de escrita
pode criar `.git/refs/replace/<forge-sha>` → `<commit-local-forjado>` por escrita direta de
arquivo (sem nenhum comando git, guard irrelevante). Com o replace ref no lugar, `git show
<forge-sha>:CHANGELOG.md` retorna conteúdo forjado — re-abrindo P3 (versão) e P4 (mensagem) nos
3 CLIs. Medido ao vivo em fixture isolada.

**Fix (uma linha por CLI):** adicionar `--no-replace-objects` como flag ao `git show` em
`defaultReleaseReadCommittedFile` (Go), `defaultReadAtCommit` (Node.js),
`default_read_at_commit` (Python). Localização exata no documento de revisão.

Consequência de ordem (P3/P4 após forge → "no gh" antes de "versão errada"): **não é vetor**,
apenas mudança de UX. Sem mitigação recomendada.

---

### Auditoria do ML-3A — **BLOQUEIO ACEITO**; o achado invalida o argumento do meu ADR

Veredito: **BLOQUEAR** (`docs/seguranca/2026-08-21-revisao-da-ancoragem-de-versao-e-mensagem.md`).
Reproduzi por conta própria:

```
sha ancorado (do forge): bf5fc158...
echo <sha-forjado> > .git/refs/replace/<sha-do-forge>    (escrita de arquivo, SEM git)

git show <sha>:CHANGELOG.md               ->  CONTEUDO FORJADO PELO ATACANTE
git --no-replace-objects show <sha>:...   ->  CONTEUDO LEGITIMO
```

**Meu ADR afirma que "dado um sha, o conteúdo é criptograficamente determinado".** É verdadeiro para
o object store e **falso para `git show`**, que passa pela camada de substituição de objetos.
Deduzi a propriedade do formato e **presumi que a ferramenta a preservava**.

**E o ataque não usa comando git nenhum** — é escrita de arquivo. O guard de branch, que bloqueia
uma dúzia de subcomandos, é **irrelevante** aqui. Ele confirmou que `git replace` não aparece em
nenhum bloco do guard.

Emenda 1 registrada no ADR, com a lição: **garantia criptográfica do formato não é garantia da
ferramenta que o lê**. Quando a segurança depende de "o sha determina o conteúdo", é preciso
verificar que o leitor não tem camada de indireção — e o `git` tem pelo menos duas.

**Duas observações dele que valem tanto quanto o achado:**

A **consequência de ordem** que eu tinha mandado avaliar **não é vetor** — o comando recusa nos dois
caminhos e nada é publicado. É mudança de UX, e ele disse isso em vez de inflar a ressalva.

E ele corrigiu um exagero **meu**: eu escrevi que a ancoragem ficou "estrutural" porque não compila.
Isso vale **só para Go** — em Node e Python a remoção é convencional, sem enforcement de compilador.
Regressão ali só apareceria em gate, não em build.

---

## Wave 4 — Corretiva do bloqueio

### ML-4A — `--no-replace-objects` nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Evidência:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity` exit 0 · `./bin/trackfw validate` exit 0 · `go test ./...` verde · `bash scripts/check-release-tag-parity.sh` exit 0 (21 RT_LABELs) · P4 detection arm validado em isolamento (exit 1 com "provenance: tag message must contain 'forge-only'").

**Critérios de aceite:**
- [x] `--no-replace-objects` como primeiro argumento, nos 3 CLIs
- [x] O exploit reproduzido acima deixa de funcionar — provado nos 3 (Scenario 17: V2=ataque genuíno, V3=flag bloqueia, post-run guard=redirect ainda ativo após CLIs)
- [x] Cenário no gate com `refs/replace/` presente (Scenario 17), e P4 (Cenário 158 em check-gates-falsify.sh)
- [x] **`.git/info/grafts` medido:** grafts substituem a lista de pais do commit (parent-chain traversal), mas `git show <sha>:<path>` resolve o objeto pelo sha, lê o ponteiro `tree`, e percorre a árvore — caminho que grafts nunca tocam. O resultado é independente de versão: o script de medição confirmou via `git cat-file -p SHA_A` que o ponteiro `tree` permanece inalterado com graft instalado, enquanto `git log` seguiu o pai virtual. A única rota de grafts para o fluxo de leitura de árvore é `git replace --convert-graft-file`, que produz entradas `refs/replace/` — já bloqueadas pela flag. Git 2.50.1 reporta grafts como deprecated (serão removidos). **Grafts não são um vetor. Não precisam de cobertura adicional.**
- [x] `make quality` verde · CI-exata (`TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`) verde

**Nota de risco reportada:** `defaultReleaseReadCommittedFile` em `internal/commands/release.go` herda o `os.Environ()` bruto — sem `cleanGitEnv()` (que existe em `internal/validator/validator_git_exec.go`). Para o padrão de leitura `<sha>:<path>` (sha-addressed), redirecionamento de `GIT_DIR` torna o objeto ausente (refusa) em vez de forjado — menos crítico que o caso `HEAD:path` (ref-addressed) fechado pelo `cleanGitEnv` do validador. Não corrigido neste ML (fora do escopo); reportado para `hades-tf`/ML-4B.

### Auditoria do ML-4A — aprovada; a exclusão medida vale tanto quanto a correção

```
--no-replace-objects presente nos 3 CLIs (uma ocorrencia cada, conferido)
sabotagem propria: removi a flag do Go
  gate -> EXIT=1: "provenance: tag message must contain 'forge-only'
                   (--no-replace-objects reads forge commit)"
restaurado -> EXIT=0
158 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

#### `.git/info/grafts` **não é vetor**, e ele provou em vez de blindar

Grafts substituem a **lista de pais** do commit — afetam travessia de cadeia (`git log`,
`rev-list`). `git show <sha>:<path>` resolve o objeto pelo sha, lê o ponteiro `tree` e percorre a
árvore: **caminho que grafts nunca tocam**. Confirmado com `git cat-file -p`: o ponteiro `tree` é
idêntico com e sem graft, enquanto o `git log` seguiu o pai virtual.

**Terceira exclusão por medição nesta série** — depois do NEL e do CR. Cada uma evitou blindagem
nominal contra vetor inexistente, e blindagem inútil não é neutra: é falso-positivo esperando
acontecer, num comando que **publica**.

**Achado que ele reportou e não corrigiu:** a leitura herda `os.Environ()` sem o `cleanGitEnv()` que
o validador aplica. Para leitura **por sha** o impacto é menor — `GIT_DIR` redirecionado torna o
objeto **ausente**, e ausente **recusa**, que é o lado seguro. Diferente do caso `HEAD:<path>`
(endereçado por ref), que o Cenário 54 já fecha. Deixado para o veredito da barreira, corretamente.


### ML-4B — Reverificação do `hades-tf`
**Status:** ✅ Concluído · **Agente:** `hades-tf` · **Dep.:** ML-4A. Quem bloqueou levanta.

## Notas
- **Fora de escopo:** reabrir o ancoramento do commit-alvo — fechado e reverificado no #194.
- Commits e branch são exclusivos do `trackfw_architect`.

### Auditoria do ML-4B — **bloqueio levantado**

Veredito: **BLOQUEIO LEVANTADO**
(`docs/seguranca/2026-08-21-reverificacao-da-ancoragem-de-versao-e-mensagem.md`).

**Sete vetores medidos, nenhuma terceira camada de indireção:**

```
GIT_DIR redirect                    -> recusa (sha ausente no decoy)
GIT_ALTERNATE_OBJECT_DIRECTORIES    -> conteudo legitimo
GIT_REPLACE_REF_BASE + arquivo      -> neutralizado por --no-replace-objects
GIT_CONFIG_COUNT (useReplaceRefs)   -> flag de CLI sobrepoe config
core.hooksPath                      -> nao afeta stdout de git show
objects/info/alternates             -> mesmo mecanismo (inferido, declarado)
promisor/partial clone              -> content-addressing valida o hash (inferido, declarado)
```

Ele **separou medido de inferido** nos sete, sem inflar.

**O achado do `os.Environ()` se sustenta, e ele mediu em vez de aceitar meu resumo:** o pior caso é
**recusa** (`releaseTagObjectAbsentFmt`), não objeto forjado. `GIT_REPLACE_REF_BASE` e `GIT_CONFIG_*`
para reativar replace são anulados pela flag de linha de comando, que tem precedência sobre config.

**Node e Python saíram de "inferido" para "medido"** — o `assert_three_way` só emite `OK` quando os
três batem byte a byte, então o cenário 17 exercitou os três runtimes de fato.

#### Dívida nomeada, aceita e registrada

O cenário P4 sabota **só o Go**. Node e Python têm cobertura em runtime pelo gate, mas não
falsificação por sabotagem de código. É a mesma assimetria que ele apontou no ML-3A sobre o "não
compila".

**Não fecho aqui, e o motivo:** é propriedade **sistêmica da suíte de falsificação**, não desta REQ.
Vários cenários sabotam Node ou Python (o 43 e o 58, por exemplo), outros só o Go — a escolha nunca
foi feita por critério explícito. Resolver caso a caso, dentro de uma REQ de segurança, seria
escopo inflado; a decisão certa é um critério para a suíte inteira.

**Wave 4 fechada. REQ pronta para PR.**

