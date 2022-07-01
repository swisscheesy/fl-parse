package csv

import (
	"encoding/csv"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

var (
	csvOutputPath  = "C:\\db_csv\\"
	textOutputPath = "C:\\db_texts\\"
)

type TextFile struct {
	Name         string
	TableColumns []string
}

func TextToCsv(file TextFile) {

	files, err := ioutil.ReadDir(textOutputPath)
	if err != nil {
		panic("Unable to read text file")
	}
	if files != nil {

	}

}

func RetrieveTextFiles(f []os.FileInfo) []string {
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

func CreateAndWriteCsv(tf TextFile) {
	csvFile, err := os.Create(tf.Name + ".csv")
	if err != nil {
		log.Panicf("Cannot create csv file: %d", err)
	}

	defer csvFile.Close()

	cWriter := csv.NewWriter(csvFile)
	defer cWriter.Flush()

}
