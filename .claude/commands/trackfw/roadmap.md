Gere um roadmap de implementação em microlotes para uma REQ do projeto.

## Passos

1. **Listar REQs disponíveis**
   Use Glob para listar `docs/req/*.md`. Se nenhum arquivo encontrado, informe:
   > Nenhuma REQ encontrada em `docs/req/`. Crie uma primeiro com `/trackfw:req`.

2. **Selecionar a REQ**
   - Se `$ARGUMENTS` foi fornecido: use como filtro (substring case-insensitive) para encontrar o arquivo
   - Se não foi fornecido ou o filtro não encontrar exatamente um: liste os arquivos disponíveis e pergunte ao usuário qual usar
   - Leia o conteúdo completo do arquivo REQ selecionado

3. **Gerar o roadmap**
   Com base no conteúdo da REQ, gere um roadmap seguindo **estritamente** este formato:

   ````markdown
   ---
   status: backlog
   date: <YYYY-MM-DD>
   req: "docs/req/<arquivo-selecionado>.md"
   squad: ""
   ---

   # Roadmap: <título derivado da REQ>

   > Created: <YYYY-MM-DD> | Status: backlog

   ## Diagnóstico / Contexto
   <resumo do problema, motivação e escopo extraídos da REQ>

   ## Wave 0 — Threat Model
   > Dependencies: none. Blocks all implementation.

   ### ML-0A — Threat model for this roadmap
   **Status:** pending
   **Files affected:**
   **Actions:**
   1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
   2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
   3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
   4. Declared residual — what this design accepts not covering.
   **Acceptance criteria:**
   - [ ] The four sections above answered with evidence, not a one-line assertion
   - [ ] No implementation line written for this ML

   **Gates da wave:**
   ```bash
   # Wave 0 gate — replace this placeholder with a project-specific check before
   # marking ML-0A done. Do not remove the gate; replace its command (AC13).
   exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
   ```

   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)
   > Dependências: Independente

   ### ML-1A — <título>
   **Status:** ⬜ Pendente
   **Arquivos afetados:**
   - `caminho/exato/do/arquivo`
   **Ações:**
   - Descrição detalhada da ação com valores, chaves e comandos exatos
   **Critérios de aceite:**
   - [ ] build sem erros
   - [ ] testes verdes
   **Comandos de validação:** `<comando de build e teste do projeto>`

   ### ML-1B — <título> (se independente de ML-1A)
   ...

   ## Wave 2 — <nome> (depende de Wave 1)
   > Dependências: Wave 1 completa
   ...
   ````

   **Princípios obrigatórios:**
   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra
   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos
   - Waves sequenciais apenas quando há dependência real de resultado
   - Critérios de aceite mensuráveis em cada ML

4. **Salvar o arquivo**
   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais
   - Crie o arquivo em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`
   - Preencha `req:` com o caminho relativo completo da REQ selecionada
   - Use a data de hoje

5. **Confirmar**
   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.
