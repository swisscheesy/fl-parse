package querylist

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var textOutputPath = "C:\\db_texts\\"
var decompPath = "C:\\FED_LOG\\TOOLS\\UTILITIES\\Decomp.exe"
var imdListPath = "C:\\FED_LOG"

type QueryParams struct {
	TableName    string
	TableColumns []string
}
type QueryList struct {
	Queries []QueryParams
}

func QueryDecomp(wg *sync.WaitGroup, jobs <-chan QueryParams) {
	for j := range jobs {
		// Holds all the columns that will be searched for the selected table
		var searchCols string

		// Set the generated .txt filepath and name
		fileName := filepath.Join(textOutputPath + j.TableName + ".txt")
		searchCols = strings.Join(j.TableColumns, ",")

		// Display Table information
		fmt.Printf("Worker Started \nTable: %v\n", j.TableName)

		queryStr := fmt.Sprintf("select %v FROM %v", searchCols, j.TableName)

		proc := exec.Command(decompPath, imdListPath, queryStr, fileName)
		if err := proc.Run(); err != nil {
			fmt.Println(err)
		}
		wg.Done()
	}
}

// AddQuery / Adds a QueryParams to the end of the QueryList
func (ql *QueryList) AddQuery(query QueryParams) {
	ql.Queries = append(ql.Queries, query)
}

// InitializeDecompPoolAndRun / Creates a worker pool to query the data from Decomp
func (ql *QueryList) InitializeDecompPoolAndRun() {
	qWorkers := 2
	qJobs := make(chan QueryParams, len(ql.Queries))
	qCount := len(ql.Queries)

	var wg sync.WaitGroup

	for w := 0; w <= qWorkers; w++ {
		go QueryDecomp(&wg, qJobs)
	}

	for j := 1; j <= qCount; j++ {
		qJobs <- ql.Queries[j-1]
		wg.Add(1)
	}
	close(qJobs)
	wg.Wait()

}
