# Zeus — REQ-2026-11-22: onboarding verificável; todos os ACs (PR #700)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/onboarding-verificavel
- **PR:** #700 (draft), CI 24/24 verde em 9f63da34; `make quality` MQ_RC=0

## O que mudou neste ciclo
- **`init`:**
  - honra `--brownfield` e `--ai-tools` com e sem TTY (D1);
  - `--brownfield` só grava governança sem `trackfw.yaml` (D2);
  - `guard` lista os três subcomandos.
- **Leitor e escritor do `trackfw.yaml` (D2a):**
  - chaves com aspas, espaço ou BOM reconhecidas pelo parse YAML; `pre-commit` do lefthook em qualquer grafia;
  - merge key expandida nos dois lados (antes, o leitor ignorava âncoras);
  - guarda de ciclo e orçamento de 10 000 visitas (âncora cíclica e bomba de alias derrubavam ou travavam o 11.8.0).
- **Site e README:**
  - site com inglês na raiz e pt-BR em `/pt/`, conteúdo corrigido nos dois idiomas;
  - README com Quick start contíguo e Windows depois de *Installation*;
  - teste que pina README e guia do site pelos comandos executados (D4).
- **REQ-2026-11-23 no backlog:** write-scope re-enraizado por `trackfw.yaml` aninhado (F-5 do red team).

## Pendente
- O KG decide quando tirar o #700 de draft e quando mergear.
- Pós-merge: fechar a REQ-2026-11-22 e mover o roadmap para `done/`.
- Changelog *Changed*: URL do site, brownfield em projeto existente, âncoras YAML passam a valer.
- Depois: REQ do GoatCounter, depois o site novo (ordem combinada com o KG).

## Lições
- O red team mediu um "rebaixamento" que não existia: o leitor já ignorava âncoras. Comparar sempre com o binário anterior, e não só com o código.
- Proteção por texto de mensagem de erro de biblioteca é contornável (ML-4G). O controle tem de estar no caminhador (ML-4H).
- Dois agentes rodaram o binário com o HOME real ou deixaram processo órfão (Chromium, falsify). Exigir HOME isolado na mesma linha de cada comando e conferir processos ao fim de cada ML.
