package csv

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	textOutputPath = "C:\\db_texts\\"
	csvOutputPath  = "C:\\db_csv\\"
)

// GetTxtFilesFromPath / returns a string slice of all the names of all txt files in a given path
func GetTxtFilesFromPath(txtPath string) []string {
	files, err := ioutil.ReadDir(txtPath)
	if err != nil {
		panic("Unable to read text file")
	}

	return retrieveTextFiles(files)
}

// retrieveTextFiles / returns a string slice containing the names of all txt files with names that match to the
// sent slice of os.FileInfo
func retrieveTextFiles(f []os.FileInfo) []string {

	var txtFiles []string
	for _, v := range f {
		//ignore all directories
		if !v.IsDir() {
			// Only utilize files ending with .txt
			if filepath.Ext(v.Name()) == ".txt" {
				txtFiles = append(txtFiles, v.Name())
			}
		}
	}

	return txtFiles
}

// WriteContentToCsv / Creates a csv file for the given name, and parses the txt file of the same name
// in order to fill it
func WriteContentToCsv(fileName string) {
	// Clean fileName by removing extension
	fName := fileName[:len(fileName)-4]

	csvFile, err := os.Create(filepath.Join(csvOutputPath, fName) + ".csv")
	if err != nil {
		log.Panicf("Unable to create file: %v", err)
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	file, err := os.Open(textOutputPath + fileName)
	if err != nil {
		log.Panicf("Unable to open file %s due to error %v", fileName, err)
	}

	scanner := bufio.NewScanner(file)
	fmt.Printf("Starting on file: %v\n", fName)
	for scanner.Scan() {
		rowTxt := scanner.Text()
		// If line is not empty
		if len(rowTxt) > 0 {
			// Split data by default '|' 'pipe' delimiter
			content := strings.Split(rowTxt, "|")

			// csv in std library doesn't easily allow additional quotes, and it's not worth rewriting.
			// Instead, fields come out as """field""" instead of "field"
			// Not necessary, but may find a workaround later
			//for i := range content {
			//	content[i] = strconv.Quote(content[i])
			//}

			err := csvWriter.Write(content)
			if err != nil {
				log.Panicf("Unable to write to csv file %v", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Panic(err)
	}

}
