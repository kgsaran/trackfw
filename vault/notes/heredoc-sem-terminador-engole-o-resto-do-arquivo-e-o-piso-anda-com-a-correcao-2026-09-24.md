# Heredoc sem terminador engole o resto do arquivo — e o piso de não-vacuidade anda junto com a correção

> 2026-09-24 · `artemis-tf` · ML-1B da
> `ROADMAP-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`
> Gate: `scripts/check-interpolated-path-in-python.sh`

Duas constatações que **não** estão no parecer do ML-0A
(`docs/seguranca/2026-09-24-caminhos-interpolados-no-codigo-python.md`) e que custam tempo a quem
mexer em qualquer varredor de corpo Python embutido em shell.

---

## 1. 🔴 Heredoc sem terminador remove sítios do censo em SILÊNCIO

Todo varredor line-based que reconheça `<<DELIM` entra em estado "dentro do corpo" e só sai ao
encontrar a linha `DELIM`. **Se o terminador não existe, o corpo consome o arquivo até o EOF** — e
todos os blocos seguintes daquele arquivo somem da contagem, sem erro, sem aviso.

**Medido ao simular a migração dos 5 blocos do ML-1A para heredoc citado.** A primeira simulação
trocou só a abertura (`python3 -c "` → `python3 - <<'PY'`) e **esqueceu de fechar**:

| simulação | corpos | expansíveis |
|---|---|---|
| árvore real | 140 | 83 |
| migração **sem** acrescentar a linha `PY` | 109 | 53 |
| migração **correta** (abertura **e** terminador) | 130 | 70 |

Os 31 corpos que sumiram não foram convertidos: foram **engolidos**. O número 53 é artefato do
instrumento, não medida da árvore — e se eu tivesse calibrado o piso por ele, teria calibrado por
um erro meu.

🔴 **É a forma fail-open desta família de gate**: o corpus encolhe, o gate examina menos, e reporta
verde. A defesa é o **piso sobre corpos reconhecidos** (PISO 1 do gate), que é exatamente o que pega
uma queda de 140 → 109.

**Regra prática:** ao simular refatoração de heredoc, converta **o par** (abertura + terminador), e
**confira a contagem total** antes de usar o resultado para qualquer coisa. Se o total caiu mais do
que o número de blocos que você tocou, o seu conversor quebrou o arquivo.

---

## 2. 🔴 O piso de não-vacuidade se move quando a correção CERTA é aplicada

Esta REQ tem **dois** precedentes vivos e legítimos para corrigir um sítio:

| precedente | forma | efeito no censo do gate |
|---|---|---|
| `check-thirdparty-parity.sh:167` | `-c "… open(sys.argv[1]) …" "$path"` | corpo continua **EXPANSÍVEL** |
| `check-thirdparty-parity.sh:176` | `python3 - "$path" <<'PY'` | corpo vira **LITERAL** |

O ML-1A escolheu o `:167`, então a árvore ficou em 83 expansíveis. **Mas o `:176` é igualmente
correto** — e se alguém migrar por ele, até 5 corpos saem de EXPANSÍVEL e entram em LITERAL.

Medido: **130 corpos / 70 expansíveis** no cenário `:176`.

🔴 **Um piso calibrado só pela árvore de hoje reprovaria por CALIBRAGEM, não por defeito** — o gate
acusaria alguém que acabou de aplicar a correção recomendada. Por isso os pisos
(`MIN_BODIES=100`, `MIN_EXPANDING=55`) estão abaixo do **pior dos dois cenários**, não do atual.

**Generalização:** quando o gate conta a população que ele examina, e a **correção que ele exige**
muda essa população, o piso tem de ser calibrado contra o estado **pós-correção mais desfavorável**.
Vale para qualquer gate cujo remédio mude a forma sintática que o discriminante enxerga.

---

## 3. Por que existem DOIS pisos, e qual carrega o peso

- **PISO 1 (corpos reconhecidos)** pega corpus vazio/podado e o detector de bloco quebrado (§1).
- **PISO 2 (corpos expansíveis)** pega o **classificador de citação** quebrado — o cenário em que
  tudo é declarado LITERAL. Medido: com todo `-c "` virando `<<'PY'`, o resultado foi
  **65 corpos / 5 expansíveis**. O PISO 1 sozinho passaria num corpus grande com o classificador
  quebrado; é o PISO 2 que reprova.

🔴 A vacuidade silenciosa deste gate **não** é "não achei arquivo": é "achei tudo e classifiquei
tudo como fora do meu alcance". Guarda que só conta arquivos não a vê.

---

## 4. Auto-referência: o braço de falsificação pode acusar o arquivo que o hospeda

`check-gates-falsify.sh` é escaneado pelo próprio gate. Um braço que escrevesse
`python3 -c "… open('$VAR') …"` verbatim faria o scanner abrir um corpo sobre o texto do `printf` e
**acusar `check-gates-falsify.sh`**. A indireção por `%s` / `"$PY200"` (mesma nota do Cenário 197)
resolve — e foi **verificada**, não presumida: com o bloco de 9 braços colado no arquivo, o gate
acusa **0**.

---

## Links

- [[msys-nao-converte-caminho-embutido-em-string-maior-2026-09-07]] — o mecanismo de fundo
- Parecer do ML-0A: `docs/seguranca/2026-09-24-caminhos-interpolados-no-codigo-python.md` (§1.4 é a
  lista de não-flag obrigatória)
- Gates irmãos com a mesma arquitetura de piso: `check-crlf-normalize-capture.sh`,
  `check-unguarded-capture-rc.sh`
