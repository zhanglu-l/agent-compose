# SQLite migrations

Migration files are embedded into the agent-compose binary and applied by the
`configstore` package before any store is used.

- Name files `NNNNNN_description.sql` with a unique, increasing six-digit
  version.
- Migrations are forward-only and must run inside a SQLite transaction.
- Never edit, rename, reorder, or remove a released migration. The runner
  stores and verifies each file's SHA-256 checksum.
- Add schema changes only as a new migration; do not add startup-time schema
  inspection or `CREATE TABLE` statements to store implementations.
- Keep connection PRAGMAs and operations such as `VACUUM` out of migrations.
