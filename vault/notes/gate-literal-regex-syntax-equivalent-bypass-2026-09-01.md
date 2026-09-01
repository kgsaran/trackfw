# Gates de regressão baseados em regex literal são contornáveis por sintaxe equivalente, sem tocar comentário

**Contexto:** PR #236, `scripts/check-shell-posix-portability.sh` — gate que deveria impedir
`barrier.js`/`barrier.py` de reverter de `sh -c` explícito para `shell:true`/`shell=True` (o shell
do SO). A ML-2A já havia se precavido contra o caso óbvio: os próprios comentários do ML-1A citam
`shell: true`/`shell=True` em prosa, e um `grep` ingênuo no arquivo inteiro reprovaria a árvore
correta. A correção foi `assert_no_code_match`, que exclui linhas de comentário (`^\s*//`, `^\s*#`)
antes de grepar o padrão proibido.

## O achado

A exclusão de comentário **não é o ponto fraco** — é sólida para o estilo de comentário atual do
arquivo. O ponto fraco é que **as duas metades do gate casam texto literal, não semântica de
código**, e as duas evitam de formas independentes que compõem num bypass real:

1. **Metade negativa** (`assert_no_code_match`, regex `shell\s*:\s*true` / `shell\s*=\s*True`):
   evadida por sintaxe funcionalmente idêntica mas textualmente diferente —
   `{["shell"]: true, ...}` (notação de propriedade computada, JS válido) e
   `subprocess.run(cmd, **{"shell": True}, ...)` (desempacotamento de kwargs, Python válido, PEP
   448). Nenhuma das duas formas contém a substring `shell: true`/`shell=True` que o regex procura.
2. **Metade positiva** (`assert_count`, exige a assinatura antiga `spawnSync('sh', ['-c', command],
   {` / `["sh", "-c", cmd],` presente exatamente N vezes): **não exclui comentários**. Colocar a
   assinatura morta dentro de um comentário (`// spawnSync('sh', ['-c', command], {`) satisfaz o
   `grep -qF`/`grep -cF` sem que o código a execute de verdade.

Com as duas metades satisfeitas dessa forma, o gate acusa `OK — N assinaturas confirmadas` (exit 0)
sobre uma árvore com uma regressão real e funcional ao shell do SO no ponto de execução de gate.
Verificado ao vivo: extraí os dois trechos, rodei `node --check`/`python3 -m py_compile` (sintaxe
válida) e **executei** — `spawnSync('echo hi', {["shell"]: true})` e
`subprocess.run(cmd, **{"shell": True})` de fato invocam o shell do SO e interpretam o comando, não
são espantalhos inertes.

## Por que isso generaliza

Qualquer `scripts/check-*.sh` deste repositório que combine (a) um `assert_count`/`assert_has`
positivo sobre uma string exata e (b) um `assert_no_code_match`/grep negativo sobre outra string
exata tem a mesma classe de fragilidade: a metade positiva pode ser satisfeita por menção morta
(comentário, string, código nunca chamado), e a metade negativa pode ser evadida por qualquer forma
sintática que produza o mesmo efeito em runtime sem casar o padrão textual. Isso não é específico
de `shell:true` — vale para qualquer par proibição-de-padrão + linguagem com mais de uma forma de
escrever a mesma coisa (quase todas).

## O que fazer diferente da próxima vez

- Gates de "nunca regrida para X" que importam de verdade (superfície de execução de shell, auth,
  etc.) não deveriam depender só de regex sobre texto fonte. A forma mais forte é uma checagem
  **comportamental**: rodar o código real com um `$PATH`/ambiente instrumentado e provar, por
  observação (não por leitura de código), que nenhuma chamada com `shell:true`/`shell=True`
  ocorre — nem por instrumentação (mock/spy no `spawnSync`/`subprocess.run`), nem por efeito
  observável (ex.: `PATH=/tmp/fakesh:$PATH` e confirmar que o fake NUNCA roda, em vez de confirmar
  que uma string específica aparece/não aparece no arquivo).
- Quando um gate baseado em regex é aceito como suficiente (custo-benefício razoável para o caso),
  **anotar `partial=` no `docs/cli-parity.md`**, nomeando explicitamente "regressão literal/
  ingênua coberta; regressão por sintaxe sintaticamente equivalente não coberta" — nunca `gate=`
  puro implicando cobertura plena, que é o que a ML-2A escreveu aqui e que este achado invalidou.
- Ao revisar um gate novo desse tipo, tentar ativamente reescrever o padrão proibido de pelo menos
  uma forma sintática alternativa válida na linguagem-alvo antes de aceitar a alegação de "gate
  reprova regressão".

## Referências

- `scripts/check-shell-posix-portability.sh`
- `docs/cli-parity.md:1951` (anotação `gate=` a corrigir para `partial=`)
- `docs/seguranca/2026-09-01-barreira-do-shell-de-gate.md` (relatório completo desta barreira, §4)
- `docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md` (Wave 0 — modelo de ameaça que
  originou o ADR/REQ desta troca)
