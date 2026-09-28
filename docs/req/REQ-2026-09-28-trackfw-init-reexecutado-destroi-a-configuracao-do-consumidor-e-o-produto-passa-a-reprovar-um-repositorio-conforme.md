---
status: Open
date: 2026-09-28
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-09-28-trackfw-init-reexecutado-preserva-a-configuracao-autorada-pelo-consumidor.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md"
---

# REQ: `trackfw init` reexecutado destrói a configuração do consumidor, e o produto passa a reprovar um repositório conforme

> Date: 2026-09-28 | Status: Open
| Linear Issue:
| Jira Issue:

Origem: **#445**, reportado por consumidor externo com reprodução, e **ocorrência real dentro deste
repositório** em 2026-09-27, com dano medido.

## Motivation

`trackfw init` reexecutado num projeto já onboardado sobrescreve `trackfw.yaml` com o template
fixo. `writeTrackfwConfig` (`internal/generators/scaffold.go`) faz `os.WriteFile` — **o valor
anterior não é lido em momento nenhum**: não há merge, não há preservação, não há aviso.

### 🔴 O dano principal não é a perda do arquivo

É o produto **passar a reportar um estado de governança que não é o do projeto**. Medido neste
repositório em 2026-09-27, quando um agente rodou `init` num trabalho não relacionado:

```diff
-governance_mode: lenient
-lenient_until: "2027-12-31"
-ci: github-actions
-forge: github
-# bloco agent_models inteiro (ADR-2026-08-21), com a justificativa de cota
+ci: none
+wip_limit: 1
+rules:
+  branch_has_wip_roadmap: error
```

| | antes | depois do `init` |
|---|---|---|
| `trackfw validate` | **170 warnings** | **156 violations** |
| exit code | `0` | `1` |

**Um repositório conforme passou a ser reprovado.**

### O efeito de segunda ordem, que é o que torna isto grave

O agente que causou o dano mediu o `validate` **depois** e reportou as 156 violações como *"todas
pré-existentes"*. Não eram — eram consequência da própria ação, três comandos antes. O `make quality`
que ele rodou em seguida também rodou sobre a árvore corrompida.

🔴 **Um comando que reescreve config em silêncio não corrompe só o arquivo: corrompe as medições que
vierem depois dele** — inclusive as de quem age de boa-fé e não sabe que o chão mudou. É por isso que
este defeito não é "config sobrescrita": é **perda de confiabilidade de toda medição subsequente**.

### E o sintoma não aponta para a causa

O consumidor vê "0 roadmaps" ou violações novas. Não vê "sua config foi sobrescrita". O tempo de
diagnóstico é gasto no lugar errado.

### População — medida em 2026-09-28, com a régua certa

`internal/generators/scaffold.go` tem **22** chamadas reais de `os.WriteFile`/`os.OpenFile`.

⚠️ O `grep` ingênuo por substring conta **25** — **3 são comentários**. A contagem correta usa
âncora de início de linha. A enumeração e classificação real é entregável da Wave 0; este número é
o teto da população a triar, não a população de defeitos.

A classificação é por **natureza do conteúdo destruído**, não por forma da chamada (ADR):
**(a)** config autorada pelo consumidor → defeito · **(b)** artefato gerado pelo produto → correto ·
**(c)** híbrido com bloco gerado → verificar se o ramo de merge está completo.

## Acceptance Criteria

- [ ] **Enumeração real** dos 22 sítios, classificada em **(a)** / **(b)** / **(c)**, com a razão
      escrita por sítio — entregável da Wave 0
- [ ] 🔴 **O AC que mede o efeito, não o token:** reexecutar `init` sobre um `trackfw.yaml` com
      `governance_mode: lenient` + `lenient_until` + bloco `agent_models` **preserva os três**, e a
      saída de `trackfw validate` é **byte-idêntica** antes e depois
- [ ] 🔴 **Comentários preservados** — o bloco `agent_models` mantém a justificativa de cota escrita
      em comentário. Preservar chaves e descartar a razão delas é preservação aparente
- [ ] **Zero diff nas linhas que já existiam** (`git diff trackfw.yaml` vazio quando nenhuma chave
      nova precisa ser acrescentada) — é a propriedade que torna a correção auditável
- [ ] **Chave nova de versão nova É acrescentada** — contra-braço: sem ele, "preservar" degenera em
      "não escrever nada" e o `init` pós-upgrade deixa de servir
- [ ] Todo sítio **(a)** corrigido por **ponto único**; todo sítio **(b)** **declarado** como
      intencional, com a razão — não corrigido
- [ ] 🔴 **Gate que impede a reintrodução**, falsificável nas duas direções: reprova quando um sítio
      (a) novo nasce truncando, **e** não reprova um sítio (b) legítimo
- [ ] `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- 🔴 **Não** trata o **#451** (dois workflows de governança coexistindo, 3× `validate` por push).
  **Mesma família, causa distinta, e a medição está escrita:** corrigir `writeTrackfwConfig` para
  mesclar **não fecha o #451** — o `trackfw-gate.yml` não sobrescreve nada, ele **cria um arquivo
  novo com outro nome**. Nenhuma preservação de valor existente o impede. O que o #451 pede é uma
  decisão de *qual artefato instalar*, governada pela **ADR-2026-08-28** (dois métodos de
  instalação). Aplicando o teste da Regra Dura — *"se eu corrigir esta causa, exatamente estas
  falhas fecham"* — o #451 não fecha. REQ própria.
- **Não** muda o comportamento dos sítios **(b)**. Sobrescrever script gerado é **como a correção
  chega** ao consumidor — foi assim que o fix de CRLF do #353 saiu daqui. Confundir (a) com (b)
  produziria o defeito oposto: congelar scripts defeituosos na máquina de quem instalou.
- **Não** trata o **#450** (`context` reporta `ADRs (0)` onde `status` reporta 145). Medição escrita:
  aquilo é **leitura incompleta** (não desce em subpastas de estado); isto é **escrita destrutiva**.
  Sintomas e causas distintos.
- **Não** introduz round-trip estrutural de YAML (`yaml.Node`/`map[string]any`) — rejeitado na ADR
  por medição.
- **Não** migra nem normaliza `trackfw.yaml` de ninguém.

## Linked ADR
ADR: docs/adr/ADR-2026-09-28-trackfw-init-reexecutado-preserva-a-configuracao-autorada-pelo-consumidor.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-28-trackfw-init-reexecutado-destroi-a-configuracao-do-consumidor-e-o-produto-passa-a-reprovar-um-repositorio-conforme.md
