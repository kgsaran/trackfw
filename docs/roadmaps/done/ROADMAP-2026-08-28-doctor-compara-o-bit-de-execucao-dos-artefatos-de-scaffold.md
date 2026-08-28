---
status: done
date: 2026-08-28
req: "docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: doctor compara o bit de execucao dos artefatos de scaffold

> Created: 2026-08-28 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md`

O `#211` derrubou o bit de execução de `scripts/trackfw-validate.sh` (100755 → 100644) e o
`.husky/pre-commit` deste repositório parou de rodar o gate — `exit 126`. **E o `scaffold doctor`,
entregue no mesmo PR, reportou `no mismatches`:** ele compara **conteúdo**, não **modo**.

**Bloqueia a release 7.3.0.**


## Acceptance Criteria

- [ ] AC1–AC8 da REQ, integralmente
- [ ] 🔴 **AC4 decide:** artefato que o gerador escreve **sem** bit de execução não pode ser acusado
      por não tê-lo
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Quais artefatos são executáveis, e onde o modo não existe
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md`

**Duas perguntas:**

1. **Quais dos 17 artefatos de scaffold o gerador escreve como executáveis, nos 3 CLIs?** Enumere
   **por busca** no código de escrita (`0755` × `0644`), não pela aparência do repositório. Divergência
   entre runtimes aqui é achado.
2. 🔴 **Onde o bit não é representável ou é enganoso?** Windows, filesystem montado com `noexec`,
   `core.fileMode=false` no git, checkout via zip/tarball. **Acusar nesses casos é o falso-positivo
   que reprova (AC4/AC5).**

**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
grep -q "Completude de enumera" docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
grep -q "Residual declarado" docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
```

## Wave 1 — Correção e cobertura

> Dependências: ML-0A auditado.

### ML-1A — Restaurar o modo e comparar o bit nos 3 CLIs
**Status:** ✅ Concluído (apolo-tf, 2026-08-28) · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

**Critérios de aceite:** AC1–AC6 da REQ · `make quality` exit 0 medido

---

### Auditoria do ML-0A — aprovada; **e ela achou que o remédio é inerte**

**Enumeração fechada:** **5** artefatos requerem o bit (validate script, 2 attention, 2 guards), com
`arquivo:linha` da escrita nos 3 runtimes. Os outros **12** são `0644` em todos — **não** podem ser
acusados (AC11).

#### 🔴 O achado que muda o escopo, e eu reproduzi

```
Go   os.WriteFile(existente_0644, conteudo, 0755)   ->  permanece 0644   [medido por mim]
Node fs.writeFileSync(existente_0644, ..., {mode})  ->  permanece 0644
Py   open(...,'w') + os.chmod(0o755)                ->  vira 0755
```

`perm` só é aplicado no evento **`O_CREATE`**; em arquivo existente o `O_TRUNC` reescreve o
**conteúdo** e não toca o **modo**. O Python é exceção porque `os.chmod` é incondicional.

**A cascata:** o `doctor` acusaria → o usuário rodaria `trackfw update` → o Go/Node reescreveriam o
conteúdo **sem restaurar o modo** → o `doctor` acusaria de novo. **Remédio que não remedia** — a mesma
classe do *"bloqueado não significa não executou"* da REQ do `barrier`.

Vira **AC9**: corrigir Go e Node faz parte desta REQ.

**AC10, do mesmo parecer:** comparar por **`mode & 0o100 != 0`**, nunca `== 0755` — umask não-padrão
produz `0750`/`0700`, que são executáveis e seriam acusados à toa.

**Residuais nomeados:** `noexec` não é detectável · Windows suprime a verificação, declarado na saída ·
`core.fileMode=false` como vetor de git não é coberto · hook files seguem fora de escopo.

---

### Auditoria do ML-1A — aprovada; o remédio deixou de ser inerte, medido por mim

```
antes                    .rw-r--r--     (bit rebaixado a mao)
doctor                ->  2 findings [scaffold-wrong-mode]
depois do update Go   ->  .rwxr-xr-x    restaurou
depois do update Node ->  .rwxr-xr-x    restaurou
make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

O ciclo fecha: o `doctor` acusa, o `update` remedia, o `doctor` cala. Antes do AC9 o remédio impresso
apontava para um comando que **não** consertava.

#### O `make quality` reprovou uma vez, e a falha foi instrutiva

```
make: *** [test-node] Error 1
npm/tests/scaffold_doctor_membership.test.js:40
  actual: { finding: 'scaffold-wrong-mode', ... }   expected: null
