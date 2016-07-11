package main

import (
	"fmt"

	"code.cloudfoundry.org/debugserver"
	"github.com/cloudfoundry/dropsonde"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/hm"
	"github.com/codegangsta/cli"

	"code.cloudfoundry.org/lager"

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
			Name:        "analyze",
			Description: "Analyzes desired state",
			Usage:       "hm analyze --config=/path/to/config --poll",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "config", Value: "", Usage: "Path to config file"},
				cli.BoolFlag{Name: "poll", Usage: "If true, poll repeatedly with an interval defined in config"},
				cli.StringFlag{Name: "debugAddr", Value: "", Usage: "address to serve debug info"},
			},
			Action: func(c *cli.Context) {
				conf := loadConfig(c, "analyzer")
				logLevel, err := conf.LogLevel()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Invalid log level in config: %s\n", err.Error())
					os.Exit(1)
				}

				logger := lager.NewLogger("analyzer")
				sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, logLevel), logLevel)
				logger.RegisterSink(sink)

				startDropsond(c, "analyzer", conf, logger)
				startDebugServer(c, logger)
				hm.Analyze(logger, sink, conf, c.Bool("poll"))
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

func loadConfig(c *cli.Context, component string) *config.Config {
	if component == "" {
		fmt.Fprintf(os.Stderr, "Empty component name\n")
		os.Exit(1)
	}

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

	return conf
}

func startDebugServer(c *cli.Context, logger lager.Logger) {
	debugAddr := c.String("debugAddr")
	if debugAddr != "" {
		_, err := debugserver.Run(debugAddr, nil)
		if err != nil {
			logger.Error("Failed to start debug server", err)
			os.Exit(1)
		}
	}
}

func startDropsond(c *cli.Context, component string, conf *config.Config, logger lager.Logger) {
	dropsondeDestination := fmt.Sprintf("127.0.0.1:%d", conf.DropsondePort)
	err := dropsonde.Initialize(dropsondeDestination, component)
	if err != nil {
		logger.Error("Failed to initialize dropsonde", err)
		os.Exit(1)
	}
}

func loadLoggerAndConfig(c *cli.Context, component string) (lager.Logger, *config.Config) {
	var logger lager.Logger

	if component == "" {
		fmt.Fprintf(os.Stderr, "Empty component name\n")
		os.Exit(1)
	}

	conf := loadConfig(c, component)
	logger = lager.NewLogger(component)

	logLevel, err := conf.LogLevel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid log level in config: %s\n", err.Error())
		os.Exit(1)
	}

	logger.RegisterSink(lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, logLevel), logLevel))

	startDropsond(c, component, conf, logger)
	startDebugServer(c, logger)

	return logger, conf
}
