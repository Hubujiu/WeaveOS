# SelectCopyManifest GREEN

Command: `go test -race -count=1 -json ./internal/apptemplates` from services/bff, fixed Go 1.27.2.

Exit: 0. Test events: {'pass': 77}. Includes existing template regression and unchanged independently written selection tests. Source patch and preceding RED snapshots retained. No database/API completion implied.
