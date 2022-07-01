package main

import (
	"bufio"
	"fl-parse/querylist"
	"fmt"
	"os"
	"strings"
)

var schemaPath = "C:\\FED_LOG\\TOOLS\\SCHEMA.txt"

// Only Doing 52 tables
func main() {
	ParseTableSchema()
}

func ParseTableSchema() {
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
	BeginDecompQuery(ql)
}

// BeginDecompQuery / Takes the supplied Querlist and runs it through Decomp to retrieve the output
func BeginDecompQuery(ql querylist.QueryList) {
	ql.InitializeDecompPoolAndRun()
}
