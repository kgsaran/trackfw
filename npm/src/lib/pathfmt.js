'use strict'

/**
 * pathfmt — ponto único de normalização de separador do CLI Node
 * (ADR-2026-09-04 "Separador POSIX nos artefatos autorados cujo consumidor não é o
 * sistema de arquivos", D3: "Não espalhar `strings.ReplaceAll` pelos chamadores. Uma
 * função por runtime, aplicada onde o valor SAI").
 *
 * 🔴 ESCOPO NEGATIVO (D2 da mesma ADR) — esta função é de EMISSÃO, nunca de travessia.
 * Não aplicar a um caminho antes de `fs.readFileSync`/`fs.statSync`/`fs.writeFileSync`:
 * normalizar cegamente quebra UNC (`\\server\share`) e o prefixo de caminho longo
 * (`\\?\`), que exige backslash EXCLUSIVAMENTE — nem o Windows converte. O modo de falha
 * seria intermitente, que é o pior. `path.join`/`path.relative` continuam sendo o certo
 * para caminho que vai a uma syscall.
 *
 * Aplicar apenas às três categorias que a D1 nomeia, cujo consumidor NÃO é o filesystem:
 *   1. texto de relatório / saída para humano   (displayPath, mensagens)
 *   2. chave de dicionário ou identificador     (provenanceKey, node ID, edge.to)
 *   3. string de comando interpretada por shell (command de hook, gate de wave)
 *
 * Substituição INCONDICIONAL de "\" por "/", não `path.posix.normalize` nem uma troca de
 * `path.sep`: em POSIX `path.sep` é "/" e uma troca por separador seria no-op estrutural,
 * o que não normalizaria um valor sujo herdado de um commit feito no Windows — exatamente
 * o defeito que esta função existe para curar
 * (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md). É a mesma
 * semântica já pinada em Go (`internal/validator/validator.go:normalizeRefSeparator`) e
 * Python (`pypi/trackfw/pathfmt.py:normalize_ref_separator`); D4 exige que os 3 sejam
 * byte-idênticos, então esta escolha não é livre — é herdada.
 *
 * Custo aceito e nomeado: em POSIX um "\" literal dentro de um nome de arquivo é DADO, e
 * esta função o converteria. Vale só para o texto emitido (display/chave), nunca para o
 * caminho realmente aberto, que não passa por aqui.
 *
 * NÃO aplicar ao buffer inteiro de um arquivo — só ao valor já extraído/derivado.
 *
 * @param {string} p valor já extraído ou derivado (nunca o buffer de um arquivo)
 * @returns {string}
 */
function normalizeRefSeparator(p) {
  return p.replace(/\\/g, '/')
}

module.exports = { normalizeRefSeparator }
