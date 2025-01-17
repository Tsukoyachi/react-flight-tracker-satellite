package service

import (
	"github.com/StartUpNationLabs/react-flight-tracker-satellite/celestrack"
	"github.com/StartUpNationLabs/react-flight-tracker-satellite/spacetrack"
	"github.com/joshuaferrara/go-satellite"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"slices"
	"sort"
)

func NewSatelliteService() *SatelliteService {
	spacetrackClient, err := spacetrack.New()
	if err != nil {
		log.Error(err)
	}
	err = spacetrackClient.Login()
	if err != nil {
		log.Error(err)

	}
	data, err := spacetrackClient.FetchData()
	if err != nil {
		log.Fatal(err)
	}

	calculatedMap := make(map[string]spacetrack.TLE)
	for _, tle := range data {
		if tle.NORAD_CAT_ID == "" {
			log.Fatalf("NORAD_CAT_ID is empty in %v", tle)
		}
		// check if the object id is already in the map
		if _, ok := calculatedMap[tle.NORAD_CAT_ID]; ok {
			log.Warnf("NORAD_CAT_ID %v is already in the map, latest version taken into account", tle.OBJECT_ID)
		}

		calculatedMap[tle.NORAD_CAT_ID] = tle

	}

	// fetch from celestrak
	celestrackClient := celestrack.New()
	celestrakData, err := celestrackClient.Scrap()
	if err != nil {
		log.Fatal(err)
	}
	addedSatellites := 0
	for k, v := range celestrakData {
		sat, ok := calculatedMap[k]
		if !ok {
			calculatedMap[k] = v
			addedSatellites++
			continue
		}
		sat.Group = append(sat.Group, v.Group...)
		calculatedMap[k] = sat
	}
	log.Info("Added satellites from celestrak: ", addedSatellites)

	log.Info("Initializing satellites for spg4")
	// initialize the satellites
	for k, v := range calculatedMap {
		goSat := GetSatFromTle(v.TLE_LINE1, v.TLE_LINE2)
		v.Sat = goSat
		calculatedMap[k] = v

	}
	// convert map into a sorted array
	dataArray := maps.Values(calculatedMap)
	// sort the array
	sort.Slice(dataArray, func(i, j int) bool {
		return dataArray[i].NORAD_CAT_ID < dataArray[j].NORAD_CAT_ID
	})

	// store the unique groups in an array
	groups := make([]string, 0)
	for _, v := range dataArray {
		for _, group := range v.Group {
			if !slices.Contains(groups, group) {
				groups = append(groups, group)
			}
		}
	}
	// sort the groups
	sort.Strings(groups)

	log.Info("Total groups: ", len(groups))

	log.Info("Total satellites: ", len(calculatedMap))
	return &SatelliteService{
		data:      calculatedMap,
		dataArray: dataArray,
		groups:    groups,
	}
}

func GetSatFromTle(line1 string, line2 string) satellite.Satellite {
	// Handle the NORAD ID in the TLE line
	tleLine1 := line1
	noradID := tleLine1[2:7]

	// If the NOARD ID begin with a letter it'll be ignored for the parsing but putted back in the response
	if !isNumeric(noradID) {
		tleLine1 = tleLine1[:2] + " " + noradID[1:] + tleLine1[7:]
	}

	goSat := satellite.TLEToSat(tleLine1, line2, satellite.GravityWGS84)
	if goSat.ErrorStr != "" {
		log.Error(goSat.ErrorStr)
	}
	return goSat
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
