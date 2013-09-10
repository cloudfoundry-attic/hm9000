# Health Manager 9000

HM 9000 is a rewrite of CloudFoundry's Health Manager.  HM 9000 is written in Golang and has a more modular architecture compard to the original ruby implementation.

As a result there are several Go Packages in this repository, each with a comprehensive set of unit tests.  What follows is a detailed breakdown:

## HM9000 components

### WIP: `hm9000`: the top level

WIP: Eventually this will house the `hm9000` CLI. This executable will wrap all the subcomponents, enacpsulate any common operations (e.g. loading configuration data from YML files), and make it possible to launch individual components a la (e.g.):

```bash
hm9000 actual_state_listener -config=/path/to/config.yml
```

### `actual_state_listener`

The `actual_state_listener` provides a simple listener daemon that monitors the `NATS` stream for app heartbeats.  It generates an entry in the `store` for each heartbeating app under `/actual/INSTANCE_GUID`.

It also maintains a `FreshnessTimestamp`  under `/actual-fresh` to allow other components to know whether or not they can trust the information under `/actual`

#### WIP:`desired_state_listener`

WIP: The `desired_state_listener` provides a simple daemon that polls the cloud controller for the desired state.  It generates an entry in the `store` for each desired app instance.

### WIP:`analyzer`

WIP: The `analyzer` runs periodically and uses the `outbox` to put `start` and `stop` messages in the queue.

### WIP:`sender`

WIP: The `sender` runs periodically and uses the `outbox` to pull messages off the queue and send them over `NATS`.  The `sender` is responsible for throttling the rate at which messages are sent over NATS.

### WIP:`api`

WIP: The `api` is a simple HTTP server that provides access to information about the actual state.  It uses the high availability store to fulfill these requests.

## Support Packages

### `config`

`config` defines a number of constants.  Eventually these will be configurable and the `config` package will be a common module that can parse an input YML file.

### `helpers`

`helpers` contains a number of useful support utilities, including:
	
- `logger`: provides a (sys)logger
- `time_provider`: provides a `TimeProvider`.  Useful for injecting time dependencies in tests.

Many more advanced test helpers are in the `MCAT` subpackages.  See below.

### `models`

`models` encapsulates the various JSON structs that are sent/received over NATS/HTTP.

### WIP:`outbox`

WIP: `outboux` is a library used by the analyzer and scheduler to add/remove messages from the message queue.  The message queue is stored in the high availability `store`.  The `outbox` guarantees that a message is only added once to the queue (i.e. if a `start` message for a given app guid, version guid, and index has not been sent yet, the `outbox` will prevent another identical `start` message from being inserted in the queue).

### `store`

The `store` is an generalized client for connecting to a Zookeeper/ETCD-like high availability store.  Components that must read or write to the high availability store use this library.

## Test Support Packages (under test_helpers)

`test_helpers` contains a (large) number of test support packages.  These range from simple fakes to comprehensive libraries used for faking out other CloudFoundry components (e.g. heartbeating DEAs) in integration tests.

### Fakes

#### `fake_logger`

Provides a fake implementation of the `helpers/logger` interface

#### `fake_time_provider`

Provides a fake implementation of the `helpers/time_provider` interface.  Useful for injecting time dependency in test.

### Fixtures

#### `app`

`app` is a simple domain object that encapsulates a running CloudFoundry app.

The `app` package can be used to generate self-consistent data structures (heartbeats, desired state).  These data structures are then passed into the other test helpers to simulate a CloudFoundry eco-system.

Think of `app` as your source of fixture test data.  It's intended to be used in integration tests *and* unit tests.

Some brief documentation -- look at the code and tests for more:

```go
//get a new fixture app, this will generate appropriate
//random APP and VERSION GUIDs
app := NewApp()

//Get the desired state for the app.  This can be passed into
//the desired state server to simulate the APP's precense in 
//the CC's DB.  By default the app is staged and started, to change
//this, modify the return value.  Time is always injected.
desiredState := app.DesiredState(UPDATED_AT_TIMESTAMP)

//get an instance at index 0.  this getter will lazily create and memoize
//instances and populate them with an INSTANCE_GUID and the correct
//INDEX.
instance0 := app.GetInstance(0)

//fetch, for example, the exit message for the instance. Time is always injected
exitedMessage := instance0.DropletExited(DropletExitReasonCrashed, TIMESTAMP)

//generate a heartbeat for the app.  first argument is the # of instances,
// second is a timestamp denoting *when the app transitioned into its current state*  
//note that the INSTANCE_GUID associated with the instance at index 0 will
//match that provided by app.GetInstance(0)
app.Heartbeat(2, TIMESTAMP)
```

### Infrastructure Helpers

#### `message_publisher`

Provides a simple mechanism to publish actual_state related messages to the NATS bus.  Handles JSON encoding.

#### `start_stop_listener`

Listens on the NATS bus for `health.start` and `health.stop` messages.  It parses these messages and makes them available via a simple interface.  Useful for testing that messages are sent by the health manager appropriately.

#### `desired_state_server`

Brings up an in-process http server that mimics the CC's bulk endpoints (including authentication via NATS and pagination).

#### `nats_runner`

Brings up and manages the lifecycle of a live NATS server.  After bringing the server up it provides a fully configured cfmessagebus object that you can pass to your test subjects.

#### `etcd_runner`

Brings up and manages the lifecycle of a live ETCD server.

## The MCAT

The MCAT is comprised of two major integration test suites:

### WIP:The `MD` Test Suite

The `MD` test suite excercises the `HM9000` components through a series of integration-level tests.  The individual components are designed to be simple and have comprehensive unit test coverage.  However, it's crucial that we have comprehensive test coverage for the *interactions* between these components.  That's what the `MD` suite is for.

### WIP:The `PHD` Benchmark Suite

The `PHD` suite is a collection of benchmark tests.  This is a slow-running suite that is intended, primarily, to evaluate the performance of the various components (especially the high-availability store) under various loads.

