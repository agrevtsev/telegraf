//go:generate ../../../tools/readme_config_includer/generator
package strings

import (
	_ "embed"
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/processors"
)

//go:embed sample.conf
var sampleConfig string

type Strings struct {
	Lowercase    []converter `toml:"lowercase"`
	Uppercase    []converter `toml:"uppercase"`
	Titlecase    []converter `toml:"titlecase"`
	Trim         []converter `toml:"trim"`
	TrimLeft     []converter `toml:"trim_left"`
	TrimRight    []converter `toml:"trim_right"`
	TrimPrefix   []converter `toml:"trim_prefix"`
	TrimSuffix   []converter `toml:"trim_suffix"`
	Replace      []converter `toml:"replace"`
	Left         []converter `toml:"left"`
	Base64Decode []converter `toml:"base64decode"`
	ValidUTF8    []converter `toml:"valid_utf8"`

	converters []converter
	init       bool
}

type convertFunc func(s string) string

type converter struct {
	Field       string `toml:"field"`
	FieldKey    string `toml:"field_key"`
	Tag         string `toml:"tag"`
	TagKey      string `toml:"tag_key"`
	Measurement string `toml:"measurement"`
	Dest        string `toml:"dest"`
	Cutset      string `toml:"cutset"`
	Suffix      string `toml:"suffix"`
	Prefix      string `toml:"prefix"`
	Old         string `toml:"old"`
	New         string `toml:"new"`
	Width       int    `toml:"width"`
	Replacement string `toml:"replacement"`

	fn convertFunc
}

func (*Strings) SampleConfig() string {
	return sampleConfig
}

func (s *Strings) Apply(in ...telegraf.Metric) []telegraf.Metric {
	s.initOnce()

	for _, metric := range in {
		for _, converter := range s.converters {
			converter.convert(metric)
		}
	}

	return in
}

func (c *converter) convertTag(metric telegraf.Metric) {
	var tags map[string]string
	if c.Tag == "*" {
		tags = metric.Tags()
	} else {
		tags = make(map[string]string)
		tv, ok := metric.GetTag(c.Tag)
		if !ok {
			return
		}
		tags[c.Tag] = tv
	}

	for dest, value := range tags {
		if c.Tag != "*" && c.Dest != "" {
			dest = c.Dest
		}
		metric.AddTag(dest, c.fn(value))
	}
}

func (c *converter) convertTagKey(metric telegraf.Metric) {
	var tags map[string]string
	if c.TagKey == "*" {
		tags = metric.Tags()
	} else {
		tags = make(map[string]string)
		tv, ok := metric.GetTag(c.TagKey)
		if !ok {
			return
		}
		tags[c.TagKey] = tv
	}

	for key, value := range tags {
		if k := c.fn(key); k != "" {
			metric.RemoveTag(key)
			metric.AddTag(k, value)
		}
	}
}

func (c *converter) convertField(metric telegraf.Metric) {
	var fields map[string]interface{}
	if c.Field == "*" {
		fields = metric.Fields()
	} else {
		fields = make(map[string]interface{})
		fv, ok := metric.GetField(c.Field)
		if !ok {
			return
		}
		fields[c.Field] = fv
	}

	for dest, value := range fields {
		if c.Field != "*" && c.Dest != "" {
			dest = c.Dest
		}
		if fv, ok := value.(string); ok {
			metric.AddField(dest, c.fn(fv))
		}
	}
}

func (c *converter) convertFieldKey(metric telegraf.Metric) {
	var fields map[string]interface{}
	if c.FieldKey == "*" {
		fields = metric.Fields()
	} else {
		fields = make(map[string]interface{})
		fv, ok := metric.GetField(c.FieldKey)
		if !ok {
			return
		}
		fields[c.FieldKey] = fv
	}

	for key, value := range fields {
		if k := c.fn(key); k != "" {
			metric.RemoveField(key)
			metric.AddField(k, value)
		}
	}
}

func (c *converter) convertMeasurement(metric telegraf.Metric) {
	if metric.Name() != c.Measurement && c.Measurement != "*" {
		return
	}

	metric.SetName(c.fn(metric.Name()))
}

func (c *converter) convert(metric telegraf.Metric) {
	if c.Field != "" {
		c.convertField(metric)
	}

	if c.FieldKey != "" {
		c.convertFieldKey(metric)
	}

	if c.Tag != "" {
		c.convertTag(metric)
	}

	if c.TagKey != "" {
		c.convertTagKey(metric)
	}

	if c.Measurement != "" {
		c.convertMeasurement(metric)
	}
}

func (s *Strings) initOnce() {
	if s.init {
		return
	}

	s.converters = make([]converter, 0)
	for _, c := range s.Lowercase {
		c.fn = strings.ToLower
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Uppercase {
		c.fn = strings.ToUpper
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Titlecase {
		c.fn = func(s string) string {
			return cases.Title(language.Und, cases.NoLower).String(s)
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Trim {
		if c.Cutset != "" {
			c.fn = func(s string) string { return strings.Trim(s, c.Cutset) }
		} else {
			c.fn = func(s string) string { return strings.TrimFunc(s, unicode.IsSpace) }
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.TrimLeft {
		if c.Cutset != "" {
			c.fn = func(s string) string { return strings.TrimLeft(s, c.Cutset) }
		} else {
			c.fn = func(s string) string { return strings.TrimLeftFunc(s, unicode.IsSpace) }
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.TrimRight {
		if c.Cutset != "" {
			c.fn = func(s string) string { return strings.TrimRight(s, c.Cutset) }
		} else {
			c.fn = func(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) }
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.TrimPrefix {
		c.fn = func(s string) string { return strings.TrimPrefix(s, c.Prefix) }
		s.converters = append(s.converters, c)
	}
	for _, c := range s.TrimSuffix {
		c.fn = func(s string) string { return strings.TrimSuffix(s, c.Suffix) }
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Replace {
		c.fn = func(s string) string {
			newString := strings.ReplaceAll(s, c.Old, c.New)
			if newString == "" {
				return s
			}

			return newString
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Left {
		c.fn = func(s string) string {
			if len(s) < c.Width {
				return s
			}

			return s[:c.Width]
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.Base64Decode {
		c.fn = func(s string) string {
			data, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return s
			}
			if utf8.Valid(data) {
				return string(data)
			}
			return s
		}
		s.converters = append(s.converters, c)
	}
	for _, c := range s.ValidUTF8 {
		c.fn = func(s string) string { return strings.ToValidUTF8(s, c.Replacement) }
		s.converters = append(s.converters, c)
	}

	s.init = true
}

func init() {
	processors.Add("strings", func() telegraf.Processor {
		return &Strings{}
	})
}
