# Health Manager 9000

[![Build Status](https://travis-ci.org/cloudfoundry/hm9000.png)](https://travis-ci.org/cloudfoundry/hm9000)

HM 9000 is a rewrite of CloudFoundry's Health Manager.  HM 9000 is written in Golang and has a more modular architecture compard to the original ruby implementation.

As a result there are several Go Packages in this repository, each with a comprehensive set of unit tests.  What follows is a detailed breakdown:

## Relocation & Status Warning

cloudfoundry/hm9000 will eventually be promoted and move to cloudfoundry/health_manager.  This is the temporary home while it is under development.

hm9000 is not yet a complete replacement for health_manager -- we'll update this README when it's ready for primetime.

## Installing HM9000

Assuming you have `go` v1.1.* installed:

1. Setup a go workspace:

        $ cd $HOME
        $ mkdir gospace
        $ export GOPATH=$HOME/gospace
        $ export PATH=$HOME/gospace/bin:$PATH
        $ cd gospace

2. Fetch HM9000 and its dependencies (not locked down for now):

        $ go get -v github.com/cloudfoundry/hm9000 #you will need mercurial and git installed

3. Install `etcd`

        $ pushd ./src/github.com/coreos/etcd
        $ ./build
        $ mv etcd $GOPATH/bin/
        $ popd

4. Start `etcd`.  Open a new terminal session and:

        $ export PATH=$HOME/gospace/bin:$PATH
        $ cd $HOME
        $ mkdir etcdstorage
        $ cd etcdstorage
        $ etcd

    `etcd` generates a number of files in CWD when run locally, hence `etcdstorage`

5. Running `hm9000`.  Back in the terminal you used to `go get` hm9000 you should be able to

        $ hm9000

    and get usage information

6. Running the tests
    
        $ go get github.com/onsi/ginkgo/ginkgo
        $ cd src/github.com/cloudfoundry/hm9000/
        $ ginkgo -r -skipMeasurements -race -failOnPending

    These tests will spin up their own instances of `etcd` as needed.  It shouldn't interfere with your long-running `etcd` server.

7. Updating hm9000.  You'll need to fetch the latest code *and* recompile the hm9000 binary:

        $ cd $GOPATH/src/github.com/cloudfoundry/hm9000
        $ git checkout master
        $ git pull
        $ go install .

## Running HM9000

`hm9000` requires a config file.  To get started:

    $ cd $GOPATH/src/github.com/cloudfoundry/hm9000
    $ cp ./config/default_config.json ./local_config.json
    $ vim ./local_config.json

You *must* specify a config file for all the `hm9000` commands.  You do this with (e.g.) `--config=./local_config.json`

### Fetching desired state

    hm9000 fetch_desired --config=./local_config.json

will connect to CC, fetch the desired state, put it in the store under `/desired`, then exit.


### Listening for active state

    hm9000 listen --config=./local_config.json

will come up, listen to NATS for heartbeats, and put them in the store under `/actual`, then exit.

### Dumping the contents of the store

`etcd` has a very simple [curlable API](http://github.com/coreos/etcd).  For convenience:

    hm9000 dump --config=./local_config.json

will dump the entire contents of the store to stdout.

## HM9000 components

### `hm9000` (the top level) and `hm`

The top level is home to the `hm9000` CLI.  The `hm` package houses the CLI logic to keep the root directory cleaner.  The `hm` package is where the other components are instantiated, fed their dependencies, and executed.

### `actualstatelistener`

The `actualstatelistener` provides a simple listener daemon that monitors the `NATS` stream for app heartbeats.  It generates an entry in the `store` for each heartbeating app under `/actual/INSTANCE_GUID`.

It also maintains a `FreshnessTimestamp`  under `/actual-fresh` to allow other components to know whether or not they can trust the information under `/actual`

Relevant config:

- `heartbeat_ttl_in_seconds`: the TTL to set on the `/actual/INSTANCE_GUID` keys
- `actual_freshness_ttl_in_seconds`: the TTL for the freshness key.  Typically this should be the same as `config.HeartbeatTTL` (if we don't hear back from NATS *at all* by `config.HeartbeatTTL` then we should probably question the reliability of our NATS connection)
- `actual_freshness_key`: the name of the freshness key to put in the store.  The default `/actual-fresh` should be adequate.
- `nats.host`, `nats.port`, `nats.user`, `nats.password`: the various bits and pieces needed to connect to NATS

#### `desiredstatefetcher`

The `desiredstatefetcher` requests the desired state from the cloud controller.  It transparently manages fetching the authentication information over NATS and making batched http requests to the bulk api endpoint.

Desired state is stored under `/desired/APP_GUID-APP_VERSION

Relevant config:

- `desired_state_ttl_in_seconds`: the TTL to set on the `/desired/APP_GUID-APP_VERSION` keys
- `desired_freshness_ttl_in_seconds`: the TTL for the freshness key.
- `desired_freshness_key`: the name of the freshness key to put in the store.  The default `/desired-fresh` should be adequate.
- `CCAuthMessageBusSubject`: the message bus subject to use to fetch the `CC` authentication credentials.  Defaults to `cloudcontroller.bulk.credentials.default`
- `CCBaseURL`: the base path for the CC API endpoint`
- `nats.host`, `nats.port`, `nats.user`, `nats.password`: the various bits and pieces needed to connect to NATS


### WIP:`analyzer`

WIP: The `analyzer` runs periodically and uses the `outbox` to put `start` and `stop` messages in the queue.

### WIP:`sender`

WIP: The `sender` runs periodically and uses the `outbox` to pull messages off the queue and send them over `NATS`.  The `sender` is responsible for throttling the rate at which messages are sent over NATS.

### WIP:`api`

WIP: The `api` is a simple HTTP server that provides access to information about the actual state.  It uses the high availability store to fulfill these requests.

## Support Packages

### `config`

`config` parses the `config.json` configuration.  Components are typically given an instance of `config` by the `hm` CLI.

### `helpers`

`helpers` contains a number of support utilities.

#### `freshnessmanager`

The `freshnessmanager` manages writing and reading/interpreting freshness keys in the store.

#### `httpclient`

A trivial wrapper around `net/http` that improves testability of http requests.

#### `logger`

Provides a (sys)logger.  Eventually this will use steno to perform logging.

#### `timeprovider`

Provides a `TimeProvider`.  Useful for injecting time dependencies in tests.

#### `workerpool`

Provides a worker pool with a configurable pool size.  Work scheduled on the pool will run concurrently, but no more `poolSize` workers can be running at any given moment.

#### WIP:`outbox`

WIP: `outboux` is a library used by the analyzer and scheduler to add/remove messages from the message queue.  The message queue is stored in the high availability `store`.  The `outbox` guarantees that a message is only added once to the queue (i.e. if a `start` message for a given app guid, version guid, and index has not been sent yet, the `outbox` will prevent another identical `start` message from being inserted in the queue).

### `models`

`models` encapsulates the various JSON structs that are sent/received over NATS/HTTP.  Simple serializing/deserializing behavior is attached to these structs.

### `store`

The `store` is an generalized client for connecting to a Zookeeper/ETCD-like high availability store.  Components that must read or write to the high availability store use this library.  Writes are performed concurrently for optimal performance.

## Test Support Packages (under testhelpers)

`testhelpers` contains a (large) number of test support packages.  These range from simple fakes to comprehensive libraries used for faking out other CloudFoundry components (e.g. heartbeating DEAs) in integration tests.

### Fakes

#### `fakefreshnessmanager`

Provides a fake implementation of the `helpers/freshnessmanager` interface

#### `fakelogger`

Provides a fake implementation of the `helpers/logger` interface

#### `faketimeprovider`

Provides a fake implementation of the `helpers/timeprovider` interface.  Useful for injecting time dependency in test.

#### `fakehttpclient`

Provdes a fake implementation of the `helpers/httpclient` interface that allows tests to have fine-grained control over the http request/response lifecycle.

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

#### `messagepublisher`

Provides a simple mechanism to publish actual state related messages to the NATS bus.  Handles JSON encoding.

#### `startstoplistener`

Listens on the NATS bus for `health.start` and `health.stop` messages.  It parses these messages and makes them available via a simple interface.  Useful for testing that messages are sent by the health manager appropriately.

#### `desiredstateserver`

Brings up an in-process http server that mimics the CC's bulk endpoints (including authentication via NATS and pagination).

#### `natsrunner`

Brings up and manages the lifecycle of a live NATS server.  After bringing the server up it provides a fully configured cfmessagebus object that you can pass to your test subjects.

#### `storerunner`

Brings up and manages the lifecycle of a live ETCD server cluster.

## The MCAT

The MCAT is comprised of two major integration test suites:

### WIP:The `MD` Test Suite

The `MD` test suite excercises the `HM9000` components through a series of integration-level tests.  The individual components are designed to be simple and have comprehensive unit test coverage.  However, it's crucial that we have comprehensive test coverage for the *interactions* between these components.  That's what the `MD` suite is for.

### The `PHD` Benchmark Suite

The `PHD` suite is a collection of benchmark tests.  This is a slow-running suite that is intended, primarily, to evaluate the performance of the various components (especially the high-availability store) under various loads.
