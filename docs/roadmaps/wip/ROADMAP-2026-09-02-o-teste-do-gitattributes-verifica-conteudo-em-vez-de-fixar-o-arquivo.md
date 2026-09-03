---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-teste-do-gitattributes-fixa-o-arquivo-inteiro-e-o-repo-nao-pode-ter-regra-propria.md"
---

# ROADMAP: o teste do `.gitattributes` verifica conteúdo em vez de fixar o arquivo

> Date: 2026-09-02 | Status: wip

REQ: docs/req/REQ-2026-09-02-teste-do-gitattributes-fixa-o-arquivo-inteiro-e-o-repo-nao-pode-ter-regra-propria.md
ADR:

## ML-1A — Contenção em vez de igualdade, nos 3 runtimes

**Status:** ✅ Concluído

Os três afirmavam igualdade do arquivo **inteiro** contra o bloco gerado. A intenção está no
comentário do próprio teste e está certa — *"o arquivo deste repositório e o que o `init` gera não
podem divergir"*. O predicado é que era forte demais: não afirma que o bloco está lá, afirma que
**não há mais nada no arquivo**.

Nenhuma linha de produção foi tocada. O `init` já anexa corretamente a um `.gitattributes` que
exista — medido.

## ML-1B — Normalização de fim de linha, e o achado de paridade que apareceu no caminho

**Status:** ✅ Concluído

Ao rodar o ML-1A no Windows o teste continuou vermelho, por **outro motivo**: com
`core.autocrlf=true` o `.gitattributes` da raiz vem em CRLF no checkout, e o bloco é constante em LF.

E aí apareceu o que vale mais que a correção. **Mesmo arquivo, mesmo teste, três respostas:**

| runtime | como lê | resultado |
|---|---|---|
| Go | `os.ReadFile` — bytes crus | 🔴 reprova |
| Node | `readFileSync(…, 'utf8')` — sem tradução | 🔴 reprova |
| Python | `open(…, encoding=…)` — modo texto, **newline universal** | 🟢 passa |

O Python passava por acidente da biblioteca padrão, não por acerto do teste. Um trio de testes de
paridade em que um dos três é verde por motivo diferente dos outros dois não é paridade — é um
buraco que se parece com cobertura.

Corrigido normalizando **os dois lados** nos três, com o porquê no comentário de cada um.

## ML-1C — Falsificação, três controles

**Status:** ✅ Concluído

Todos medidos num clone descartável, na mesma sessão, para não misturar estado.

**Controle 1 — o bloco deriva** (`merge=union` → `merge=ours`, uma palavra). Os três **acendem**:

```
go: FAIL   node: fail 1   python: 1 failed, 4 passed
```

É isso que o teste existe para pegar, e a contenção não enfraqueceu.

**Controle 2 — repositório com regra própria antes do bloco.** Os três **passam**, e o predicado
antigo reprovaria o mesmo caso:

```
go: ok     node: fail 0   python: 5 passed
  igualdade (antes): REPROVA
  contenção (agora): PASSA
```

**Controle 3 — só o CRLF, sem deriva.** Antes, com o código do upstream:

```
go: FAIL   node: fail 1   python: 5 passed     ← a divergência de paridade
```

Depois:

```
go: ok     node: fail 0   python: 5 passed
```

## ML-1D — Um erro de medição meu, registrado

**Status:** ✅ Concluído

A primeira rodada do controle 2 deu Node reprovando enquanto Go e Python passavam. Quase escrevi
isso como defeito da minha própria edição. Era artefato: o clone não tinha `node_modules`, e
`node --test` reportou `pass 0 / fail 1` — que **se parece com um teste reprovando** e na verdade era
o arquivo inteiro falhando ao carregar com `Cannot find module 'yaml'`.

Fica registrado porque a forma do sintoma engana: no `node --test`, arquivo que não carrega e teste
que reprova produzem a mesma linha de contagem. O discriminante é `pass 0` — nenhum teste chegou a
rodar.

## Escopo negativo

Não muda o gerador nem o bloco. Não acrescenta regra de EOL ao `.gitattributes` do upstream — a
medição está na REQ, mas a decisão é deles; esta mudança é sobre o teste que **impede** a decisão.
