# Comentário inline citando `<<` arma o rastreador de heredoc e derruba o arquivo inteiro da população do gate

> 2026-09-02 · ML-2B do `ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate`
> Arquivo: `scripts/check-output-encoding-declared.sh` · Achado B1 de `docs/qualidade/2026-09-02-parecer-codificacao-declarada.md`

## O mecanismo, em uma frase

Um parser de shell que rastreia heredoc procurando o delimitador **na linha inteira** entra em
estado de heredoc por causa de uma linha de **prosa** — `true  # exemplo de heredoc: <<EOF` — e
descarta todo o resto do arquivo. Se esse mesmo parser decide **quem entra na população** que o gate
mede, o arquivo some da varredura e a regressão que ele deveria pegar passa com `exit 0`.

Proteger a linha **inteiramente** comentada (`^\s*#`) não cobre isso: o `#` do comentário inline vem
*depois* de código, e o teste de comentário roda antes do teste de heredoc — a linha é classificada
como código e a busca pelo delimitador acha o `<<` que está dentro do comentário.

## Por que é caro descobrir isso amanhã

O gate continua **verde**. Não há mensagem, não há contagem óbvia fora do lugar — a linha de nota diz
`38 → 37 invocam python3` e ninguém olha um número que cai de um. As três guardas de vacuidade do
gate **não pegam**: a de glob só mede o `glob`, a de população só dispara com a população
**totalmente** vazia, e a de auto-aplicação só assevera o próprio gate. O gatilho é acidental: a
árvore já tinha cinco comentários com `<<` (`check-gates-falsify.sh:227,246,5600`,
`check-doctor-parity.sh:86`, `trackfw-git-branch-guard.sh:96-97`), inertes **só** por serem de linha
inteira. Uma reflow de parágrafo os arma.

Medido (cópias em sandbox, `scripts/` inteiro + os 3 geradores):

| árvore | mutação | população | veredito |
|---|---|---|---|
| baseline | nenhuma | 38 invocadores, 37 checados | `OK`, exit 0 |
| composta | comentário inline com `<<` **+** `export PYTHONIOENCODING=utf-8` removido de `check-tty-detection.sh` | 37, 36 | **`OK`, exit 0** ❌ fail-open |
| controle | só a remoção do `export` | 38, 37 | `FAIL`, exit 1 ✅ |

## A regra que fica

**População e declaração são predicados diferentes, e a exclusão de heredoc só serve a um deles.**

- **População** ("este arquivo está no escopo do que eu meço?"): predicado *loose* — exclui só a
  linha inteiramente comentada, **sem** estado de heredoc. Errar aqui é *fail-open*.
- **Declaração** ("esta linha tem efeito no ambiente?"): predicado *strict* — exclui comentário
  **e** corpo de heredoc, porque menção morta não exporta nada. Errar aqui é *fail-closed*.

A assertiva "o arquivo da allowlist ainda invoca `python3`?" é pergunta de **população**, não de
declaração — sob o strict, um comentário inline naquele arquivo dispararia `ALLOWLIST SEM OBJETO`
sem motivo.

No dia da correção as duas populações eram idênticas (**strict 38 = loose 38, delta vazio nas duas
direções**), então a separação é drop-in e não muda contagem nenhuma.

## Dois residuais declarados — os dois FECHADOS, e nenhum deles se conserta revertendo a separação

1. **Do lado da declaração o mecanismo sobrevive.** Um comentário inline com `<<` colocado *acima* do
   `export` esconde a declaração e o gate reprova com `NAO declara` um arquivo que declara uma linha
   abaixo. Medido: `rc=1`. É ruidoso e enganoso, **nunca permissivo**. O remédio para quem topar com
   isso é **mover o `<<` para um comentário de linha inteira**, não reverter a separação de
   predicados nem afrouxar a exclusão de heredoc. Fechar isso "direito" exigiria decidir se um `#` é
   comentário de verdade (`echo "a # b" <<EOF` não é) — e essa heurística falha na direção
   **aberta**, que é justamente a que não se pode aceitar aqui.
2. **`first_py3` passou a vir das linhas loose.** Uma menção morta a `python3` dentro de corpo de
   heredoc, *acima* do `export`, entraria na população e ainda dispararia a checagem de ordem
   (`declaração depois da primeira invocação`). Não acontece hoje — os `export` ficam logo após o
   `set -euo pipefail` —, e a falha seria fechada.

## Efeito colateral na guarda de auto-aplicação, para quem for testá-la

Sob o predicado loose, o corpo do heredoc Python do próprio gate passa a contar como código — e ele
menciona `python3` em ~14 mensagens e comentários. A guarda de auto-inclusão continua correta, mas
**não é mais falsificável só escondendo a linha de invocação**: para tirar o gate da própria
população é preciso remover a palavra do arquivo inteiro (na sandbox: trocar tudo por um token
neutro e remontar `PY3_RE`/`ATTENTION_CALL_RE`/`PREFIX_RE` por concatenação de literais). Com isso as
três guardas foram re-falsificadas por execução no ML-2B. A propriedade que importa — remover o
`export` **deste** gate faz ele se nomear na lista de infratores — continua valendo e foi medida.

## Ver também

- `vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md` — mesma família: a "metade
  positiva" (contagem que não exclui comentário) e a "metade negativa" (regex literal evadível).
- `vault/notes/handler-de-erro-em-pythonioencoding-reintroduz-byte-invalido-e-os-3-serve-divergem-2026-09-02.md`
  — o outro fail-open do mesmo gate, corrigido no mesmo ML-2B.
