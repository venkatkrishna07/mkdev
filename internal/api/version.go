package api

// APIVersion is the protocol version negotiated between daemon and clients.
// Bump on breaking changes; clients refuse to connect on mismatch.
const APIVersion = "v1"
