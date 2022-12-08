/*
 * @Author: Easton Man manyang.me@outlook.com
 * @Date: 2022-12-07 12:57:24
 * @LastEditors: Easton Man manyang.me@outlook.com
 * @LastEditTime: 2022-12-08 10:05:53
 * @FilePath: /fuzzplag/main.go
 * @Description: Main entry point
 */
package main

import (
	"encoding/csv"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/eastonman/fuzzplag/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type outputRow struct {
	Source   string
	Dest     string
	Distance string
}
type output struct {
	Rows []outputRow
}

func (o *output) Len() int {
	return len(o.Rows)
}

func (o *output) Swap(i, j int) {
	t := o.Rows[i]
	o.Rows[i] = o.Rows[j]
	o.Rows[j] = t
}

func (o *output) Less(i, j int) bool {
	return o.Rows[i].Source < o.Rows[j].Source
}

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

	viper.SetDefault("patterns.binary", "^$")
	viper.SetDefault("patterns.text", "^$")
	viper.SetDefault("patterns.ignore", "^$")

	inputFilePath := viper.GetString("input.path")
	outputFilePath := viper.GetString("output.path")
	textPattern := viper.GetString("patterns.text")
	binaryPattern := viper.GetString("patterns.binary")
	ignorePattern := viper.GetStringSlice("patterns.ignore")

	utils.FileThreshold = viper.GetInt("threshold.smallfile")
	parallelNum := viper.GetInt("parallel")
	textThreshold := viper.GetInt("threshold.text")
	binaryThreshold := viper.GetInt("threshold.binary")

	// Build Regex
	ignoreEngine := make([]*regexp.Regexp, 0)
	textEngine, err := regexp.Compile(textPattern)
	if err != nil {
		log.Fatalf("Error compiling regex %s: %s", textPattern, err.Error())
	}
	binaryEngine, err := regexp.Compile(binaryPattern)
	if err != nil {
		log.Fatalf("Error compiling regex %s: %s", binaryPattern, err.Error())
	}
	for _, p := range ignorePattern {
		r, err := regexp.Compile(p)
		if err != nil {
			log.Fatalf("Error compiling regex %s: %s", p, err.Error())
		}
		ignoreEngine = append(ignoreEngine, r)
	}

	hash := utils.HashForZip(inputFilePath, parallelNum)

	log.Infof("Total files: %d", len(hash))

	filteredHash := make([]utils.Hash, 0)
	for _, h := range hash {
		ignore := true
		// Match accept
		if textEngine.MatchString(h.Path) || binaryEngine.MatchString(h.Path) {
			ignore = false
		} else {
			continue
		}
		// Match ignore
		for _, r := range ignoreEngine {
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

	output := output{
		Rows: make([]outputRow, 0),
	}
	for _, a := range filteredHash {
		for _, b := range filteredHash {
			if a.Path[0:9] == b.Path[0:9] { // Ignore same person
				continue
			}
			distance := a.Hash.Diff(b.Hash)
			if binaryEngine.MatchString(a.Path) {
				if distance <= binaryThreshold {
					output.Rows = append(output.Rows, outputRow{a.Path, b.Path, strconv.Itoa(distance)})
				}
			} else {
				if distance <= textThreshold {
					output.Rows = append(output.Rows, outputRow{a.Path, b.Path, strconv.Itoa(distance)})
				}
			}
		}
	}
	sort.Sort(&output)
	for _, v := range output.Rows {
		err = outputWriter.Write([]string{v.Source, v.Dest, v.Distance})
	}

	if err != nil {
		log.Fatalf("Error writing to file %s: %s", outputFilePath, err.Error())
	}
}
