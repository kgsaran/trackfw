---
status: wip
date: 2026-09-24
req: "docs/req/REQ-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md"
squad: [hades-tf, ares-tf, artemis-tf]
---

# Roadmap: treze rótulos falham no censo de Windows, e cinco são `setup`

> Criado em: 2026-09-24 | Status: wip

REQ: `docs/req/REQ-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`

## Diagnóstico

Primeira triagem do cluster de Windows com **número real**, não estimativa. Censo `36036473391`
(`main`, 8/8 shards, sem `TOTAL INCOMPLETO`): **OK=347 · FAIL=11 · 13 rótulos distintos em FAIL ·
3 ausentes**.

🔴 **A hipótese que organiza tudo, e que pode cair:** 5 dos 13 são `setup-*`, e um `setup` que falha
**aborta o cenário inteiro**. Se ela se confirmar, a população real é de poucas causas. Se cair,
isso fica escrito e a triagem segue rótulo a rótulo.

⚠️ **VM investiga, CI mede.** O mecanismo se acha na VM (`ssh powershell-vm`); **todo número que
virar afirmação sai do `windows-census.yml`**.

⚠️ **Custo de CPU:** teto de 2 agentes simultâneos, `go test` só do pacote tocado, `make quality`
apenas na barreira do arquiteto.

---

## Wave 0 — Triagem por mecanismo (1 ML, bloqueia tudo)

### ML-0A — Agrupar os 16 rótulos por causa, e medir a hipótese do `setup`
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-09-24
**Entregue:** `docs/seguranca/2026-09-25-triagem-cluster-windows.md`
**Arquivos afetados:** nenhum de produto — entrega `docs/seguranca/2026-09-25-triagem-cluster-windows.md`

**Ações:**
1. **Rode os gates na VM** e agrupe por **mecanismo**, não por nome de rótulo. Os 13 em `FAIL` e os
   3 ausentes estão na REQ.
2. 🔴 **Meça a hipótese do `setup`**: quantos dos 8 não-`setup` fecham quando o `setup` do mesmo
   cenário passa? Force o setup a passar (ou rode o cenário a partir do ponto seguinte) e veja o que
   sobra. **Se nada fechar, a hipótese cai e isso vira o achado principal.**
3. 🔴 **Confirme ou descarte o #307.** Ele afirma, com medição de terceiro, que
   `check-release-tag-parity.sh` falha no Windows porque o `ln -s` degrada para cópia e o `python3`
   copiado não inicia — e que **a guarda de vacuidade acusa o `git` quando o culpado é o `python3`**.
   4 dos 5 `setup-*` são desse gate. Se for o mesmo mecanismo, **absorva**; se não, escreva a
   diferença.
4. **#308 é candidato** para `git-branch-guard-dedup/*` (MSYS expandindo `{owner}/{repo}` ao
   reconstruir `argv` de pai nativo). Candidato, não membro.
5. Para cada grupo: a frase *"corrijo esta causa, exatamente estes rótulos fecham, e nenhum outro"*.
6. **Threat model:** vários rótulos são de controle de segurança (`credential-guard-*`,
   `git-branch-guard-*`). Para cada um, **qual garantia fica sem prova no Windows** e se há cobertura
   equivalente por outro caminho.

**Critérios de aceite:**
- [x] Tabela com **16 linhas**: 14 reais + 2 marcadas **fantasma**, com evidência. Cinco grupos:

| grupo | n | mecanismo | categoria |
|---|---|---|---|
| **G1** | 6 | `check-release-tag-parity`: filho nativo morre no `NO_FORGE_PATH` curado (`0xC0000005`) | A — recusa ruidosa |
| **G2** | 4 | `check-update-parity` morre no `ln -sf` do Cenário 9 sob `set -euo pipefail` | A — recusa ruidosa |
| **G3** | 1 | `chmod 0644` não retira o bit — `ls -la` mostra `-rwxr-xr-x` **depois** | **C — passagem vacuosa** |
| **G4** | 2 | grafia MSYS de `HOME` cruza para o Go; `normalizeGuardPath` só converte com letra de unidade | 🔴 **D — produto** |
| **G5** | 1 | dreno `read -r -t 2 -d ''` não sustenta 200 KB; **guard sai 0** | 🔴 **D — produto** |
- [x] 🔴 **A hipótese CAIU, por dois lados.** Fonte: `falsify_fail_point` **retorna 0** sob
      `TRACKFW_FALSIFY_ENUMERATE=1`, o modo do censo — não aborta nada. Log: o shard 3 imprime **nove
      rótulos** depois de `FAIL [setup-s75]`, incluindo cenários inteiros.
      **O que o setup reprovado causa de fato:** o `echo OK` do `else` não sai — explica **os 3
      ausentes** e **zero** dos 11 FAIL. O que sobrevive é **causa comum**, não encadeamento:
      corrigir o braço de setup não fecharia os outros; **fechar o gate fecha os dois**
- [x] **#307 ABSORVIDO** — os dois bloqueios dele explicam **10 dos 14** (G1 e G2), com **três
      diferenças medidas** escritas. **#308 DESCARTADO** com a diferença: o G4 diverge **inteiramente
      dentro do Go**, sem `argv`, sem chaves e sem processo nativo no caminho
