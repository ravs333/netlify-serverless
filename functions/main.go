package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server struct {
		// Host is the local machine IP Address to bind the HTTP Server to
		Host string `yaml:"host"`

		// Port is the local machine TCP Port to bind the HTTP Server to
		Port    string `yaml:"port"`
		Timeout struct {
			// Server is the general server timeout to use
			// for graceful shutdowns
			Server time.Duration `yaml:"server"`

			// Write is the amount of time to wait until an HTTP server
			// write opperation is cancelled
			Write time.Duration `yaml:"write"`

			// Read is the amount of time to wait until an HTTP server
			// read operation is cancelled
			Read time.Duration `yaml:"read"`

			// Read is the amount of time to wait
			// until an IDLE HTTP session is closed
			Idle time.Duration `yaml:"idle"`
		} `yaml:"timeout"`
	} `yaml:"server"`
	API struct {
		Url   string `yaml:"url"`
		Token string `yaml:"token"`
	} `yaml:"api"`
}

// Struct to represent the JSON data
type BodyJson struct {
	FacilityCode        string `json:"facilityCode"`
	WaitTimeInSeconds   string `json:"waitTimeInSeconds"`
	DataCaptureDateTime string `json:"dataCaptureDateTime"`
}

type rss struct {
	Version     string    `xml:"version,attr"`
	XHTML       RSSXHTML  `xml:"channel>xhtml:meta"`
	Title       string    `xml:"channel>title"`
	Description string    `xml:"channel>description"`
	Items       []RSSItem `xml:"channel>item"`
}

type RSSXHTML struct {
	XMLName xml.Name `xml:"xhtml:meta"`
	Xmlns   string   `xml:"xmlns:xhtml,attr"`
	Name    string   `xml:"name,attr"`
	Title   string   `xml:"content,attr"`
}

type RSSItem struct {
	XMLName     xml.Name `xml:"item"`
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	Time        string   `xml:"time"`
}

var apiURL = ""
var apiToken = ""

func erWaitTimes(w http.ResponseWriter, r *http.Request) {

	// fmt.Printf("Handling function with %s request\n", r.Method)
	// start := time.Now()

	if faccodes := r.URL.Query().Get("faccodes"); faccodes != "" {
		xml, err := getFacilityErWaitTimes(faccodes)
		if err != nil {
			log.Fatal(err)
		}
		// elapsed := time.Since(start).Seconds()
		// e := strconv.FormatFloat(elapsed, 'f', -1, 64)

		// fmt.Printf("Response Time: %s", e)
		w.Header().Set("Content-Type", "application/rss+xml")
		// w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding,User-Agent,UserAgent")
		// fmt.Println(xml)
		w.Write(xml)
		// fmt.Fprint(w, string(xml))
		// fmt.Fprintf(xml)
		// fmt.Println(xml)
	} else {
		// fmt.Fprint(w, "No Parameters Passed")
		log.Fatal("No Parameters Passed")
		// blankXML := ("<?xml version='1.0' encoding='UTF-8'?>
		// <rss version='2.0'>
		// 	<channel>
		// 		<xhtml:meta xmlns:xhtml='http://www.w3.org/1999/xhtml' name='robots' content='noindex' />
		// 		<title>ER Wait Time</title>
		// 		<description>RSS feed for ER wait time</description>
		// 	</channel>
		// </rss>")
		// fmt.Fprint(w, blankXML)
	}
}

func getFacilityErWaitTimes(faccodes string) ([]byte, error) {

	apiFinalUrl := apiURL + faccodes
	req, err := http.NewRequest("GET", apiFinalUrl, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// jsonData := []byte(bodyBytes)

	// Create an instance of the struct to hold the parsed data
	var bodyJson []BodyJson

	// Parse the JSON data into the struct
	errParse := json.Unmarshal(bodyBytes, &bodyJson)
	// if errParse != nil {
	// 	fmt.Println("Error:", errParse)
	// 	return errParse
	// }

	return renderXML(bodyJson), errParse
}

func timeout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Timeout attempt")
	time.Sleep(2 * time.Second)
	fmt.Fprint(w, "Did *not* timeout")
}

