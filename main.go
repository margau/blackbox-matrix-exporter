package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
)

var (
	c      = &Config{}
	logger log.Logger
)

type DNSProbe struct {
	SOAInconsistencyFail bool `yaml:"soa-inconsistency-fail,omitempty"`
}

type Matrix struct {
	Prober    string   `yaml:"prober,omitempty"`
	Instances []string `yaml:"instances,omitempty"`
	DNS       DNSProbe `yaml:"dns,omitempty"`
	Names     []string `yaml:"names,omitempty"`
}

type Config struct {
	Matrixes map[string]Matrix `yaml:"matrixes"`
}

func loadConfig() (err error) {

	yamlReader, err := os.Open("blackbox-matrix.yaml")
	if err != nil {
		return fmt.Errorf("error reading config file: %s", err)
	}
	defer yamlReader.Close()
	decoder := yaml.NewDecoder(yamlReader)
	decoder.KnownFields(true)

	if err = decoder.Decode(c); err != nil {
		return fmt.Errorf("error parsing config file: %s", err)
	}

	return nil
}

func probeHandler(w http.ResponseWriter, r *http.Request) {
	matrix := r.URL.Query().Get("matrix")
	if matrix == "" {
		http.Error(w, "No matrix configuration given", http.StatusBadRequest)
		return
	}

	_, ok := c.Matrixes[matrix]

	// Check if matrix is configured
	if ok == false {
		//do something here
		http.Error(w, "Matrix Configuration not found", http.StatusBadRequest)
		return
	}

	// Build Gauges to return, inspired by the upstream blackbox exporter
	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	start := time.Now()

	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)

	// TODO: Execute Probe

	duration := time.Since(start).Seconds()
	probeDurationGauge.Set(duration)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func main() {
	os.Exit(run())
}

func run() int {
	logger = log.NewLogfmtLogger(os.Stderr)
	logger = level.NewFilter(logger, level.AllowDebug())
	logger = log.With(logger, "ts", log.DefaultTimestampUTC)

	if err := loadConfig(); err != nil {
		level.Error(logger).Log("msg", "Error loading config", "err", err)
		return 1
	}
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})
	http.ListenAndServe(":9999", nil) // TODO: Reserve Port in prometheus exporter port list
	return 0
}
