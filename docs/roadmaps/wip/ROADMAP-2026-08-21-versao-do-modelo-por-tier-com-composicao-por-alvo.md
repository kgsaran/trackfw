---
status: wip
date: 2026-08-21
req: "docs/req/REQ-2026-08-21-versao-do-modelo-dos-agentes-configuravel-por-tier-no-trackfw-yaml.md"
adr: "docs/adr/ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: versão do modelo por tier, com composição por alvo

> Created: 2026-08-21 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-21-versao-do-modelo-dos-agentes-configuravel-por-tier-no-trackfw-yaml.md`

O usuário não pode escolher a versão do modelo dos agentes. Pinar exige editar arquivo **gerado**, e
o próximo `agents update` reverte sem aviso. O conflito já é concreto: a regra de verbosidade do
#198 só chega ao arquivo local via update, e o update desfaz o pin.

## 🔴 Riscos que valem para todos os MLs

1. **Vazamento de namespace é o risco dominante.** `claude-sonnet-4-6` chegando ao mapeamento do
   Codex, Cursor ou Antigravity quebra os três — e quebra no **artefato gerado**, não no `trackfw`,
   então o usuário só descobre quando o agente não sobe. Precisa ser **gate**, não cuidado.
2. **Config ausente não pode mudar nada.** Sem `agent_models`, comportamento idêntico ao de hoje.
   Regressão aqui atinge todo usuário do trackfw.
3. **O motivo é cota, não custo.** Sonnet 4.6 consome ~30% menos tokens (tokenizador pré-4.7) e custa
   **mais** por token. Sem isso escrito, um leitor futuro "corrige" a escolha para o lado errado.
4. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.

---

## Wave 1 — Decisão e composição

### ML-1A — ADR do formato e da composição
**Status:** ✅ Concluído · **Agente:** `zeus-tf` (arquiteto — **não delegar**)
`ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md`, com o formato, as três regras
de composição, o escape hatch, a fronteira de namespace, e o motivo (**cota, não custo**) registrado
com a medição da doc oficial.
Decisão material: formato de `agent_models`, as três regras de composição, o escape hatch, e a
fronteira de namespace. Decisão de formato é do arquiteto; o roadmap anterior atribuiu isso a
executor por engano e foi corrigido.

### ML-1B — Resolução e composição por alvo
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A
**Arquivos (3 stacks):** leitura de config, `internal/integrations/render.go` + espelhos, testes.

- `agent_models` por tier, guardando **versão**.
- Composição: ponto→traço; versão maior **omite o minor**; cada alvo compõe a **própria** forma.
- Escape hatch: valor que não parece versão é usado **literalmente**.

**Critérios de aceite:**
- [ ] As três regras de composição corretas, provadas por caso
- [ ] **Sem vazamento**: Codex, Cursor e Antigravity seguem com os próprios valores mesmo com
      `agent_models` configurado — é o AC que mais importa
- [ ] Config ausente → comportamento idêntico ao de hoje
- [ ] `make quality` verde

---

---

### Auditoria do ML-1B — aprovada, com uma lacuna pequena para o ML-2A

Verifiquei **end-to-end**, gerando artefatos reais com os 3 alvos, não pelos testes dele:

```
                        sem agent_models        com agent_models
Claude   model:         sonnet              ->  claude-sonnet-4-6     COMPOE
Codex    model =        "gpt-5.4-mini"      ->  "gpt-5.4-mini"        INTOCADO
Cursor   model:         composer-2.5[...]   ->  composer-2.5[...]     INTOCADO
```

**Zero vazamento de namespace** — o AC que mais importava do lote, provado no artefato gerado e não
só em teste unitário.

As três regras de composição, cada uma com caso próprio:

```
"5"                            -> claude-opus-5 / claude-sonnet-5      (maior sem minor)
"4.6"                          -> claude-opus-4-6 / claude-sonnet-4-6  (ponto vira traco)
"claude-sonnet-4-5-20250929"   -> literal                              (escape hatch)
"4.6-beta"                     -> literal                              (escape hatch)
```

**Escape hatch com critério explícito:** `^[0-9]+(\.[0-9]+)*$`. Trade-off documentado e coerente
com o que pedi — prefere falso-negativo (tratar como literal) a falso-positivo (compor errado a
partir de algo que não era versão).

#### 🔴 Lacuna que encontrei, e vai para o ML-2A

`"4.6-beta"` vira **literalmente** `model: 4.6-beta` no frontmatter. É o escape hatch funcionando
como especificado — mas o resultado é um valor de modelo **inválido**, escrito em silêncio. O agente
falha ao subir, e a causa fica a duas camadas de distância.

É a mesma classe do `reson=` que a REQ do contrato pinado tratou: entrada de forma desconhecida
aceita sem sinal. O AC da REQ pede que a resolução *"não falhe de forma obscura"*, e este caminho
falha.

**Não bloqueia**, e o remédio já tem lugar natural: o **ML-2A** entrega o comando de resolução
efetiva, que é exatamente onde isso deve aparecer. Acrescentado como critério lá — avisar quando o
valor não é versão **nem** parece ID de modelo (não começa com `claude-`).


## Wave 2 — Visibilidade e catálogo

### ML-2A — Comando de resolução efetiva
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-1B
Lista, por agente e por alvo, o modelo **efetivamente resolvido**. Sem isso o usuário configura e não
confirma — foi exatamente a situação em que ninguém sabia dizer qual modelo os agentes usavam.

**Critérios de aceite:**
- [ ] Saída mostra agente · tier · alvo · valor resolvido
- [ ] **Avisa** quando o valor configurado não é versão **nem** parece ID de modelo (não começa com
      `claude-`) — hoje `"4.6-beta"` vira `model: 4.6-beta` em silêncio, e o agente falha ao subir
      com a causa a duas camadas de distância (lacuna medida na auditoria do ML-1B)
- [ ] Byte-idêntica nos 3 CLIs
- [ ] `make quality` verde

### ML-2B — Catálogo pina as versões
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-2A
`agents update` passa a **reforçar** o pin em vez de desfazê-lo.

**Critérios de aceite:**
- [ ] Após `agents update`, os arquivos gerados trazem as versões pinadas
- [ ] Provado end-to-end com os 3 binários, em fixture com `HOME` redirecionado
- [ ] `make quality` verde

---

## Wave 3 — Gate

### ML-3A — Gate de paridade + P4
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **Dep.:** ML-2B
**Antes de criar gate novo, verificar se algum existente cobre** — nesta série um comparador paralelo
quase foi criado sem necessidade.

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais**, incluindo o caso de não-vazamento
- [ ] Cenário P4 com baseline e detecção
- [ ] Anotação `trackfw-contract` atualizada; checker de cobertura exit 0
- [ ] `make quality` verde · **CI verde**

---

## Wave 4 — Barreira

### ML-4A — `hades-tf`
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md`
Config do usuário passa a influenciar o que é escrito em arquivo de agente. Avaliar injeção via valor
de versão, e se o escape hatch permite escrever algo perigoso no frontmatter. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** trocar o tier de um agente; mudar mapeamento de Codex/Cursor/Antigravity;
  modelo por agente individual.
- Commits e branch são exclusivos do `trackfw_architect`.
