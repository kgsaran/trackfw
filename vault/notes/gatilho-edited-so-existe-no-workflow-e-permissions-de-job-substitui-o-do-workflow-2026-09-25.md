# O gatilho `edited` só existe no *workflow*, e `permissions:` de job **substitui** o do workflow

> Data: 2026-09-25 · Autor: `ares-tf` · ML-N3 da
> `ROADMAP-2026-09-10-gate-de-palavra-chave-de-fechamento-nao-reavalia-em-edited-e-le-exemplo-citado-como-diretiva.md`
> Sítios: `.github/workflows/quality.yml`, `.github/workflows/pr-closing-keyword.yml`,
> `scripts/check-pr-closing-keyword.sh`

## Os três fatos que custam tempo amanhã

### 1. `types:` é do **workflow**, não do job — e o `if:` não contém o custo

`on.pull_request.types` decide se o **workflow inteiro** roda. Não existe "tipo de evento por job".
Logo, acrescentar `edited` ao `quality.yml` para atender **um** job dispara **todos** os outros.

Medido em 2026-09-25 no `quality.yml` (já sem o job do gate):

```
python3 -c "import yaml; d=yaml.safe_load(open('.github/workflows/quality.yml'));
            j=d['jobs']; print(len(j), len([k for k,v in j.items() if 'if' not in v]),
            {k:v['if'] for k,v in j.items() if 'if' in v})"
→ 13 jobs · 11 sem `if:` · {'shim-byte-identity-gate': 'always()', 'parity': 'always()'}
```

**Nenhum dos 13 tem `if:` que exclua `edited`** — inclusive as três suítes de `windows-latest`.
Escrever esse `if:` job a job "resolveria", e deixaria o **próximo job novo** sem ele: contrato que
se quebra por omissão. 🔴 **A fronteira correta é o arquivo**: um workflow próprio, com o gatilho que
aquele gate precisa. Preso por `scripts/check-workflow-yaml.py` (6 mutações falsificadas).

### 2. `permissions:` no job **SUBSTITUI** o do workflow — não se soma

Escrever só `permissions: {pull-requests: read}` no job faz `contents` virar `none`, e
`actions/checkout` falha. O bloco tem de declarar **os dois escopos**:

```yaml
permissions:
  contents: read        # actions/checkout
  pull-requests: read   # gh pr view --json body — nada além
```

### 3. O payload de um evento `edited` carrega o corpo **já editado** — e é isso que torna a mensagem antiga uma mentira

O gate dizia, no caminho de degradação, *"payload do evento (corpo de **ABERTURA** do PR #N)"*. Isso
era verdade enquanto os únicos tipos eram `opened · synchronize · reopened`. No instante em que
`edited` entra no gatilho, o payload passa a poder conter o corpo **pós-edição** — e a frase vira
falsa **no mesmo commit que corrige outra frase falsa** (o comentário de `quality.yml:46-49`).

🔴 **A afirmação segura é a que não diz qual corpo é:** *"o payload é imutável e reflete o corpo no
instante daquele evento (ação: X)"*. A ação sai do mesmo `json.load` que já extrai o número —
`GITHUB_EVENT_ACTION` **não existe** como variável de ambiente do GitHub Actions; o valor vive em
`.action` no payload (ou em `${{ github.event.action }}`).

## E o que estava desligado: `GH_TOKEN`

O caminho "corpo vivo pela API" foi entregue no PR #416 e ficou **inerte em CI**:
`grep -c 'GH_TOKEN' .github/workflows/quality.yml` → **0** (2026-09-25). Consequência medida: o PR
**#293** mergeou **verde** com `**Não fecha #290**` e `**Não fecha #275**` no corpo vivo — a negação
entrou por **edição posterior** ao último evento com payload, e o gate mediu o texto antigo.

⚠️ **Ordem travada, e é o ponto não óbvio:** ligar o `GH_TOKEN` **antes** do fix da forma 4 (negação
lida como afirmação) red-linaria todo PR que declara escopo negativo. O defeito do payload estava
**mascarando** a forma 4. Por isso este ML veio por último.

## A degradação silenciosa que podia desligar tudo de novo

`gh pr view ... 2>/dev/null` tornava indistinguíveis *"sem token"*, *"sem `pull-requests: read`"*,
*"rate limit"* e *"rede caiu"*. Se alguém removesse a permissão do workflow, o gate voltaria a ler o
payload velho **sem sinal nenhum** — a mesma classe de
`guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24.md`.

Agora a queda imprime **a ação do evento, a causa, o rc e o stderr do `gh`**, e emite `::warning::`
sob `GITHUB_ACTIONS`.

🔴 **Degradação não é vacuidade, e a distinção é deliberada:** o payload é um corpo **real** — o gate
mede e dá veredito (0 ou 1). O que se perde é a *precisão da fonte*. Vacuidade é **não ter corpo**, e
continua sendo `exit 2`. Confundir as duas transformaria "fonte menos precisa" em "reprovou", e um
gate que reprova por não ter token é um gate que alguém desliga.
