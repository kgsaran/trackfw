Gere um roadmap de implementação em microlotes para uma REQ do projeto trackfw.

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

   ```markdown
   # Roadmap: <título derivado da REQ>

   > Criado em: <YYYY-MM-DD> | Status: ⬜ Backlog

   ## Diagnóstico / Contexto
   <resumo do problema, motivação e escopo extraídos da REQ>

   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)
   > Dependências: Independente

   ### ML-1A — <título>
   **Status:** ⬜ Pendente
   **Arquivos afetados:**
   - `caminho/exato/do/arquivo.go`
   **Ações:**
   - Descrição detalhada da ação com valores, chaves e comandos exatos
   **Critérios de aceite:**
   - [ ] build sem erros
   - [ ] testes verdes
   **Comandos de validação:** `go build ./... && go test ./...`

   ### ML-1B — <título> (se independente de ML-1A)
   ...

   ## Wave 2 — <nome> (depende de Wave 1)
   > Dependências: Wave 1 completa
   ...
   ```

   **Princípios obrigatórios:**
   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)
   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra
   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos
   - Waves sequenciais apenas quando há dependência real de resultado
   - Critérios de aceite mensuráveis em cada ML

4. **Salvar o arquivo**
   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais
   - Crie o arquivo em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`
   - Use a data de hoje

5. **Confirmar**
   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.
