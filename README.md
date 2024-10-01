# midea2influx

Write status of Midea dehumidifers to InfluxDB.

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

Details TK ([#1](https://github.com/cdzombak/midea2influx/issues/1)); for now see `config.example.json`.

## License

MIT; see [`LICENSE`](LICENSE) in this repository.

## Author

Chris Dzombak.

- [dzombak.com](https://www.dzombak.com)
- [github.com/cdzombak](https://www.github.com/cdzombak)
