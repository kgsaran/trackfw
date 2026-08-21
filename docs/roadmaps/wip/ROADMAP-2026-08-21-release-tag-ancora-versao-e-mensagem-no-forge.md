---
status: wip
date: 2026-08-21
req: "docs/req/REQ-2026-08-19-release-tag-confia-em-conteudo-local-para-versao-e-mensagem-da-tag.md"
adr: "docs/adr/ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `release tag` ancora versão e mensagem no forge

> Created: 2026-08-21 | Status: wip

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
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-2A
**Estender `scripts/check-release-tag-parity.sh`**, que já tem 18 cenários — não criar paralelo.

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais** nos caminhos novos de recusa
- [ ] Cenário do fluxo legítimo, provando que **não** recusa
- [ ] Cenário P4 com baseline e detecção
- [ ] Anotação `trackfw-contract` atualizada; `cli-parity.md` nomeia o gate
- [ ] `make quality` verde · CI-exata verde

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-ancoragem-de-versao-e-mensagem.md`

Foi ele quem levantou o achado. Avaliar se a âncora fecha o vetor que ele descreveu, e se a
verificação criou caminho novo de recusa indevida. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** reabrir o ancoramento do commit-alvo — fechado e reverificado no #194.
- Commits e branch são exclusivos do `trackfw_architect`.