- [x] Uma por grupo, **com previsão de delta pareada** (G1: `FAIL −4` e `OK +2`, **não** −6)
- [x] **Quatro** categorias, não duas — a distinção *falha ruidosa* ≠ *garantia sem prova* se abre
      em A (recusa ruidosa, risco baixo), C (**passagem vacuosa**: `test -x` verdadeiro nos dois
      braços, o `OK` não prova nada) e D (**controle ausente no Windows**)
- [x] 🔴 Nenhuma implementação — zero arquivos em `internal/` ou `scripts/`
- [x] 🔴 `make quality` não rodado; **VM não usada** — todo o mecanismo saiu de fonte + log do censo

**Comandos de validação:** `trackfw barrier <roadmap> --wave 0`

**Auditoria do arquiteto (medida por mim):**

🔴 **O achado mais importante é uma correção a mim, e é o erro nº 11 do instrumento nesta campanha
— literalmente o que a minha própria memória descreve.** Eu enumerei os FAIL com
`grep -ao 'FAIL \[falsify/…'` **sem âncora**, e capturei texto **citado dentro** de mensagens
`PROOF …/non-vacuity`. Remedi com a âncora correta para o formato do `gh run view --log` (as linhas
começam com `job\tstep\ttimestamp`, então `^FAIL` dá **zero** — outra armadilha):

```
grep -aoE '[0-9]Z FAIL \[falsify/[^]]*\]'   →  11 ocorrências, 11 distintos
```

E `credential-guard-script-integrity/detected` aparece como **`OK`**. 🔴 **A inflação caiu
exatamente sobre a superfície de segurança** — os dois fantasmas eram os dois controles de
integridade de script, ambos **verdes**. Uma wave inteira teria sido desenhada para um buraco
inexistente.

**Decisões minhas, tomadas sobre as três que ele levantou:**

1. **#421 e o `.venv` ENTRAM na REQ.** As duas exclusões eram minhas e estavam erradas: o bit em
   NTFS é a **mesma causa** do G3 (e o censo **confirma a #421 em x64**, que a issue declarava não
   medido), e o Cenário 9 **não usa venv** — é symlink pendurado sintético, mesmo mecanismo do #307.
   *"Está fora do escopo declarado"* é exatamente o que a Regra Dura recusa.
2. **População corrigida para 14** na REQ, com a razão escrita — senão a recontagem procuraria um
   delta que nunca existiu.
3. **A Forma B vira ML próprio nesta REQ** (abaixo).

**Ordem aceita: G4 e G5 primeiro.** O argumento dele é o certo — os outros três são gates que não
rodam e **falham alto**; inverter gastaria as primeiras waves deixando o CI verde enquanto dois
defeitos de **produto** seguem vivos.

⚠️ **E ele declarou o que não determinou:** a razão do `0xC0000005`, qual elo do G4 dispara primeiro,
e a causa do G5 entre orçamento de tempo e semântica de `read -d ''`. A ausência de cobertura
equivalente do G4 está marcada como **presunção, não medição** — é o primeiro item a verificar na
wave do G4.

---

### ML-0B — A Forma B: o rótulo afirma que o gate passou limpo, e o gate reprovou
**Owner:** `artemis-tf`
**Status:** ⬜ Pendente

Achado do ML-0A, **fora dos 14 e fora de qualquer REQ**: nos Cenários 87 (`:6066`) e 158 (`:6123`) o
`echo OK` está **fora do `if`**. A baseline reprova, e o rótulo que afirma *"o gate passa limpo"*
imprime `OK` na linha seguinte.

🔴 **É o único falso verde do cluster.** Os braços de detecção não são vazios — o que se perde é o
**discriminante de delta único**.

**Detector mecânico, já nomeado:** a Forma B **não chama** `falsify_count_success`, logo a contagem
de `^OK` no log **≠** tally. A divergência é medível sem inspeção.

**Critérios de aceite:**
- [ ] Os dois sítios corrigidos: o `OK` passa a ser condicional à baseline ter passado
- [ ] 🔴 **Varredura da forma** — `echo OK` fora do `if` que decide o veredito — com veredito por
      sítio; a divergência `^OK` vs tally é o instrumento
- [ ] Falsificação nas duas direções
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 1+ — Um grupo por wave
> Dependências: Wave 0 auditada. **O detalhamento é escrito depois do ML-0A** — ML detalhado sobre
> mecanismo não medido é o erro do `IsAbs`.

**Critérios que já valem, qualquer que seja o agrupamento:**
- [ ] Um **grupo** por wave, não um rótulo por wave
- [ ] Falsificação nas duas direções, exercitada no **Windows**
- [ ] 🔴 **Recontagem no CI ao fim da wave**, com o delta **atribuído** ao grupo — e se o delta não
      bater com o previsto, isso é achado, não ruído
- [ ] 🔴 Nenhum rótulo silenciado por `skip` para reduzir contagem. Supressão exige nomear a
      garantia não exercitada

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.
