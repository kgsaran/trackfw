'use strict'

// maxSlugLength é o tamanho máximo aceito para um slug gerado.
const MAX_SLUG_LENGTH = 40

// collapseDashes colapsa sequências de '-' em um único '-'.
function collapseDashes(value) {
  let out = ''
  let previousDash = false
  for (const char of value) {
    if (char === '-') {
      if (previousDash) continue
      previousDash = true
    } else {
      previousDash = false
    }
    out += char
  }
  return out
}

// slugify converte input em um slug normalizado que casa com
// ^[a-z0-9]+(-[a-z0-9]+)*$.
//
// Passos, na ordem exata (espelham internal/identity/slug.go):
//   1. Normalização Unicode NFD + remoção de marcas diacríticas (\p{Mn}).
//   2. Lowercase.
//   3. Espaços (incluindo sequências) e underscores viram '-'.
//   4. Qualquer caractere fora de [a-z0-9-] é descartado.
//   5. Sequências de '-' colapsam em um único '-'.
//   6. '-' nas bordas (início/fim) são removidos.
//   7. Resultado vazio é erro.
//   8. Resultado com mais de 40 caracteres é erro.
//
// slugify nunca "corrige" silenciosamente uma entrada degenerada — sempre
// lança erro em vez de adivinhar.
function slugify(input) {
  const folded = String(input).normalize('NFD').replace(/\p{Mn}/gu, '')
  const lowered = folded.toLowerCase()
  const replaced = lowered.replace(/[ _]/g, '-')

  let filtered = ''
  for (const char of replaced) {
    if ((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char === '-') {
      filtered += char
    }
  }

  const collapsed = collapseDashes(filtered)
  const trimmed = collapsed.replace(/^-+/, '').replace(/-+$/, '')

  if (trimmed === '') {
    throw new Error(`identity: slug vazio para "${input}"`)
  }
  if (trimmed.length > MAX_SLUG_LENGTH) {
    throw new Error(`identity: slug "${trimmed}" excede o tamanho maximo de ${MAX_SLUG_LENGTH} caracteres (tem ${trimmed.length})`)
  }

  return trimmed
}

module.exports = { slugify, MAX_SLUG_LENGTH }
