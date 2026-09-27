# Wave 0 — ML-3A: Modelo de ameaça do sítio distribuído (literal embutido sem CRLF)

> 2026-09-27 · hades-tf · REQ-2026-09-23 · Wave 3 reabertura
> Branch: `fix/literal-embutido-nao-normaliza-crlf-e-o-gate-nao-o-varre`

---

## 1. Completude da enumeração

O roadmap declara dois sítios no literal distribuído:

| linha | captura | classificação dada |
|---|---|---|
| `scaffold.go:924` | `TOOL=$(echo "$INPUT" \| PYTHONIOENCODING=utf-8 python3 -c "...get('tool_name',...)")` | captura stdout de python3 sem strip |
| `scaffold.go:925` | `MSG=$(echo "$INPUT" \| PYTHONIOENCODING=utf-8 python3 -c "...[:300])")` | captura stdout de python3 sem strip |
| `scaffold.go:2202` | `py_compile` invocação | fora — não captura saída |

Verificado com `grep -n "PYTHONIOENCODING\|python3" internal/generators/scaffold.go` e leitura direta das linhas 911–946.

**Superfície não examinada pelo roadmap — mesmo sintoma, mesmo arquivo:**

O script distribuído contém uma quarta captura que nunca passou pelo filtro da enumeração porque não vem de python3:

```bash
# scaffold.go:928, no script gerado
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 \
  | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' \
  | tr -d '"' | tr -d "'" || true)
```

Se `trackfw.yaml` tiver terminações CRLF (edição com Notepad no Windows), `grep` emite a linha com `\r\n`. O `sed` remove o prefixo de chave e comentários mas não toca `\r`. Os dois `tr -d` removem apenas `"` e `'`. O `\r` sobrevive.

Medido:

```
printf "roadmap_dir: docs/roadmaps\r\n" > /tmp/test.yaml
val=$(grep "^roadmap_dir:" /tmp/test.yaml | head -1 | sed "s/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//" | tr -d '"' | tr -d "'")
printf "%s" "$val" | xxd | head -1
# 00000000: 646f 6373 2f72 6f61 646d 6170 730d  docs/roadmaps.
```

`ROADMAP_DIR` recebe `docs/roadmaps\r` (0x0d ao final).

Diferença crítica em relação a TOOL/MSG: `ROADMAP_DIR` é um **caminho de arquivo**, não um valor JSON. Ele vai direto para `mkdir -p "$ROADMAP_DIR"` e para o redirecionamento `> "$ROADMAP_DIR/.trackfw-attention.json"` — sem passar por `tr -d '\000-\037'`. O `case` de validação de caminho só rejeita caminhos absolutos e traversals; `\r` passa.

**Veredito de completude:** A enumeração do roadmap está **incompleta** em uma superfície. Os dois sítios de python3 estão corretos. O `ROADMAP_DIR` é o mesmo sintoma (bash consumindo saída CRLF), instância diferente (de `grep` sobre arquivo, não de python3), consequência diferente (afeta path de escrita, não valor JSON). A regra do projeto ("mesmo sintoma investiga no mesmo roadmap") exige veredito antes de fechar a Wave 3. Veredito ao final desta seção.

---

## 2. Modelo de ameaça

### 2.1 Superfície: TOOL e MSG (python3 stdout)

**O que o adversário emite:** JSON com `tool_input.command` ou `tool_input.question` que contém `\r`, `\r\n`, caracteres de controle, `"`, ou `\`.

**O que o script faz com isso:**

```bash
# captura com CRLF (Windows text mode: python3 print() emite \r\n)
TOOL=$(echo "$INPUT" | python3 -c "...print(d.get('tool_name',''))..." 2>/dev/null || echo "")
MSG=$(echo "$INPUT"  | python3 -c "...print(...)[:300]..."              2>/dev/null || echo "Agent needs attention")

