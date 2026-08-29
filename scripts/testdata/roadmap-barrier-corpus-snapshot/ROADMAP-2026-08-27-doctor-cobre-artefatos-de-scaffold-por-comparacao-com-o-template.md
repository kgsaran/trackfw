---
status: done
date: 2026-08-27
req: "docs/req/REQ-2026-08-27-doctor-nao-cobre-os-artefatos-do-scaffold-e-um-slash-command-defasado-nao-e-acusado.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: doctor cobre artefatos de scaffold por comparacao com o template

> Created: 2026-08-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-27-doctor-nao-cobre-os-artefatos-do-scaffold-e-um-slash-command-defasado-nao-e-acusado.md`
ADR: `docs/adr/ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md`

**Medido:** manifesto tem 290 artefatos e **zero** de scaffold. Um slash command defasado — ensinando
a estrutura antiga de roadmap, sem Wave 0 — **não é acusado por nada**. Só o `update` revela, e ele
corrige no mesmo passo: o usuário nunca sabe que esteve defasado.


## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] 🔴 **AC4 decide a entrega:** projeto íntegro reporta `no mismatches`. Ruído em projeto correto
      treina o usuário a ignorar o `doctor` — e aí perdemos também o que ele já detecta.
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Completude do inventário e o risco de falso-positivo
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md`

**Duas perguntas decidem a entrega:**

1. **O inventário está completo?** Enumere **por busca** tudo o que o `Scaffold` escreve, nos 3 CLIs —
   e diga o que já tem cobertura (os 2 guards, via `validate`) e o que não tem. A lista da REQ é
   hipótese.
2. 🔴 **Onde isto gera falso-positivo?** Customização deliberada, projeto que nunca rodou `init`
   completo, artefato de outro produto no mesmo caminho, binário antigo em projeto novo. **AC4 é o
   critério que reprova** — ruído em projeto íntegro mata o `doctor` inteiro.

**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
grep -q "Completude de enumera" docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
grep -q "Residual declarado" docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
```

### Auditoria do ML-0A — aprovada; **cinco bloqueios antes de existir código**

**Inventário: 17 artefatos, não 13.** Faltavam `.gitlab-ci-trackfw.yml` e os arquivos de hook
(husky/lefthook) — todos condicionais. → **AC11**

**Os cinco problemas que a implementação teria encontrado tarde:**

1. 🔴 **`scripts/trackfw-validate.sh` é cfg-dependente** — `buildValidateScript` varia com
   `cfg.Backend`/`cfg.Frontend`. Comparar contra um cfg padrão embutido faria **todo projeto com
   `backend:` configurado virar falso-positivo imediato**. O `doctor` tem de renderizar o template a
   partir do `trackfw.yaml` **do projeto**. → **AC12**
2. **Condicionais não podem virar "ausente"** quando não configurados. → **AC13**
3. **`discover --init` é um terceiro escritor** e **não escreve slash commands** — projeto
   inicializado só por ele tem ausência **legítima**. → **AC14**
4. 🔴 **`ClassifyDoctor` não tem case para `!Registered && StateModified`** — hoje cai no `default`
   implícito e **não gera finding nenhum**. Sem esse case, o falso-negativo estaria garantido
   justamente para os artefatos que motivaram a REQ: eu teria "implementado" a cobertura e ela
   continuaria cega. → **AC15**
5. **O AC5 não é satisfazível por conteúdo** — nenhum artefato carrega stamp de versão. A mensagem
   tem de ser **neutra quanto à culpa**, em vez de afirmar que o projeto está defasado. → **AC16**

**Baseline medido neste repositório:** `update --dry-run` reporta `validate-script: skipped`,
`ci-workflow: skipped`, `claude-commands: skipped` — **zero divergência** nos targets in-scope. O AC4
tem base limpa para ser verificado.

---

## Wave 1 — Cobertura no doctor

> Dependências: ML-0A auditado. **ML único:** os 3 stacks saem byte-idênticos.

### ML-1A — `doctor` compara artefatos de scaffold com o template
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/integrations/doctor.go` + equivalentes. Propriedade por caminho, comparação com template,
**sem** tocar o manifesto.

**Critérios de aceite:** AC1–AC7 da REQ · **projeto íntegro reporta `no mismatches`** ·
`make quality` exit 0 medido

---

### Auditoria do ML-1A (+1B, +1C) — aprovada; e a regra de pertencimento é decisão minha

**Reprovei o ML-1A num ponto e decidi um bloqueio que o ML-1B levantou.**

**1. "Python reduced surface" não passou.** Ele excluiu `scripts/trackfw-validate.sh` e os workflows
do scaffold doctor do Python. A **premissa** estava certa — a divergência de bytes desse arquivo é
pré-existente e documentada (`init_gen.py:489+`), só o contrato de estado é pinado. **A conclusão
não seguia:** isso justifica não comparar os runtimes entre si, **não** o Python deixar de verificar.
Excluir reabria, num runtime, a lacuna que a REQ existe para fechar.

**2. O ML-1B então mediu o efeito colateral e sinalizou em vez de decidir** — comportamento certo:
com template por runtime, o Python acusava `scaffold-divergent` neste repositório, porque o arquivo
foi escrito pelo Go. **Achado verdadeiro pela regra; a regra é que estava estreita demais.**

