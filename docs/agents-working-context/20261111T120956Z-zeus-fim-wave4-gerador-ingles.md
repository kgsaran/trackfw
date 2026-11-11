# Zeus: fim das Waves 3 e 4 da REQ de idioma (gerador em inglês pronto para o PR 2)

- **Branch:** `feat/idioma-do-produto-ingles-primeiro-gerador`. Waves 3 e 4 entregues; os barriers das Waves 3 e 4 passam (`--trust-local-gates`), e `make quality` deu rc=0.
- **Release anterior:** v11.6.0 publicada nos três canais com o leitor bilíngue (ML-2B, PRs #677 e #678).
- **Wave 3:**
  - ML-3A: gerador de roadmap na forma D4.
  - ML-3C: conteúdo do consumidor e presets.
  - ML-3B1 e ML-3B2: mensagens do CLI e dashboard. Os primeiros agentes caíram por ENOTFOUND e foram retomados por agentes novos.
  - ML-3D: guard `internal/i18nguard` que reprova literal PT; houve corretivo de fail-open em parse.
  - ML-3E: saída só em inglês; os catálogos pt-BR/es-ES foram removidos por decisão do KG (2026-11-11).
  - ML-3F: modelo de `/trackfw:roadmap` com `## Acceptance Criteria`.
  - ML-3G: palavras PT sem acento no guard e mais 5 mensagens traduzidas.
- **Red team (ML-4A):** a rodada 1 reprovou (F1 i18n por locale, F4 modelo sem critérios); a rodada 2 aprovou com ressalvas. O F5 entrou na mesma REQ (ML-3G), contra a sugestão de REQ separada, por ser a mesma causa do AC2.
- **Achado de processo:** o `ml_scope` do barrier lê o roadmap commitado, então é preciso commitar o roadmap antes de reexecutar. "os `_test.go` correspondentes" não declara escopo; liste os caminhos.
- **Pendente:**
  - PR 2, quando o KG pedir.
  - Depois do merge, o PR de release com CHANGELOG, incluindo o aviso F3 (binário anterior a v11.6.0 conta zero gates em `**Wave gates:**`), e a tag minor.
  - Fechar a REQ e mover o roadmap para `done/`, depois do merge.
  - Issue #676: REQ própria em seguida.
