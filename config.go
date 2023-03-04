package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/knadh/koanf/maps"
	yaml "gopkg.in/yaml.v3"
)

type config struct {
	ListenPort        int                       `yaml:"listenPort"`
	ListenAddress     string                    `yaml:"listenAddress"`
	ServerConfigs     []*serverConfig           `yaml:"serverConfigs"`
	UserDataTemplates map[string]map[string]any `yaml:"userDataTemplates"`
}

type serverConfig struct {
	Name             string          `yaml:"name"`
	MatchPatterns    []string        `yaml:"matchPatterns"`
	InstanceConfig   *instanceConfig `yaml:"instanceConfig"`
	UserDataTemplate string          `yaml:"userDataTemplate"`
	Replacements     map[string]any  `yaml:"replacements"`

	compiledMatchers []*regexp.Regexp
	renderedUserData []byte
}

type instanceConfig struct {
	Hostname               string `yaml:"hostname"`
	EnableInstanceIDSuffix bool   `yaml:"enableInstanceIDSuffix"`
	EnableHostnameSuffix   bool   `yaml:"enableHostnameSuffix"`
	GeneratedSuffixSize    int    `yaml:"hostnameSuffixSize"`
}

type metaData struct {
	InstanceID    string `yaml:"instance-id"`
	LocalHostname string `yaml:"local-hostname"`
	Hostname      string `yaml:"hostname"`
}

const (
	defaultListenAddress = "0.0.0.0"
	defaultListenPort    = 8000
	defaultSuffixLength  = 4
)

func loadConfig(path string) (*config, error) {
	by, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg config
	if err := yaml.Unmarshal(by, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.ServerConfigs) == 0 {
		return nil, fmt.Errorf("config file %q has no serving configurations", path)
	}
	for _, c := range cfg.ServerConfigs {
		if err := c.loadMatchers(); err != nil {
			return nil, fmt.Errorf("config %q has invalid matchers: %w", c.Name, err)
		}
		if c.InstanceConfig == nil {
			return nil, fmt.Errorf("config %q does not have an instanceConfig set", c.Name)
		}
		if err := c.InstanceConfig.validate(); err != nil {
			return nil, fmt.Errorf("invalid instance config: %w", err)
		}
		if c.UserDataTemplate == "" && len(c.Replacements) > 0 {
			return nil, fmt.Errorf("replacers can only be configured when referencing a user data template")
		}
		userData, ok := cfg.UserDataTemplates[c.UserDataTemplate]
		if ok {
			clone := maps.Copy(userData)
			if len(c.Replacements) > 0 {
				maps.Merge(c.Replacements, clone)
			}
			by, err := yaml.Marshal(clone)
			if err != nil {
				return nil, fmt.Errorf("render user data after replacements: %w", err)
			}
			c.renderedUserData = by
		}
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = defaultListenAddress
	}
	if cfg.ListenPort == 0 {
		cfg.ListenPort = defaultListenPort
	}
	return &cfg, nil
}

func (c config) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, s := range c.ServerConfigs {
		if s.Match(r.URL.Path) {
			log.Printf("%s: matched by %s", r.URL.Path, s.Name)
			s.ServeHTTP(w, r)
			return
		}
	}

	http.NotFound(w, r)
}

func (c *serverConfig) loadMatchers() error {
	if len(c.MatchPatterns) == 0 {
		return fmt.Errorf("no matchers specified")
	}
	for _, m := range c.MatchPatterns {
		re, err := regexp.Compile(m)
		if err != nil {
			return fmt.Errorf("compile pattern %q: %w", m, err)
		}
		c.compiledMatchers = append(c.compiledMatchers, re)
	}
	return nil
}

func (c *serverConfig) Match(s string) bool {
	for _, re := range c.compiledMatchers {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func (c serverConfig) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	split := strings.Split(r.URL.Path, "/")
	switch suffix := split[len(split)-1]; suffix {
	case "meta-data":
		serial := split[len(split)-2]
		by, err := c.InstanceConfig.RenderMetaData(serial)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(by)
	case "user-data":
		w.Write(c.renderedUserData)
	case "vendor-data":
		break
	default:
		http.NotFound(w, r)
	}
}

func (c *instanceConfig) RenderMetaData(serial string) ([]byte, error) {
	md := metaData{
		InstanceID:    "i-" + serial,
		Hostname:      c.Hostname,
		LocalHostname: c.Hostname,
	}
	var suffix string
	if c.EnableHostnameSuffix || c.EnableInstanceIDSuffix {
		s, err := genSuffix(c.GeneratedSuffixSize)
		if err != nil {
			return nil, fmt.Errorf("generate suffix: %w", err)
		}
		suffix = s
	}
	if c.EnableHostnameSuffix {
		md.Hostname += suffix
		md.LocalHostname += suffix
	}
	if c.EnableInstanceIDSuffix {
		md.InstanceID += suffix
	}
	return yaml.Marshal(md)
}

func genSuffix(n int) (string, error) {
	if n <= 0 {
		n = defaultSuffixLength
	}
	by := make([]byte, n)
	if _, err := rand.Read(by); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return "-" + hex.EncodeToString(by), nil
}

func (c *instanceConfig) validate() error {
	if c.Hostname == "" {
		return fmt.Errorf("hostname field must be set")
	}
	return nil
}
