# Troubleshooting

## `session.error: INVALID_REQUEST`

Check the first envelope type, JSON payload shape, and feature-gated
operations. For example, `session.list_jobs` requires the `list_jobs`
feature to be negotiated.

## Missing feature

Both sides advertise features and the runtime uses the intersection.
Use `Client.HasFeature` after connect to confirm the effective set.
`provisioned_credentials` is advertised only when the server has a
credential provisioner configured.

## `PERMISSION_DENIED`

Lease patterns did not match the operation target. Validate with
`JobContext.ValidateLeaseOp` before invoking external tools or models.

## `BUDGET_EXHAUSTED`

The job consumed its `cost.budget`, or an upstream credential provider
reported budget exhaustion. Treat it as a terminal policy boundary
unless the agent can continue without the costly operation.

## WebSocket connect failures

Confirm the handler path, scheme, and any auth verifier. WebSocket
URLs passed to `transport.DialWebSocket` must point at the ARCP
upgrade endpoint.
