---
status: backlog
date: 2026-09-24
req: "docs/req/REQ-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md"
squad: [hades-tf, ares-tf, artemis-tf]
---

# Roadmap: caminho POSIX interpolado no código Python não é convertido pelo MSYS

> Criado em: 2026-09-24 | Status: ⬜ Backlog

REQ: `docs/req/REQ-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`

## Diagnóstico

O Git Bash converte caminho POSIX em `argv`, **não** dentro de string de código. O Python do Windows
lê `/tmp` como `C:\tmp` e o `open()` morre. Origem: #363 + PR #417 (`dc95ff34`), que corrigiu 5
sítios de 1 arquivo e **declarou** os restantes.

🔴 **Duas medições de partida discordam** — 15/10 (reportante) vs 7 candidatos (arquiteto), por
critérios diferentes. Nenhuma é o veredito; reconciliá-las é entregável da Wave 0.

⚠️ **VM investiga, CI mede.** A reprodução mínima roda em qualquer Git Bash; o número que vira
afirmação sai do CI.

⚠️ **Custo de CPU:** teto de 2 agentes simultâneos, `go test` só do pacote tocado, `make quality`
apenas na barreira do arquiteto.

---

## Wave 0 — Enumeração e reconciliação (1 ML, bloqueia tudo)

### ML-0A — A enumeração real, e por que os dois números de partida divergem
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Arquivos afetados:** nenhum de produto

**Ações:**
1. Enumere pelo critério **"interpola caminho dentro do texto do programa Python E usa esse caminho
   para abrir/ler/escrever arquivo"**. Classifique **(a)** defeito · **(b)** correto · **(c)** o `$`
   não é caminho.
2. 🔴 **Reconcilie os dois números de partida** — 15/10 do #417 e 7 do arquiteto. Um dos dois (ou os
   dois) usou critério mais largo ou mais estreito; diga qual e por quê. Não escolha o maior por
   precaução nem o menor por conveniência.
3. Threat model: algum sítio (a) está em gate de **segurança**? Se sim, a garantia fica sem prova no
   Windows — nomeie qual.
4. Frase de fechamento: *"corrijo esta causa, exatamente estes sítios fecham, e nenhum outro."*

**Critérios de aceite:**
- [ ] Tabela por sítio, com `arquivo:linha` e veredito, e o comando que produziu a lista
- [ ] Critério aplicável por terceiro, não julgamento do revisor
- [ ] Reconciliação escrita dos dois números divergentes
- [ ] 🔴 Nenhuma linha de implementação
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 1 — Correção e gate (2 MLs em paralelo, arquivos disjuntos)
> Dependências: Wave 0 auditada.

### ML-1A — Sítios (a) passam o caminho por `argv`
**Owner:** `ares-tf` · **Status:** ⬜ Pendente

O padrão já existe no repositório: `check-thirdparty-parity.sh:167` e o
`_normalize_version_in_file` do `check-doctor-parity.sh`. Não invente terceiro idioma.

- [ ] Todo (a) por `argv`; fixtures gerados **byte a byte idênticos** aos de antes
- [ ] Braço POSIX: nada muda no que já passava
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

### ML-1B — Gate anti-reintrodução
**Owner:** `artemis-tf` · **Status:** ⬜ Pendente

- [ ] Reprova interpolação de caminho no código Python — provado por injeção, forma a forma
- [ ] Não reprova os (b) legítimos — provado na árvore
- [ ] Formas não cobertas declaradas no cabeçalho, com a razão medida
- [ ] Guarda de não-vacuidade com piso e o comando que o produziu
- [ ] 🔴 Uma frase por teste novo
- [ ] 🔴 **NÃO rodar `make quality`**

---

## Wave 2 — Prova no Windows (1 ML)
> Dependências: Wave 1 mergeada.

- [ ] Falsificação nas duas direções **exercitada no Windows** — a plataforma onde o defeito vive
- [ ] 🔴 Se a #363 não fechar, a razão fica escrita: ela depende também do cenário do bit em NTFS

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`, CI verde.
