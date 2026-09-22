---
name: marcador-de-fim-nunca-escrito
description: Subagente que roda make quality em background e espera um marcador (exit=/RC=) trava para sempre — o make termina mas o marcador nunca é escrito
metadata:
  type: project
---

**O padrão que trava o subagente, observado 3× em 2026-09-20 e 2× em 2026-09-18:**

```bash
make quality > quality.log 2>&1; echo "exit=$?" >> quality.log   &   # background
until grep -q "^exit=" quality.log; do sleep N; done                # espera eterna
```

O `make` **termina normalmente** — o log fica com **1468 linhas**, que é o tamanho de um run completo,
e a última linha é `run-gates-falsify-parallel: suite completa ... guarda de conjunto OK`. Mas a linha
`exit=` **nunca é escrita**, e o laço espera para sempre.

**Why:** o processo que escreveria o marcador morre entre o fim do `make` e o `echo` — o harness
encerra o shell de background, ou o pai termina antes. O marcador depende de um segundo comando que
não é atômico com o primeiro.

**How to apply — diagnóstico em três medidas, nesta ordem:**

```bash
ps -eo comm | grep -cx make                    # 0 = nenhum make real rodando
wc -l < <log>                                  # 1468 = run completo
echo $(( $(date +%s) - $(stat -f %m <log>) ))  # idade; > 600s com make ausente = travado
```

🔴 **Nunca use `pgrep -f "make quality"`** para saber se a barreira roda: o próprio laço de espera
contém essa string no seu comando, e o `pgrep -f` **casa a si mesmo**. Custou 5 horas em 2026-09-20,
reportando "ainda rodando" sobre um `make` terminado. Use `ps -eo comm | grep -cx make`.

**Prevenção, a pôr em todo handoff:** *"rode `make quality` em **primeiro plano**; nada de laço de
polling em background"*. Já está nos prompts, e ainda assim reincide — o executor recria o padrão
sozinho quando o comando é longo.

🔴 **Consequência que não é só cosmética:** na terceira ocorrência o subagente travou **antes de fazer
o trabalho** — rodou a barreira como baseline, ficou preso no laço, e a árvore ficou **limpa**
(`git status` vazio). O ML precisou ser **redespachado do zero**. Sempre confira `git status` antes de
concluir que "só faltou o relatório".

Ver [[processos-orfaos-de-subagente]] e [[o-instrumento-mente]].
