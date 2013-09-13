package config

import (
	"fmt"
)

const HEARTBEAT_TTL = 30
const ACTUAL_FRESHNESS_TTL = HEARTBEAT_TTL

const GRACE_PERIOD = 30

const DESIRED_STATE_TTL = 10 * 60
const DESIRED_FRESHNESS_TTL = 2 * 60
const DESIRED_STATE_POLLING_BATCH_SIZE = 500

const ACTUAL_FRESHNESS_KEY = "/actual-fresh"
const DESIRED_FRESHNESS_KEY = "/desired-fresh"

func ETCD_URL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
