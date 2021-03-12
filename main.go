package main

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gocarina/gocsv"
)

//go:embed Ride_2019-04-28_ebike-connect.gpx
var gpxFile []byte

type gpxStruct struct {
	Trk struct {
		Name   string `xml:"name"`
		TrkSeg []struct {
			TrkPt []struct {
				Lat  float64 `xml:"lat,attr"`
				Lon  float64 `xml:"lon,attr"`
				Ele  float64 `xml:"ele"`
				Time string  `xml:"time"`
			} `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

func reverseHaversine(lat, lon, bearing, distance float64) (float64, float64) {
	R := 6378.137 * 1000 // Earth
	lat1 := lat * math.Pi / 180.0
	lon1 := lon * math.Pi / 180.0
	theta := bearing * math.Pi / 180.0

	lat2 := math.Asin(math.Sin(lat1)*math.Cos(distance/R) +
		math.Cos(lat1)*math.Sin(distance/R)*math.Cos(theta))

	lon2 := lon1 + math.Atan2(math.Sin(theta)*math.Sin(distance/R)*math.Cos(lat1),
		math.Cos(distance/R)-math.Sin(lat1)*math.Sin(lat2))

	return lat2 * 180.0 / math.Pi, lon2 * 180.0 / math.Pi
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	rlat1 := lat1 * math.Pi / 180.0
	rlon1 := lon1 * math.Pi / 180.0
	rlat2 := lat2 * math.Pi / 180.0
	rlon2 := lon2 * math.Pi / 180.0
	R := 6378.137 * 1000.0 // Earth
	phi := math.Sin((rlat2 - rlat1) / 2.0)
	lambda := math.Sin((rlon2 - rlon1) / 2.0)
	return math.Asin(math.Sqrt(phi*phi+math.Cos(rlat1)*math.Cos(rlat2)*lambda*lambda)) * 2 * R
}

type CSV struct {
	Time            int64   `csv:"time"`
	Lat             float64 `csv:"lat"`
	Lon             float64 `csv:"lon"`
	Speed           float64 `csv:"speed"`
	Altitude        float64 `csv:"altitude"`
	Cadence         float64 `csv:"riderCadence"`
	RemainingEnergy float64 `csv:"remainingEnergy"`
	Power           float64 `csv:"power"`
}

func main() {
	csvChan := make(chan CSV)
	endChan := make(chan struct{})
	gpx := gpxStruct{}
	err := xml.Unmarshal(gpxFile, &gpx)
	if err != nil {
		_ = fmt.Errorf("%v", err)
	}
	if len(gpx.Trk.TrkSeg) == 0 {
		_ = fmt.Errorf("No TrkSeg")
	}
	if len(gpx.Trk.TrkSeg[0].TrkPt) == 0 {
		_ = fmt.Errorf("No TrkSeg")
	}

	last := gpx.Trk.TrkSeg[0].TrkPt[0]
	lastTime, err := time.Parse(time.RFC3339, last.Time)
	if err != nil {
		_ = fmt.Errorf("Couldn't parse %s", last.Time)
	}
	go func(csvChan chan CSV) {
		csvList := []CSV{}

		for csv := range csvChan {
			csvList = append(csvList, csv)
		}

		fh, err := os.Create("output.csv")
		if err != nil {
			fmt.Errorf("could not create %v", err)
		}
		gocsv.MarshalFile(&csvList, fh)
		fh.Close()
		endChan <- struct{}{}
	}(csvChan)
	for _, pt := range gpx.Trk.TrkSeg[0].TrkPt[1:] {
		t, err := time.Parse(time.RFC3339, pt.Time)
		if err != nil {
			_ = fmt.Errorf("Couldn't parse %s", pt.Time)
		}

		tdiff := t.Sub(lastTime)
		d := haversine(pt.Lat, pt.Lon, last.Lat, last.Lon)
		fmt.Printf("d = %fm , v = %f dt=%f\n", d, d/tdiff.Hours()/1000.0, tdiff.Seconds())

		for i := 0; i < int(tdiff.Seconds()); i++ {
			lat := last.Lat + float64(i)*(pt.Lat-last.Lat)
			lon := last.Lon + float64(i)*(pt.Lon-last.Lon)
			ele := last.Ele + float64(i)*(pt.Ele-last.Ele)
			csv := CSV{Time: t.Unix() + int64(i), Speed: d / tdiff.Hours() / 10., Lat: lat, Lon: lon,
				Altitude: ele}
			csvChan <- csv
		}
		last = pt
		lastTime = t
	}
	close(csvChan)
	<-endChan
}
