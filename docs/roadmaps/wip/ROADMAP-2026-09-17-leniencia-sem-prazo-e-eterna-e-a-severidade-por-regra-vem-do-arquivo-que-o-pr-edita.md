---
status: wip
date: 2026-09-17
---

# Roadmap: leniência sem prazo é eterna, e a severidade por regra vem do arquivo que o PR edita

> Created: 2026-09-17 | Status: wip

**Issue:** #387 · **REQ:** `docs/req/REQ-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md`
**ADR:** `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`

## Diagnóstico

Medido nesta árvore em 2026-09-17, com binário compilado do fonte:

```
$ trackfw validate ; echo $?
172 warning(s)
0
```

`governance-go-install` e `governance-install-script` — 2 dos 8 required checks — são **exit-0 por
construção**. Composição das 172:

| quantidade | natureza | destino |
|---|---|---|
| 122 | REQ sem ADR vinculado | dívida histórica — fica sob `lenient` com prazo |
| 35 | REQ sem roadmap vinculado | dívida histórica — idem |
| 7 | ADR órfão | dívida histórica — idem |
| 🔴 **6** | **REQ aberta cujo roadmap está em `done/`** | **carve-out: passa a bloquear** |
| 🔴 **2** | **caminho de estado obsoleto no vínculo** | **carve-out: passa a bloquear** |

As 8 do carve-out são de 11 a 17/09 e **foram produzidas por nós**, incluindo a do trabalho fechado
hoje. O `lenient` esconde defeito que estamos criando agora, não só dívida de junho.

**Defeito de produto, maior que a config deste repositório:** `trackfw discover` escreve
`governance_mode: lenient` **sem** `lenient_until` (`discover.go:497-499`), enquanto o `init`
brownfield escreve o prazo (`scaffold.go:735`); e `IsLenient()` (`validator.go:381-383`) trata prazo
ausente como `true`. O mecanismo de expiração existe e é burlado por **omitir um campo**.

---

## Wave 0 — Threat model
> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — quem continua conseguindo afrouxar a própria verificação depois desta REQ
**Status:** ⬜ Pendente · **Papel:** `hades-tf`

**Entregável:** `docs/portabilidade/2026-09-17-threat-model-severidade-da-validacao.md`

**Perguntas, todas com evidência de leitura — não asserção:**

1. **Completude da enumeração.** Os interruptores são só `governance_mode` e `rules: {off}`? Procure
   **outros caminhos** pelos quais o conteúdo do repositório altere a severidade ou o conjunto de
   regras avaliadas: chaves de `trackfw.yaml` que excluam caminhos, `req_dir`/`roadmap_dir` apontando
   para diretório vazio, `.trackfwignore` ou equivalente, variável de ambiente lida no CI, arquivo de
   allowlist. 🔴 **Não se limite aos sítios que a REQ nomeia.** Um `req_dir` apontando para pasta
   vazia zera as 172 sem tocar em `governance_mode` — verifique se isso é possível e diga.
2. **Ordem 3-antes-de-1.** A ADR afirma que apertar o `lenient` sem ancorar a severidade por regra
   troca um buraco por outro. **Confirme ou refute lendo o código.** Se a ancoragem do AC1 não cobrir
   algum caminho, nomeie-o.
3. **A ancoragem em HEAD é ela mesma contornável?** As 3 regras de credential-guard já usam esse
   padrão. Como alguém o derrota — `trackfw.yaml` ausente em HEAD (projeto novo), branch sem HEAD
   comparável, PR que cria o arquivo, repositório raso (`--depth 1`) no CI? 🔴 **`actions/checkout`
   faz fetch raso por default** — verifique se `git show HEAD:trackfw.yaml` funciona nos workflows
   deste repositório, e o que acontece se não funcionar: falha aberta ou fechada?
4. **O carve-out.** Dado o critério ("contradição entre dois artefatos vivos"), quem consegue
   produzir inconsistência ativa que **não** caia na lista fechada? E o inverso: alguma das 157
   históricas cai no carve-out por acidente e reprova o repositório?
