The builder's tools: read how the office is written, and draft a change to it, which a person decides.

`office_files`, `office_read`, `office_write`, `office_remove` and `office_check` read the office and work on the worker's draft; `office_put` puts the draft to be decided and waits; `manual` reads a page of the manual. Nothing changes in the office until a person says yes. A worker is a builder by naming this file.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

```yaml
tools_file: [./.bulldog/tools/build/build.yaml]
```

## Changed

- 0.2.0: moved here from `.bulldog/build.yaml`.
