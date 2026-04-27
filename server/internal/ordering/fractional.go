// Package ordering provides fractional-indexing sort keys for ordered collections.
//
// The algorithm is a Go port of the dgreensp fractional-indexing library
// (https://github.com/rocicorp/fractional-indexing), restricted to lowercase
// alphabet [a-z] (26 characters).
//
// Sort keys have an integer part (a head character encoding magnitude plus
// base-26 digits) followed by an optional fractional part for arbitrary
// precision between consecutive integers. Keys compare correctly with plain
// lexicographic string comparison.
package ordering

import (
	"errors"
	"strings"
)

// digits is the ordered character set used for both the integer body and
// fractional part of sort keys. Using lowercase a-z gives base-26 arithmetic.
const digits = "abcdefghijklmnopqrstuvwxyz"

// Sentinel errors returned by Between.
var (
	ErrLowerGEUpper    = errors.New("ordering: lower >= upper")
	ErrInvalidChar     = errors.New("ordering: key contains characters outside [a-z]")
	ErrInvalidKey      = errors.New("ordering: invalid order key")
	ErrTrailingZero    = errors.New("ordering: key has trailing 'a' in fractional part")
	ErrKeyTooShort     = errors.New("ordering: key too short for its head character")
	ErrCannotIncrement = errors.New("ordering: cannot increment past maximum integer")
	ErrCannotDecrement = errors.New("ordering: cannot decrement past minimum integer")
)

// Between returns a sort key k such that lower < k < upper lexicographically.
//
// Either bound may be empty, meaning "no bound on that side":
//   - Between("", "")  — returns a default starting key ("an")
//   - Between("", "x") — returns a key before x
//   - Between("x", "") — returns a key after x
//
// Both lower and upper (when non-empty) must be valid order keys containing
// only characters in [a-z]. The function returns an error (never panics) if
// lower >= upper when both are non-empty, or if either key is structurally
// invalid.
func Between(lower, upper string) (string, error) {
	if lower != "" {
		if err := validateOrderKey(lower); err != nil {
			return "", err
		}
	}
	if upper != "" {
		if err := validateOrderKey(upper); err != nil {
			return "", err
		}
	}
	if lower != "" && upper != "" && lower >= upper {
		return "", ErrLowerGEUpper
	}

	switch {
	case lower == "" && upper == "":
		return "an", nil

	case lower == "":
		return generateBefore(upper)

	case upper == "":
		return generateAfter(lower)

	default:
		return generateBetween(lower, upper)
	}
}

// generateBefore returns a key before upper (lower is empty/null).
func generateBefore(upper string) (string, error) {
	ib, err := getIntegerPart(upper)
	if err != nil {
		return "", err
	}
	fb := upper[len(ib):]

	// If this is the smallest possible integer "aa", we cannot decrement
	// further. Instead, extend fractionally below.
	if ib == "aa" {
		return ib + fractionalMidpoint("", fb), nil
	}

	// If the integer part alone is less than the full key, returning the
	// integer part gives us a valid key strictly less than upper.
	if ib < upper {
		return ib, nil
	}

	// Otherwise decrement the integer.
	res, err := decrementInteger(ib)
	if err != nil {
		return "", err
	}
	return res, nil
}

// generateAfter returns a key after lower (upper is empty/null).
func generateAfter(lower string) (string, error) {
	ia, err := getIntegerPart(lower)
	if err != nil {
		return "", err
	}
	fa := lower[len(ia):]

	inc, err := incrementInteger(ia)
	if err != nil {
		// Cannot increment — extend fractionally instead.
		return ia + fractionalMidpoint(fa, ""), nil
	}
	return inc, nil
}

// generateBetween returns a key between lower and upper (both non-empty).
func generateBetween(lower, upper string) (string, error) {
	ia, err := getIntegerPart(lower)
	if err != nil {
		return "", err
	}
	fa := lower[len(ia):]

	ib, err := getIntegerPart(upper)
	if err != nil {
		return "", err
	}
	fb := upper[len(ib):]

	// Same integer part — use fractional midpoint.
	if ia == ib {
		return ia + fractionalMidpoint(fa, fb), nil
	}

	// Different integer parts — try incrementing the lower integer.
	inc, err := incrementInteger(ia)
	if err != nil {
		return "", err
	}
	if inc < upper {
		return inc, nil
	}

	// The incremented integer is not less than upper (they must be
	// consecutive integers). Extend the lower key fractionally.
	return ia + fractionalMidpoint(fa, ""), nil
}

