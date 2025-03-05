package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// Config contains persistent configuration for the program.
type Config struct {
	// ColorizeOutput toggles colorized output. Ex red for fail and green for pass.
	ColorizeOutput bool
}

// config contains the default configuration for the program.
// This can be overridden by a config file.
var globalConfig = &Config{
	ColorizeOutput: true, // Default to on for colorized output.
}

type configOptions struct {
	// configData is the raw input that the config will load from.
	configData io.Reader
}

// ConfigOption is used to set custom configuration values.
type ConfigOption func(*configOptions) error

// WithConfigString sets a config via a provided string.
func WithConfigString(c string) ConfigOption {
	return func(co *configOptions) error {
		co.configData = strings.NewReader(c)

		return nil
	}
}

// WithDefaultConfig loads the users default config dir and file.
func WithDefaultConfig() ConfigOption {
	return func(co *configOptions) error {
		// get xdg config dir
		configDir, err := os.UserConfigDir()
		if err != nil {
			return err
		}

		configPath := filepath.Join(configDir, "go-test", "config")
		if *verbose {
			fmt.Println("Loading config from", configPath)
		}

		f, err := os.Open(configPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			// Create directory if it doesn't exist
			err = os.MkdirAll(filepath.Dir(configPath), 0755)
			if err != nil {
				panic(err)
			}

			f, err = os.Create(configPath)
			if err != nil {
				panic(err)
			}

			co.configData = f

			return nil
		case err != nil:
			return err
		default:
			co.configData = f
		}

		return nil
	}
}

func loadConfig(config *Config, cOpts ...ConfigOption) error {
	var opts configOptions
	for _, co := range cOpts {
		err := co(&opts)
		if err != nil {
			return err
		}
	}

	// get the reflect value of the config struct so we can set the fields
	v := reflect.ValueOf(config)
	elem := v.Elem()

	// read file line by line
	scnr := bufio.NewScanner(opts.configData)
	line := -1
	for scnr.Scan() {
		line++

		txt := scnr.Text()
		switch {
		// Comment or empty line
		case txt[0] == '#' || txt == "":
			continue
		default:
			// parse the line
			parts := strings.Split(txt, "=")
			if len(parts) != 2 {
				fmt.Printf("Invalid format on line %d\n", line)
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])

			f := elem.FieldByName(key)

			if !f.IsValid() {
				fmt.Printf("Unkown setting %s\n", key)
				continue
			}

			switch f.Kind() {
			case reflect.Bool:
				b, err := strconv.ParseBool(val)
				if err != nil {
					fmt.Printf("Invalid bool value '%s' for %s\n", val, key)
					continue
				}

				f.SetBool(b)
			}
		}
	}

	return nil
}
