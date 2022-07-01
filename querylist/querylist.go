package querylist

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var textOutputPath = "C:\\db_texts\\"
var decompPath = "C:\\FED_LOG\\TOOLS\\UTILITIES\\Decomp.exe"
var imdListPath = "C:\\FED_LOG"

type QueryParams struct {
	TableName    string
	TableColumns []string
}
type QueryList struct {
	Queries       []QueryParams
	QueuedQueries chan func()
}

type Query interface {
	QueryDecomp(tableName string, tableColumns []string)
}

func (qp *QueryParams) QueryDecomp(tableName string, tableColumns []string) {

}

func (ql *QueryList) AddQuery(query QueryParams) {
	ql.Queries = append(ql.Queries, query)
}

func (ql *QueryList) RunDecompQueries() {
	// Holds all the columns that will be searched for the selected table
	var searchCols []string
	const numDecompWorkers = 5
	qJobs := make(chan string, len(ql.Queries))
	qResults := make(chan int, len(ql.Queries))

	for i := 0; i <= numDecompWorkers; i++ {
	}

	// Iterate over the queries held in the QueryList
	for i := 0; i < len(ql.Queries); i++ {
		// Set the generated .txt filepath and name
		fileName := filepath.Join(textOutputPath + ql.Queries[i].TableName + ".txt")
		// Iterate over the TableColumns in the selected QueryParams in order to build the select statement
		for _, v2 := range ql.Queries {
			searchCols = append(searchCols, strings.Join(v2.TableColumns, ","))
		}

		// Display the args that will be used with decomp
		fmt.Printf("\"%v\" \"%v\" \"select %v FROM %v\" \"%v\"", decompPath, imdListPath, searchCols[i], ql.Queries[i].TableName, fileName)

		queryStr := fmt.Sprintf("select %v FROM %v", searchCols[i], ql.Queries[i].TableName)

		proc := exec.Command(decompPath, imdListPath, queryStr, fileName)
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr
		if err := proc.Run(); err != nil {
			fmt.Println(err)
		}
	}
}

func NewQuery(tableName string, tableCols []string) QueryParams {
	return QueryParams{
		TableName:    tableName,
		TableColumns: tableCols,
	}
}
