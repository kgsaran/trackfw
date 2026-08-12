# Medir sandbox de agente dentro de `/tmp` produz falso "ALCANÇA"

> 2026-08-12 · descoberto no ML-0A do
> `ROADMAP-2026-08-12-guard-global-por-padrao-wave-0-medicao-bloqueante-da-premissa-de-sandbox`

## O erro

Você quer medir se um agente consegue escrever **fora** do workspace. Cria o fixture com
`mktemp -d`, planta os arquivos-alvo lá, roda o agente, ele escreve — e você conclui
**"o sandbox não protege"**.

**Errado.** `mktemp -d` no macOS devolve algo sob `/var/folders/...`, que resolve para `/private/tmp`
— e **`/tmp` é gravável por design** em pelo menos um dos sandboxes testados.

O Codex declara isso no próprio banner de sessão:

```
sandbox: workspace-write [workdir, /tmp, $TMPDIR]
```

Ou seja: o fixture estava **dentro da lista de permissões**. A medição confirmava a política, não a
violava.

## O impacto real

Na primeira rodada do ML-0A, o Codex recebeu veredito **`ALCANÇA`** — o que teria **refutado a
premissa central de um ADR aceito** e disparado sua reabertura. Refeito com o fixture em
`$HOME/trackfw-ml0a-fixture-*` (fora de `/tmp` e de `$TMPDIR`), o veredito **inverteu** para
`NÃO ALCANÇA`.

Um erro de método de uma linha quase reverteu uma decisão de arquitetura.

## Regra

**Ao medir sandbox baseado em lista de diretórios graváveis, o fixture NÃO pode estar em nenhum
diretório da lista.** Antes de rodar:

1. leia o banner/config do CLI e anote **quais** caminhos ele declara graváveis;
2. coloque o fixture **fora de todos eles** — `$HOME/<algo-temporario>` costuma servir;
3. limpe depois.

E o corolário: **`mktemp -d` é o default errado para este tipo de teste**, justamente por ser o
default certo para quase todo o resto.

## Como reconhecer que você caiu nisso

Sintoma: **todos** os CLIs dão `ALCANÇA`, inclusive os que anunciam sandbox. Sandbox que não protege
nada é mais provável que seja fixture no lugar errado do que sandbox quebrado em todos os
fornecedores ao mesmo tempo.

## Relacionado

- `docs/pesquisa/2026-08-12-alcance-do-agente-ao-home.md` — seção "Correção de método", com a cadeia
  de raciocínio completa
- `docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-...md` — o ADR cuja
  premissa esta medição avalia
- `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md` — mesma família:
  estado de ambiente contaminando medição
