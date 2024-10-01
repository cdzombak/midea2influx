package main

import (
	"fmt"
	"strconv"
	"strings"
)

func ConvBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", s)
	}
}

func ConvFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}
