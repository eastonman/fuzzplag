/*
 * @Author: Easton Man manyang.me@outlook.com
 * @Date: 2022-12-07 12:57:24
 * @LastEditors: Easton Man manyang.me@outlook.com
 * @LastEditTime: 2022-12-07 16:37:30
 * @FilePath: /fuzzplag/main.go
 * @Description: Main entry point
 */
package main

import (
	"encoding/csv"
	"os"
	"regexp"
	"strconv"

	"github.com/eastonman/fuzzplag/utils"
	"github.com/hbollon/go-edlib"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	var err error
	var logLevel = log.InfoLevel
	log.Info("Starting FuzzPlag")

	// Read config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./conf")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config: %s", err.Error())
	}

	// Setup logging
	log.SetLevel(logLevel)

	inputFilePath := viper.GetString("input.path")
	outputFilePath := viper.GetString("output.path")
	acceptPattern := viper.GetStringSlice("accept-patterns")
	ignorePattern := viper.GetStringSlice("ignore-patterns")

	utils.FileThreshold = viper.GetInt("smallfile-threshold")
	parallelNum := viper.GetInt("parallel")
	distanceThreshold := viper.GetInt("distance-threshold")

	// Build Regex
	ignoreEngine := make([]*regexp.Regexp, 0)
	acceptEngine := make([]*regexp.Regexp, 0)
	for _, p := range ignorePattern {
		r, err := regexp.Compile(p)
		if err != nil {
			log.Fatalf("Error compiling regex %s: %s", p, err.Error())
		}
		ignoreEngine = append(ignoreEngine, r)
	}
	for _, p := range acceptPattern {
		r, err := regexp.Compile(p)
		if err != nil {
			log.Fatalf("Error compiling regex %s: %s", p, err.Error())
		}
		acceptEngine = append(acceptEngine, r)
	}

	hash := utils.HashForZip(inputFilePath, parallelNum)

	log.Infof("Total files: %d", len(hash))

	filteredHash := make([]utils.Hash, 0)
	for _, h := range hash {
		ignore := true
		for _, r := range acceptEngine {
			if r.MatchString(h.Path) {
				ignore = false
				break
			}
		}
		if ignore {
			continue
		}
		for _, r := range ignoreEngine { // March ignorePattern
			if r.MatchString(h.Path) {
				ignore = true
				break
			}
		}
		if !ignore {
			filteredHash = append(filteredHash, h)
		}
	}

	log.Infof("Total files after filter: %d", len(filteredHash))

	outputFile, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		log.Fatalf("Error opening file %s: %s", outputFilePath, err.Error())
	}
	defer outputFile.Close()
	outputWriter := csv.NewWriter(outputFile)
	outputWriter.Write([]string{"Source", "Dest", "Distance"}) // Header
	defer outputWriter.Flush()
	for _, a := range filteredHash {
		for _, b := range filteredHash {
			if a.Path[0:9] == b.Path[0:9] { // Ignore same person
				continue
			}
			distance, err := edlib.HammingDistance(a.Hash, b.Hash)
			if err != nil {
				log.Warnf("Error computing distance: %s", err.Error())
			}

			if distance <= distanceThreshold {
				err = outputWriter.Write([]string{a.Path, b.Path, strconv.Itoa(distance)})
				if err != nil {
					log.Fatalf("Error writing to file %s: %s", outputFilePath, err.Error())
				}
			}
		}
	}

}