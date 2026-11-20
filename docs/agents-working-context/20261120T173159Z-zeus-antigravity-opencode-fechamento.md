# Zeus — REQ-2026-11-17: Antigravity, Gemini CLI legado e OpenCode; todos os ACs (PR #692)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** feat/guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli
- **PR:** #692 (draft), CI 24/24 verde em 82947751

## O que mudou neste ciclo
- **Antigravity (Waves 0–4):**
  - Guard de credencial e de git-branch no projeto (`.agents/hooks.json`) e no global (`~/.gemini/config/hooks.json`, `USERPROFILE` no Windows).
  - `agy` medido no Mac e no Windows. App 2.29.0 e IDE medidos no Mac. A IDE ignora o hook de projeto, e a regra warning-only `antigravity_ide_global_guard` cobre isso (ML-4E).
- **Gemini CLI (M12, contrato §14):** o 0.53.0 com login Google individual é recusado (`IneligibleTierError`, rc=55); o contra-braço sem credencial dá erro de auth (rc=41). O caminho por chave de API não foi medido, porque o KG não tem chave e não quis custo.
- **OpenCode (Wave 5, issue #691; ADR-2026-11-19 D1–D8):**
  - Plugin `trackfw-guard.js` gerado pelo Go para projeto e global; `validate`/`doctor` auditam; `update harness` reporta `missing`.
  - Três rodadas de red team. O D8 encerra a corrida de truques de caminho: a detecção é o controle principal.
  - ML-5H, prova real com o plugin do produto: 7/7 no Mac e no Windows (contrato OpenCode §8.1).
- **ML-FB2:** 3 testes da Wave 4 reprovavam no `windows-full-suites` porque isolavam só `HOME`. A correção alinha `USERPROFILE` (só testes).

## Pendente
- O KG decide quando tirar o PR #692 de draft e quando mergear.
- Pós-merge: fechar a REQ, mover o roadmap para `done/` e fechar o #691 com palavra-chave em inglês.

## Lições
- O `windows-full-suites` reprovou desde o 1º run do PR, e só olhei no ML-FB. Com PR draft aberto para CI, ler o resultado a cada push.
- Teste que isola home precisa isolar `HOME` e `USERPROFILE` juntos (mesma família do ML-FA2 da REQ-2026-11-15).
