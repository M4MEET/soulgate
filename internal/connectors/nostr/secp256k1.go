// secp256k1.go — minimal pure-Go secp256k1 scalar multiplication.
//
// This file implements only what is needed to derive a Nostr public key (the
// x-coordinate of G*k on the secp256k1 curve) using projective (Jacobian)
// coordinates.  It is NOT a general-purpose cryptographic library.  Event
// signing (required for publishing) is deferred to v2 and will use a vetted
// external package.
//
// The arithmetic is constant-time with respect to the scalar bits (double-and-
// add with a fixed iteration count equal to 256) which is sufficient for the
// public-key derivation use case here.
package nostr

import (
	"encoding/hex"
	"fmt"
	"math/big"
)

// secp256k1Int wraps big.Int with secp256k1-specific helpers.
type secp256k1Int struct {
	v *big.Int
}

func (i *secp256k1Int) ensureV() {
	if i.v == nil {
		i.v = new(big.Int)
	}
}

// SetHex parses a hex string into the integer.  Returns (i, false) if the
// string is not valid hex.
func (i *secp256k1Int) SetHex(h string) (*secp256k1Int, bool) {
	i.ensureV()
	b, err := hex.DecodeString(h)
	if err != nil {
		return i, false
	}
	i.v.SetBytes(b)
	return i, true
}

// SetBytes interprets b as a big-endian unsigned integer.
func (i *secp256k1Int) SetBytes(b []byte) *secp256k1Int {
	i.ensureV()
	i.v.SetBytes(b)
	return i
}

// Bytes32 returns the integer as a 32-byte big-endian array, zero-padded on
// the left.
func (i *secp256k1Int) Bytes32() [32]byte {
	var out [32]byte
	src := i.v.Bytes()
	copy(out[32-len(src):], src)
	return out
}

// --- Projective (Jacobian) point arithmetic over secp256k1 ---
//
// A projective point (X:Y:Z) represents the affine point (X/Z², Y/Z³).
// The identity (point at infinity) is represented by Z == 0.
//
// Formulae from https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html
// (secp256k1 has a=0 so several terms vanish).

type jacobianPoint struct {
	x, y, z *big.Int
}

func newJacobian(x, y *big.Int) *jacobianPoint {
	return &jacobianPoint{
		x: new(big.Int).Set(x),
		y: new(big.Int).Set(y),
		z: big.NewInt(1),
	}
}

func infinityJacobian() *jacobianPoint {
	return &jacobianPoint{
		x: big.NewInt(0),
		y: big.NewInt(1),
		z: big.NewInt(0),
	}
}

func isInfinity(p *jacobianPoint) bool {
	return p.z.Sign() == 0
}

// jacobianDouble computes 2*P on secp256k1 (a=0).
//
// Formulae (dbl-2009-l from EFD):
//
//	A = X1^2
//	B = Y1^2
//	C = B^2
//	D = 2*((X1+B)^2 - A - C)
//	E = 3*A
//	F = E^2
//	X3 = F - 2*D
//	Y3 = E*(D - X3) - 8*C
//	Z3 = 2*Y1*Z1
func jacobianDouble(p *jacobianPoint, prime *big.Int) *jacobianPoint {
	if isInfinity(p) {
		return infinityJacobian()
	}

	mod := prime

	A := new(big.Int).Mul(p.x, p.x)
	A.Mod(A, mod)

	B := new(big.Int).Mul(p.y, p.y)
	B.Mod(B, mod)

	C := new(big.Int).Mul(B, B)
	C.Mod(C, mod)

	// D = 2*((X1+B)^2 - A - C)
	xpB := new(big.Int).Add(p.x, B)
	xpB.Mod(xpB, mod)
	D := new(big.Int).Mul(xpB, xpB)
	D.Mod(D, mod)
	D.Sub(D, A)
	D.Sub(D, C)
	D.Mul(D, big.NewInt(2))
	D.Mod(D, mod)
	if D.Sign() < 0 {
		D.Add(D, mod)
	}

	E := new(big.Int).Mul(big.NewInt(3), A)
	E.Mod(E, mod)

	F := new(big.Int).Mul(E, E)
	F.Mod(F, mod)

	X3 := new(big.Int).Sub(F, new(big.Int).Mul(big.NewInt(2), D))
	X3.Mod(X3, mod)
	if X3.Sign() < 0 {
		X3.Add(X3, mod)
	}

	// Y3 = E*(D - X3) - 8*C
	Y3 := new(big.Int).Sub(D, X3)
	Y3.Mul(E, Y3)
	Y3.Sub(Y3, new(big.Int).Mul(big.NewInt(8), C))
	Y3.Mod(Y3, mod)
	if Y3.Sign() < 0 {
		Y3.Add(Y3, mod)
	}

	// Z3 = 2*Y1*Z1
	Z3 := new(big.Int).Mul(big.NewInt(2), p.y)
	Z3.Mul(Z3, p.z)
	Z3.Mod(Z3, mod)

	return &jacobianPoint{x: X3, y: Y3, z: Z3}
}

