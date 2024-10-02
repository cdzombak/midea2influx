package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/cdzombak/libwx"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

var version = "<dev>"

const (
	programName = "midea2influx"
	mCliName    = "midea-beautiful-air-cli"

	influxTimeout    = 3 * time.Second
	influxAttempts   = 3
	influxRetryDelay = 1 * time.Second

	hbTimeout = 10 * time.Second
)

func main() {
	configFile := flag.String("config", DefaultCfgPath(), "Configuration JSON file.")
	printVersion := flag.Bool("version", false, "Print version and exit.")
	debug := flag.Bool("debug", false, "Enable debug logging.")
	flag.Parse()

	debugLog := DebugLogger(*debug)

	if *printVersion {
		fmt.Fprintln(os.Stderr, programName+" "+version)
		os.Exit(0)
	}

	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "-config is required.")
		os.Exit(6)
	}

	config, err := ConfigFromFile(*configFile)
	if err != nil {
		log.Printf("Loading config from '%s' failed: %s", *configFile, err)
		os.Exit(6)
	}

	mCliPath, err := exec.LookPath(mCliName)
	if err != nil {
		log.Fatalf("Could not find %s in PATH: %s", mCliName, err)
	} else {
		debugLog(fmt.Sprintf("%s found at %s", mCliName, mCliPath))
	}

	authString := ""
	if config.InfluxUser != "" || config.InfluxPass != "" {
		authString = fmt.Sprintf("%s:%s", config.InfluxUser, config.InfluxPass)
	} else if config.InfluxToken != "" {
		authString = config.InfluxToken
	}
	influxClient := influxdb2.NewClient(config.InfluxServer, authString)
	if !config.InfluxHealthCheckDisabled {
		ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
		defer cancel()
		health, err := influxClient.Health(ctx)
		if err != nil {
			log.Fatalf("Failed to check InfluxDB health: %v", err)
		}
		if health.Status != "pass" {
			log.Fatalf("InfluxDB did not pass health check: status %s; message '%s'", health.Status, *health.Message)
		} else {
			debugLog("InfluxDB passed health check")
		}
	}
	influxWriteAPI := influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)

	args := []string{"discover"}
	args = append(args, config.MideaArgs...)
	out, err := exec.Command(mCliName, args...).Output()
	outStr := string(out)
	if err != nil {
		log.Println("stdout: " + outStr)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			log.Println("stderr: " + string(exitErr.Stderr))
		}
		log.Fatalf("Failed to run %s: %s", mCliName, err)
	}

	currentID := ""
	points := make([]*write.Point, 0)
	for _, l := range strings.Split(outStr, "\n") {
		if strings.HasPrefix(l, "id ") {
			currentID = strings.Split(strings.TrimSpace(l), "/")[1]
			points = append(points, influxdb2.NewPointWithMeasurement(config.DehumidifierMeasurementName))
			continue
		}
		l := strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "=", 2)
		if len(parts) < 2 {
			debugLog(fmt.Sprintf("ignoring line of unknown format: '%s'", l))
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "id":
			points[len(points)-1].AddTag("id", value)
		case "addr":
			points[len(points)-1].AddTag("addr", value)
		case "s/n":
			points[len(points)-1].AddTag("sn", value)
		case "name":
			points[len(points)-1].AddTag("name", value)
		case "version":
			points[len(points)-1].AddTag("version", value)
		case "online":
			b, err := ConvBool(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'online' to bool: %s", err))
			} else {
				points[len(points)-1].AddField("online", b)
			}
		case "running":
			b, err := ConvBool(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'running' to bool: %s", err))
			} else {
				points[len(points)-1].AddField("running", b)
			}
		case "humid%":
			f, err := ConvFloat(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'humid%%' to float: %s", err))
			} else {
				points[len(points)-1].AddField("humidity_pct", f)
			}
		case "target%":
			f, err := ConvFloat(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'target%%' to float: %s", err))
			} else {
				points[len(points)-1].AddField("target_humidity_pct", f)
			}
		case "temp":
			f, err := ConvFloat(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'temp' to float: %s", err))
			} else {
				points[len(points)-1].AddField("temp_c", f)
				points[len(points)-1].AddField("temp_f", libwx.TempC(f).F().Unwrap())
			}
		case "fan":
			f, err := ConvFloat(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'fan' to float: %s", err))
			} else {
				points[len(points)-1].AddField("fan", f)
			}
		case "tank":
			b, err := ConvBool(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'tank' to bool: %s", err))
			} else {
				points[len(points)-1].AddField("tank_full", b)
			}
		case "filter":
			b, err := ConvBool(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'filter' to bool: %s", err))
			} else {
				points[len(points)-1].AddField("filter_needs_cleaning", b)
			}
		case "sleep":
			b, err := ConvBool(value)
			if err != nil {
				debugLog(fmt.Sprintf("failed to convert 'sleep' to bool: %s", err))
			} else {
				points[len(points)-1].AddField("sleep", b)
			}
		case "error":
			if value != "0" {
				log.Printf("[WARN] device %s reports error %s", currentID, value)
			}
		}
	}
	if len(points) == 0 {
		log.Fatalf("no devices with data to report found")
	}

	if err := retry.Do(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
		defer cancel()
		return influxWriteAPI.WritePoint(ctx, points...)
	}, retry.Attempts(influxAttempts), retry.Delay(influxRetryDelay)); err != nil {
		log.Fatalf("Failed to write %d points to Influx: %s", len(points), err)
	} else {
		log.Printf("Wrote %d points to Influx", len(points))
	}

	if config.HeartbeatURL != "" {
		hbClient := http.DefaultClient
		hbClient.Timeout = hbTimeout
		if _, err := hbClient.Get(config.HeartbeatURL); err != nil {
			log.Printf("Failed to send heartbeat to %s: %s", config.HeartbeatURL, err)
		} else {
			debugLog(fmt.Sprintf("Sent heartbeat to %s", config.HeartbeatURL))
		}
	}
}
