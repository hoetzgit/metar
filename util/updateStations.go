// This program updates the METAR stations list.

// It compiles lists from NOAA and ourairports.com web sites.
// Change `dataFile` assigment here below to the actual path where your `data.go` file lives.
// Once updated, you will need to recompile or run metar.go to hardcode the updated stations into the metar binary.

// Warning: do not change the var declaration `var CountryList` in data.go as it works as a marker for this program.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Station stores METAR Station details
type Station struct {
	name, airportSize, icao, iata, country string
	lat, long                              float64
}

type Countries struct {
	countryCode, countryName, continentCode string
}

const (
	// CAUTION ! Change dataFile path to where this data.go lives
	dataFile     string = "/home/jeanluc/golang/src/jeanluc/metar/data/data.go"
	airportsURL  string = "https://davidmegginson.github.io/ourairports-data/airports.csv"
	countriesURL string = "https://davidmegginson.github.io/ourairports-data/countries.csv"

	// List all airports (NOT recommended - large file). `false` : only take medium and large size airports
	listAllAirports bool = false
)

func main() {

	stationsOurAirports := make(map[string]Station, 10000) // Preallocate for large airport lists
	countries := make([]Countries, 0, 300)                 // Preallocate for all countries

	start := time.Now()

	// initialize an empty wait group
	wg := sync.WaitGroup{}

	// get and process airports csv file
	wg.Add(1)
	go func() {
		defer wg.Done()
		csvFile, err := wget(airportsURL, 5)
		if err != nil {
			log.Fatalf("\nError getting %s\n  %s\n\n", airportsURL, err)
		}

		// parse CSV file
		records, err := processCsvData(csvFile)
		if err != nil {
			log.Fatalf("\nError parsing data from %s\n  %s\n", airportsURL, err)
		}

		// Process parsed airports records
		for _, l := range records[1:] {

			// replace `"` by `\"` in airport name
			l[3] = strings.ReplaceAll(l[3], "\"", `\"`)

			// parse and conv coord. set lat and long to 999.0 if err (wito do later: use NOAA coord.)
			lt, errLt := strconv.ParseFloat(l[4], 64)
			lg, errLg := strconv.ParseFloat(l[5], 64)
			if errLt != nil || errLg != nil {
				lt, lg = 999.0, 999.0
			}

			// if `municipality` not empty replace `"` by `\"`
			if l[10] != "" {
				l[10] = strings.ReplaceAll(l[10], "\"", `\"`)
				l[10] = fmt.Sprintf(" (%s)", l[10])
			}
			stationsOurAirports[l[1]] = Station{
				name:        l[3] + l[10],
				airportSize: l[2],
				icao:        l[1],
				iata:        l[13],
				country:     l[8],
				lat:         lt,
				long:        lg,
			}
		}
	}()

	// get and process countries csv file
	wg.Add(1)
	go func() {

		defer wg.Done()
		csvFile, err := wget(countriesURL, 5)
		if err != nil {
			log.Fatalf("\nError getting %s\n  %s\n\n", countriesURL, err)
		}

		// parse CSV file
		records, err := processCsvData(csvFile)
		if err != nil {
			log.Fatalf("\nError parsing data from %s\n  %s\n", countriesURL, err)
		}

		// Process parsed countries records
		for _, l := range records[1:] {
			countries = append(countries,
				Countries{
					l[2],
					l[1],
					l[3],
				})

		}
	}()

	// wait for all go routines to finish
	wg.Wait()

	airports := make([]Station, 0, len(stationsOurAirports))
	for _, ad := range stationsOurAirports {
		if listAllAirports && ad.airportSize != "closed" {
			airports = append(airports, ad)
		} else if ad.airportSize == "medium_airport" || ad.airportSize == "large_airport" {
			airports = append(airports, ad)
		}
	}

	// Sort `countries` on country code
	sort.Slice(countries, func(i, j int) bool { return countries[i].countryCode < countries[j].countryCode })

	// Sort `airports` on icao code
	sort.Slice(airports, func(i, j int) bool { return airports[i].icao < airports[j].icao })

	// Use a buffer to batch file writes for speed
	var buf bytes.Buffer

	// delete lines after `var AdList = []string{`
	f, err := os.OpenFile(dataFile, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("\nError loading %s\n %s", dataFile, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	bytesRead := 0
	for scanner.Scan() {
		t := scanner.Text()
		bytesRead += len(t) + 1
		if strings.Contains(t, "var CountryList") {
			break
		}
	}

	err = f.Truncate(int64(bytesRead))
	if err != nil {
		log.Fatalf("Error truncating file %s\n %s\n\n", dataFile, err)
	}

	// Add countries to buffer
	for _, v := range countries {
		buf.WriteString(fmt.Sprintf("\t\"%s;%s;%s\",\n", v.countryName, v.countryCode, v.continentCode))
	}

	// Add header for airports
	buf.WriteString("}\n\n// AdList list of ICAO/IATA airport codes + description\nvar AdList = []string{\n")

	// Add all airports to buffer
	for _, v := range airports {
		buf.WriteString(fmt.Sprintf("\t\"%s;%s;%s;%s;%.3f;%.3f\",\n", v.icao, v.iata, v.name, v.country, v.lat, v.long))
	}

	buf.WriteString("}\n")

	// Write buffer to file in one go
	if _, err := f.Write(buf.Bytes()); err != nil {
		log.Fatalf("Error writing buffer to %s\n %s\n\n", dataFile, err)
	}

	fmt.Printf(
		"\n %d records updated in %.3f sec.\n you can now recompile metar.go with the updated stations.\n\n",
		len(airports),
		time.Since(start).Seconds(),
	)

}

/*
	FUNC's
*/

func processCsvData(csvFile string) ([][]string, error) {
	// make a new reader from string `s`
	r := csv.NewReader(strings.NewReader(csvFile))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	return records, err

}

func wget(urlString string, wgetTimeout int) (string, error) {
	timeout := time.Duration(wgetTimeout) * time.Second
	client := http.Client{Timeout: timeout}

	// Get page and check for error (timeout, http ...)
	res, err := client.Get(urlString)

	// unwrap url.error and check error and its type
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// if not HTTP 200 OK in response header
	if res.StatusCode != http.StatusOK {
		err := new(url.Error)
		*err = url.Error{
			Op:  "Get",
			URL: urlString,
			Err: fmt.Errorf("HTTP error: %s", http.StatusText(res.StatusCode)),
		}
		return "", err
	}

	// return res.Body, nil
	wgetAnswer, err := io.ReadAll(res.Body)
	if err != nil {
		err := new(url.Error)
		*err = url.Error{
			Op:  "Get",
			URL: urlString,
			Err: fmt.Errorf("Error reading response body"),
		}
	}

	// return output (after removing trailing \n)
	return string(wgetAnswer[:len(wgetAnswer)-1]), nil
}

// uncompress gziped string
// return guziped string and error
func gunzipGz(gzString string) (string, error) {

	reader := bytes.NewReader([]byte(gzString))
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return "", err
	}

	// err != io.ErrUnexpectedEOF to get rid of the "unexpected EOF" normal error
	output, err := io.ReadAll(gzReader)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}

	return string(output), nil
}
