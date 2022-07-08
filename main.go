package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Sensor struct {
	Id string
	Name string
	Value string
	Unit string
}

const (
	defaultMetricsPath string = "/metrics"
	defaultPort uint16 = 2112
	scriptPath string = "scripts/gw"
	reportInterval time.Duration = 5 * time.Second
)

var (
	gwSensorGaugeMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gw_sensor",
			Help: "GoodWe sensors",
		},
		[]string{"id", "name"},
	)
	port uint16
	getSensorsCommand []byte = []byte("get_sensors\n")
)

func init() {
	prometheus.MustRegister(gwSensorGaugeMetric)
}

func main() {
	inverterIp := net.ParseIP(os.Getenv("INVERTER_IP"))

	if inverterIp == nil {
		log.Fatalf("Invalid inverter IP address (env: INVERTER_IP) %s", os.Getenv("INVERTER_IP"))
	}

	portUint64, err := strconv.ParseUint(os.Getenv("METRICS_PORT"), 10, 16)

	if err == nil {
		port = uint16(portUint64)
	} else {
		port = defaultPort
	}

	go handleInterrupt()
	go runPythonProcess()

	http.HandleFunc(
		"/",
		func(w http.ResponseWriter, req *http.Request) {
			if (req.URL.Path == "/") {
				http.Redirect(w, req, defaultMetricsPath, http.StatusTemporaryRedirect)
			} else {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("403 HTTP status code returned!"))
			}
		},
	)

	http.Handle(defaultMetricsPath, promhttp.Handler())

	log.Printf("Starting http server on port %v", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handleInterrupt() {
	log.Print("Handling SIGINT & SIGTERM signals started.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	for sig := range c {
        log.Printf("Captured %v, exiting...", sig)
		os.Exit(1)
    }
}

func runPythonProcess() {
	for {
		cmd := exec.Command(scriptPath)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("Error obtaining stdin: %s", err.Error())
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatalf("Error obtaining stdout: %s", err.Error())
		}
		
		reader := bufio.NewReader(stdout)

		err = cmd.Start()
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Process started with PID: %d", cmd.Process.Pid)

		ticker := time.NewTicker(reportInterval)
		done := make(chan bool)

		go func() {
			for {
				stdin.Write(getSensorsCommand)
		
				data, err := reader.ReadBytes('\n')
				if err == nil {
					reportData(data)
				} else {
					log.Printf("Unable to read bytes from stdout %s", err.Error())
				}

				reader.Reset(stdout)

				select {
					case <-done:
						return
					case <-ticker.C:
						continue
				}
			}
		}()

		err = cmd.Wait()
		if err != nil { 
			log.Print(err.Error())
		}
		log.Printf("Process with PID: %d exited with exit code: %d", cmd.ProcessState.Pid(), cmd.ProcessState.ExitCode())
		ticker.Stop()
		done <- true
		time.Sleep(reportInterval)
	}
}

func reportData(data []byte) {
	var sensors []Sensor
	err := json.Unmarshal(data, &sensors)
	if err != nil {
		log.Printf("Error during json deserialization %v", err.Error())
		log.Printf("Received response: %s", string(data))
		return
	}

	log.Print("Received ok response from process")

	for _, sensor := range sensors {
		value, err := strconv.ParseFloat(sensor.Value, 32);
		if err == nil {
			gwSensorGaugeMetric.WithLabelValues(sensor.Id, sensor.Name).Set(value)
		}
	}
}