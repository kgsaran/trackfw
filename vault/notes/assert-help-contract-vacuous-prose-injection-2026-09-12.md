# assert_help_contract: asserção vacuosa por injeção de prosa no help — 2026-09-12

## Contexto

`scripts/check-integration-cli-parity.sh` tem uma função `assert_help_contract` que
verifica se `agents` e `skills` aparecem como subcomandos no help raiz de cada
runtime (Go/Node/Python), e se `list`/`install`/`uninstall`/`update` aparecem no
help de cada subcomando.

## Causa raiz

A asserção original usava `grep -Eq "(^|[[:space:]])${kind}([[:space:]]|$)"` sobre
o texto inteiro do help. O padrão `(^|[[:space:]])` (não ancorado ao início de
linha) casa qualquer ocorrência da palavra, inclusive no meio de uma frase.

ML-3A adicionou ao comando `commit` do Node (no campo `description` do Commander)
texto de ajuda com exemplos:
```
    trackfw req new --agent <agent> "title"  # by_agent project with 2+ agents
    trackfw roadmap new --agent <agent> "title"  # by_agent project with 2+ agents
```

A palavra `agents` aparece ao fim dessas linhas (separada por espaços). O grep
`[[:space:]]agents[[:space:]]|$` casava essas ocorrências e o gate passava
_mesmo após remover `require('./agents')` do registro de comandos do Node_.

## Por que não foi detectado imediatamente

O `falsify/integration-cli-parity/missing-agents` scenario ficou verde antes do
ML-3A porque o texto de prosa não existia. Após ML-3A, o cenário passou a exercer
o gate contra uma árvore onde `agents` continuava no registro — e passou. Só quando
a A/B com `grep -v "require('./agents')"` foi medida explicitamente (pelo arquiteto,
antes do handoff) a vacuidade ficou visível.

## Mecanismo da vacuidade

`(^|[[:space:]])palavra` não exige que `palavra` seja o primeiro token de uma linha.
Qualquer ocorrência precedida de espaço (e seguida de espaço ou fim de linha) é
suficiente. Uma descrição de comando que mencione a palavra como parte de prosa é
indistinguível, para esse padrão, de uma entrada de subcomando.

## Correção

Extrair a região de listagem de comandos por runtime antes do grep, e ancorar o
padrão em `^[[:space:]]+${kind}`:

| Runtime | Marcador de início | Marcador de fim |
|---------|-------------------|-----------------|
| Go (cobra) | `^Available Commands:` | primeira linha em branco |
| Node (commander) | `^Commands:` | awk coleta só linhas com `^  [^ ]` |
| Python (argparse) | `^positional arguments:` | `^options:` |

O padrão `^[[:space:]]+agents([[:space:]]|$)` ancora `agents` como PRIMEIRO token
da linha (após espaços iniciais), o que exclui qualquer ocorrência no meio de frase.

## Varredura realizada

```bash
grep -n "grep -E" scripts/check-integration-cli-parity.sh
grep -rn 'grep -E' scripts/check-*.sh | grep -v "check-integration-cli-parity"
```

Achados:
- `scripts/check-integration-cli-parity.sh:72` — corrigido neste ML (linha 72, root help, `agents`/`skills`)
- `scripts/check-integration-cli-parity.sh:78` — corrigido neste ML (linha 78, subcommand help, `list`/`install`/`uninstall`/`update`)
- `scripts/check-cli-parity.sh:86` — mesmo defeito, mesmo mecanismo; não corrigido aqui (decisão do arquiteto; sítio de mesma causa, reportado conforme Regra Dura de Causa Raiz)

## Lição geral

Uma asserção de presença de palavra-chave em texto longo é re-vacuável sempre que
o texto puder crescer por outra razão (help text, exemplo de uso, comentário inline).
O padrão seguro para verificar que um subcomando está _registrado_ é extrair a
seção de listagem de comandos do help e ancorar o grep ao início de linha.
