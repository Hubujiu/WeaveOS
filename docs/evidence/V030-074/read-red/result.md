# Read-side RED

go test -race -count=1 -json ./cmd/bff -run ^TestRootStructureCopy, Go1.27.2, PostgreSQL18.6 hot35 with auth_app, Redis8.2.10. Exit1. All three tests reach target behavior: valid plan and CAS setup rejected by stub; forbidden actor returns wrong error. Earlier environment-only failure retained separately and is not RED.
