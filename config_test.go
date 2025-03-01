package main

import (
	"reflect"
	"testing"
)

func Test_loadConfig(t *testing.T) {
	tests := []struct {
		Name     string
		In       string
		Expected *Config
	}{
		{
			Name:     "basic",
			In:       "ColorizeOutput=true",
			Expected: &Config{ColorizeOutput: true},
		},
		{
			Name:     "basic with comment",
			In:       "# Some Comment.\n ColorizeOutput=true\n",
			Expected: &Config{ColorizeOutput: true},
		},
		{
			Name:     "basic unknown field",
			In:       "SomeNewField=true",
			Expected: &Config{ColorizeOutput: false},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			newConfig := &Config{}
			err := loadConfig(newConfig, WithConfigString(test.In))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(test.Expected, newConfig) {
				t.Fatalf("expected %#v got %#v", test.Expected, newConfig)
			}
		})
	}
}
