// Package messages defines the typed payload structs for every ARCP
// wire type. Each file registers its concrete payloads against the
// root arcp envelope registry in an init() block; importing this
// package transitively registers every supported type.
//
// The init-time registration is the only sanctioned package-level
// side effect across the SDK. The wire-type string is owned by the
// payload struct's ARCPType method, so registration sits next to the
// struct that owns the key — the same pattern used by image and
// database/sql for format/driver registration.
package messages
