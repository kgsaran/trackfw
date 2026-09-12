# O `grep` deste ambiente omite silenciosamente os dois fontes centrais do CLI Node

> 2026-09-12 · descoberto ao triar a frente paralela da REQ órfã, depois de concluir **errado** que
> `note_orphan` estava ausente no Node

## O erro que isto produziu

Eu ia despachar um agente para implementar `note_orphan` no CLI Node, com base nesta medição:

```
$ grep -rn "note_orphan" npm/src/
(nada)
```

**A regra já estava implementada** — três sítios, incluindo o `applyRule`:

```
$ /usr/bin/grep -rn "note_orphan" npm/src/
npm/src/validator/index.js:1621:  return [inspectionDiagnostic('note_orphan', indexPath, err)]
npm/src/validator/index.js:3469:  note_orphan: 'warning',
npm/src/validator/index.js:3758:  applyRule('note_orphan', validateNoteOrphan(), violations, warnings)
```

## Causa

O shell do Claude Code **sombreia** o `grep` com uma função (`shell-snapshots/snapshot-*.sh`, ~linha
3115) que executa:

```
ugrep -G --ignore-files --hidden -I --exclude-dir=.git ...
```

`-I` = *ignore binary files*. E ugrep classifica como binário qualquer arquivo com **um** NUL byte.

**Dois fontes do repositório têm NUL literal, e o uso é legítimo** — separador de chave composta,
idioma comum para evitar colisão de delimitador:

```
npm/src/validator/index.js:2021   `${m.raw}\0${m.typeIsCommand}`
npm/src/integrations/doctor.js:94  [...].join('\0')
```

🔴 **`npm/src/validator/index.js` tem 190 KB e é o maior e mais central fonte do CLI Node.** Ele é
invisível ao `grep` padrão deste ambiente.

## Por que passa despercebido

O GNU grep imprime `Binary file X matches`. O ugrep com `-I` **não imprime nada** — o arquivo
simplesmente não aparece na saída. A busca "funciona", retorna zero, e zero parece resposta.

Prova de que é omissão e não leitura errada:

```
$ grep -rc "note_orphan" npm/src/validator/
npm/src/validator/git-exec.js:0
npm/src/validator/traceid.js:0
        ← index.js nem é listado
```

## Como trabalhar

- Para conclusão de **ausência** (*"não existe em lugar nenhum"*), usar `/usr/bin/grep`. Conclusão
  negativa é a que este defeito falsifica; conclusão positiva não é afetada.
- Alternativa: `git grep`, que não usa a heurística de binário do ugrep para arquivos rastreados.
- 🔴 **Toda REQ de paridade cuja evidência é "ausente no Node" precisa ser reverificada** — pode
  estar obsoleta. Já confirmado num caso: `REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node`.

## O padrão

É o mesmo do `FORCE_COLOR` (ver [[dois-falsos-vermelhos-ambiente-e-heuristica-2026-09-12]]), com o
sinal invertido e por isso muito pior: lá o ambiente produzia **falso vermelho**, que se percebe
porque incomoda. Aqui ele produz **falso verde** — "não achei nada" — que se aceita e vira decisão.

E a decisão que ele quase produziu era despachar um agente para reimplementar código que já existe.
