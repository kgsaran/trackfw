# Cenário de falsificação por-vírgula-fica-vácuo contra reversão TOTAL do parser, só pega reversão parcial

> Data: 2026-08-02 | Autor: Ártemis (com correção do advisor antes de fechar) | Domínio: gates / testes

## Sintoma que quase passou despercebido

Ao escrever o Cenário 35 (`scripts/check-gates-falsify.sh`, lista YAML inline com vírgula
dentro de aspas — caso 8 do contrato), a primeira versão configurava `agents: ["ka, tsu",
"obi"]` com **exatamente** esses dois diretórios no disco (`docs/roadmaps/ka, tsu/` e
`docs/roadmaps/obi/`). O braço de detecção corrompia só o ramo de detecção de aspas em
`splitTopLevelCommas` (`case r == '"' ...` → `case false:`), provando a regressão pontual do
caso 8. Passou nos três CLIs.

## Causa — o conjunto configurado coincidia com o conjunto em disco

Se alguém revertesse o suporte a lista inline **por inteiro** (não só o scanner de aspas —
apagasse `isInlineList`/`parseInlineList`), `agents: [...]` cairia no modo bloco, não
encontraria `- item` nas linhas seguintes, e `cfg.Agents` ficaria vazio. Com `agents` vazio,
`resolveStateDirs` (internal/validator/validator.go) cai no **fallback**: varre
`docs/roadmaps/*` e usa os nomes de diretório como agentes. Como a fixture só tinha `ka, tsu`
e `obi` no disco — o MESMO conjunto do `agents:` configurado — o fallback reproduzia
**byte-a-byte** a mesma saída do parser correto. O braço de detecção do caso 8 continuava
válido (ele corrompia um ponto diferente), mas o cenário como um todo não protegia contra essa
classe maior de regressão (reversão total do ML-1A), e teria morrido no setup de
`corrupt_literal` com "expected exactly 1 occurrence... got 0" assim que alguém removesse as
funções inteiras — o mesmo sintoma do Cenário 28 (ver
`cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`), só que por reversão
completa em vez de refactor.

## Como foi pego

Não por execução — o `assert_fails_with`/comparação byte-a-byte passava normalmente contra a
corrupção pontual planejada. Foi pego em revisão (advisor), ao traçar manualmente "o que
acontece se alguém reverter tudo, não só o pedaço que estou corrompendo" — e confirmado
empiricamente revertendo `bc00010` inteiro num binário Go isolado contra a fixture original: a
saída ficava idêntica ao pinado (`git apply -R` do commit inteiro, rebuild, roda contra a
fixture — zero divergência).

## Como corrigir — mesmo padrão do Cenário 34, generalizado

Adicionar ao disco um diretório de agente **fora** da lista configurada, com roadmap em wip/
próprio (`zeta`, seguindo o precedente de `zeus` no Cenário 34). Com `agents:` corretamente
parseado (não vazio), `resolveStateDirs` itera só os agentes configurados — `zeta` nunca entra
na conta, `S35_EXPECTED` não muda. Se o parser cair no fallback (por qualquer motivo — reversão
pontual OU total), `zeta` reaparece na saída, e a comparação byte-a-byte reprova.

**Regra generalizável**: em qualquer cenário de falsificação sobre um mecanismo com fallback
(`agents:` vazio → varre disco; e por extensão, qualquer "se X não está configurado, infere de
Y"), a fixture PRECISA ter algo em `Y` que não está em `X`, mesmo quando o objetivo declarado do
cenário é uma corrupção pontual e não o fallback em si. Sem isso, a fixture "coincide" com o
resultado do fallback e o cenário fica cego para toda a classe de regressão "componente inteiro
removido/desabilitado", não só para o bug pontual que motivou o cenário.

**Como verificar antes de fechar um cenário**: perguntar "se eu revertesse o commit INTEIRO que
introduziu esta feature (não só o trecho que estou corrompendo), a saída pinada ainda bateria?"
Se a resposta for sim, a fixture está vácua contra a classe mais ampla de regressão — mesmo que
o braço de detecção pontual planejado continue funcionando.

Relacionado: `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`.
