/**
 * Fractional indexing sort keys for docs/collections ordering.
 *
 * TypeScript port of the Go implementation at server/internal/ordering/fractional.go,
 * which is itself a port of the dgreensp fractional-indexing library
 * (https://github.com/rocicorp/fractional-indexing) restricted to lowercase a-z.
 *
 * Both implementations MUST produce identical output for the same inputs.
 * Compliance is enforced by shared test vectors at
 * server/internal/ordering/testdata/vectors.json.
 */

const DIGITS = 'abcdefghijklmnopqrstuvwxyz'

function getIntegerLength(head: string): number {
  const code = head.charCodeAt(0)
  if (code >= 97 && code <= 122) {
    // 'a' = 97
    return code - 97 + 2
  }
  throw new Error(`ordering: invalid head character '${head}'`)
}

function getIntegerPart(key: string): string {
  if (key.length === 0) throw new Error('ordering: invalid order key')
  const length = getIntegerLength(key[0])
  if (length > key.length) throw new Error('ordering: key too short for its head character')
  return key.slice(0, length)
}

function validateOrderKey(key: string): void {
  if (key.length === 0) throw new Error('ordering: invalid order key')
  for (let i = 0; i < key.length; i++) {
    const c = key.charCodeAt(i)
    if (c < 97 || c > 122) throw new Error('ordering: key contains characters outside [a-z]')
  }
  const intPart = getIntegerPart(key)
  const frac = key.slice(intPart.length)
  if (frac.length > 0 && frac[frac.length - 1] === DIGITS[0]) {
    throw new Error("ordering: key has trailing 'a' in fractional part")
  }
}

function validateInteger(intPart: string): void {
  if (intPart.length === 0) throw new Error('ordering: invalid order key')
  const expected = getIntegerLength(intPart[0])
  if (intPart.length !== expected) throw new Error('ordering: invalid order key')
}

function incrementInteger(x: string): string {
  validateInteger(x)
  const head = x[0]
  const digs = x.slice(1).split('')

  let carry = true
  for (let i = digs.length - 1; carry && i >= 0; i--) {
    const d = DIGITS.indexOf(digs[i]) + 1
    if (d === DIGITS.length) {
      digs[i] = DIGITS[0]
    } else {
      digs[i] = DIGITS[d]
      carry = false
    }
  }

  if (carry) {
    if (head === 'z') throw new Error('ordering: cannot increment past maximum integer')
    const h = String.fromCharCode(head.charCodeAt(0) + 1)
    return h + digs.join('') + DIGITS[0]
  }
  return head + digs.join('')
}

function decrementInteger(x: string): string {
  validateInteger(x)
  const head = x[0]
  const digs = x.slice(1).split('')

  let borrow = true
  for (let i = digs.length - 1; borrow && i >= 0; i--) {
    const d = DIGITS.indexOf(digs[i]) - 1
    if (d === -1) {
      digs[i] = DIGITS[DIGITS.length - 1]
    } else {
      digs[i] = DIGITS[d]
      borrow = false
    }
  }

  if (borrow) {
    if (head === 'a') throw new Error('ordering: cannot decrement past minimum integer')
    const h = String.fromCharCode(head.charCodeAt(0) - 1)
    return h + digs.slice(0, -1).join('')
  }
  return head + digs.join('')
}

function charAt(s: string, i: number, defaultVal: string): string {
  return i < s.length ? s[i] : defaultVal
}

function fractionalMidpoint(a: string, b: string): string {
  const zero = DIGITS[0]

  if (b !== '') {
    let n = 0
    while (n < b.length && charAt(a, n, zero) === b[n]) {
      n++
    }
    if (n > 0) {
      return b.slice(0, n) + fractionalMidpoint(a.length > n ? a.slice(n) : '', b.slice(n))
    }
  }

  const digitA = a.length > 0 ? DIGITS.indexOf(a[0]) : 0
  const digitB = b !== '' ? DIGITS.indexOf(b[0]) : DIGITS.length

  if (digitB - digitA > 1) {
    const mid = Math.round((digitA + digitB) / 2)
    return DIGITS[mid]
  }

  if (b !== '' && b.length > 1) {
    return b[0]
  }

  return DIGITS[digitA] + fractionalMidpoint(a.length > 1 ? a.slice(1) : '', '')
}

function generateBefore(upper: string): string {
  const ib = getIntegerPart(upper)
  const fb = upper.slice(ib.length)

  if (ib === 'aa') {
    return ib + fractionalMidpoint('', fb)
  }
  if (ib < upper) {
    return ib
  }
  return decrementInteger(ib)
}

function generateAfter(lower: string): string {
  const ia = getIntegerPart(lower)
  const fa = lower.slice(ia.length)

  try {
    return incrementInteger(ia)
  } catch {
    return ia + fractionalMidpoint(fa, '')
  }
}

function generateBetween(lower: string, upper: string): string {
  const ia = getIntegerPart(lower)
  const fa = lower.slice(ia.length)
  const ib = getIntegerPart(upper)
  const fb = upper.slice(ib.length)

  if (ia === ib) {
    return ia + fractionalMidpoint(fa, fb)
  }

  const inc = incrementInteger(ia)
  if (inc < upper) {
    return inc
  }

  return ia + fractionalMidpoint(fa, '')
}

/**
 * Returns a sort key k such that lower < k < upper lexicographically.
 *
 * Either bound may be empty, meaning "no bound on that side":
 *   - between("", "")  → a default starting key ("an")
 *   - between("", "x") → a key before x
 *   - between("x", "") → a key after x
 *
 * @throws if lower >= upper when both non-empty, or if either key is invalid.
 */
export function between(lower: string, upper: string): string {
  if (lower !== '') validateOrderKey(lower)
  if (upper !== '') validateOrderKey(upper)
  if (lower !== '' && upper !== '' && lower >= upper) {
    throw new Error('ordering: lower >= upper')
  }

  if (lower === '' && upper === '') return 'an'
  if (lower === '') return generateBefore(upper)
  if (upper === '') return generateAfter(lower)
  return generateBetween(lower, upper)
}
