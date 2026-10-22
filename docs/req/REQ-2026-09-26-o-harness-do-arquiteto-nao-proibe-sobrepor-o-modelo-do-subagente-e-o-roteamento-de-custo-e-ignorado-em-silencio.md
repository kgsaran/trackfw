---
status: Open
date: 2026-09-26
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md"
---

# REQ: o harness do arquiteto nao proibe sobrepor o modelo do subagente e o roteamento de custo e ignorado em silencio

> Date: 2026-09-26 | Status: Open
| Linear Issue: 
| Jira Issue: 

Origem: **erro do próprio arquiteto**, medido em 2026-09-26 e apontado pelo usuário.
`author: trackfw_architect`

## Motivation

O `trackfw` gera o arquivo de cada agente com um `model:` no frontmatter, e a escolha é do projeto —
**é roteamento de custo**:

```
apolo-tf · hades-tf · ares-tf · demais especialistas   model: claude-sonnet-4-6
zeus-tf (arquiteto)                                     model: claude-opus-5-5
```

🔴 **O parâmetro `model` do Agent tool tem precedência sobre esse frontmatter.** Quando o arquiteto o
passa, a configuração do projeto é **silenciosamente ignorada** — não há erro, não há aviso, e o custo
só aparece na fatura.

### O que aconteceu, medido

Em **2026-09-26**, numa única sessão, o arquiteto passou `model: opus` em **doze despachos**,
sobrescrevendo `claude-sonnet-4-6` em todos. Ninguém percebeu até o usuário perguntar.

E a racionalização que o arquiteto deu quando questionado — *"refutação degrada em modelo menor"* —
🔴 **nunca foi medida, e é infalsificável do jeito que foi usada**: ele só observou o braço em que já
havia trocado o modelo. As dezessete refutações que os executores produziram naquela campanha teriam
acontecido em Sonnet, e a hipótese nunca foi testada.

### A causa está no harness, não na pessoa

`internal/integrations/assets/agents/architect.md:41` tem o **Dispatch contract**, que diz:

> *"Every dispatch to a specialist MUST pass the Agent tool's `subagent_type` parameter explicitly…
> The correct `subagent_type` value is the `name:` from the frontmatter of that role's installed agent
> file."*

⚠️ **Ele manda ler o arquivo do agente — e nomeia só o `name:`.** O `model:` está na **linha ao lado**
e não é mencionado em lugar nenhum. O arquiteto leu o que o contrato mandou ler, e o contrato tinha
régua estreita demais.

🔴 **É a mesma classe de defeito que este produto persegue:** um contrato que descreve metade da
verificação, e o leitor cumprindo a letra sem alcançar a razão.

## Acceptance Criteria

- [ ] **O Dispatch contract proíbe explicitamente passar `model`**, com a razão escrita (roteamento de
      custo) — não como convenção, como regra
- [ ] O contrato manda ler **`name:` e `model:`** do arquivo do agente, e diz que o segundo é **do
      projeto**, não do arquiteto
- [ ] 🔴 **Falsificação nas duas direções:** um `architect.md` gerado **sem** a proibição reprova; o
      gerado **com** ela passa. Sem o braço negativo, o gate afirma o que não verifica
- [ ] ⚠️ **Enumerar os outros sítios da mesma classe antes de fechar** (Regra Dura): o Dispatch
      contract manda ler o arquivo do agente e nomeia **só** um campo — há **outro** contrato neste
      produto que manda ler um artefato e nomeia um subconjunto dos campos que importam? A resposta
      fica escrita **mesmo que seja "nenhum"**
- [ ] O golden `internal/integrations/testdata/architect.subagent.golden.md` acompanha, e a mudança é
      **visível no diff** — golden que muda sem ninguém ver é a próxima claim falsa
- [ ] `make quality` e **CI** verdes

### Ampliação de 2026-10-22: escape TOML do destino Codex (decisão do KG)

O red team do ML-2B achou que o destino Codex (`custom-agent-toml`, `internal/integrations/render.go:217-224`)
serializa `name`, `description`, `model` e `developer_instructions` com `strconv.Quote`, que é um escape de Go, não de TOML.
A causa é outra, mas o KG decidiu em 2026-10-22: *"o Escape TOML entra nesta PR"*.

Medido em 2026-10-22, com `strconv.Quote` e depois `tomllib` no Python 3.15:
- `\a` (0x07) e `\v` (0x0B) dão TOML inválido (*Unescaped '\'*);
- para outros bytes de controle (0x01, 0x7F) e para UTF-8 inválido (0xFF), o `strconv.Quote` emite `\xHH`, que só existe no TOML 1.1;
- no TOML 1.0, que a CLI de destino pode estar usando, o arquivo inteiro deixa de carregar.
O `description` recebe o `DisplayName` da identidade configurada pelo usuário (`render.go:210`).

- [ ] **AC7** — Os quatro campos do TOML do Codex são serializados como *basic string* do TOML 1.0: só `\b \t \n \f \r \" \\` e `\uXXXX` para os demais caracteres de controle. Há falsificação nas duas direções: a saída para entradas com 0x00–0x1F e 0x7F é TOML 1.0 válido e decodifica de volta à entrada; UTF-8 inválido é recusado com erro, sem arquivo escrito (decisão do arquiteto em 2026-10-22, alinhada a `render.go:665`); e a versão antiga (`strconv.Quote`) reprova no mesmo teste.
- [ ] **AC8** — Mesma família, achada na Wave 0 da ampliação: o escape YAML não deixa sair byte de controle cru; `identity.Load` valida o slug e recusa DEL e C1; as referências de terceiro não corrompem o TOML nem o JSON do destino.

## Negative scope — o que esta REQ NÃO faz

- **Não** muda o `model:` de nenhum agente. A escolha de qual modelo cada papel usa é do projeto e
  está fora daqui — esta REQ trata de **quem pode sobrepô-la**, que é ninguém.
- **Não** introduz mecanismo técnico de bloqueio no Agent tool — o harness do trackfw gera
  **instrução**, não intercepta chamada. Se um bloqueio duro for desejável, é outra decisão, com outro
  custo.
- **Não** reescreve o Dispatch contract além do necessário. Ele já funciona para o `subagent_type`; o
  defeito é uma omissão, não o desenho.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md`
