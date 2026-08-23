# Contributing

Contributions are welcome — bug reports, fixes, and harness improvements
alike. Build and test locally before opening a pull request
(`go build ./... && go vet ./... && go test ./...`), keep changes
self-contained, and read the section below: it is the bar every
contribution is held to.

## Upstreaming from a private instance

A contribution is harness-worthy iff any deployer could have written it: mechanism or method that survives with your facts removed. Before opening a PR, de-instance it — measured numbers become config defaults with the rationale restated, never the measurement quoted; incident stories become the invariant the incident taught; persona and crew names become roles; operator names, hostnames, and machine paths go; private-tracker ids become a design-doc section reference, or drop. Cost, plan, spend, quota, and utilization figures never cross, in any form. Credential facts never cross — not values, and not the map: no storage names or locations, no auth topology, no account or plan identity. When a change needs your instance's numbers to justify itself, keep the numbers in your private tracker and let the PR carry the qualitative rationale. When in doubt, leave it out: a fact kept private can be published later; the reverse is a history purge.