```

O fixture escrito **ontem** no ML-1C gravava o script sem o bit — irrelevante até existir verificação
de modo. Com ela, **o fixture que representava "artefato íntegro" deixou de ser íntegro**.

Deixei explícito no handoff o que eu **não** aceitaria: relaxar a verificação no produto para o teste
passar. É a tentação óbvia, e apagaria justamente a detecção recém-construída. Ele corrigiu o
fixture, e notou o que importa: **os casos negativos não foram afetados** — a verificação de conteúdo
tem precedência sobre a de modo, então continuam falhando pelo motivo original, sem virar asserção
decorativa.

**Residuais nomeados e aceitos:** a guarda de Windows tem teste unitário só no Go (Node/Python
dependem do parity script) · `discover.go:83` escreve o validate script sem `Chmod` posterior — fora
de escopo porque o remédio que o `doctor` imprime é `trackfw update`, que já tem o `Chmod`, **mas a
barreira deve olhar**, já que o `discover --init` é escritor legítimo.

---

## Wave 2 — Gate

### ML-2A — Falsificação nas duas direções
**Status:** ✅ Concluído (apolo-tf, 2026-08-28) · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC7, AC8 da REQ

---

### Auditoria do ML-2A — aprovada; e o terceiro cenário é o que protege o remédio

Pedi **três** direções, não duas. A terceira não estava no roadmap e é a mais valiosa:

- **179 (A)** — verificação de modo silenciada ⇒ `check-doctor-parity` reprova.
- **180 (B)** — verificação aplicada a **todos** os artefatos ⇒ 9 falsos-positivos nos slash commands
  (`0644`) detectados.
- **181 (C)** — 🔴 **o `os.Chmod` do ML-1A removido** ⇒ o remédio volta a ser inerte.

**O 181 está construído do jeito certo**, e conferi as asserções:

```bash
if ! cmp -s "$WORK/s181-canonical.sh" "$T181_SCRIPT"; then
  FAIL: binario sabotado nao restaurou o CONTEUDO      <- guard de vacuidade
if test -x "$T181_SCRIPT"; then
  FAIL: binario sabotado restaurou o BIT               <- o que se quer provar
```

A primeira asserção é o que dá valor à segunda: prova que o `update` **rodou** e fez o trabalho dele.
Sem ela, *"o bit não voltou"* seria compatível com *"o comando nem executou"*, e o cenário passaria
por acidente. Mesma disciplina do sentinela no `barrier`: verificar o **efeito**, e provar que a
máquina estava ligada quando o efeito não veio.

**Sem o 181**, alguém removeria o `Chmod` num refactor e o `doctor` voltaria a apontar para um comando
que não conserta — em silêncio, porque o `update` continuaria saindo com exit 0.

```
make quality (CI-exata, minha)  exit 0 · 181 cenarios
validate                        16 warnings, 0 violations
```

---

## Wave 3 — Barreira de segurança

### ML-3A — Reverificação final pós-ML-2A (barreira)
**Status:** ✅ Concluído (hades-tf, 2026-08-28) · **Agente:** `hades-tf` · **Dep.:** ML-2A

**Escreve:** `docs/seguranca/2026-08-28-barreira-do-bit-de-execucao.md`

**Veredito:** APROVADO. AC9 fechado nos 5 pontos de Go e Node (medido artefato por artefato).
Dois resíduos aceitos: `discover/discover.go:writeValidateScript` sem `os.Chmod` posterior (cadeia de remédio fecha o loop); guarda de Windows em Node sem teste unitário que injete `_setPlatformForTest` (implementação correta, cobertura mais fraca que Go). Nenhum falso-positivo novo de modo. Sem mascaramento de conteúdo por modo. Nenhuma mentira nova do doctor.

```
bash scripts/check-doctor-parity.sh → All check-doctor-parity.sh scenarios passed.
```

---

### Auditoria do ML-3A — **APROVADO**; e a pergunta 4 era a que podia derrubar a entrega

**A resposta que eu mais queria:** um arquivo com **conteúdo errado E bit ausente** reporta apenas
`scaffold-divergent`. O código verifica **conteúdo primeiro** (`scaffold_doctor.go:313`,
`scaffold_doctor.js:244`) e encerra — o modo só é avaliado quando o conteúdo passou. **A correção não
esconde o pior dos dois defeitos**, e o `update` restaura ambos.

**Ele achou o sexto escritor que eu tinha perguntado:** `internal/discover/discover.go:83`
(`writeValidateScript`) usa `os.WriteFile(..., 0755)` **sem `Chmod` posterior`. Os outros quatro
artefatos do `InstallGates` passam pelas funções centrais de `generators/`, que já têm o `Chmod`.

**Mas ele fechou o raciocínio em vez de parar no achado:** a cadeia de remédio funciona — o `doctor`
detecta `scaffold-wrong-mode` e o remédio impresso é `trackfw update`, que chama
`generateValidateScript` **com** `Chmod`. O `discover --init` pode criar o problema; o `update` o
resolve. Resíduo aceito.

**Resíduo real e nomeado:** a guarda de plataforma tem `_setPlatformForTest` no código de produção do
Node **e nenhum teste a invoca** — confirmei: `grep -rn "_setPlatformForTest" npm/tests/` → **0**. A
implementação está correta (`_platform !== 'win32'`); falta o teste unitário isolado. O Go tem
`TestWindowsPlatformGuard`.

**Nenhum falso-positivo novo:** `0750`, `0700`, `0555`, setuid e outro dono passam — o discriminante
`mode & 0o100` aceita qualquer modo com bit de execução do dono, independente de umask ou bits extras.

```
grep -c "Chmod" scaffold.go   ->  6   (5 write sites + a constante/uso auxiliar)
check-doctor-parity           ->  All scenarios passed
make quality (CI-exata)       ->  exit 0 · 181 cenarios
```

**Entrega completa. A release 7.3.0 está desbloqueada.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
