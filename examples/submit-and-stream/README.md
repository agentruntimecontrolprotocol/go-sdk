# submit-and-stream

Spec §7.1, §8.2. The simplest happy path: submit a job, range over
its event channel, print the terminal result.

```sh
go run ./examples/submit-and-stream/server &
go run ./examples/submit-and-stream/client
```
