# provisioned-credentials

Spec §9.7 and §9.8. The server configures an in-memory credential
provisioner. The client requests `model.use` and `cost.budget`; the
accepted job includes a bearer credential constrained to that lease,
and the runtime revokes it when the job terminates.
