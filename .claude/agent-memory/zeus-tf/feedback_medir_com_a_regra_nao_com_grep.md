---
name: medir-com-a-regra-nao-com-grep
description: Contagem de defeito só vale se vier da regra/gate real; grep, tail e testes de string dão número plausível e errado — já aconteceu 4 vezes neste projeto
metadata:
  type: feedback
---

**Toda contagem que vai virar escopo de ML, número em roadmap ou frase de relatório tem de vir da
regra/gate que a define — nunca de `grep`, `tail` ou teste de string improvisado.**

**Why:** o modo de falha é sempre o mesmo — comando sai com exit 0, devolve um número **plausível**, e
ninguém questiona. Quatro ocorrências medidas neste projeto:

| o que eu media | com o quê | reportei | era |
|---|---|---|---|
| falhas de CI no Windows | `grep` sem o prefixo por linha do `gh run view --log` | 69 | **101** |
| REQs com `adr:` vazio | `[ -z "$v" ]` — mas o valor é `""`, string de 2 chars, não vazia | 0 | **128** |
| roadmaps de `done/` que falhariam em `wip/` | `grep` de marcador decorado | 13 | **~43** |
| FAIL no `make quality` | `make quality \| tail -40` | 0 | indeterminado — só vi 40 de 3816 linhas |

**How to apply:**

- Contagem de violação ⇒ rode o próprio `trackfw validate` / o gate, e conte a saída **inteira**
  redirecionada para arquivo. `grep -c '^FAIL' arquivo.log`, nunca `| tail`.
- Para saber o que uma regra faria num corpus que ela não varre hoje (ex.: `done/` vs `wip/`),
  **copie o corpus para onde a regra varre**, rode, conte, reverta. A árvore limpa e pushada torna
  isso barato — e é a única medição que não é palpite.
- Delta absurdo entre duas medições ⇒ **re-medir a base**, não explicar a subida. Foi assim que o
  69-vs-101 apareceu.
- Número vindo de subagente entra no handoff só depois de eu reproduzir. O 13-vs-43 era relatório de
  agente que eu ia repassar como "arquivos exatos, valores exatos".

⚠️ `npm/src/validator/index.js` tem um **byte NUL literal** no fonte: `grep` comum trata como binário
e **pula em silêncio**. Use `grep -a` sempre nesse arquivo, ou conclui-se que algo não existe quando
existe.

Relacionado: [[verificacao-visual-obrigatoria]] — mesma família (gate verde não prova entrega),
[[ler-a-req-nao-o-adr-vizinho]].
