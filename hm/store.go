package hm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
	"code.cloudfoundry.org/clock"
)

func Dump(l lager.Logger, conf *config.Config, raw bool) {
	if raw {
		dumpRaw(l, conf)
	} else {
		dumpStructured(l, conf)
	}
}

func dumpStructured(l lager.Logger, conf *config.Config) {
	clock := buildClock(l)
	store := connectToStore(l, conf)
	fmt.Printf("Dump - Current timestamp %d\n", clock.Now().Unix())
	err := store.VerifyFreshness(clock.Now())
	if err == nil {
		fmt.Printf("Store is fresh\n")
	} else {
		fmt.Printf("STORE IS NOT FRESH: %s\n", err.Error())
	}
	fmt.Printf("====================\n")

	apps, err := store.GetApps()
	if err != nil {
		fmt.Printf("Failed to fetch apps: %s\n", err.Error())
		os.Exit(1)
	}

	starts, err := store.GetPendingStartMessages()
	if err != nil {
		fmt.Printf("Failed to fetch starts: %s\n", err.Error())
		os.Exit(1)
	}

	stops, err := store.GetPendingStopMessages()
	if err != nil {
		fmt.Printf("Failed to fetch stops: %s\n", err.Error())
		os.Exit(1)
	}

	appKeys := sort.StringSlice{}
	for appKey := range apps {
		appKeys = append(appKeys, appKey)
	}
	sort.Sort(appKeys)
	for _, appKey := range appKeys {
		dumpApp(apps[appKey], starts, stops, clock)
	}
}

func dumpApp(app *models.App, starts map[string]models.PendingStartMessage, stops map[string]models.PendingStopMessage, clock clock.Clock) {
	fmt.Printf("\n")
	fmt.Printf("Guid: %s | Version: %s\n", app.AppGuid, app.AppVersion)
	if app.IsDesired() {
		fmt.Printf("  Desired: [%d] instances, (%s, %s)\n", app.Desired.NumberOfInstances, app.Desired.State, app.Desired.PackageState)
	} else {
		fmt.Printf("  Desired: NO\n")
	}

	if len(app.InstanceHeartbeats) == 0 {
		fmt.Printf("  Heartbeats: NONE\n")
	} else {
		fmt.Printf("  Heartbeats:\n")
		for _, heartbeat := range app.InstanceHeartbeats {
			fmt.Printf("    [%d %s] %s on %s\n", heartbeat.InstanceIndex, heartbeat.State, heartbeat.InstanceGuid, heartbeat.DeaGuid[0:5])
		}
	}

	if len(app.CrashCounts) != 0 {
		fmt.Printf("  CrashCounts:")
		for _, crashCount := range app.CrashCounts {
			fmt.Printf(" [%d]:%d", crashCount.InstanceIndex, crashCount.CrashCount)
		}
		fmt.Printf("\n")
	}

	appStarts := []models.PendingStartMessage{}
	appStops := []models.PendingStopMessage{}

	for _, start := range starts {
		if start.AppGuid == app.AppGuid && start.AppVersion == app.AppVersion {
			appStarts = append(appStarts, start)
		}
	}

	for _, stop := range stops {
		if stop.AppGuid == app.AppGuid && stop.AppVersion == app.AppVersion {
			appStops = append(appStops, stop)
		}
	}

	if len(appStarts) > 0 {
		fmt.Printf("  Pending Starts:\n")
		for _, start := range appStarts {
			message := []string{}
			message = append(message, fmt.Sprintf("[%d]", start.IndexToStart))
			message = append(message, fmt.Sprintf("priority:%.2f", start.Priority))
			if start.SkipVerification {
				message = append(message, "NO VERIFICATION")
			}
			if start.SentOn != 0 {
				message = append(message, "send:SENT")
				message = append(message, fmt.Sprintf("delete:%s", time.Unix(start.SentOn+int64(start.KeepAlive), 0).Sub(clock.Now())))
			} else {
				message = append(message, fmt.Sprintf("send:%s", time.Unix(start.SendOn, 0).Sub(clock.Now())))
			}

			fmt.Printf("    %s\n", strings.Join(message, " "))
		}
	}

	if len(appStops) > 0 {
		fmt.Printf("  Pending Stops:\n")
		for _, stop := range appStops {
			message := []string{}
			message = append(message, stop.InstanceGuid)
			if stop.SentOn != 0 {
				message = append(message, "send:SENT")
				message = append(message, fmt.Sprintf("delete:%s", time.Unix(stop.SentOn+int64(stop.KeepAlive), 0).Sub(clock.Now())))
			} else {
				message = append(message, fmt.Sprintf("send:%s", time.Unix(stop.SendOn, 0).Sub(clock.Now())))
			}

			fmt.Printf("    %s\n", strings.Join(message, " "))
		}
	}
}

func dumpRaw(l lager.Logger, conf *config.Config) {
	storeAdapter := connectToStoreAdapter(l, conf)
	fmt.Printf("Raw Dump - Current timestamp %d\n", time.Now().Unix())

	entries := sort.StringSlice{}

	node, err := storeAdapter.ListRecursively("/hm")
	if err != nil {
		panic(err)
	}
	walk(node, func(node storeadapter.StoreNode) {
		ttl := fmt.Sprintf("[TTL:%ds]", node.TTL)
		if node.TTL == 0 {
			ttl = "[TTL: ∞]"
		}
		buf := &bytes.Buffer{}
		err := json.Indent(buf, node.Value, "    ", "  ")
		value := buf.String()
		if err != nil {
			value = string(node.Value)
		}
		entries = append(entries, fmt.Sprintf("%s %s:\n    %s", node.Key, ttl, value))
	})

	sort.Sort(entries)
	for _, entry := range entries {
		fmt.Printf(entry + "\n")
	}
}

func walk(node storeadapter.StoreNode, callback func(storeadapter.StoreNode)) {
	for _, node := range node.ChildNodes {
		if node.Dir {
			walk(node, callback)
		} else {
			callback(node)
		}
	}
}
