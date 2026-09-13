# Falsificar mutando fonte Python deixa `.pyc` obsoleto que sobrevive à restauração

> 2026-09-13 · descoberto auditando o ML-1A da v8, e a contaminação foi **do próprio auditor**

## O que aconteceu

Para falsificar um gate de versão, mutei `pypi/trackfw/__init__.py` de `7.6.0` para `9.9.9`, rodei o
gate (reprovou corretamente), e restaurei o arquivo com `cp`. O `git status` ficou **limpo**.

Duas horas depois, `make quality` reprovou:

```
FAILED pypi/tests/test_discover_ci_workflow_pin.py::test_go_install_pin_is_not_hardcoded
AssertionError: '9.9.9' unexpectedly found in 'name: trackfw validate...'
```

## 🔴 A medição que confunde

```
grep no arquivo em disco        →  7.6.0
git status                      →  limpo
python3 -c "import trackfw"     →  9.9.9      ← o bytecode
```

**Disco e git concordam; o interpretador discorda dos dois.** O `pypi/trackfw/__pycache__/__init__.cpython-314.pyc`
foi compilado enquanto o fonte tinha `9.9.9`, e não foi invalidado pela restauração.

`find pypi -name __pycache__ -exec rm -rf {} +` ⇒ `__version__ = 7.6.0`, **8 passed**.

## Por que isso engana especialmente bem

O sintoma aponta para **código**, e três sinais confirmam a leitura errada:

1. o teste que falha é de **outra área** (pin de workflow), não do que foi mutado
2. o mesmo teste **passa na `main`** — o que parece provar regressão de branch
3. a árvore está **limpa** — o que parece descartar contaminação local

Eu segui os três e concluí *"é regressão do ML-1A"*. Estava errado. Antes disso, o agente tinha
concluído *"é flakiness pré-existente do `check-doctor-parity`"* — também errado, e nem era o teste
que falhava.

**Duas pessoas, dois diagnósticos, nenhum correto** — porque as três evidências convergiam para o
lugar errado.

## O que fazer

- 🔴 **Toda falsificação que muta fonte Python limpa o `__pycache__` na restauração**, não só o
  arquivo. Restaurar o fonte **não** restaura o estado do interpretador.
- Quando `git status` estiver limpo e o comportamento discordar do disco, **suspeite de cache antes
  de suspeitar de código** — em Python é `__pycache__`; em Node é o cache do `npm`
  (ver [[dois-falsos-vermelhos-ambiente-e-heuristica-2026-09-12]], onde o mesmo padrão me fez
  concluir que o `npm` ignora `HTTPS_PROXY`).
- O padrão seguro é falsificar **em cópia descartável da árvore**, não na de trabalho.

## O que isto diz sobre as suítes

É a terceira ocorrência em dois dias de **ambiente contaminado produzindo sintoma de código**, e as
três apontaram para o lugar errado:

```
2026-09-12  npm/node_modules ausente        →  900 testes vermelhos, parecia código quebrado
2026-09-12  cache quente do npm             →  "o npm ignora HTTPS_PROXY", concluído e errado
2026-09-13  __pycache__ obsoleto            →  "regressão do ML-1A", concluído e errado
```

Reforça a `REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado`, e
sugere que o escopo dela deveria incluir **ambiente contaminado**, não só **incompleto** — são
sintomas diferentes da mesma cegueira.