5. **Residual declarado** — o que este desenho aceita não cobrir.

**Aceite:** parecer com os vetores enumerados e, por vetor, veredito (fecha / mitiga / não toca) com
evidência de leitura. 🔴 Vetor não fechado entra **nomeado**. Seção final sobre o que ficou em aberto.

**Critérios de aceite:**
- [ ] Os cinco eixos respondidos com evidência de leitura, não asserção de uma linha
- [ ] Veredito explícito sobre o fetch raso do `actions/checkout` (eixo 3)
- [ ] Veredito explícito sobre caminhos de configuração fora de `governance_mode`/`rules` (eixo 1)
- [ ] Vetores não fechados nomeados; residual declarado em seção própria
- [ ] Nenhuma linha de implementação escrita neste ML

**Gate da wave 0:**
```bash
P=docs/portabilidade/2026-09-17-threat-model-severidade-da-validacao.md
test -s "$P" || { echo "FAIL: parecer ausente ou vazio: $P"; exit 1; }
for termo in "IsLenient" "ruleSeverity" "checkout" "req_dir" "carve"; do
  grep -qi -- "$termo" "$P" || { echo "FAIL: parecer nao cobre: $termo"; exit 1; }
done
grep -qiE "residual|nao consegui|não consegui|indetermin" "$P" \
  || { echo "FAIL: parecer nao declara o que ficou em aberto"; exit 1; }
echo "OK [wave0/threat-model-severidade]: parecer presente e cobre os cinco eixos"
```

---

## Wave 1 — Ancoragem (precede tudo, por decisão da ADR)
> Dependências: **Wave 0 auditada.**

### ML-1A — severidade por regra ancorada em HEAD
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre **AC1** e **AC7**.

🔴 **Esta wave vem primeiro por decisão normativa da ADR.** Apertar o `lenient` (Wave 2) sem ancorar
a severidade por regra reabre a superfície `rules: {<regra>: off}` para as ~21 regras hoje não
ancoradas — troca-se um buraco por outro.

**Arquivos afetados:**
- `internal/validator/validator.go` — `ruleSeverity` (`:207-212`), `diskRuleSeverity` (`:218`)
- `internal/validator/validator_credential_guard_integrity.go` — `credentialGuardAnchoredRules` (`:238`) e o comparador HEAD↔disco já existente
- testes em `internal/validator/`

**Ações:**
1. Generalizar o padrão já validado: comparar a severidade declarada em **HEAD** com a do **disco** e
   adotar **a mais estrita**, para **todas** as regras — não só as 3 de credential-guard.
2. **Reaproveite o comparador existente**, não escreva um segundo. Se a lista nomeada deixar de ter
   função, diga isso no relatório em vez de mantê-la por inércia.
3. Comportamento quando **não há HEAD comparável** (projeto novo, `trackfw.yaml` criado no próprio
   PR, checkout raso): decida e **escreva a decisão no código**. ⚠️ O parecer da Wave 0 vai se
   pronunciar sobre isso — leia-o antes. A direção default é **falhar fechada**, coerente com a ADR.

