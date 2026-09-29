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

- [x] **Enumeração real** dos 22 sítios, classificada em **(a)** / **(b)** / **(c)**, com a razão
      escrita por sítio — entregável da Wave 0
      → **(a)=2 · (b)=14 · (c)=6**; os 6 sítios (c) têm ramo de merge **completo** e são o precedente
      correto. 🔴 **A Wave 0 refutou a premissa da REQ de que (a) seria só `trackfw.yaml`**
- [x] 🔴 **Segundo sítio (a) descoberto:** `generateLefthookHook` (`scaffold.go:2806`) sobrescreve
      `lefthook.yml` incondicionalmente, destruindo hooks do consumidor. **Alcançável só pelo wizard**
      (`init.go:226` oferece `lefthook`); o caminho não-interativo usa `Hooks: "none"` (`init.go:110`)
      e não o atinge — razão pela qual a primeira reprodução do arquiteto **não** o reproduziu
- [x] 🔴 **O AC que mede o efeito, não o token** — MEDIDO duas vezes de forma independente (arquiteto
      na auditoria do ML-1A, e Wave 2 reconstruindo o cenário do zero): `validate` **byte-idêntico**
      antes e depois, `governance_mode`/`lenient_until`/`ci`/`forge` preservados. Detalhe original: reexecutar `init` sobre um `trackfw.yaml` com
      `governance_mode: lenient` + `lenient_until` + bloco `agent_models` **preserva os três**, e a
      saída de `trackfw validate` é **byte-idêntica** antes e depois
- [x] 🔴 **Comentários preservados** — confirmado na Wave 2 com bloco `agent_models` comentado. — o bloco `agent_models` mantém a justificativa de cota escrita
      em comentário. Preservar chaves e descartar a razão delas é preservação aparente
- [x] **Zero diff nas linhas que já existiam** — medido: o diff pós-`init` mostra só chaves novas
      ao fim; segundo `init` produz diff vazio (idempotente). (`git diff trackfw.yaml` vazio quando nenhuma chave
      nova precisa ser acrescentada) — é a propriedade que torna a correção auditável
- [x] **Chave nova de versão nova É acrescentada** — verificado para **chave de nível 0**.
      🔴 **Sub-chave nova sob bloco de nível 0 já presente NÃO é entregue, por decisão** — ver
      **Emenda 1** da ADR. Entregar significaria injetar regra de severidade `error` num repositório
      conforme, que é o dano que esta REQ elimina. Medido na Wave 2: consumidor com
      `rules: {some_other_rule}` não recebe `branch_has_wip_roadmap`
- [x] Os **2** sítios (a) corrigidos (ML-1A e ML-1C); os 14 sítios (b) **declarados** intencionais no
      parecer da Wave 0, não corrigidos. Original:; todo sítio **(b)** **declarado** como
      intencional, com a razão — não corrigido
- [x] 🔴 **Gate que impede a reintrodução** — criado (ML-1B), e **dois falsos verdes seus foram
      achados e corrigidos** (ML-1D: guarda em comentário; e o espelho, write-site em comentário).
      Limite conhecido **declarado**, não escondido. Original:, falsificável nas duas direções: reprova quando um sítio
      (a) novo nasce truncando, **e** não reprova um sítio (b) legítimo
- [x] `make quality` **RC=0** — verificado pelo arquiteto, não aceito do relatório:
      1380 `OK`, 0 FAIL real (os 6 `FAIL` do log vêm de `/var/folders/.../arm1.go`, fixtures dos
      braços de self-test, e os 3 braços dão `PASS`). Os 4 sítios da árvore passam **via
      `os.ReadFile`**, nenhum por marcador. CI: ver PR

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
