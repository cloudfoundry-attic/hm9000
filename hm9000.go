package main

import (
	"fmt"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry/dropsonde"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/hm"
	"github.com/codegangsta/cli"

	"github.com/pivotal-golang/lager"

	"os"
)

const (
	dropsondeOrigin = "HM9000"
)

func main() {
	app := cli.NewApp()
	app.Name = "HM9000"
	app.Usage = "Start the various HM9000 components"
	app.Version = "0.0.9000"
	app.Commands = []cli.Command{
		{
			Name:        "fetch_desired",
			Description: "Fetches desired state",
			Usage:       "hm fetch_desired --config=/path/to/config --poll",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "poll", Usage: "If true, poll repeatedly with an interval defined in config"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "fetcher")
				hm.FetchDesiredState(logger, conf, c.Bool("poll"))
			},
		},
		{
			Name:        "listen",
			Description: "Listens for the actual state over NATS and via HTTP",
			Usage:       "hm listen --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "listener")
				hm.StartListeningForActual(logger, conf)
			},
		},
		{
			Name:        "analyze",
			Description: "Analyze the desired and actual state and enqueue start/stop messages",
			Usage:       "hm analyze --config=/path/to/config --poll",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "poll", Usage: "If true, poll repeatedly with an interval defined in config"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "analyzer")
				hm.Analyze(logger, conf, c.Bool("poll"))
			},
		},
		{
			Name:        "send",
			Description: "Send the enqueued start/stop messages",
			Usage:       "hm send --config=/path/to/config --poll",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "poll", Usage: "If true, poll repeatedly with an interval defined in config"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "sender")
				hm.Send(logger, conf, c.Bool("poll"))
			},
		},
		{
			Name:        "evacuator",
			Description: "Send NATS start message for evacuated apps",
			Usage:       "hm evacuator --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "evacuator")
				hm.StartEvacuator(logger, conf)
			},
		},
		{
			Name:        "serve_api",
			Description: "Serve app API over http",
			Usage:       "hm serve_api --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "apiserver")
				hm.ServeAPI(logger, conf)
			},
		},
		{
			Name:        "shred",
			Description: "Deletes empty directories from the store",
			Usage:       "hm shred --config=/path/to/config --poll",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "poll", Usage: "If true, poll repeatedly with an interval defined in config"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "shredder")
				hm.Shred(logger, conf, c.Bool("poll"))
			},
		},
		{
			Name:        "dump",
			Description: "Dumps contents of the data store",
			Usage:       "hm dump --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "raw", Usage: "If set, dump the unstructured contents of the database"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				logger, conf := loadLoggerAndConfig(c, "dumper")
				hm.Dump(logger, conf, c.Bool("raw"))
			},
		},
	}

	app.Run(os.Args)
}

func loadLoggerAndConfig(c *cli.Context, component string) (lager.Logger, *config.Config) {
	var logger lager.Logger
	configPath := c.String("config")
	if configPath == "" {
		fmt.Fprintf(os.Stderr, "Config path required\n")
		os.Exit(1)
	}

	conf, err := config.FromFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %s\n", configPath)
		os.Exit(1)
	}

	if component == "" {
		fmt.Fprintf(os.Stderr, "Empty component name\n")
		os.Exit(1)
	}

	logger = lager.NewLogger(component)

	logLevel, err := conf.LogLevel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	logger.RegisterSink(lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, logLevel), logLevel))

	debugAddr := c.String("debugAddr")
	if debugAddr != "" {
		_, err = cf_debug_server.Run(debugAddr, nil)
		if err != nil {
			logger.Error("Failed to start debug server", err)
			os.Exit(1)
		}
	}

	dropsondeDestination := fmt.Sprintf("127.0.0.1:%d", conf.DropsondePort)
	err = dropsonde.Initialize(dropsondeDestination, component)
	if err != nil {
		logger.Error("Failed to initialize dropsonde", err)
		os.Exit(1)
	}

	return logger, conf
}
