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
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	ec "github.com/cdzombak/exitcode_go"
	"github.com/cdzombak/libwx"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/samber/lo"
)

var version = "<dev>"

const (
	programName = "midea2influx"
	mCliName    = "midea-beautiful-air-cli"

	influxTimeout    = 3 * time.Second
	influxAttempts   = 3
	influxRetryDelay = 1 * time.Second

	mqttTimeout    = 3 * time.Second
	mqttAttempts   = 3
	mqttRetryDelay = 1 * time.Second

	hbTimeout = 10 * time.Second

	ecNoDevices = 11
)

func main() {
	configFile := flag.String("config", DefaultCfgPath(), "Configuration JSON file.")
	printVersion := flag.Bool("version", false, "Print version and exit.")
	debug := flag.Bool("debug", false, "Enable debug logging.")
	flag.Parse()

	debugLog := DebugLogger(*debug)

	if *printVersion {
		fmt.Fprintln(os.Stderr, programName+" "+version)
		os.Exit(ec.Success)
	}

	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "-config is required.")
		os.Exit(ec.NotConfigured)
	}

	config, err := ConfigFromFile(*configFile)
	if err != nil {
		log.Printf("Loading config from '%s' failed: %s", *configFile, err)
		os.Exit(ec.NotConfigured)
	}

	mCliPath, err := exec.LookPath(mCliName)
	if err != nil {
		log.Fatalf("Could not find %s in PATH: %s", mCliName, err)
	} else {
		debugLog(fmt.Sprintf("%s found at %s", mCliName, mCliPath))
	}

	var influxClient influxdb2.Client
	var influxWriteAPI api.WriteAPIBlocking
	var mqttClient mqtt.Client

	influxConfigured := config.InfluxServer != "" && config.InfluxBucket != ""
	mqttConfigured := config.MQTTHost != "" && config.MQTTTopic != ""

	if influxConfigured {
		authString := ""
		if config.InfluxUser != "" || config.InfluxPass != "" {
			authString = fmt.Sprintf("%s:%s", config.InfluxUser, config.InfluxPass)
		} else if config.InfluxToken != "" {
			authString = config.InfluxToken
		}
		influxClient = influxdb2.NewClient(config.InfluxServer, authString)
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
		influxWriteAPI = influxClient.WriteAPIBlocking(config.InfluxOrg, config.InfluxBucket)
	}

	if mqttConfigured {
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("tcp://%s:%d", config.MQTTHost, config.MQTTPort))
		opts.SetClientID(fmt.Sprintf("%s-%d", programName, time.Now().Unix()))
		opts.SetConnectTimeout(mqttTimeout)
		if config.MQTTUsername != "" {
			opts.SetUsername(config.MQTTUsername)
		}
		if config.MQTTPassword != "" {
			opts.SetPassword(config.MQTTPassword)
		}

		mqttClient = mqtt.NewClient(opts)
		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
		} else {
			debugLog("Connected to MQTT broker")
		}
		defer mqttClient.Disconnect(250)
	}

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

	points = lo.Filter(points, func(point *write.Point, _ int) bool {
		for _, f := range point.FieldList() {
			if f.Key == "online" && !f.Value.(bool) {
				return false
			}
			if f.Key == "temp_c" && f.Value.(float64) == 0 {
				return false
			}
		}
		return true
	})
	if len(points) == 0 {
		log.Printf("no devices with data to report found")
		os.Exit(ecNoDevices)
	}

	if influxConfigured {
		if err := retry.Do(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
			defer cancel()
			return influxWriteAPI.WritePoint(ctx, points...)
		}, retry.Attempts(influxAttempts), retry.Delay(influxRetryDelay)); err != nil {
			log.Fatalf("Failed to write %d points to Influx: %s", len(points), err)
		} else {
			log.Printf("Wrote %d points to Influx", len(points))
		}
	}

	if mqttConfigured {
		if err := retry.Do(func() error {
			return publishToMQTT(mqttClient, config.MQTTTopic, points, debugLog)
		}, retry.Attempts(mqttAttempts), retry.Delay(mqttRetryDelay)); err != nil {
			log.Fatalf("Failed to publish %d measurements to MQTT: %s", len(points), err)
		} else {
			log.Printf("Published %d measurements to MQTT", len(points))
		}
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

func publishToMQTT(client mqtt.Client, baseTopic string, points []*write.Point, debugLog func(string)) error {
	for _, point := range points {
		deviceID := ""
		for _, tag := range point.TagList() {
			if tag.Key == "id" {
				deviceID = tag.Value
				break
			}
		}

		for _, field := range point.FieldList() {
			topic := fmt.Sprintf("%s/%s/%s", baseTopic, deviceID, field.Key)
			var payload string

			switch v := field.Value.(type) {
			case bool:
				payload = strconv.FormatBool(v)
			case float64:
				payload = strconv.FormatFloat(v, 'f', -1, 64)
			case int64:
				payload = strconv.FormatInt(v, 10)
			default:
				payload = fmt.Sprintf("%v", v)
			}

			token := client.Publish(topic, 0, false, payload)
			if token.Wait() && token.Error() != nil {
				return fmt.Errorf("failed to publish to topic %s: %w", topic, token.Error())
			}
			debugLog(fmt.Sprintf("Published to MQTT topic %s: %s", topic, payload))
		}
	}
	return nil
}
