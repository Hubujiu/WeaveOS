# Copy receipt schema RED

Real migrated hot35 restricted role rejects all four valid structure.copy receipts at ck_operation_kind. Invalid-shape tests also reach SQL but identify missing copy-specific check. Exit1. Commands: go test -race -count=1 -json ./cmd/bff -run ^TestRootStructureCopyReceiptSchema$. Fixed Go1.27.2/PG18.6/Redis8.2.10.
