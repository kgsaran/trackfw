# Revisão de segurança — Barreira final da REQ CRLF

> Produzido por: `hades-tf` · 2026-09-23
> Branch: `fix/bash-consome-stdout-de-python3-sem-normalizar-crlf`
> Wave 0: `docs/seguranca/2026-09-23-enumeracao-crlf-python3-em-bash.md`

---

## Veredito: APROVADO COM RESSALVAS

A implementação está correta para os 20 sítios conhecidos. A normalização é segura, o gate
estrutural funciona para o padrão dominante (`$(python3...)`), e a barreira está verde. Três
evasões do gate e um gap de cobertura no Windows foram confirmados por execução e são nomeados
abaixo. Nenhum representa exploração de um sítio existente — todos são riscos futuros de
reintrodução.

---

## Resposta 1 — A normalização come `\r` que é DADO em algum sítio?

**Veredito: NÃO. Seguro para todos os 20 sítios.**

### Evidência: comportamento de `strip_cr`

```bash
printf 'a\rb\n' | sed $'s/\r$//' | xxd | head -1
# 00000000: 610d 620a  →  a[CR]b[LF]  — CR do meio preservado
printf 'path/to/file\r\n' | sed $'s/\r$//' | xxd | head -1
# 00000000: 7061 7468 2f74 6f2f 6669 6c65 0a   →  path/to/file[LF]  — terminal CR removido
```

`sed $'s/\r$//'` remove apenas `\r` **terminal de linha** — o artefato de modo texto Windows.
`tr -d '\r'` removeria qualquer `\r`, incluindo os do meio.

### Corpus dos 20 sítios

Os valores emitidos pelos 20 sítios são: strings de status JSON (`"pass"`, `"not_evaluated"`,
`"passed"`, `"MISSING"`), nomes de label (`falsify/nome`), caminhos de arquivo, versões
semânticas, objetos JSON, strings de versão Python, contagens inteiras. Nenhum desses termina
legitimamente em `\r`.

A distinção delimitador/conteúdo da Wave 0 confirma: `write_fixture_crlf` em
`check-roadmap-barrier-contract.sh:1091` escreve CRLF intencionais **em arquivo em modo
binário**, não como stdout capturado por bash — está fora da população. Demonstrado no working
context pelo próprio Zeus:

> "valor\r\n → CR removido · valorr\n → os dois r preservados · a\rb\n → 61 0d 62 0a, CR
> no MEIO preservado"

**Conclusão:** `strip_cr` é seguro e correto para todo o corpus. `tr -d '\r'` teria sido errado
pelo motivo da Wave 0.

---

## Resposta 2 — O gate pode ser enganado?

**Veredito: SIM — três evasões confirmadas por execução. Nenhuma afeta código atual; todas são
vetores futuros.**

### Evasão 1: variável `$PY_BIN` em vez de literal `python3`

**Confirmação:**

```bash
# Corpus com $($PY_BIN -c "print(...)")
cat > /corpus/scripts/test_pybin.sh << 'EOF'
PY_BIN=python3
RESULT=$($PY_BIN -c "print('hello')")
EOF

CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh \
  --scan-root /corpus
# → 0 candidates, vacuity FAIL (não viola, não detecta)
```

O ERE do gate é `\$\([^\)]*python3`. `$($PY_BIN ...)` não contém `python3` na linha de abertura —
o gate passa sem inspecionar.

**Impacto atual:** os dois sítios `$("$PY_BIN" ...)` existentes (`PROSE_PAYLOAD` em
`check-gates-falsify.sh:3917` e `T65_BIG_PAYLOAD` em `:4433`) já têm `| strip_cr)` confirmado.
Nenhuma violação atual.

**Risco:** novo sítio com `$($PY_BIN -c "... print(...)")` entra sem ser verificado.

### Evasão 2: `sys.stdout.write('...\n')` — gap no discriminante de condição 2

**Confirmação:**

```bash
cat > /corpus/scripts/test_stdout_write.sh << 'EOF'
RESULT=$(python3 -c "import sys; sys.stdout.write('hello\n')")
EOF

CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh --scan-root /corpus
# RC=0 — OK [exempt/condition-2/...] no newline emitter
```

O gate verifica ausência de `print(`, `writelines` e `os.linesep`. `sys.stdout.write('x\n')`
não contém nenhum desses tokens — e é isento mesmo emitindo `\n` (que em Windows texto vira
`\r\n`).

**Impacto atual:** os 5 sítios isentos por condição-2 usam `sys.stdout.write(hexdigest())`,
`sys.stdout.write(sha256data.encode(...))` — sem `\n`. Nenhuma violação atual.

