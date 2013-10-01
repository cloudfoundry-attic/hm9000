package main

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/hm"
	"github.com/codegangsta/cli"

	"os"
)

func main() {
	l := logger.NewRealLogger()

	app := cli.NewApp()
	app.Name = "HM9000"
	app.Usage = "Start the various HM9000 components"
	app.Version = "0.0.9000"
	app.Commands = []cli.Command{
		cli.Command{
			Name:        "fetch_desired",
			Description: "Fetches desired state",
			Usage:       "hm fetch_desired --config=/path/to/config --pollEvery=duration",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
				cli.IntFlag{"pollEvery", 0, "Polling interval in seconds (leave blank to run just once)"},
			},
			Action: func(c *cli.Context) {
				hm.FetchDesiredState(l, loadConfig(l, c), c.Int("pollEvery"))
			},
		},
		cli.Command{
			Name:        "listen",
			Description: "Listens over the NATS for the actual state",
			Usage:       "hm listen --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
			},
			Action: func(c *cli.Context) {
				hm.StartListeningForActual(l, loadConfig(l, c))
			},
		},
		cli.Command{
			Name:        "analyze",
			Description: "Analyze the desired and actual state and enqueue start/stop messages",
			Usage:       "hm analyze --config=/path/to/config --pollEvery=duration",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
				cli.IntFlag{"pollEvery", 0, "Polling interval in seconds (leave blank to run just once)"},
			},
			Action: func(c *cli.Context) {
				hm.Analyze(l, loadConfig(l, c), c.Int("pollEvery"))
			},
		},
		cli.Command{
			Name:        "send",
			Description: "Send the enqueued start/stop messages",
			Usage:       "hm send --config=/path/to/config --pollEvery=duration --noop",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
				cli.IntFlag{"pollEvery", 0, "Polling interval in seconds (leave blank to run just once)"},
				cli.BoolFlag{"noop", "Enable noop mode to prevent sending messages over nats (messages will simply be logged, instead)"},
			},
			Action: func(c *cli.Context) {
				hm.Send(l, loadConfig(l, c), c.Int("pollEvery"), c.Bool("noop"))
			},
		},
		cli.Command{
			Name:        "dump",
			Description: "Dumps contents of the data store",
			Usage:       "hm dump --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
			},
			Action: func(c *cli.Context) {
				hm.Dump(l, loadConfig(l, c))
			},
		},
		cli.Command{
			Name:        "clear_store",
			Description: "Clears contents of the data store",
			Usage:       "hm clear_store --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
			},
			Action: func(c *cli.Context) {
				hm.Clear(l, loadConfig(l, c))
			},
		},
	}

	app.Run(os.Args)
}

func loadConfig(l logger.Logger, c *cli.Context) config.Config {
	configPath := c.String("config")
	if configPath == "" {
		l.Info("Config path required", nil)
		os.Exit(1)
	}

	conf, err := config.FromFile(configPath)
	if err != nil {
		l.Info("Failed to load config", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	return conf
}
