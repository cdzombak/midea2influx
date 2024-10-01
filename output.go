package main

import "log"

func DebugLogger(debug bool) func(m string) {
	if debug {
		return func(m string) {
			log.Printf("[DEBUG] %s", m)
		}
	} else {
		return func(m string) {}
	}
}
