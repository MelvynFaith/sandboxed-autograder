# Project documentation

| Path | Contents | Cadence |
|---|---|---|
| [`progress/`](progress/) | Weekly progress notes, `minggu-NN.md` | Every week |
| [`sprint/`](sprint/) | Sprint review and retrospective, `sprint-N.md` | End of each sprint |
| [`uat/`](uat/) | Lecturer workflow guide and UAT scenarios | When the UAT flow changes |

Start new notes by copying the `_template.md` in each folder.

The requirements specification (SRS) and the course rubric are maintained privately by the team and
are not published in this repository.

## API reference

[`openapi.yaml`](openapi.yaml) documents the scheduler's registered HTTP operations, including
request and response formats, authorization, role and object access, pagination, errors, health,
and the ticket-based submission WebSocket.

From the repository root, preview it locally with Redocly CLI:

```sh
npx @redocly/cli preview-docs docs/openapi.yaml
```

The router-coverage check compares every documented path/method to the Go router registrations:

```sh
cd scheduler
go test ./internal/api -run '^TestOpenAPICoversRegisteredRoutes$'
```
