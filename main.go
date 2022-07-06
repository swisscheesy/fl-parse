package main

import (
	"bufio"
	"fl-parse/csv"
	"fl-parse/querylist"
	"fmt"
	"os"
	"strings"
)

var (
	// Location of Fedlog's SCHEMA.txt
	schemaPath = "C:\\FED_LOG\\TOOLS\\SCHEMA.txt"
	// Destination for decomp generated txt files
	textOutputPath = "C:\\db_texts\\"
)

func main() {
	openSchema()
}

// openSchema / Parses SCHEMA.txt file that is included with FEDLOG tools in order to retrieve
// all table names and columns that will be searched.
// Assuming SCHEMA.txt will be updated with any table additions or removals for each version.
func openSchema() {
	ql := querylist.QueryList{}

	file, err := os.Open(schemaPath)
	if err != nil {
		fmt.Println("error opening file: ", err)
	}

	queryCount := 0
	var curTable = ""
	curColList := make([]string, 0)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		curLine := scanner.Text()

		if len(curLine) != 0 {
			// Ignore any lines that begin with "`", these are usually at the top and act as comments
			if !strings.HasPrefix(curLine, "-") {
				if strings.Contains(curLine, "Disc") {
					tableSlice := strings.Split(curLine, " ")
					curTable = tableSlice[0]
					queryCount += 1
				} else {
					trimLine := strings.TrimSpace(curLine)
					colSlice := strings.Split(trimLine, " ")
					curColList = append(curColList, colSlice[0])
				}
			}

		}
		// Reached an empty line, create query from curTable and curColList
		if len(curLine) == 0 && len(curTable) != 0 && len(curColList) != 0 {
			ql.AddQuery(querylist.QueryParams{
				TableName:    curTable,
				TableColumns: curColList,
			})
			curTable = ""
			curColList = make([]string, 0)
		}
	}
	fmt.Println("QueryParams Count: ", queryCount)
	// Begin querying decomp with the table data
	ql.InitializeDecompPoolAndRun()

	// Retrieve file info for all created .txt files
	txtOutputFiles := csv.GetTxtFilesFromPath(textOutputPath)

	// Convert txt files to csv if they exist
	if len(txtOutputFiles) > 0 {
		for i := range txtOutputFiles {
			csv.WriteContentToCsv(txtOutputFiles[i])
		}
	}

}
