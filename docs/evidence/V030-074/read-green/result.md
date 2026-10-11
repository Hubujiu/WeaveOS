# Read-side GREEN

Fixed Go1.27.2 with PostgreSQL18.6 hot35/auth_app and Redis8.2.10. go test -race -count=1 -json ./cmd/bff -run ^TestRoot(StructureCopy|TemplateExport|TemplatePreflight), exit0. Single-connection plan/preflight, no persistent writes, current source/create authority and source/policy CAS pass; original export/preflight regression also passes. HTTP and copy execution not implemented.
