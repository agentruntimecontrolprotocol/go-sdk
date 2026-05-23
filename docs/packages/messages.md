# Package `messages`

Import path: `github.com/agentruntimecontrolprotocol/go-sdk/messages`

`messages` contains typed payload structs, wire-type tokens, and
enum constants for every ARCP envelope.

Use it when you need direct payload construction, raw event decoding,
or protocol-level tests. Most application code should go through
`client` and `server`.

## Wire-type tokens

`messages.Type<Name>` strings name the envelope `type` field. The
core tokens are `TypeSessionHello`, `TypeSessionWelcome`,
`TypeSessionError`, `TypeSessionBye`, `TypeSessionPing`,
`TypeSessionPong`, `TypeSessionAck`, `TypeSessionListJobs`,
`TypeSessionJobs`, `TypeJobSubmit`, `TypeJobAccepted`, `TypeJobEvent`,
`TypeJobResult`, `TypeJobError`, `TypeJobCancel`, `TypeJobSubscribe`,
`TypeJobSubscribed`, `TypeJobUnsubscribe`.

## Event kinds

`messages.Kind<Name>` strings name the `JobEvent.Kind` field:
`KindLog`, `KindThought`, `KindToolCall`, `KindToolResult`,
`KindStatus`, `KindMetric`, `KindArtifactRef`, `KindDelegate`,
`KindProgress`, `KindResultChunk`.

## Status enum

`messages.Status<Name>` names a `JobInfo.Status` value: `StatusPending`,
`StatusRunning`, `StatusSuccess`, `StatusError`, `StatusCancelled`,
`StatusTimedOut`. The reserved status-event phase
`messages.PhaseCredentialRotated` ("credential_rotated") fires when
`JobContext.RotateCredential` succeeds.

## Payload groups

| Group | Structs |
| --- | --- |
| Session | `SessionHello`, `ResumeRequest`, `SessionWelcome`, `SessionError`, `SessionBye`, `SessionPing`, `SessionPong`, `SessionAck`, `SessionListJobs`, `SessionJobs`, `ClientInfo`, `RuntimeInfo`, `AuthInfo`, `HelloCapabilities`, `WelcomeCapabilities`, `AgentEntry`, `ListJobsFilter`, `JobInfo`. |
| Jobs | `JobSubmit`, `JobAccepted`, `JobResult`, `JobError`, `JobCancel`, `JobSubscribe`, `JobSubscribed`, `JobUnsubscribe`, `LeaseConstraints`. |
| Event bodies | `LogBody`, `ThoughtBody`, `ToolCallBody`, `ToolResultBody`, `ToolError`, `StatusBody`, `MetricBody`, `ArtifactRefBody`, `DelegateBody`, `ProgressBody`, `ResultChunkBody`. |
| Credentials | `Credential`, `CredentialConstraints`. |
| Agents | `AgentRef`, `ParseAgentRef`. |

## Helpers

- `DecodeEventBody(*JobEvent) (any, error)` — unmarshal `Body` into
  the kind-specific struct; unknown kinds return the raw bytes.
- `NewEventBody(v any) (json.RawMessage, error)` — marshal a typed
  body for `JobContext.emitEvent` callers.
- `ParseAgentRef(s)` — parse `"name"` or `"name@version"`.

Each struct implements `ARCPType() string` (the `messages.MessageType`
interface in the root package), and the package's `init()` registers
every payload so `arcp.NewPayloadForType(typeStr)` returns a zero
value of the right shape.