**Critérios de aceite:**
- [ ] AC1 — todas as regras ancoradas; a mais estrita entre HEAD e disco prevalece
- [ ] AC7 — PR que commite `rules: {<regra>: off}` **não** rebaixa a severidade dessa regra
- [ ] AC7 contra-braço — **subir** a severidade no disco **é** respeitado (a regra é "a mais estrita
      vence", não "HEAD sempre vence"); sem este braço o teste não discrimina
- [ ] Ausência de HEAD comparável tem comportamento decidido, escrito e testado
- [ ] Reconciliação: uma frase por teste novo
- [ ] `go build ./...`, `make test`, `make quality` — RC medido sem pipe

---

## Wave 2 — Prazo e carve-out
> Dependências: **Wave 1 auditada.** Mesmo arquivo (`validator.go`) — não paralelizar com a Wave 1.

### ML-2A — leniência exige prazo, e o carve-out estrutural
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre **AC2**, **AC3**, **AC4** e **AC6**.

**Arquivos afetados:**
- `internal/validator/validator.go` — `IsLenient` (`:376-384`) e os **dois** sítios do bloco lenient (`:626` e `:942`)
- `internal/discover/discover.go` (`:497-499`)
- `internal/commands/help.go` (`:76-80`, documentação da chave)
- testes em `internal/validator/`, `internal/discover/`

**Ações:**
1. **AC2** — `IsLenient()` exige `lenient_until`. Prazo ausente ⇒ **strict**. 🔴 Cobrir **os dois**
   sítios do bloco lenient; o issue cita um só, e eu mesmo só tinha achado um na primeira auditoria.
2. **AC3** — `discover` escreve `lenient_until` com prazo default, como o `init` já faz. Use o
   **mesmo** default do `init`; se divergirem, diga por quê.
3. **AC4** — carve-out: conjunto **nomeado e fechado** de regras não cobertas pelo `lenient`.
   O critério de pertencimento vai **escrito no código**: contradição entre dois artefatos vivos, não
   ausência de artefato histórico. Calibre contra a medição: as 8 ativas entram, as 157 históricas
   não.
4. **AC6** — falsificação em duas direções **com contra-braço**: sem prazo reprova; com prazo futuro
   segue leniente fora do carve-out; e a versão **sem** a correção trata "sem prazo" como leniente.

**Critérios de aceite:**
- [ ] AC2 nos dois sítios de `IsLenient()`
- [ ] AC3 — `discover` e `init` param de divergir
- [ ] AC4 — lista fechada, critério escrito no código
- [ ] AC6 — as duas direções e o contra-braço demonstrados
- [ ] Reconciliação: uma frase por teste novo
- [ ] `go build ./...`, `make test`, `make quality` — RC sem pipe

---

## Wave 3 — Postura deste repositório
> Dependências: **Wave 2 auditada.**

### ML-3A — nosso trackfw.yaml, a medição que fecha e a nota de comportamento
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre **AC5**, **AC8** e **AC9**.

**Arquivos afetados:** `trackfw.yaml`, `CHANGELOG.md`, `docs/cli-parity.md` (se o contrato mudar)

**Ações:**
1. **AC5** — acrescentar `lenient_until` ao `trackfw.yaml`. 🔴 **Medição que fecha a REQ:**
   `trackfw validate` sai **≠ 0** apontando **exatamente** as 6 REQs abertas com roadmap em `done/` e
   os 2 caminhos de estado obsoletos — **e não as 157 históricas**. Se reprovar as 157, o carve-out
   está largo demais e volta para o ML-2A.
2. **Corrigir as 8 inconsistências ativas** — são nossas e recentes. Depois disso o `validate`
   volta a 0, agora por estar consistente, não por ser leniente.
3. **AC8** — CHANGELOG com nota **no topo da seção**: é mudança de comportamento para todo consumidor
   onboardado por `discover`, cujo `validate` passa a reprovar. ⚠️ Não proponha número de versão —
   isso sai no protocolo de release.

**Critérios de aceite:**
- [ ] AC5 — `validate` reprova as 8 ativas e **não** as 157 históricas (colar a saída)
- [ ] As 8 corrigidas; `validate` volta a 0 por consistência
- [ ] AC8 — nota de mudança de comportamento no topo da seção do CHANGELOG
- [ ] AC9 — `doctor`, `make quality` e os 8 required checks verdes; nenhum job renomeado
- [ ] `python3 scripts/check-required-status-checks.py --scope dw` RC=0

**Comandos de validação:**
```bash
go build ./... ; echo "RC=$?"
make test ; echo "RC=$?"
make build && ./bin/trackfw validate ; echo "VALIDATE_RC=$?"
./bin/trackfw doctor | head -1
python3 scripts/check-required-status-checks.py --scope dw ; echo "RC=$?"
make quality ; echo "RC=$?"
```

## Legenda de status

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado
