# B5a root repair: actual cost observations

Same isolated PG18.6, actual auth_app, direct application method. Statement
counts exclude BEGIN/COMMIT/ROLLBACK and SET ROLE. List has five samples;
members three. Latency below is the sample median in milliseconds.

| Method | Size | Before SQL | After SQL | Before ms | After ms |
|---|---:|---:|---:|---:|---:|
| list | 10 | 32 | 2 | 2.020 | 0.835 |
| list | 100 | 302 | 2 | 31.484 | 1.223 |
| list | 1000 | 3002 | 2 | 238.995 | 3.897 |
| members.replace | 10 | 37 | 19 | 7.757 | 5.635 |
| members.replace | 100 | 217 | 19 | 28.881 | 7.612 |
| members.replace | 1000 | 2017 | 19 | 161.214 | 18.174 |

These observations have the limits recorded in repair-cost-observations.json.
List fixtures exercise owner filtering at all scales; independent real SQL
tests cover membership, enabled groups, complete grant tuples, catalogue/menu
existence, caller flag forgery, revocation and a concurrent RR snapshot.
Fixed query counts preserve complete results, ordering and empty arrays.

Grant replacement uses 20 statements and one zipped-tuple validation, for
empty, single and 1000 duplicated submitted grants. B5a permits one actual
workspace resource per app, so unique valid G cannot reach 1000. No resource
registry extension was invented. Mixed valid/unknown sets reject atomically.

List transfers O(R) returned applications, with SQL join/filter/sort cost
dependent on database cardinalities and plan. Normalization is O(M + U log U)
or O(G + V log V), retaining O(M + U) or O(G + V) input/unique memory. Bulk
existence/delete/insert work still scales with normalized/existing rows; only
network round trips are fixed. No permission cache or pagination was added.
