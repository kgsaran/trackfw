'use strict'

/**
 * localDateISO — retorna a data LOCAL atual no formato YYYY-MM-DD.
 *
 * Usa getFullYear/getMonth/getDate (métodos de hora local) em vez de
 * toISOString() (que retorna sempre UTC). Isso garante que uma REQ criada às
 * 22h no Brasil seja datada do dia correto, independentemente do fuso da
 * máquina — comportamento idêntico ao de Go (time.Now().Format("2006-01-02"))
 * e Python (date.today().isoformat()).
 *
 * @returns {string} Data no formato "YYYY-MM-DD"
 */
function localDateISO() {
  const d = new Date()
  return [
    d.getFullYear(),
    String(d.getMonth() + 1).padStart(2, '0'),
    String(d.getDate()).padStart(2, '0'),
  ].join('-')
}

module.exports = { localDateISO }