// fractionalMidpoint returns a string m such that a < m < b lexicographically,
// using the digits character set. a may be empty; b may be empty (meaning
// unbounded above). Neither may have a trailing zero (digits[0] = 'a').
func fractionalMidpoint(a, b string) string {
	zero := digits[0]

	if b != "" {
		// Remove the longest common prefix. Pad a with zeros as we go.
		n := 0
		for n < len(b) && charAt(a, n, zero) == b[n] {
			n++
		}
		if n > 0 {
			return b[:n] + fractionalMidpoint(suffix(a, n), b[n:])
		}
	}

	// First digits (or lack of digit) are different.
	digitA := 0
	if len(a) > 0 {
		digitA = strings.IndexByte(digits, a[0])
	}
	digitB := len(digits)
	if b != "" {
		digitB = strings.IndexByte(digits, b[0])
	}

	if digitB-digitA > 1 {
		mid := (digitA + digitB + 1) / 2 // round half up, same as Math.round
		return string(digits[mid])
	}

	// First digits are consecutive.
	if b != "" && len(b) > 1 {
		return string(b[0])
	}

	// b is empty or a single digit. Recurse on the tail of a.
	return string(digits[digitA]) + fractionalMidpoint(suffix(a, 1), "")
}

// getIntegerLength returns the total length of the integer part (head + body
// digits) given the head character.
//
// Head 'a' → length 2, 'b' → 3, …, 'z' → 27.
func getIntegerLength(head byte) (int, error) {
	if head >= 'a' && head <= 'z' {
		return int(head-'a') + 2, nil
	}
	return 0, ErrInvalidKey
}

// getIntegerPart extracts the integer prefix from a valid order key.
func getIntegerPart(key string) (string, error) {
	if len(key) == 0 {
		return "", ErrInvalidKey
	}
	length, err := getIntegerLength(key[0])
	if err != nil {
		return "", err
	}
	if length > len(key) {
		return "", ErrKeyTooShort
	}
	return key[:length], nil
}

// validateOrderKey checks that key is a structurally valid order key:
// all characters in [a-z], valid integer part, no trailing zero in the
// fractional part.
func validateOrderKey(key string) error {
	if len(key) == 0 {
		return ErrInvalidKey
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 'a' || key[i] > 'z' {
			return ErrInvalidChar
		}
	}
	intPart, err := getIntegerPart(key)
	if err != nil {
		return err
	}
	frac := key[len(intPart):]
	if len(frac) > 0 && frac[len(frac)-1] == digits[0] {
		return ErrTrailingZero
	}
	return nil
}

// validateInteger checks that an integer part has the correct length for its
// head character.
func validateInteger(intPart string) error {
	if len(intPart) == 0 {
		return ErrInvalidKey
	}
	expected, err := getIntegerLength(intPart[0])
	if err != nil {
		return err
	}
	if len(intPart) != expected {
		return ErrInvalidKey
	}
	return nil
}

// incrementInteger adds one to the integer part. Returns the next integer or
// an error if the maximum is reached (head 'z' with all digits maxed).
func incrementInteger(x string) (string, error) {
	if err := validateInteger(x); err != nil {
		return "", err
	}
	head := x[0]
	digs := []byte(x[1:])

	carry := true
	for i := len(digs) - 1; carry && i >= 0; i-- {
		d := strings.IndexByte(digits, digs[i]) + 1
		if d == len(digits) {
			digs[i] = digits[0]
		} else {
			digs[i] = digits[d]
			carry = false
		}
	}

	if carry {
		if head == 'z' {
			return "", ErrCannotIncrement
		}
		// Move to the next head character. The integer part grows by one
		// digit, so append a zero.
		h := head + 1
		return string(h) + string(digs) + string(digits[0]), nil
	}
	return string(head) + string(digs), nil
}

// decrementInteger subtracts one from the integer part. Returns the previous
// integer or an error if the minimum is reached (head 'a' with all digits at
// zero).
func decrementInteger(x string) (string, error) {
	if err := validateInteger(x); err != nil {
		return "", err
	}
	head := x[0]
	digs := []byte(x[1:])

	borrow := true
	for i := len(digs) - 1; borrow && i >= 0; i-- {
		d := strings.IndexByte(digits, digs[i]) - 1
		if d == -1 {
			digs[i] = digits[len(digits)-1]
		} else {
			digs[i] = digits[d]
			borrow = false
		}
	}

	if borrow {
		if head == 'a' {
			return "", ErrCannotDecrement
		}
		// Move to the previous head character. The integer part shrinks by
		// one digit, so drop the last one.
		h := head - 1
		return string(h) + string(digs[:len(digs)-1]), nil
	}
	return string(head) + string(digs), nil
}

// charAt returns the byte at position i in s, or defaultVal if i is out of
// bounds.
func charAt(s string, i int, defaultVal byte) byte {
	if i < len(s) {
		return s[i]
	}
	return defaultVal
}

// suffix returns s[n:] if n < len(s), or "" otherwise.
func suffix(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}
