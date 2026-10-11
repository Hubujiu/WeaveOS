# Recovery and concurrency acceptance

Real PG COMMIT reply loss gives HTTP503 original operation only, no Location or Cookies; original receipt remains durable. Confirmed-commit Session revocation preserves201 and clears both Cookies. Two simultaneous same-key calls converge to one result; different keys with same target CAS admit one copy. No duplicate table/audit/receipt. Fixed Go1.27.2 -race PG18.6/auth_app and Redis8.2.10, exit0.
