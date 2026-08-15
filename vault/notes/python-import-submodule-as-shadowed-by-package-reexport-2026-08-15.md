# `import package.submodule as x` can silently bind the WRONG object when `__init__.py` re-exports a same-named symbol

> 2026-08-15 · ML-2B do `ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas`
> (`pypi/trackfw/thirdparty/__init__.py` re-exports `fetch` the function;
> `pypi/trackfw/thirdparty/fetch.py` is the module of the same name)

## O erro de enquadramento

`pypi/trackfw/thirdparty/__init__.py` faz `from .fetch import ThirdPartyFetchError, fetch` — isso
vincula o atributo `fetch` do **objeto de pacote** `trackfw.thirdparty` à **função**
`trackfw.thirdparty.fetch.fetch`, sobrescrevendo o atributo que o Python normalmente atribuiria
automaticamente ao **submódulo** `trackfw.thirdparty.fetch` quando ele é importado.

Um teste que precisa acessar o submódulo diretamente (para monkeypatchar `MAX_CONTENT_SIZE`,
`MAX_REDIRECTS`, `ThirdPartyFetchError`, etc. — símbolos que só existem no módulo, não
re-exportados pelo pacote) e escreve:

```python
import trackfw.thirdparty.fetch as fetch_mod
```

recebe silenciosamente a **função**, não o módulo — porque a forma `import a.b.c as x` do Python
não faz lookup direto em `sys.modules['a.b.c']`; ela importa a cadeia completa (populando
`sys.modules`) e depois resolve `x` por **travessia de atributos a partir do pacote-topo**:
`x = sys.modules['a'].b.c`. Como `trackfw.thirdparty.__init__` já rebindou o atributo `fetch` do
pacote para a função (na ordem de execução do próprio `__init__.py`, que importa o submódulo e
imediatamente sobrescreve o atributo), a travessia `trackfw.thirdparty.fetch` devolve a função.

Sintoma: `AttributeError: 'function' object has no attribute 'ThirdPartyFetchError'` (ou
`'MAX_CONTENT_SIZE'`, etc.) num teste que claramente importou "o módulo certo" pelo caminho
pontuado — sem nenhum erro de import, o que torna a causa não óbvia.

## A regra

Quando um pacote Python re-exporta em `__init__.py` um símbolo com o **mesmo nome** de um dos
seus submódulos (aqui: `fetch` função vs. `fetch.py` módulo), `import pacote.submodulo as alias`
deixa de ser confiável para obter o submódulo. Use:

```python
from importlib import import_module
fetch_mod = import_module("trackfw.thirdparty.fetch")
```

`import_module` resolve direto por `sys.modules`, sem travessia de atributos, e por isso não é
afetado pelo shadowing do `__init__.py`.

## Como reconhecer que você caiu nisso

- `import pacote.submodulo as alias` não levanta `ImportError`/`ModuleNotFoundError`.
- `alias` tem `type(alias) is not <class 'module'>` — no exemplo, `type(alias) is function`.
- O pacote em questão tem um `__init__.py` cujo `from .submodulo import X` inclui um nome `X`
  igual ao nome do próprio submódulo (aqui, `fetch`).

## Relacionado

- `pypi/trackfw/thirdparty/__init__.py` — `from .fetch import ThirdPartyFetchError, fetch`.
- `pypi/trackfw/thirdparty/fetch.py` — o módulo shadowado.
- `pypi/tests/test_thirdparty.py` — `fetch_mod = import_module("trackfw.thirdparty.fetch")`.