// use viper package to read .env file
// return the value of the key
func viperEnvVariable(key string) string {

	// SetConfigFile explicitly defines the path, name and extension of the config file.
	// Viper will use this and not check any of the config paths.
	// .env - It will search for the .env file in the current directory
	viper.SetConfigFile(".env")

	// Find and read the config file
	err := viper.ReadInConfig()

	if err != nil {
		log.Fatalf("Error while reading config file %s", err)
	}

	// viper.Get() returns an empty interface{}
	// to get the underlying type of the key,
	// we have to do the type assertion, we know the underlying value is string
	// if we type assert to other type it will throw an error
	value, ok := viper.Get(key).(string)

	// If the type is a string then ok will be true
	// ok will make sure the program not break
	if !ok {
		log.Fatalf("Invalid type assertion")
	}

	return value
}

func renderXML(erWaitTimes []BodyJson) []byte {
	// fmt.Println(erWaitTimes)

	w := &bytes.Buffer{}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")

	w.Write([]byte(xml.Header))

	rHead := &rss{
		Version: "2.0",
		XHTML: RSSXHTML{
			Xmlns: "http://www.w3.org/1999/xhtml",
			Name:  "robots",
			Title: "noindex",
		},
		Description: "RSS feed for ER wait time",
		Title:       "ER Wait Time",
		Items:       []RSSItem{},
	}

	for eWKey := range erWaitTimes {

		erWaitTime := reflect.ValueOf(erWaitTimes[eWKey])
		waitTime, _ := time.ParseDuration(erWaitTime.FieldByName("WaitTimeInSeconds").String() + "s")
		roundedUpWT := int64(math.Ceil(waitTime.Seconds() / 60))
		erItem := RSSItem{
			Title:       "Hospital Code " + erWaitTime.FieldByName("FacilityCode").String(),
			Description: fmt.Sprintf("%d Minutes", roundedUpWT),
			Time:        fmt.Sprintf("%d", roundedUpWT),
		}
		rHead.AddItem(erItem)
	}

	if err := enc.Encode(rHead); err != nil {
		fmt.Printf("error: %v\n", err)
	}

	formattedXML, fErr := SelfClosing(w.Bytes())
	if fErr != nil {
		fmt.Printf("error: %v\n", fErr)
	}
	// fmt.Println(string(formattedXML))
	return formattedXML
}

func SelfClosing(xml []byte) ([]byte, error) {
	regex, err := regexp.Compile(`></[A-Za-z0-9_:]+>`)
	return regex.ReplaceAll(xml, []byte("/>")), err
}

func (rss *rss) AddItem(item RSSItem) []RSSItem {
	rss.Items = append(rss.Items, item)
	return rss.Items
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	var configPath string

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

// NewRouter generates the router used in the HTTP Server
func NewRouter() *http.ServeMux {
	// Create router and define routes and return that router
	router := http.NewServeMux()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
	})

	router.HandleFunc("/billboard", erWaitTimes)
	router.HandleFunc("/timeout", timeout)

	return router
}

// Run will run the HTTP Server
func (config Config) Run() {
	// Set up a channel to listen to for interrupt signals
	var runChan = make(chan os.Signal, 1)

	// Set up a context to allow for graceful server shutdowns in the event
	// of an OS interrupt (defers the cancel just in case)
	ctx, cancel := context.WithTimeout(
		context.Background(),
		config.Server.Timeout.Server,
	)
	defer cancel()

	// Define server options
	server := &http.Server{
		Addr:         config.Server.Host + ":" + config.Server.Port,
		Handler:      NewRouter(),
		ReadTimeout:  config.Server.Timeout.Read * time.Second,
		WriteTimeout: config.Server.Timeout.Write * time.Second,
		IdleTimeout:  config.Server.Timeout.Idle * time.Second,
	}

	// Handle ctrl+c/ctrl+x interrupt
	signal.Notify(runChan, os.Interrupt, syscall.SIGTSTP)

	// Alert the user that the server is starting
	log.Printf("Server is starting on %s\n", server.Addr)

	// Run the server on a new goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				// Normal interrupt operation, ignore
			} else {
				log.Fatalf("Server failed to start due to err: %v", err)
			}
		}
	}()

	// Block on this channel listeninf for those previously defined syscalls assign
	// to variable so we can let the user know why the server is shutting down
	interrupt := <-runChan

	// If we get one of the pre-prescribed syscalls, gracefully terminate the server
	// while alerting the user
	log.Printf("Server is shutting down due to %+v\n", interrupt)
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server was unable to gracefully shutdown due to err: %+v", err)
	}
}

func main() {

	// Generate our config based on the config supplied
	// by the user in the flags
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	//Set Variables
	apiURL = cfg.API.Url
	apiToken = cfg.API.Token

	// Run the server
	cfg.Run()
}
