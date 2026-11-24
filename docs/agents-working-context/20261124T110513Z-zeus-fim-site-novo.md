# Zeus — REQ-2026-11-23: site novo do trackfw; todos os ACs (PR #702)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** feat/site-do-trackfw-visao-do-produto
- **PR:** #702 (draft), CI 24/24 verde em 31fe4980; `make quality` MQ_RC=0

## O que mudou neste ciclo
- **Método protótipo primeiro:**
  - conceito da atena, aprovado pelo KG (hero com o pitch do README, cenário npm/pip como card, board no protótipo, demo em vídeo);
  - protótipo isolado, aprovado na rodada 1;
  - ADR-2026-11-23 (site), D1–D8.
- **Home nova** em inglês e português.
- **Imagens geradas pelo binário** (`scripts/site-images/`), num ambiente fixo.
- **Scanner de vazamento** que lê transcrições e bytes, com canário real.
- **Deploy** com permissões por job e sem credencial persistida.
- **REQs no backlog:**
  - `/api/chain` do `serve` expõe caminho absoluto;
  - write-scope re-enraizado por `trackfw.yaml` aninhado (F-5).

## Pendente
- O KG decide quando tirar o #702 de draft e quando mergear.
- Pós-merge: conferir o deploy e as capturas do site publicado; fechar a REQ e mover o roadmap para `done/`.

## Lições
- Scanner de bytes não vê texto renderizado em imagem: a verificação precisa ler a transcrição. E o script versionado tinha o próprio caminho do usuário, achado pela revisão, não pelo scanner.
- VHS deixa processos do Chrome vivos e às vezes não sobrescreve `Screenshot`: conferir processos e mtimes depois de cada geração.
- Autoteste que passa com o controle removido é vazio: medir por mutação.
