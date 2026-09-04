"""pathfmt — ponto único de normalização de separador do CLI Python.

ADR-2026-09-04 "Separador POSIX nos artefatos autorados cujo consumidor não é o sistema de
arquivos", D3: "Não espalhar ``strings.ReplaceAll`` pelos chamadores. Uma função por
runtime, aplicada onde o valor SAI".

🔴 ESCOPO NEGATIVO (D2 da mesma ADR) — esta função é de EMISSÃO, nunca de travessia.
Não aplicar a um caminho antes de ``open()``/``os.stat``/``os.listdir``: normalizar
cegamente quebra UNC (``\\\\server\\share``) e o prefixo de caminho longo (``\\\\?\\``), que
exige backslash EXCLUSIVAMENTE — nem o Windows converte. O modo de falha seria
intermitente, que é o pior. ``os.path.join``/``os.path.relpath`` continuam sendo o certo
para caminho que vai a uma syscall.

Aplicar apenas às três categorias que a D1 nomeia, cujo consumidor NÃO é o filesystem:

1. texto de relatório / saída para humano   (display_path, mensagens)
2. chave de dicionário ou identificador     (provenance_key, node ID, edge.to)
3. string de comando interpretada por shell (command de hook, gate de wave)

Substituição INCONDICIONAL de ``\\`` por ``/``, não ``posixpath.normpath`` nem uma troca de
``os.sep``: em POSIX ``os.sep`` é ``/`` e uma troca por separador seria no-op estrutural, o
que não normalizaria um valor sujo herdado de um commit feito no Windows — exatamente o
defeito que esta função existe para curar
(docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md). É a mesma
semântica já pinada em Go (``internal/validator/validator.go:normalizeRefSeparator``) e
Node (``npm/src/lib/pathfmt.js``); D4 exige que os 3 sejam byte-idênticos, então esta
escolha não é livre — é herdada.

Custo aceito e nomeado: em POSIX um ``\\`` literal dentro de um nome de arquivo é DADO, e
esta função o converteria. Vale só para o texto emitido (display/chave), nunca para o
caminho realmente aberto, que não passa por aqui.

NÃO aplicar ao buffer inteiro de um arquivo — só ao valor já extraído/derivado.

Este módulo é FOLHA de propósito: não importa nada de ``trackfw``. É o que permite que
``serve/``, ``commands/``, ``generators/`` e ``validator.py`` o importem sem risco de
import circular — a mesma razão pela qual ``integrations/manager.py`` não pôde importar
``_tildeify`` de ``commands/update_harness.py``.
"""


def normalize_ref_separator(p: str) -> str:
    """Converte separador nativo do Windows para ``/`` num valor já extraído/derivado."""
    return p.replace("\\", "/")