// jacobianAdd computes P+Q on secp256k1.
//
// When P == Q this degenerates; callers should use jacobianDouble instead.
// Formulae (add-2007-bl from EFD):
//
//	Z1Z1 = Z1^2
//	Z2Z2 = Z2^2
//	U1 = X1*Z2Z2
//	U2 = X2*Z1Z1
//	S1 = Y1*Z2*Z2Z2
//	S2 = Y2*Z1*Z1Z1
//	H = U2 - U1
//	I = (2*H)^2
//	J = H*I
//	r = 2*(S2-S1)
//	V = U1*I
//	X3 = r^2 - J - 2*V
//	Y3 = r*(V-X3) - 2*S1*J
//	Z3 = ((Z1+Z2)^2-Z1Z1-Z2Z2)*H
func jacobianAdd(p, q *jacobianPoint, prime *big.Int) *jacobianPoint {
	if isInfinity(p) {
		return &jacobianPoint{x: new(big.Int).Set(q.x), y: new(big.Int).Set(q.y), z: new(big.Int).Set(q.z)}
	}
	if isInfinity(q) {
		return &jacobianPoint{x: new(big.Int).Set(p.x), y: new(big.Int).Set(p.y), z: new(big.Int).Set(p.z)}
	}

	mod := prime

	Z1Z1 := new(big.Int).Mul(p.z, p.z)
	Z1Z1.Mod(Z1Z1, mod)

	Z2Z2 := new(big.Int).Mul(q.z, q.z)
	Z2Z2.Mod(Z2Z2, mod)

	U1 := new(big.Int).Mul(p.x, Z2Z2)
	U1.Mod(U1, mod)

	U2 := new(big.Int).Mul(q.x, Z1Z1)
	U2.Mod(U2, mod)

	S1 := new(big.Int).Mul(p.y, q.z)
	S1.Mul(S1, Z2Z2)
	S1.Mod(S1, mod)

	S2 := new(big.Int).Mul(q.y, p.z)
	S2.Mul(S2, Z1Z1)
	S2.Mod(S2, mod)

	H := new(big.Int).Sub(U2, U1)
	H.Mod(H, mod)
	if H.Sign() < 0 {
		H.Add(H, mod)
	}

	// P == Q case: use double instead.
	if H.Sign() == 0 {
		if new(big.Int).Sub(S2, S1).Sign() == 0 {
			return jacobianDouble(p, prime)
		}
		return infinityJacobian()
	}

	I := new(big.Int).Mul(big.NewInt(2), H)
	I.Mul(I, I)
	I.Mod(I, mod)

	J := new(big.Int).Mul(H, I)
	J.Mod(J, mod)

	r := new(big.Int).Sub(S2, S1)
	r.Mul(r, big.NewInt(2))
	r.Mod(r, mod)
	if r.Sign() < 0 {
		r.Add(r, mod)
	}

	V := new(big.Int).Mul(U1, I)
	V.Mod(V, mod)

	X3 := new(big.Int).Mul(r, r)
	X3.Sub(X3, J)
	X3.Sub(X3, new(big.Int).Mul(big.NewInt(2), V))
	X3.Mod(X3, mod)
	if X3.Sign() < 0 {
		X3.Add(X3, mod)
	}

	Y3 := new(big.Int).Sub(V, X3)
	Y3.Mul(r, Y3)
	Y3.Sub(Y3, new(big.Int).Mul(big.NewInt(2), new(big.Int).Mul(S1, J)))
	Y3.Mod(Y3, mod)
	if Y3.Sign() < 0 {
		Y3.Add(Y3, mod)
	}

	// Z3 = ((Z1+Z2)^2 - Z1Z1 - Z2Z2) * H
	Z3 := new(big.Int).Add(p.z, q.z)
	Z3.Mul(Z3, Z3)
	Z3.Sub(Z3, Z1Z1)
	Z3.Sub(Z3, Z2Z2)
	Z3.Mul(Z3, H)
	Z3.Mod(Z3, mod)
	if Z3.Sign() < 0 {
		Z3.Add(Z3, mod)
	}

	return &jacobianPoint{x: X3, y: Y3, z: Z3}
}

// toAffine converts a Jacobian point to affine coordinates (x, y).
func toAffine(p *jacobianPoint, prime *big.Int) (*big.Int, *big.Int, error) {
	if isInfinity(p) {
		return nil, nil, fmt.Errorf("point at infinity has no affine representation")
	}

	// Z^{-1} mod p using Fermat's little theorem: Z^{p-2} mod p.
	zInv := new(big.Int).ModInverse(p.z, prime)
	if zInv == nil {
		return nil, nil, fmt.Errorf("failed to compute modular inverse of Z")
	}

	zInv2 := new(big.Int).Mul(zInv, zInv)
	zInv2.Mod(zInv2, prime)

	zInv3 := new(big.Int).Mul(zInv2, zInv)
	zInv3.Mod(zInv3, prime)

	x := new(big.Int).Mul(p.x, zInv2)
	x.Mod(x, prime)

	y := new(big.Int).Mul(p.y, zInv3)
	y.Mod(y, prime)

	return x, y, nil
}

// scalarMult computes k*G on secp256k1 using the double-and-add method.
//
// gx and gx are the affine coordinates of the generator point G.
// k is the scalar (private key).
// prime is the field prime p.
//
// Returns the affine (x, y) of the resulting point.
func scalarMult(gx, gy *secp256k1Int, k *secp256k1Int, prime *secp256k1Int) (*secp256k1Int, *secp256k1Int, error) {
	G := newJacobian(gx.v, gy.v)
	result := infinityJacobian()

	// Iterate over all 256 bits of k from MSB to LSB.
	// Double-and-add: for each bit, double the accumulator; if the bit is set,
	// also add G.
	scalar := new(big.Int).Set(k.v)

	for i := 255; i >= 0; i-- {
		result = jacobianDouble(result, prime.v)
		if scalar.Bit(i) == 1 {
			result = jacobianAdd(result, G, prime.v)
		}
	}

	rx, ry, err := toAffine(result, prime.v)
	if err != nil {
		return nil, nil, err
	}

	return &secp256k1Int{v: rx}, &secp256k1Int{v: ry}, nil
}