# sanitização antes do JSON
TOOL_ESC=$(echo "$TOOL" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')
MSG_ESC=$(echo "$MSG"   | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')
```

**Achado medido — refutação da afirmação do roadmap:**

O roadmap afirma que "o defeito do #353 volta a ser distribuído" (JSON com CR). A medição refuta isso para o conteúdo do JSON:

```bash
printf "content\r" | tr -d '\000-\037' | xxd | head -1
# 00000000: 636f 6e74 656e 74  content   ← \r removido

printf "\r" | tr -d '\000-\037' | wc -c
# 0   ← \r (0x0D = octal 015) está no intervalo \000-\037

# [:300] com CR na posição 300:
python3 -c "s='A'*299+chr(13)+'X'*50; print(s[:300],end='')" | tr -d '\000-\037' | wc -c
# 299   ← CR removido; os 299 As permanecem
```

`\r` é ASCII 13 = octal 015, dentro do intervalo `\000-\037` (octal 0–31). A sanitização existente **já remove** todo `\r` de TOOL e MSG antes que chegue ao JSON, em ambos os caminhos (jq e python3). O script gerado sem `strip_cr` produz JSON sem `\r` hoje.

**Consequência para a justificativa do ML-3B:** a correção do literal não previne JSON inválido (o JSON já está protegido). A justificativa correta é: (a) paridade literal↔cópia versionada (ML-3D) e (b) eliminar o `\r` intermediário na variável bash antes da sanitização — defesa em profundidade que reduz dependência da sanitização downstream.

**Injeção de JSON:** `"` e `\` são escapados por `sed`. `\000-\037` remove controles. O `printf` usa `%s` (não format string). Não há caminho de injeção de estrutura JSON pelo conteúdo de TOOL ou MSG.

### 2.2 Superfície: ROADMAP_DIR (grep sobre arquivo)

**Ameaça medida:** `trackfw.yaml` com CRLF → `ROADMAP_DIR="docs/roadmaps\r"`. Este valor não passa por nenhuma sanitização de controles. Ele entra em:

1. `mkdir -p "$ROADMAP_DIR"` — cria diretório com `\r` literal no nome. No Linux/macOS, `\r` é caractere válido em nome de arquivo; o diretório é criado com esse nome e o `.trackfw-attention.json` é escrito nele — **invisível ao board** (`trackfw serve` observa `docs/roadmaps`, não `docs/roadmaps\r`).
2. `"$ROADMAP_DIR/.trackfw-attention.json"` — o arquivo é gravado no diretório errado; o sinal de atenção nunca aparece.

O `case` de sanitização de caminho (`/*|../*|*/../*|*/..|..`) não cobre `\r` no final.

**Severidade:** funcional — o hook não injeta, não vaza, não executa nada novo. Mas o sinal de atenção é silenciado em projetos com `trackfw.yaml` CRLF, exatamente o contexto Windows onde a Wave 3 se aplica.

**Classificação:** mesmo sintoma que os sítios de python3. Causa raiz diferente (grep sobre arquivo, não python3 stdout). A regra do roadmap exige veredito: este sítio entra como ML adicional nesta REQ ou é declarado fora do escopo com razão?

**Veredito sobre ROADMAP_DIR:** a causa é diferente (não é python3 stdout — é o modo texto do sistema de arquivos / git checkout no Windows). A correção é `sed 's/\r//'` na extração de ROADMAP_DIR, isolada da correção de python3. Mesma REQ, novo ML (ML-3E), ou fora do escopo declarando a razão. Recomendo ML-3E **nesta REQ** porque: (1) mesmo sintoma, (2) mesmo script distribuído, (3) a Wave 3 já está aberta. Fica registrado; a decisão de escopo é do arquiteto.

### 2.3 Adversário: o implementador apressado

O adversário deste Wave 0 é quem fecha a Wave 3 sem medir o ROADMAP_DIR. O caminho: o gate ML-3A (`strip_cr >= 1 em scaffold.go`) ficará verde quando ML-3B adicionar strip_cr às linhas 924–925. O ML-3D comparará a cópia versionada com o literal. Mas nada mede se ROADMAP_DIR está protegido. O gate `check-crlf-normalize-capture.sh` não cobre `grep` sobre arquivo — ele só olha captura de stdout de python3.

O gate passa. CI fica verde. O sinal de atenção silencia em projetos CRLF no Windows. Ninguém percebe porque o sintoma é ausência de notificação, não erro visível.

---

## 3. Falsificação nas duas direções

### 3.1 Pergunta 2 — `strip_cr` antes ou depois de `[:300]`?

**Decisão: DEPOIS. Não é negociável estruturalmente.**

Raciocínio:

- `[:300]` executa **dentro do subprocesso python3**, sobre a string Python, antes de `print()`.
- `strip_cr` (= `sed $'s/\r$//'`) executa **no bash**, sobre o stdout do subprocesso após o processo encerrar.
- Eles vivem em contextos diferentes; não é possível executar `strip_cr` antes de `[:300]` sem reescrever o python3 para receber o valor como argumento e fazer o strip interno — o que troca a forma de captura e está fora do escopo do ML-3B.

**O `\r` de interesse (Windows text mode) é gerado por `print()` DEPOIS de `[:300]`:**
- Python3 faz `s[:300]` → obtém a string truncada.
- `print(s[:300])` emite `conteudo\n` em Unix ou `conteudo\r\n` em Windows (modo texto).
- O `\r` é o terminador de linha, sempre pós-conteúdo. Nunca entre `[:300]` e o conteúdo.

**Caso adversarial com `\r` embutido no conteúdo:**
- `tool_input.command` = 299 'A' + `\r` + 50 'X'. Python3 faz `[:300]` → 299 'A' + `\r` + 'X'. Emite com `\n` (ou `\r\n` no Windows).
- Após `sed 's/\r$//'`: remove apenas o `\r` terminal (da saída de print). O `\r` embutido em posição 300 permanece na variável.
- Após `tr -d '\000-\037'` na sanitização: o `\r` embutido é removido antes do JSON.
- Não há JSON escape sendo partido: o JSON encoding (`sed 's/\\/\\\\/g; s/"/\\"/g'`) roda depois da sanitização. A string que entra na sanitização é o valor Python cru, sem codificação JSON.

**Nenhuma das duas ordens produz JSON inválido.** A diferença é somente se o `\r` embutido permanece na variável bash (antes da sanitização) ou não. Com `sed 's/\r$//'` após `[:300]`, permanece; é removido na sanitização. O resultado final no JSON é idêntico.

**Falsificação — falso positivo (normalização longe demais):**
A preocupação do roadmap era: `strip_cr` poderia comer um `\r` legítimo dentro dos 300 chars. Com `sed $'s/\r$//'` (somente `\r` terminal), isso não ocorre: `\r` na posição 299 de um valor de 300 chars NÃO é removido por `sed 's/\r$//'` (não é o último char se houver o 300º). Somente o `\r` que é o último char da linha (terminador de `print()`) é removido. Medido: `printf "A\rB" | sed $'s/\r$//'` emite `A\rB` sem alteração.

**Falsificação — falso negativo (normalização incompleta):**
Com `sed 's/\r$//'`, um valor que termine em `\r` antes do terminador do `print()` não existe na prática: python3 em modo texto emite `valor\r\n`, e o `\r` antes do `\n` é removido por `sed 's/\r$//'`. Caso especial: `print("X\r")` em Windows → `X\r\r\n`. `sed 's/\r$//'` remove o último `\r`, deixando `X\r\r` → então command substitution strips `\n`... wait, sed opera linha a linha. Medido localmente não existe risco real porque o bash command substitution retira trailing newlines e `tr -d '\000-\037'` remove qualquer `\r` residual.

**Constraint obrigatório para ML-3B — pipefail:**

Antes da adição: `MSG=$(... | python3 ... 2>/dev/null || echo "fallback")`
Depois: `MSG=$(... | python3 ... 2>/dev/null | sed $'s/\r$//' || echo "fallback")`

Com `set -o pipefail` (presente no script, linha 3): `sed` succeeds, mas `pipefail` propaga o exit status não-zero de python3. O `||` dispara. Verificado:

```bash
set -euo pipefail
MSG=$(echo "NOT JSON" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" \
  2>/dev/null | sed $'s/\r$//' || echo "Agent needs attention")
# MSG = "Agent needs attention"   ← correto
```

Sem `pipefail`: `sed` seria o último processo (exit=0), `||` nunca dispararia, `MSG` ficaria vazio. O script tem `set -euo pipefail` na linha 3, mas **ML-3B deve documentar essa dependência** em comentário inline — qualquer refatoração que remova ou condicione `pipefail` quebraria o fallback silenciosamente.

### 3.2 Pergunta 3 — Regeneração sobrescreve conteúdo do consumidor?

**Sim. Sem aviso. Mesma causa que #445.**

Evidência: `scaffold.go:995`:
```go
if err := os.WriteFile(signalPath, []byte(attentionSignalScript), 0755); err != nil {
```

`os.WriteFile` abre com `O_WRONLY|O_CREATE|O_TRUNC`. Sem verificação de existência prévia, sem diff, sem prompt. Idêntico a `scaffold.go:789` (`writeTrackfwConfig` que sobrescreve `trackfw.yaml` — causa do #445).

**É a mesma causa?** Mecanismo idêntico: `os.WriteFile` sem guard sobre arquivo gerado e potencialmente customizado pelo consumidor. A mesma correção que resolver #445 (preservar valores existentes, acrescentar só o ausente, ou pelo menos recusar e avisar) resolveria os dois.

**Routing correto pela Regra Dura de Causa Raiz:** o fix vai na REQ do #445, não aqui. Não se abre REQ nova porque a causa é a mesma. Não se expande o escopo desta Wave 3 porque o esforço de fix (guard de existência em `os.WriteFile` + lógica de merge/aviso) não é trivial e já tem issue e REQ aberta.

**Assimetria de severidade que justifica o routing separado:** uma vez que ML-3B corrija o literal, regeneração do attention signal passa a escrever conteúdo correto. O consumidor que tinha customizado o script perde a customização, mas o script distribuído está certo. Para `trackfw.yaml`, o consumidor perde sua configuração deliberada e os comandos passam a enxergar um estado de governança que não é o real — o dano é persistente e silencioso. Prioridade do #445 é maior; o attention signal overwrite é registrado como residual.

---

## 4. Residual declarado

| item | aceito ou não coberto | razão |
|---|---|---|
| JSON estrutural por TOOL/MSG | aceito como seguro | `tr -d '\000-\037'` + sed-escape cobrem todos os vetores medidos |
| `\r` em TOOL/MSG via python3 | aceito: coberto pela sanitização existente | `strip_cr` no literal é defesa em profundidade, não barreira primária |
| `\r` em ROADMAP_DIR via grep/CRLF yaml | **não coberto por ML-3B/3C** | causa raiz diferente (não python3 stdout); requer ML-3E ou declaração explícita de fora do escopo pelo arquiteto |
| Regeneração sobrescreve customização do consumidor | **não coberto nesta Wave 3** | mesma causa que #445; pertence à REQ do #445 |
| `\r` sem `pipefail` no fallback de MSG | aceito como dependência documentada | script tem `set -euo pipefail` fixo; ML-3B deve comentar |
| jq path (quando jq disponível) | aceito: jq opera em binário, não emite CRLF | o problema é exclusivo do fallback python3 |

---

## Vereditos por pergunta

### Q1 — O que muda no modelo de ameaça ao introduzir `strip_cr`

**`strip_cr` não altera o modelo de ameaça para o JSON gravado.**

A sanitização `tr -d '\000-\037'` já remove `\r` (ASCII 13, octal 015, dentro do intervalo) de TOOL e MSG antes do JSON. Medido com hexdump. O conteúdo de `.trackfw-attention.json` é idêntico com ou sem `strip_cr` no literal.

O que `strip_cr` muda: remove o `\r` da variável bash intermediária, reduzindo dependência da sanitização downstream. É defesa em profundidade, não prevenção de defeito ativo.

A afirmação do roadmap "o defeito do #353 volta a ser distribuído" está **parcialmente incorreta para o JSON**: o CR não chega ao JSON gravado no script atual. A afirmação é correta para a paridade literal↔cópia versionada (o literal ainda não tem strip_cr; a cópia versionada tinha) e para a superfície ROADMAP_DIR (que não tem sanitização de `\r` e afeta o caminho de escrita).

### Q2 — `strip_cr` antes ou depois de `[:300]`?

**DEPOIS. Implementar como `| sed $'s/\r$//'` na pipe do subprocesso python3.**

Razão estrutural: `[:300]` é interno ao python3; `strip_cr` é externo no bash. Não há como reordenar.

Razão de segurança: o `\r` de interesse é o terminador de linha adicionado por `print()` APÓS `[:300]`. Nenhuma sequência de escape JSON pode ser partida (o encoding JSON é downstream).

Forma autocontida recomendada: `sed $'s/\r$//'`
- Remove apenas o `\r` terminal (não embedded CRs no conteúdo)
- `$'...'` é ANSI-C quoting — portável entre BSD sed (macOS) e GNU sed (Linux), igual ao `lib-crlf-normalize.sh`
- Não introduz nova dependência de sistema

Constraint obrigatório: ML-3B deve incluir comentário inline dizendo que a cadeia `... | sed $'s/\r$//' || echo "fallback"` depende de `set -o pipefail` para que o fallback dispare em falha de python3. Sem `pipefail`, `sed` (last in pipeline) retorna 0 e o fallback silencia.

### Q3 — Regeneração pode sobrescrever conteúdo do consumidor?

**Sim. Sem aviso. É a mesma causa que #445.**

`scaffold.go:995` usa `os.WriteFile` incondicional. Mesma linha de código que `#445` usou em `scaffold.go:789`. O fix pertence à REQ do #445 (mesma causa); não abre REQ nova. Registrado como residual desta Wave 3.

---

## Itens para o arquiteto

1. **ROADMAP_DIR com CRLF** (medido, não coberto): decidir se entra como ML-3E nesta REQ ou é declarado fora de escopo com razão escrita. Recomendo ML-3E: mesmo script distribuído, mesma Wave 3 já aberta.

2. **Afirmação do roadmap sobre #353**: a frase "o defeito do #353 volta a ser distribuído" deve ser qualificada — o CR não chega ao JSON (sanitização existente cobre). O defeito que a Wave 3 corrige é a divergência literal↔cópia e o `\r` intermediário na variável bash.

3. **Constraint de pipefail**: adicionado ao handoff do ML-3B.
