# Package `messages`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/messages`

`messages` contains typed payload structs and wire constants for ARCP
envelopes.

Use it when you need direct payload construction, raw event decoding,
or protocol-level tests. Most application code should go through
`client` and `server`.

Notable groups:

| Group | Examples |
| --- | --- |
| Session | `SessionHello`, `SessionWelcome`, `SessionJobs`. |
| Jobs | `JobSubmit`, `JobAccepted`, `JobResult`, `JobError`. |
| Events | `StatusBody`, `ToolCallBody`, `ProgressBody`, `ResultChunkBody`. |
| Credentials | `Credential`, `CredentialConstraints`. |
