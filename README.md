# midea2influx

Write status of Midea dehumidifers to InfluxDB and/or MQTT.

## Goals and Limitations

I have a Midea dehumidifier that I want to monitor with InfluxDB.

I cannot get [the `midea-beautiful-air` Python client](https://github.com/nbogojevic/midea-beautiful-air) to work completely with my dehumidifier (see [nbogojevic/midea-beautiful-air/issues/36](https://github.com/nbogojevic/midea-beautiful-air/issues/36)). I _can_ get it to print out some useful information when _discovering_ devices.

Therefore:

- This program just calls `midea-beautiful-air-cli discover` with user-configurable flags.
- This is explicitly a minimal-effort project, since I don't trust that `midea-beautiful-air` will continue working at all long-term.
- This program only supports dehumidifiers right now, as I don't have any other Midea devices.
- This project is intended to work primarily as a Docker container, since it expects `midea-beautiful-air-cli` to be in the `PATH`.
- No effort was spent on simplifying initial setup & figuring out what arguments you need to pass to `midea-beautiful-air-cli discover`. You need to find a set of working arguments and give them in the config JSON file.

## Usage

A minimal usage example is this, which one would run as desired via a cron job:

```sh
docker run --rm --network host -v /home/cdzombak/.config/midea2influx.json:/config.json:ro cdzombak/midea2influx:1
```

This assumes you're storing your JSON configuration file at `/home/cdzombak/.config/midea2influx.json`. Configuration is discussed in the next section.

Note that [the `midea-beautiful-air` library must be able to broadcast UDP packets on the same network as your dehumidifier(s)](https://github.com/nbogojevic/midea-beautiful-air?tab=readme-ov-file#discovery). This is possible with Docker on Linux if the Docker host is on the same network as your dehumidifier(s) and you pass the `--network host` flag to `docker run` as shown above.

## Configuration

Configuration is provided via a JSON file. At least one output method (InfluxDB or MQTT) must be configured.

### InfluxDB Configuration

- `influx_server`: InfluxDB server URL (e.g., `http://192.168.1.2:8086`)
- `influx_bucket`: InfluxDB bucket name
- `influx_user`: InfluxDB username (optional, use with `influx_password`)
- `influx_password`: InfluxDB password (optional, use with `influx_user`)
- `influx_token`: InfluxDB token (optional, alternative to username/password)
- `influx_org`: InfluxDB organization (optional)
- `influx_health_check_disabled`: Disable InfluxDB health check (optional, default: false)

### MQTT Configuration

- `mqtt_host`: MQTT broker hostname or IP address
- `mqtt_port`: MQTT broker port (optional, default: 1883)
- `mqtt_topic`: Base MQTT topic (e.g., `home/dehumidifier`)
- `mqtt_username`: MQTT username (optional)
- `mqtt_password`: MQTT password (optional)

### MQTT Topic Structure

When using MQTT output, each measurement is published to a subtopic under the configured base topic:
- `{base_topic}/{device_id}/temp_c` - Temperature in Celsius
- `{base_topic}/{device_id}/temp_f` - Temperature in Fahrenheit  
- `{base_topic}/{device_id}/humidity_pct` - Current humidity percentage
- `{base_topic}/{device_id}/target_humidity_pct` - Target humidity percentage
- `{base_topic}/{device_id}/online` - Device online status (true/false)
- `{base_topic}/{device_id}/running` - Device running status (true/false)
- `{base_topic}/{device_id}/fan` - Fan speed
- `{base_topic}/{device_id}/tank_full` - Water tank full status (true/false)
- `{base_topic}/{device_id}/filter_needs_cleaning` - Filter cleaning needed (true/false)
- `{base_topic}/{device_id}/sleep` - Sleep mode status (true/false)

### Home Assistant Configuration

To use the MQTT output with Home Assistant, add sensors like these to your `configuration.yaml`:

```yaml
mqtt:
  sensor:
    - name: "Dehumidifier Temperature"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/temp_c"
      unit_of_measurement: "°C"
      device_class: "temperature"
      
    - name: "Dehumidifier Humidity"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/humidity_pct"
      unit_of_measurement: "%"
      device_class: "humidity"
      
    - name: "Dehumidifier Target Humidity"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/target_humidity_pct"
      unit_of_measurement: "%"
      device_class: "humidity"
      
    - name: "Dehumidifier Fan Speed"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/fan"
      
  binary_sensor:
    - name: "Dehumidifier Online"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/online"
      payload_on: "true"
      payload_off: "false"
      device_class: "connectivity"
      
    - name: "Dehumidifier Running"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/running"
      payload_on: "true"
      payload_off: "false"
      device_class: "running"
      
    - name: "Dehumidifier Tank Full"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/tank_full"
      payload_on: "true"
      payload_off: "false"
      device_class: "problem"
      
    - name: "Dehumidifier Filter Needs Cleaning"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/filter_needs_cleaning"
      payload_on: "true"
      payload_off: "false"
      device_class: "problem"
      
    - name: "Dehumidifier Sleep Mode"
      state_topic: "home/dehumidifier/YOUR_DEVICE_ID/sleep"
      payload_on: "true"
      payload_off: "false"
```

Replace `YOUR_DEVICE_ID` with the actual device ID from your dehumidifier (visible in the program's debug output).

## License

MIT; see [`LICENSE`](LICENSE) in this repository.

## Author

Chris Dzombak.

- [dzombak.com](https://www.dzombak.com)
- [github.com/cdzombak](https://www.github.com/cdzombak)
