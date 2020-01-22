package xtime

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
)

// Duration just wraps time.Duration so we can get a better format when used
// with github.com/spf13/viper
type Duration time.Duration

// MarshalJSON implements json.Unmarshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(FormatDuration(time.Duration(d)))
}

// DecodeHook is a helper function to wrap StringToDurationHookFunc for viper.
// It should be used with viper.Unmarshal to translate environment variable to
// Duration
func DecodeHook(c *mapstructure.DecoderConfig) {
	if c.DecodeHook == nil {
		c.DecodeHook = StringToDurationHookFunc()
	} else {
		c.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			StringToDurationHookFunc(),
			c.DecodeHook,
		)
	}
}

// StringToDurationHookFunc is a helper function for mapstructure to turn string
// into Duration, it could be used independently with mapstructure, but normally
// use DecodeHook above with viper.Unmarshal is more convenient
func StringToDurationHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		if t != reflect.TypeOf(Duration(0)) {
			return data, nil
		}
		return time.ParseDuration(data.(string))
	}
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// FormatDuration removes unnecessary suffix from format
func FormatDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}

	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
