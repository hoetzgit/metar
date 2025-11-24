## UTIL: update METAR stations
`updateStations.go` updates the METAR stations list.

It combines lists from [aviationweather.gov](https://aviationweather.gov/data/api/) and [ourairports.com](https://ourairports.com/data/) web sites.

Change the `dataFile` variable to the actual path where your `data.go` file lives, typically `path/to/the/metar/data/data.go`. The program will insert the updated data into `data.go`

```
const (
	// CAUTION ! Change dataFile path to where this data.go lives
	dataFile     string = "/home/jeanluc/golang/src/jeanluc/metarDEV/data/data.go"
	airportsURL  string = "https://davidmegginson.github.io/ourairports-data/airports.csv"
	countriesURL string = "https://davidmegginson.github.io/ourairports-data/countries.csv"

	// List all airports (NOT recommended - large file). `false` : only take medium and large size airports
	listAllAirports bool = true
)
```

Once updated, you will need to recompile or run `metar.go` to hardcode the updated stations into the metar binary.

__Warning__: do not change the var declaration `var CountryList` in `data.go` as it works as a marker for this program.
