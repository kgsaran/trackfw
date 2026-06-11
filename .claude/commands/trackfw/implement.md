Você é o orquestrador de implementação do trackfw. Siga o fluxo abaixo **sem pular etapas**.

## Argumento

`$ARGUMENTS` é opcional. Se fornecido, é usado como filtro (substring case-insensitive) sobre os nomes de arquivo das REQs.

---

## Passo 1 — Selecionar a REQ

Use Glob para listar `docs/req/*.md`.

- Se **nenhum arquivo encontrado**: informe que não há REQs disponíveis e sugira criar com `/trackfw:req`.
- Se **`$ARGUMENTS` foi fornecido** e filtra para exatamente uma REQ: use-a diretamente.
- Em **todos os outros casos** (sem argumento, ou argumento ambíguo): apresente a lista de REQs disponíveis e pergunte ao usuário qual deseja implementar. Exiba os nomes dos arquivos sem o path completo para facilitar a leitura.

Leia o conteúdo completo da REQ selecionada.

---

## Passo 2 — Encontrar ou gerar o Roadmap

Verifique se existe um roadmap vinculado à REQ buscando em `docs/roadmaps/` (backlog, wip, blocked, done, abandoned) por arquivo cujo nome contenha o slug da REQ.

**Se o roadmap ainda não existe:**
- Informe o usuário: "Nenhum roadmap encontrado para esta REQ. Gerando agora..."
- Execute o fluxo completo de geração do `/trackfw:roadmap` (leia o arquivo `.claude/commands/trackfw/roadmap.md` para seguir as instruções exatas), passando a REQ já selecionada — não pergunte novamente.
- Salve o roadmap gerado em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`.

**Se o roadmap existe e já está em `done/` ou `abandoned/`:**
- Informe o usuário e pergunte se deseja criar um novo roadmap ou encerrar.

**Se o roadmap existe em `backlog/` ou `blocked/`:**
- Prossiga para o Passo 3.

**Se já está em `wip/`:**
- Prossiga diretamente para o Passo 4 (já está em execução).

---

## Passo 3 — Mover roadmap para WIP

Execute:
```bash
trackfw roadmap move <nome-do-roadmap> wip
```

Confirme que o arquivo foi movido para `docs/roadmaps/wip/`.

---

## Passo 4 — Ler e apresentar o plano

Leia o roadmap (agora em `wip/`). Apresente ao usuário:
- Título do roadmap
- Total de Waves e MLs
- Lista resumida dos MLs por Wave

Confirme: "Iniciando implementação. Vou executar cada ML em ordem e atualizar o roadmap a cada conclusão."

---

## Passo 5 — Executar cada ML em ordem

Para cada Wave (em sequência), execute os MLs da Wave:

### Para cada ML:

**5a. Anunciar:** informe qual ML está sendo executado (ex: "Executando ML-1A — Criar client.go").

**5b. Implementar:** execute as ações descritas no ML usando suas ferramentas (Read, Write, Edit, Bash). Siga exatamente os arquivos afetados, ações e critérios de aceite listados no roadmap.

**5c. Validar:** execute os comandos de validação do ML (ex: `go build ./...`, `go test ./...`). Se falhar, corrija antes de avançar.

**5d. Atualizar o roadmap:** edite o arquivo de roadmap em `docs/roadmaps/wip/` substituindo o status do ML:
- `**Status:** ⬜ Pendente` → `**Status:** ✅ Concluído`

**5e. Commitar:**
```bash
git add -A
git commit -m "feat(<escopo>): <descrição do ML>"
```

**Atenção:** MLs dentro da mesma Wave que forem marcados como independentes podem ser executados em sequência rápida. Só avance para a próxima Wave após todos os MLs da Wave atual estarem ✅.

---

## Passo 6 — Finalizar

Quando todos os MLs estiverem ✅:

**6a.** Execute `trackfw validate` — deve passar com zero violations.

**6b.** Mova o roadmap para done:
```bash
trackfw roadmap move <nome-do-roadmap> done
```

**6c.** Faça o commit final:
```bash
git add docs/roadmaps/
git commit -m "docs(trackfw): roadmap <nome> → done"
```

**6d.** Informe o usuário:
```
✅ Implementação concluída.
Roadmap: docs/roadmaps/done/<nome>.md
Próximo passo: abrir PR com gh pr create
```
