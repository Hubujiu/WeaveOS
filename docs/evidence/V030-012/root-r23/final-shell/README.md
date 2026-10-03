# Root R23 nine-case Shell run

The final route wiring with reviewed R22 (`d36a039`) and R23 focus/StrictMode fixes passed 8/9 in Chromium. The only remaining failure is Root's create/edit test checking for the second record detail GET immediately after confirming the PATCH; the required runtime refresh and detail reread are asynchronous. All route access, create-only, error, candidate search, stale-query, dirty-dismissal/focus, design denial and return-origin cases passed. The GET-count assertion and full nine-case pass remain pending Root's test-owner correction. Exit code is 1.
