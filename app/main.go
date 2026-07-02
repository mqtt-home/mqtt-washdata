package main

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/mqtt-home/mqtt-washdata/config"
	"github.com/mqtt-home/mqtt-washdata/dryer"
	"github.com/mqtt-home/mqtt-washdata/version"
	"github.com/mqtt-home/mqtt-washdata/web"
	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

func initPprof() {
	go func() {
		_ = http.ListenAndServe(":6060", nil)
	}()
}

func main() {
	logger.Init("info", logger.Logger())
	logger.Info("mqtt-washdata", "version", version.Info())

	if len(os.Args) < 2 {
		logger.Error("No configuration file specified")
		os.Exit(1)
	}

	configFile := os.Args[1]
	logger.Info("Configuration file", "path", configFile)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return
	}
	logger.SetLevel(cfg.LogLevel)

	initPprof()

	// Runs are persisted next to the config file.
	dataDir := filepath.Dir(configFile)
	store, err := dryer.NewStore(filepath.Join(dataDir, "runs"))
	if err != nil {
		logger.Error("Failed to initialize run store", "error", err)
		return
	}
	if err := store.Load(); err != nil {
		logger.Warn("Failed to load existing runs", "error", err)
	}

	mqtt.Start(cfg.MQTT, "washdata_mqtt")

	manager := dryer.NewManager(cfg.Dryer, cfg.MQTT.Topic, cfg.MQTT.Retain, store)

	if !cfg.Web.Enabled {
		logger.Info("Web interface is disabled in the configuration")
	} else {
		webServer := web.NewWebServer(manager)
		manager.SetStatusCallback(webServer.BroadcastStatus)
		go func() {
			if err := webServer.Start(cfg.Web.Port); err != nil {
				logger.Error("Failed to start web server", "error", err)
			}
		}()
		logger.Info("Web interface available at http://localhost:" + strconv.Itoa(cfg.Web.Port))
	}

	manager.Start()
	logger.Info("Application is now ready. Press Ctrl+C to quit.")

	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)
	<-quitChannel

	logger.Info("Received quit signal")
}
