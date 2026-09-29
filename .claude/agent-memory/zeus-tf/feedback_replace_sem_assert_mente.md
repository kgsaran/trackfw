---
name: replace-sem-assert-mente
description: Toda edição programática de artefato precisa de assert na âncora e verificação do efeito depois — replace silencioso produz commit que afirma o que não fez
metadata:
  type: feedback
---

🔴 **`str.replace()` que não casa não falha — devolve a string intacta.** Toda edição programática de
roadmap/REQ/ADR precisa de **`assert` na âncora antes** e **verificação do efeito depois**.

**Why:** em 2026-09-28 (REQ do #445) marquei os ACs do ML-1A e ML-1B com `replace` usando strings
que continham **negrito markdown inexistente no arquivo**. Nenhum casou. Eu **escrevi no commit que
marquei** — e não marquei. O roadmap ficou com `**Status:** ✅ Concluído` convivendo com ACs abertos,
e um AC saiu **corrompido** por concatenação parcial. Só descobri turnos depois, ao estranhar
"25 ACs abertos".

É o mesmo defeito que eu cobro dos executores em todo handoff: **não verifiquei que a minha própria
edição fez efeito**.

**How to apply:**
```python
assert old in s, old[:60]      # antes — falha alto se a âncora mudou
s = s.replace(old, new, 1)
```
E **depois**, sempre, uma medição que o próximo leitor possa repetir:
```bash
grep -c '^- \[ \]' "$M"                                   # quantos sobraram
awk '/^### ML-/{ml=$0} /^\*\*Status:\*\*/{d=($0~/✅/)} /^- \[ \]/{if(d)print ml}' "$M"   # contradição Status×AC
```

⚠️ **Sintoma que denuncia:** `Status: ✅` na mesma seção que um `- [ ]`. Se existir, uma das duas
afirmações é falsa — e o `barrier` **vai** pegar, mas só na hora em que você queria fechar.

Relacionado: [[medir-com-a-regra-nao-com-grep]], [[o-instrumento-mente]].
