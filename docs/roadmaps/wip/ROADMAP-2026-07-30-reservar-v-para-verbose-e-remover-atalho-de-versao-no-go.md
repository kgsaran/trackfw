---
status: wip
date: 2026-07-30
req: "REQ-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go"
squad: ""
---

# Roadmap: reservar -v para verbose e remover atalho de versao no Go

> Created: 2026-07-30 | Status: wip

## Contexto

REQ: `docs/req/REQ-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md`

`-v` é aceito apenas pelo Go, como atalho de `--version`, exposto por default do cobra
(`InitDefaultVersionFlag` registra o shorthand `v` quando o campo `Version` está preenchido). Decisão:
**remover** e **reservar** `-v` / `--verbose` para futuro modo verboso nos três.

**Escopo negativo:** não implementar verbose; não aceitar `-v` como no-op; não unificar mensagem nem
exit code de flag desconhecida (divergência pré-existente de framework, vale para toda flag).

## Critérios de Aceite

- [ ] `trackfw -v` não imprime versão em runtime nenhum e sai com código não-zero nos três.
- [ ] `--version` e `version` permanecem inalterados, byte-idênticos nos três.
- [ ] `cli-parity.md` registra `-v` / `--verbose` como reservado, com proibição explícita e motivo.
- [ ] Gate cobre `-v` nos três, com prova de falsificação reintroduzindo o atalho.
- [ ] `make quality` exit 0 e `validate --json` 0 violações.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier
Wave 2 — ML-2A (Go, único com mudança de código)
   ↓ barrier
Wave 3 — ML-3A (gate + falsificação)
```

**Não há waves paralelas nesta entrega.** Só o Go muda comportamento; Node.js e Python já rejeitam `-v`
e não têm o que alterar. Criar MLs vazios para eles seria cerimônia sem conteúdo — a paridade é
verificada pelo gate no ML-3A, que é onde ela pertence.

### Risco específico desta entrega

O `-v` do Go **não é declarado no código** — vem do default do cobra. Um implementador que procure por
`"v"` em `internal/commands/` não encontra nada e pode concluir que já está removido. O ML-2A precisa
verificar por **execução**, não por leitura.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar a reserva do `-v`
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:** `docs/cli-parity.md` — seção `## Version output`

**Deve pinar:**
1. `-v` **não é** atalho de `--version` em runtime nenhum; sai com código não-zero nos três.
2. `-v` / `--verbose` é **reservado** para modo verboso. Proibido vinculá-lo a outra semântica.
3. A razão da reserva: convenção do ecossistema (`docker`, `kubectl`, `ansible`, `ssh`, `curl`), e o
   fato de que nenhum dos três CLIs tem `--verbose` hoje.
4. **Fronteira explícita:** mensagem e exit code de flag desconhecida **não** são unificados —
   divergência pré-existente de framework (cobra 1, commander 1, argparse 2), válida para toda flag.
   Registrar o baseline medido, para ninguém tentar alcançar identidade byte-a-byte e recorrer a hack.
5. Que a reserva é **de contrato**, não de superfície: nenhum runtime aceita `-v` como no-op, porque
   flag aceita sem efeito é indistinguível de flag quebrada.

**Seção escrita:** `### \`-v\` is reserved for verbose — never bound to \`--version\`` em
`docs/cli-parity.md`, substituindo a antiga `### Out of scope: the -v shorthand`.

**Critérios de aceite:**
- [x] Reserva e proibição registradas com o motivo — convenção do ecossistema e ausência de `--verbose`
      nos três hoje. Registrado também que a flag foi exposta por **default de framework**, não por
      decisão de design.
- [x] Fronteira de não-unificação registrada com o baseline medido (`--zzz`: cobra 1, commander 1,
      argparse 2, três textos distintos), com a justificativa de por que forçar identidade seria escopo
      muito maior — e a nota explícita de que a fronteira existe para ninguém tentar e recorrer a hack.
- [x] Razão de não aceitar no-op registrada: flag aceita sem efeito é indistinguível de flag quebrada.
- [x] Registrado que implementar verbose **não** faz parte da reserva, e por quê.

---

## Wave 2 — Remover o atalho no Go (1 ML)
> Dependências: ML-1A completo.

### ML-2A — Desvincular `-v` de `--version` no Go
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `internal/commands/root.go`, testes correspondentes

**Diagnóstico:** o shorthand vem do cobra, não do código do projeto. Com `Version` preenchido
(`root.go:22`), o `InitDefaultVersionFlag` registra `--version` com shorthand `v` se o atalho estiver
livre. **Não existe nenhuma declaração de `-v` para procurar e apagar.**

**Caminhos candidatos** (escolha do implementador, desde que o comportamento observável bata):
- Chamar `InitDefaultVersionFlag()` e então zerar o shorthand:
  `root.Flags().Lookup("version").Shorthand = ""`.
- Declarar a flag `version` manualmente **sem** shorthand antes que o cobra a registre — o cobra só
  adiciona a dele se `Flags().Lookup("version") == nil`.

**Cuidado:** o `SetVersionTemplate("trackfw {{.Version}}\n")` de `root.go:27` deve continuar valendo. Se
o caminho escolhido substituir a flag do cobra, o template pode deixar de ser aplicado e o `--version`
regride — o que os testes do PR #91 devem pegar, mas confirme por execução.

**Critérios de aceite:**
- [ ] `trackfw -v` não imprime versão e sai com código não-zero.
- [ ] `trackfw --version` e `trackfw version` inalterados: `trackfw <semver>`, byte-idênticos entre si.
- [ ] Teste travando a rejeição de `-v` **e** a preservação das duas superfícies.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passam.

---

## Wave 3 — Gate e falsificação (1 ML)
> Dependências: **barrier** — ML-2A concluído.

### ML-3A — Cobrir `-v` no gate e provar não-vacuidade
**Status:** ⬜ Pendente
**Agente:** Artemis

**Ações:**
1. Cenário em `scripts/check-cli-parity.sh` afirmando, para os **três** runtimes: `-v` sai com código
   não-zero **e** sua saída **não** casa `^trackfw [0-9]+\.[0-9]+\.[0-9]+$`.
   A segunda asserção é a que importa — só o exit code não distingue "flag rejeitada" de "flag aceita
   que falhou por outro motivo".
2. Cenário de falsificação: seam que reintroduz o shorthand no Go e prova que o gate reprova. Corromper
   a **implementação**, nunca a asserção, com guarda de padrão contra `sed` obsoleto.
3. Confirmar que os cenários de `version` / `--version` do PR #91 continuam verdes — esta entrega não
   pode regredi-los.

**Critérios de aceite:**
- [ ] Cenário cobre `-v` nos três, com as duas asserções.
- [ ] Seam verificado por execução: com o atalho reintroduzido, o gate **falha**.
- [ ] Cenários de `version` / `--version` inalterados e verdes.
- [ ] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.
