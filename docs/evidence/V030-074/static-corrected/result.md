# Final static and response checks

Node24.14.0:580 Node tests pass without skips, typecheck/build/verify/tasks pass. Go1.27.2 vet/build/gofmt pass. Local-only OpenAPI3.2.1 lint valid with14 baseline warnings. Actual HTTP envelopes: plan1/preflight1/execute1/errors23; all independent Ajv schema checks pass. Final Gitleaks8.30.1 no leaks, raw logs scanned before lossless archiving. Original Redocly telemetry-blocked attempt retained separately; official installed CLI option disables telemetry, no remote upload used.
