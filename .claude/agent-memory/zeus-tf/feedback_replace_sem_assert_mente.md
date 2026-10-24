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

**Recorrência 2026-10-23 (corte por posição):** para tirar um bloco que um agente anexou ao `docs/agents-working-context.md` do checkout principal, cortei o arquivo em `s[:index(marcador)]`, supondo que o bloco estava no fim. Ele estava logo depois do título. Resultado: 47.329 linhas apagadas, recuperadas com `git show HEAD:<arquivo> > <arquivo>` só porque o diff anterior tinha 0 linhas removidas. Para remover um bloco, apague **o próprio bloco** (do marcador ao fim dele) e confira `git diff --numstat` antes e depois. Se o arquivo não tinha mudança local antes do agente, a restauração certa é o conteúdo do HEAD, não um corte.
