package querylist

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// Destination for decomp generated txt files
	textOutputPath = "C:\\db_texts\\"
	// Location of decomp
	decompPath = "C:\\FED_LOG\\TOOLS\\UTILITIES\\Decomp.exe"
	// Location of IMDLST
	imdListPath = "C:\\FED_LOG"
)

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

		// Remove naming prefix and convert to lowercase
		sFileName := strings.ToLower(j.TableName[2:len(j.TableName)])

		// Set the generated .txt filepath and name
		fileName := filepath.Join(textOutputPath + sFileName + ".txt")
		//fileName := filepath.Join(textOutputPath + j.TableName + ".txt")
		searchCols = strings.Join(j.TableColumns, ",")

		// Display Table information
		fmt.Printf("Worker Started \nTable: %v\n", j.TableName)

		queryStr := fmt.Sprintf("select %v FROM %v", searchCols, j.TableName)

		// Execute decomp command with generated parameters
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
	qWorkers := 5
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
