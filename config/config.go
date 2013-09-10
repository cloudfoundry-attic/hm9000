package config

import (
	"fmt"
)

const HEARTBEAT_TTL = 30
const ACTUAL_FRESHNESS_TTL = HEARTBEAT_TTL

const GRACE_PERIOD = 30

const ACTUAL_FRESHNESS_KEY = "/actual-fresh"

func ETCD_URL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
