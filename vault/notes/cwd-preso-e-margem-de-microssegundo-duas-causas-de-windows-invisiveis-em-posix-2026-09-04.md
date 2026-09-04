---
title: cwd preso e margem de microssegundo — duas causas de Windows invisíveis em POSIX
date: 2026-09-04
tags: [windows, testes, tempfile, relogio, stale_wip]
roadmap: ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz (ML-4B)
---

# cwd preso e margem de microssegundo

Duas causas de falha de Windows medidas no ML-4B. As duas são **invisíveis em POSIX por
construção** — não é falta de cobertura local, é o POSIX não ter a restrição.

## 1. `WinError 32` na LIMPEZA do `TemporaryDirectory`, não no corpo do teste

`PermissionError: [WinError 32] The process cannot access the file because it is being used by
another process` no `os.rmdir(path)` de `shutil._rmtree_unsafe`, chamado pelo `__exit__` do
`tempfile.TemporaryDirectory`.

**Quem segura o handle é o próprio processo de teste: o `os.chdir()` para dentro do tmpdir.** No
Windows o cwd mantém um handle aberto sobre o diretório e ele não pode ser removido. Em POSIX um
diretório é removível enquanto é o cwd — o defeito não existe lá.

**O discriminante já estava na população:** em `pypi/tests/test_generators_adr.py::TestAdrCommandScope`,
os **4** testes que fazem `os.chdir` falharam e o **único** que não faz
(`test_scope_global_com_dir_da_erro_claro`) passou.

🔴 **`tearDown` NÃO resolve.** Ele roda depois que o `with tempfile.TemporaryDirectory()` já
tentou apagar o diretório. O restore tem de acontecer **dentro** do bloco.

🔴 **A ordem do `with` carrega o peso.** Gerenciadores saem em ordem inversa, então o
`_chdir(...)` tem de ser o **último** da cadeia:

```python
with tempfile.TemporaryDirectory() as home, \
     tempfile.TemporaryDirectory() as cwd_dir, \
     _chdir(cwd_dir):          # <- ULTIMO: sai primeiro, restaura o cwd antes da limpeza
```

Colocado **primeiro**, o bug volta e **continua passando em POSIX**. Por isso a falsificação
local não é o `WinError` (irreproduzível fora do Windows) e sim a propriedade: *no instante em
que a limpeza roda, o cwd já está fora do tmpdir*.

## 2. `stale_wip` "9 days" em vez de "10 days" — margem de ~10 µs, não fuso horário

```
AssertionError: '10 days' not found in
'roadmap/wip/roadmap-antigo.md has been in WIP for 9 days (last modified 2026-08-24)'
```

O teste gravava `mtime = time.time() - 10 dias` e deixava a produção ler
`datetime.now().timestamp()` sozinha. `age_days` é `int(age/86400)` — **floor**.

**Margem medida nesta árvore (200 amostras, macOS): 9 a 21 microssegundos.** Qualquer desvio
não-positivo entre as duas leituras de relógio derruba 10.0000001 dias para 9.

- **Não é fuso horário:** `2026-08-24 → 2026-09-03` não cruza transição, e o runner é UTC.
- **Não é defeito de produto:** floor é a semântica certa de "está em WIP há N dias", e em
  produção a idade é de dias, não de microssegundos.
- **É intermitente**, e a intermitência é a assinatura: falhou no run `33810452454` (2026-09-03)
  e **passou** no `33885303160` (2026-09-04), sem mudança no código dessa regra.

**Remédio:** `validate_stale_wip` já aceita `now=`. Injetar o `now` e dar folga (10 dias **e 1
hora**) tira os dois relógios da equação. O limite exato continua coberto por um teste dedicado
que falsifica nas duas direções (`now` exato → "10 days"; `now - 1 µs` → "9 days").

🔴 **Qual quirk de relógio do Windows come a margem NÃO foi medido** — só o CI fecha isso. A
verdade que basta é: a asserção dependia de ~10 µs de acordo entre dois relógios independentes.

## 3. Achado de PRODUTO, não corrigido — `stale_wip` fica MUDA no Windows

`TestStaleWIPReportsWIPWalkError` (`internal/validator/validator_stale_wip_contract_xfail_test.go:127`)
falha nos **dois** runs de Windows, sem intermitência:

```
esperava diagnostico para erro de walk/ENOTDIR em wip/; warnings=[]
```

O teste põe um **arquivo regular** no lugar de `docs/roadmaps/wip/` e espera um diagnóstico de
inspeção. Em POSIX `os.ReadDir` devolve `ENOTDIR`, `os.IsNotExist` é falso e o warning sai. No
Windows o `warnings` volta **vazio** — a regra vai a silêncio em vez de reportar que não
conseguiu inspecionar `wip/`.

**Isso é robustez de produto (`internal/validator/validator.go:1698-1702`), não fixture** —
mesma classe do ML-1C: não mascarar como correção de teste. Fica **fora do ML-4B** e no colo da
Wave 3, que já é dona de `internal/validator/`.
