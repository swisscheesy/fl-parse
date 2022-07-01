package main

import (
	"bufio"
	"fl-parse/querylist"
	"fmt"
	"os"
	"strings"
)

var schemaPath = "C:\\FED_LOG\\TOOLS\\SCHEMA.txt"
var decompPath = "C:\\FED_LOG\\TOOLS\\UTILITIES\\Decomp.exe"
var textOutputPath = "C:\\db_texts\\"
var csvFilePath = "C:\\db_csv"
var imdListPath = "C:\\FED_LOG"

func main() {
	openSchema()
}

func openSchema() {
	//var queryWg sync.WaitGroup
	ql := querylist.QueryList{}

	file, err := os.Open(schemaPath)
	if err != nil {
		fmt.Println("error opening file: ", err)
	}

	// Ignore all line starting with '-'

	// Table columns to query
	//tableCols := make([]string, 0)

	queryCount := 0
	var curTable = ""
	curColList := make([]string, 0)
	//var queryBlock []string

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
	test(ql)
}

// No channels -- 20% cpu, 50% ram
// 35m -- 34/100

func test(ql querylist.QueryList) {

}
