# Health Manager 9000

HM 9000 is a rewrite of CloudFoundry's Health Manager.  HM 9000 is written in Golang and has a more modular architecture compard to the original ruby implementation.

As a result there are several Go Packages in this repository, each with a comprehensive set of unit tests.  What follows is a detailed breakdown:

## HM9000 components

### `hm9000`: the top level

WIP: Eventually this will house the `hm9000` CLI. This executable will wrap all the subcomponents, enacpsulate any common operations (e.g. loading configuration data from YML files), and make it possible to launch individual components a la (e.g.):

```bash
hm9000 actual_state_listener -config=/path/to/config.yml
```

### `actual_state_listener`

The `actual_state_listener` provides a simple listener daemon that monitors the `NATS` stream for app heartbeats.  It generates an entry in the `store` for each heartbeating app under.

#### `desired_state_listener`

WIP: The `desired_state_listener` provides a simple daemon that polls the cloud controller for the desired state.  It generates an entry in the `store` for each desired app instance.

### `analyzer`

WIP: The `analyzer` runs periodically and uses the `outbox` to put `start` and `stop` messages in the queue.

### `sender`

WIP: The `sender` runs periodically and uses the `outbox` to pull messages off the queue and send them over `NATS`.  The `sender` is responsible for throttling the rate at which messages are sent over NATS.

### `api`

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

### `outbox`

WIP: `outboux` is a library used by the analyzer and scheduler to add/remove messages from the message queue.  The message queue is stored in the high availability `store`.  The `outbox` guarantees that a message is only added once to the queue (i.e. if a `start` message for a given app guid, version guid, and index has not been sent yet, the `outbox` will prevent another identical `start` message from being inserted in the queue).

### `store`

The `store` is an generalized client for connecting to a Zookeeper/ETCD-like high availability store.  Components that must read or write to the high availability store use this library.

## Test Support Packages (under test_helpers)

### `test_helpers`

`test_helpers` contains a (large) number of test support packages.  These range from simple fakes to comprehensive libraries used for faking out other CloudFoundry components (e.g. heartbeating DEAs) in integration tests.

- `app`: a simple domain object that encapsulates a running CloudFoundry app.

  The `app` package can be used to generate consistent data structures (heartbeats, desired state).  These data structures are then passed into the other test helpers to simulate a CloudFoundry eco-system.
  
  Think of `app` as your source of fixture test data.  It's used in integration tests *and* unit tests.
- `fake_logger`: provides a fake logger
- `fake_time_provider`: provides a fake `TimeProvider`
- `message_publisher`: publishes message to the NATS bus.
- `nats_runner`: starts and manages a NATS server.
- `etcd_runner`: starts and manages an ETCD server.

## The MCAT

The MCAT is comprised of two major integration test suites:

### The `MD` Test Suite

The `MD` test suite excercises the `HM9000` components through a series of integration-level tests.  The individual components are designed to be simple and have comprehensive unit test coverage.  However, it's crucial that we have comprehensive test coverage for the *interactions* between these components.  That's what the `MD` suite is for.

### The `PHD` Benchmark Suite

The `PHD` suite is a collection of benchmark tests.  This is a slow-running suite that is intended, primarily, to evaluate the performance of the various components (especially the high-availability store) under various loads.

