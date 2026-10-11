# Authorization order GREEN

Scope manager/catalog/create checks now precede target validation without taking app locks; all checks repeat under final sorted app gates. Original receipt replay remains first. All TestRootStructureCopy real storage/HTTP/receipt/Down/fault/concurrency tests pass under race, exit0.