**Decisão (ML-1C): pertencimento a conjunto, escopado a este artefato.** O `doctor` aceita **qualquer**
template de runtime conhecido para `trackfw-validate.sh`, porque um arquivo em qualquer dessas formas
**é** artefato legítimo do trackfw — acusá-lo é o falso-positivo que o **AC4** proíbe. Todos os demais
artefatos têm bytes pinados por gate e seguem com igualdade a template único. **Não generalizar.**

#### Medição minha

```
projeto integro:      Go / Node / Python  ->  no mismatches found     AC4 restaurado
near-miss (set -e -> set -x na forma do Go):
                      Go / Node / Python  ->  scaffold-divergent      os 3 acusam
restaurado         ->  no mismatches
make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

O **near-miss** é o teste que importa: um caractere de diferença continua sendo acusado nos 3. O
critério afrouxou **só** onde o produto já declara variação legítima.

**Exclusão dos workflows de CI no Python — aceita, e pelo motivo dele, melhor que o meu
enquadramento:** o `update` do Python **não gerencia** `ci-workflow`, então um finding cujo remédio é
`trackfw update` seria **enganoso**. Exclusão por **propriedade**, não por conveniência.

#### 🔴 Risco residual que herda para o ML-2A

`_build_go_node_validate_script` (Python) é uma **terceira cópia** da lógica de `buildValidateScript`.
Se Go ou Node mudarem o template, o espelho fica velho e o `doctor` do Python passa a aceitar a forma
nova **sem verificar se bate**. Ele mitigou com 5 testes de membership por runtime — mas isso só pega
se rodarem no mesmo commit da mudança. **É a mesma classe que motivou o
`check-attention-scripts-parity`: cópia de string deriva.** O ML-2A precisa de gate para isso.

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC8, AC9, AC10 da REQ

---

### Auditoria do ML-2A — aprovada; a deriva do espelho é detectada, provado por sabotagem minha

Ele **não colou** a prova de load-bearing que eu exigi, então fiz:

```
sabotagem: buildValidateScript (Go)  set -e -> set -e -o pipefail
           SEM tocar o espelho do Python

check-doctor-parity.sh -> EXIT 1
  FAIL [doctor-parity/scaffold-baseline-clean-text/node]: vacuity guard: stdout missing
       'no mismatches found'
  FAIL [doctor-parity/scaffold-baseline-clean-text/py]:   idem
restaurado -> EXIT 0, scaffold.go com diff vazio

make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

**A proteção é mais direta do que eu tinha desenhado:** a deriva quebra o cenário **baseline-clean**,
que roda sempre — não só o cenário dedicado de espelho. Mudar o template de um runtime sem atualizar
os espelhos faz o arquivo gerado pelo Go deixar de ser aceito pelo Node **e** pelo Python.

Isso fecha o risco que ele mesmo declarou no ML-1C: **a deriva não depende mais de alguém lembrar de
rodar o teste unitário no commit certo.**

30 asserções no gate · cenários 177 e 178 fecham as duas direções · total vai a 178.

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

**Escreve:** `docs/seguranca/2026-08-27-barreira-da-cobertura-de-scaffold.md`

**Veredito:** APROVADO com residuais nomeados. Os cinco bloqueios da Wave 0 estão fechados.
Três residuais declarados: (A) Python remedy omite versão binária quando não pip-installed;
(B) forma Python aceita em projeto Go — lacuna de capability, custo declarado da decisão ML-1C;
(C) artefato de outro produto no caminho `trackfw-*.yml` — teórico, probabilidade baixa.

---

### Auditoria do ML-3A — **APROVADO**; e ele corrigiu uma premissa minha

**A correção que importa: o meu AC15 estava mal formulado.** Eu exigi um case
`!Registered && StateModified` no `ClassifyDoctor`. Ele mediu: **o case existe** (`doctor.go:135`,
gera `DoctorUnknownContent`) **mas serve aos artefatos do manifesto** — o scaffold segue por
`RunScaffoldDoctor` → `checkScaffoldArtifact`, que **nunca passa por `ClassifyDoctor`**.

Ou seja: **o bloqueio foi fechado por arquitetura, não pelo case que eu pedi.** A prova que vale é
comportamental, e ele a fez: `INJECTED_LINE` → `scaffold-divergent` nos 3 runtimes. Registro porque
eu poderia ter "verificado o AC15" olhando o case certo e concluindo errado sobre o caminho errado.

**A minha decisão de pertencimento foi atacada e sobreviveu.** Conjunto de cardinalidade **2**, com
`bytes.Equal` exato. Três ataques testados — **linha extra**, **linha removida**, **concatenação das
duas formas** — todos rejeitados nos 3 runtimes. O afrouxamento não virou porta.

**Falso-positivo em campo: zero medido.** Três configurações (`backend: go`, `ci: none`,
`frontend: react + pnpm`) × 3 runtimes = **9 execuções**, todas `no mismatches found`. O AC4 deixou de
ser verificado só neste repositório.

**Inventário: 16 de 17 cobertos.** Os arquivos de hook (#17) ficam fora, **declarado** na Wave 0 e na
REQ — exclusão não silenciosa.

**Três residuais nomeados, todos aceitos:**
- **A** — Python emite `vunknown` na mensagem quando não está pip-installed (`PackageNotFoundError`);
  degrada só em desenvolvimento a partir do fonte.
- **B** — forma do Python aceita em projeto Go: o `go build ./...` daquele projeto não roda. É lacuna
  de **capacidade**, não de **detecção de adulteração**. **Custo declarado da minha decisão do ML-1C.**
- **C** — artefato de outro produto no mesmo caminho: aceito, o nome de arquivo é específico do
  trackfw.

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
