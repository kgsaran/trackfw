# `mktemp` sem template ignora `TMPDIR` no macOS, e a medição de vazamento por contagem fica cega

> Data: 2026-11-11 · REQ-2026-11-11 (mktemp sem limpeza garantida, #676) · ML-1A

## Sintoma

Para medir se um script vaza temporário, exporta-se um `TMPDIR` isolado e compara-se a contagem de entradas antes
e depois. Nos scripts que chamam `mktemp -d` sem template, a contagem dá delta zero mesmo com vazamento real:
os diretórios foram criados em outro lugar.

## Medido

```bash
TMPDIR=<dir-isolado>/ /usr/bin/mktemp -d     # → /var/folders/.../T/tmp.XXXX  (ignorou TMPDIR)
TMPDIR=<dir-isolado> bash -c 'mktemp'        # → /var/folders/.../T/tmp.XXXX  (idem)
mktemp "${TMPDIR:-/tmp}/x.XXXXXX"            # → respeita o TMPDIR (template explícito)
```

Ambiente: macOS (Darwin 27), `/usr/bin/mktemp`, sem shim (`type -a mktemp`). Medido duas vezes, pelo ML-1A e
pelo arquiteto. O GNU `mktemp` (Linux, Git for Windows) não foi medido aqui.

## Regra

- Medição de vazamento por contagem num `TMPDIR` isolado só vale para `mktemp` com template explícito. Para
  `mktemp -d` sem template, capture o caminho criado (por exemplo, imprimindo-o no stderr da cópia instrumentada)
  e teste a existência dele depois da saída.
- Um delta zero sem esse cuidado não prova ausência de vazamento.