**Risco:** novo sítio usando `sys.stdout.write('resultado\n')` obteria isenção errônea.

### Evasão 3: `eval "$CMD"` onde `$CMD` expande para `python3 -c "..."`

**Confirmação:**

```bash
cat > /corpus/scripts/test_eval.sh << 'EOF'
CMD='python3 -c "print('"'"'hello'"'"')"'
RESULT=$(eval "$CMD")
EOF

CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh --scan-root /corpus
# RC=0 — gate passa, 0 candidatos para este sítio
```

`$(eval ...)` não contém `python3` na linha de abertura. Evasão total.

**Impacto atual:** nenhum uso de `eval` para capturar Python no codebase.

### Sem evasão: multi-linha com `strip_cr` além de 20 linhas

```bash
# strip_cr na linha 22 a partir da abertura $(python3...)
CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh --scan-root /corpus
# RC=1 — FAIL [test_multiline.sh:2] python3 capture without strip_cr
```

O gate CORRETAMENTE falha. `LOOKAHEAD_LINES=20` é suficiente para todos os padrões existentes no
repositório (heredocs fecham em < 15 linhas, pipelines em < 5).

### Processo substitution `< <(python3 ...)`

O gate não detecta este padrão (0 candidatos). O único sítio existente
(`check-no-literal-nul-in-source.sh:328`) foi corrigido aplicando `strip_cr` dentro da função
`_text_files_from_repo`. O gate não cobre, mas o sítio está normalizado.

---

## Resposta 3 — As 6 isenções derivadas são seguras?

**Veredito: sim para o corpus atual; uma das isenções tem brecha de staleness não declarada.**

### Como as isenções funcionam

As isenções são **derivadas do texto fonte** a cada execução — não são uma lista declarada.
Consequência: se um sítio hoje isento adicionar `print(` ao seu bloco, o gate remove a isenção
automaticamente na próxima execução. Não há problema do tipo "allowlist-sem-objeto" (o bug do
`check-output-encoding-declared`).

```
Condição-1 (1 isenção): $(command -v python3) — path da ferramenta, não execução
Condição-2 (5 isenções): bloco sem print( / writelines / os.linesep
```

Os 5 sítios de condição-2 usam:
- `check-agent-models-parity.sh:927,928`: `sys.stdout.write(hexdigest())`
- `check-thirdparty-parity.sh:84,85,87`: `sys.stdout.write(...)` sem `\n`

Nenhum emite `\n` hoje — isenção correta.

### A brecha: `sys.stdout.write('...\n')` não está no discriminante

A mesma brecha da Resposta 2 afeta as isenções: se um sítio isento hoje adicionar
`sys.stdout.write('resultado\n')` ao seu bloco, ele permanece isento mesmo passando a emitir
`\n`. O gate não detecta. Esta brecha é **consistente** entre evasão e isenção — é uma lacuna
de design no discriminante de condição-2, não uma inconsistência de implementação.

**Declaração explícita:** o discriminante de condição-2 não cobre `sys.stdout.write` com `\n`
como argumento de string literal. Aceito como residual derivado (ver Seção 5).

---

## Resposta 4 — Os 19 rótulos remanescentes — leitura de segurança

**Veredito: gap de cobertura de teste Windows para controles de segurança. Não é CRLF. Não
bloqueia esta REQ, mas merece rastreamento separado.**

### O que "ausente" significa

No shard runner (`run-gates-falsify-shard.sh:120`), "AUSENTE" = rótulo esperado no manifesto mas
não encontrado no arquivo `shard.actual` do run. Pode ser: shard que abortou antes de chegar ao
cenário, ou cenário que não emitiu o rótulo por um bug de infraestrutura no Windows.

### Distribuição dos 19

| grupo | contagem | é controle de segurança? |
|---|---|---|
| `git-branch-guard/*` | 9 | sim — previne agentes de criar branches |
| `git-branch-guard-global-script-integrity` | 2 | sim |
| `integration-assets/*` | 2 | não diretamente |
| `roadmap-req-frontmatter-path/*` | 2 | não diretamente |
| `credential-guard` | 1 | sim |
| `barrier` | 1 | sim |
| `trust-check` | 1 | sim |

### Contexto: o CRLF causou 87% dos 146 rótulos ausentes

A fix reduziu 146 → 19. Os 9 rótulos de `git-branch-guard` não eram CRLF — sobreviveram ao fix.
Isso significa que o mecanismo é outro. Hipóteses: (a) `assert_guard_exit` faz algo diferente no
Windows que aborta o shard antes de chegar ao rótulo; (b) dependência do binário Go compilado que
não está disponível no ambiente dos shards.

### Leitura de segurança

