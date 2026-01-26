package server

// ServerVersion is a monotonically increasing version number for lsvd-server.
// IMPORTANT: This MUST be incremented whenever any changes are made to the
// lsvd-server code that require running instances to be upgraded. The main
// miren server will check this version and trigger a graceful restart if
// the running lsvd-server has an older version.
const ServerVersion uint64 = 1
