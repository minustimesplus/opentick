# Testing

## Backend

- Always via mage: `mage test:web`, `mage test:feature`, `mage test:filter <go-test-filter>`. Plain `go test` does not work.
- `mage test:filter` runs most packages with `-short` but re-runs `pkg/webtests` without it, so a filter naming a web test actually executes it.
- Save output to a file and read the file: `mage test:filter Foo 2>&1 | tee /tmp/out.log`. Tests are expensive; never re-run one just to grep differently.
- Fixtures live in `pkg/db/fixtures/`. Use them instead of inventing data.
- Every permission path needs a positive and a negative test: the allowed user succeeds, an unrelated user is denied.

## Frontend

- Unit tests for pure logic in `web/src/lib/` use Vitest: put them next to the code as `*.test.ts` and run `npm test` in `web/`. Components have no tests; type-check with `npm run typecheck` in `web/` and verify behaviour in the running app.