Os 9 cenários de `git-branch-guard` + `credential-guard` + `barrier` + `trust-check` são 12 de
19 — representam **verificações de comportamento de controles de segurança no Windows que não
estão sendo executadas** no CI. O guard em si é um binário Go portable — o comportamento não deve
diferir. O que não temos é a **evidência de que funciona**, não a evidência de que falha.

Isso é diferente de "o guard está quebrado no Windows". É "não sabemos, porque o harness não
termina esses cenários". Para controles de segurança, "não sabemos" merece ser rastreado.

**Não bloqueia esta REQ.** É escopo do próximo trabalho sobre o Windows census.

---

## Resposta 5 — Residual declarado

### 5.1 Fechado desde a Wave 0

| item | status |
|---|---|
| `PROSE_PAYLOAD` / `T65_BIG_PAYLOAD` (check-gates-falsify.sh:3917,4433) | FECHADO — ambos têm `\| strip_cr)` confirmado no código |
| `trackfw-attention-signal.sh` — classificado (a) mas fora do CI | FECHADO — linhas 18-19 têm `\| strip_cr \|\|` confirmado |

### 5.2 Aberto (residual declarado desta revisão)

**R1 — Variável `$PY_BIN` como invocador: gate não detecta**
O gate cobre `$(python3 ...)`. Captura via `$($PY_BIN ...)` ou `$("$PY_BIN" ...)` passa
invisível. Não afeta código atual — todos os sítios `$PY_BIN` já têm `strip_cr`. Risco: futuro.

**R2 — `sys.stdout.write('...\n')` não está no discriminante de condição-2**
Gate examina `print(`, `writelines`, `os.linesep`. Um sítio usando `sys.stdout.write('x\n')`
seria isento incorretamente. Não afeta código atual (todos os sítios `sys.stdout.write` sem `\n`
confirmados). Risco: futuro.

**R3 — Processo substitution `< <(python3 ...)` não coberto pelo gate**
O único sítio existente foi corrigido a nível de função. O gate não cobrirá futuro sítio neste
padrão. Risco: futuro.

**R4 — 19 rótulos remanescentes no Windows census**
12 dos 19 cobrem controles de segurança. Causa diferente de CRLF. Escopo da próxima fase do
Windows census.

**R5 — Fontes não-Python de CRLF**
Herança da Wave 0: comportamento de `git ls-files --eol` em Git Bash no Windows não medido.
Binários `.exe` nativos invocados por bash não examinados.

### 5.3 O que eu bloquearia

Nada nesta REQ está bloqueado para merge. As ressalvas R1–R3 são lacunas de design de um gate
novo — declaráveis como residuais do escopo desta REQ. R4 é trabalho medido para a próxima fase.

Se o próximo PR introduzir **novos sítios** usando `$PY_BIN`, `sys.stdout.write('\n')` ou
`eval`, o gate não os pegará — e o revisor precisa checar manualmente até que o gate seja
estendido.

---

## Evidências de comando (sumário)

```bash
# strip_cr: CR do meio preservado
printf 'a\rb\n' | sed $'s/\r$//' | xxd | head -1
# → 00000000: 610d 620a

# strip_cr: terminal CR removido
printf 'path/to/file\r\n' | sed $'s/\r$//' | xxd | head -1
# → 00000000: 7061 7468 2f74 6f2f 6669 6c65 0a

# Evasão $PY_BIN: RC=1 apenas por vacuidade, 0 candidatos detectados
CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh \
  --scan-root /corpus_pybin
# → Candidates: 0, vacuity FAIL

# Evasão sys.stdout.write: RC=0, isento por condição-2
CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh \
  --scan-root /corpus_stdwrite
# → OK [exempt/condition-2/...] no newline emitter

# Evasão eval: RC=0, 0 candidatos
CRLF_GATE_MIN_CAPTURES=1 bash scripts/check-crlf-normalize-capture.sh \
  --scan-root /corpus_eval
# → Candidates: 0 (vacuity fires, not violation)

# Multi-linha além de 20 linhas: gate CORRETAMENTE falha
# → FAIL [test_multiline.sh:2] python3 capture without strip_cr

# Sítios $PY_BIN sem strip_cr no codebase atual: 0
grep -rn '\$(\s*\$PY_BIN\|\$("\$PY_BIN"' scripts/*.sh | grep -v 'strip_cr'
# → scripts/check-gates-falsify.sh:3917 (tem | strip_cr) na linha 3925)

# strip_cr confirmado nos 20 sítios: 80 ocorrências (incluindo funções que cobrem N call sites)
grep -rn 'strip_cr' scripts/*.sh | grep -v 'lib-crlf-normalize' | grep -v 'check-crlf-normalize' | wc -l
# → 80
```
