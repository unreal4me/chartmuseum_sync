package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
)

type ChartVersion struct {
	Version string `json:"version"`
}

type ChartData map[string][]ChartVersion

func fetchCharts(url string) (ChartData, error) {
	resp, err := http.Get(url + "/api/charts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data ChartData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func compareCharts(data1, data2 ChartData) map[string][]string {
	diff := make(map[string][]string)
	for chart, versions1 := range data1 {
		versionSet2 := make(map[string]struct{})
		if versions2, exists := data2[chart]; exists {
			for _, v := range versions2 {
				versionSet2[v.Version] = struct{}{}
			}
		}
		for _, v := range versions1 {
			if _, found := versionSet2[v.Version]; !found {
				diff[chart] = append(diff[chart], v.Version)
			}
		}
	}
	return diff
}

func syncCharts(server1, server2 string) {
	data1, err1 := fetchCharts(server1)
	data2, err2 := fetchCharts(server2)
	if err1 != nil || err2 != nil {
		fmt.Println("Error fetching charts:", err1, err2)
		return
	}

	diff := compareCharts(data1, data2)

	totalCharts := 0
	for _, versions := range diff {
		totalCharts += len(versions)
	}

	bar := progressbar.Default(int64(totalCharts), "Syncing Charts")
	chartsSynced := 0

	for chart, versions := range diff {
		for _, version := range versions {
			chartURL := fmt.Sprintf("%s/charts/%s-%s.tgz", server1, chart, version)
			resp, err := http.Get(chartURL)
			if err != nil || resp.StatusCode != 200 {
				fmt.Printf("Failed to fetch %s-%s from %s %v\n", chart, version, server1, err)
				continue
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Printf("Failed to read %s-%s %v\n", chart, version, err)
				continue
			}

			postURL := server2 + "/api/charts"
			req, err := http.NewRequest("POST", postURL, bytes.NewReader(data))
			if err != nil {
				fmt.Printf("Failed to create request for %s-%s %v\n", chart, version, err)
				continue
			}
			req.Header.Set("Content-Type", "application/gzip")
			client := &http.Client{}
			resp, err = client.Do(req)
			if err != nil || resp.StatusCode != 201 {
				fmt.Printf("Failed to sync %s-%s to %s %v\n", chart, version, server2, err)
			} else {
				//fmt.Printf("Successfully synced %s-%s to %s\n", chart, version, server2)
				chartsSynced++

				bar.Describe(chart + "-" + version)
				bar.Add(1)
			}
			resp.Body.Close()
		}
	}
}

func checkInfoEndpoint(u string) error {
	resp, err := http.Get(u)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return fmt.Errorf("error decoding JSON: %w", err)
	}

	if _, ok := data["version"]; !ok {
		return fmt.Errorf("missing 'version' key in JSON")
	}

	return nil
}

func main() {

	source := flag.String("s", "http://localhost:8080", "source, a valid chartmuseum url")
	destination := flag.String("d", "http://localhost:8080", "destination, a valid chartmuseum url")

	flag.Parse()
	if *source == "http://localhost:8080" && *destination == "http://localhost:8080" {
		fmt.Println("You must have at least one source or one destination.")
		fmt.Println("cm_sync -s http://source_url -d http://destination_url")
		fmt.Println("if you omit either of them, http://localhost:8080 will be used instead")
		fmt.Println("cm_sync -s http://source_url (*implies -d http://localhost:8080)")
		fmt.Println("---")
		fmt.Println("chartmuseum --storage local --storage-local-rootdir /tmp/chartmuseum/ --port 8080")
		flag.Usage()
		os.Exit(1)
	}

	if err := checkInfoEndpoint(*source + "/info"); err != nil {
		fmt.Println("Error checking source:", *source+"/info", "\n", err)
		os.Exit(1)
	}

	if err := checkInfoEndpoint(*destination + "/info"); err != nil {
		fmt.Println("Error checking destination:", *destination+"/info", "\n", err)
		os.Exit(1)
	}

	syncCharts(*source, *destination)
}
