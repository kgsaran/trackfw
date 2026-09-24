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
**Status:** ⬜ Pendente
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
- [ ] Tabela: rótulo → mecanismo → grupo, para os **16**
- [ ] Hipótese do `setup` **medida**, com o resultado escrito mesmo que ela caia
- [ ] #307 absorvido ou descartado **com medição**; #308 idem
- [ ] Frase de fechamento por grupo
- [ ] Threat model dos rótulos de segurança
- [ ] 🔴 Nenhuma linha de implementação
- [ ] 🔴 **NÃO rodar `make quality`**

**Comandos de validação:** `trackfw barrier <roadmap> --wave 0`

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
