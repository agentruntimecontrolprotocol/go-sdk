// Package auth implements the ARCP authentication schemes defined in
// RFC §8.2: bearer, signed_jwt, and none (anonymous, gated on the
// `anonymous` capability). The mtls and oauth2 schemes are deferred
// to v0.2 and currently return arcp.ErrUnimplemented.
package auth
