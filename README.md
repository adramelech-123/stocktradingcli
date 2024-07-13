# STOCK TRADING CLI

This a golang based stock trading CLI that uses the Open Price Gap (OPG) Strategy to analyse stocks based on the gap in price from the previous day's closing price to the current day's opening price. The key to this strategy is speed, the stock market opens by 0900hrs and by 0910hrs we should be done with the analysis and decided on which stocks to trade.

The analysis heavily relies on the news, and in a short period, news on many stocks needs to be processed for the analysis.

## Program Requirements

The following steps are needed for the this CLI Application:

1. Load stocks from a CSV File
2. Filter out unworthy stocks based on some criteria
3. Calculate positions for each stock based on quantity & price
4. Fetch latest news on each stock
5. Output analysis results as JSON

## 1. Setup

Intitialise module to github as follows:

```bash
go mod init github.com/github-username/stocktradingcli
```

Create a `main.go` file and start coding!!! 😇

## 2. Code

### Load Stocks from CSV

We will use a function called `Load()` to take care of getting data from our csv file

```go
func Load(path string) ([]Stock, error) {
	// Open file using the os module
	f, err := os.Open(path)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Defer closing the file if error occurs
	defer f.Close()

	// Reader of csv files
	r := csv.NewReader(f)

	// Read content of the file
	rows, err := r.ReadAll()

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	// Delete the first row of the file since its a header
	rows = slices.Delete(rows, 0, 1)

	// Declare variable to store our stock data
	var stocks []Stock

	// Loop through file and get data in each row
	for _, row := range rows {

		ticker:= row[0]

		gap, err := strconv.ParseFloat(row[1], 64) 
		if err != nil {
			continue
		}

		openingPrice, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			continue
		}

		stocks = append(stocks, Stock {
			Ticker:       ticker, 
			Gap:          gap,
			OpeningPrice: openingPrice,
		})
	}

	f.Close()

	return stocks, nil
}
```
   
### Filter out unworthy stocks

```go
// Money in the trading account
var accountBalance = 10000.0

// Percentage of balance i can tolerate losing
var lossTolerance = .02

// Max amount i can tolerate losing
var maxLossPerTrade = accountBalance * lossTolerance

// Percentage of gap i want to take as profit
var profitPercent = .8

// Structure of our desired output
type Position struct {
	EntryPrice        float64
	Shares            int
	TakeProfitPrice   float64
	StopLossPrice     float64
	Profit            float64
}

func Calculate(gapPercent, openingPrice float64) Position {
	closingPrice := openingPrice / (1 + gapPercent)
	gapValue := closingPrice - openingPrice
	profitFromGap := profitPercent * gapValue

	stopLoss := openingPrice - profitFromGap
	takeProfit := openingPrice + profitFromGap

	shares := int(maxLossPerTrade / math.Abs(stopLoss-openingPrice))

	profit := math.Abs(openingPrice-takeProfit) * float64(shares)
	profit = math.Round(profit*100) / 100

	return Position {
		EntryPrice:      math.Round(openingPrice*100) /100,       
		Shares:          shares,            
		TakeProfitPrice: math.Round(takeProfit*100) /100,    
		StopLossPrice:   math.Round(stopLoss*100) /100,
		Profit:          math.Round(profit*100) /100,         
	}
}

// The stocks we select after applying filter
type Selection struct {
	Ticker string
	Position
}

func main() {
	// Load stocks data from the CSV File
	stocks, err := Load("./opg.csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Update the stocks slice to exclude the filtered out stocks that dont meet our criteria
	stocks = slices.DeleteFunc(stocks, func(s Stock) bool {
		return math.Abs(s.Gap) < .1 
	})

	// More Operations to come
}
```

### Fetch News on each stock

We will fetch data from the `Seeking Alpha` api registered on RapidAPI. We will need the url for fetching data, the API header and API Key.

```go
const (
	url          = "https://seeking-alpha.p.rapidapi.com/news/v2/list-by-symbol?size=5&id="
	apiKeyHeader = "x-rapidapi-key"
	apiKey       = "491efa8017msh412a08ea1e0faccp1c43c1jsn5835a507z813" // Dummy API Key
)
```
We also need to model the data coming as a response from the API using structs as follows:

```go
// We model the actual attributes we want from the response, which are housed under the attributes object which is in turn housed under the data object
type attributes struct {
	PublishOn time.Time  `json:"publishOn"`
	Title     string     `json:"title"`
}

// Within the data object we have an attributes object that contains the data we are interested in
type seekingAlphaNews struct {
	Attributes  attributes  `json:"attributes"`
}

//This models the data object we recieve from the API
type seekingAlphaResponse struct {
	Data []seekingAlphaNews `json:"data"`
}

// We need this type to represent the actual data we will work with from the data modelled from the API
type Article struct {
	PublishOn time.Time  
	Headline     string    
}
```

Now we need to wrap our fetch functionality in a function called `FetchNews` as follows:

```go
func FetchNews(ticker string) ([]Article, error) {
	req, err := http.NewRequest(http.MethodGet, url+ticker, nil)

	if err != nil {
		return nil, err
	}

	req.Header.Add(apiKeyHeader, apiKey)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("unsuccessful status code %d recieved", resp.StatusCode) 
	}

	res := &seekingAlphaResponse{}
	json.NewDecoder(resp.Body).Decode(res)

	var articles []Article

	for _, item := range res.Data {
		art := Article {
			PublishOn: item.Attributes.PublishOn,
			Headline: item.Attributes.Title,
		}

		articles = append(articles, art)
	}

	return articles, nil
}

```

We also need a function to Deliver the output as a `json` file after running the analysis. We do this using the Encoder functions as follows:

```go
//  The Function takes a path and selected/filtered stocks with their repsective outputs as parameters
func Deliver(filePath string, selections []Selection) error {

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(selections)
	if err != nil {
		return fmt.Errorf("error encoding selections: %w", err)
	}

	return nil
}
```

Finally in the `main` function, we process our Load, FetchNews and Deliver functions as follows:

```go

func main() {
	stocks, err := Load("./opg.csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	stocks = slices.DeleteFunc(stocks , func(s Stock) bool {
		return math.Abs(s.Gap) < .1 
	})

	var selections []Selection

	for _, stock := range stocks {
		// For each stock we run our analysis and generate articles on the stock
		position := Calculate(stock.Gap, stock.OpeningPrice)
		articles, err := FetchNews(stock.Ticker)

		if err != nil {
			log.Printf("error loading news about %s, %v", stock.Ticker, err)
			continue
		} else {
			log.Printf("Found %d articles about %s", len(articles), stock.Ticker)
		}

		// We provide each selected stock with its calculated position and related articles
		sel := Selection {
			Ticker:   stock.Ticker,
			Position: position,
			Articles: articles,
		}

		// Append each selected stock to the array/slice of selected and analysed stocks
		selections = append(selections, sel)
	}

	outputPath := "./opg.json"

	// Output the results
	err = Deliver(outputPath, selections)
	if err != nil {
		log.Printf("Error writing output, %v", err)
		return
	}

	log.Printf("Finished writing output to %s\n", outputPath)

}
```

Now we can run our application and produce a json file with our results. However, we have run this application on a small dataset therefore if we run it on a very large dataset, we will face performance problems. To minimise performance problems and get our output results faster, we can use `concurrency` to make our application faster.

## 3. GO ROUTINES

Goroutines are functions or methods in the Go programming language that can execute independently and simultaneously with other goroutines in a program. They are a fundamental building block of concurrent programming in Go and are similar to `lightweight threads`. Goroutines are managed by the Go runtime and are more efficient than traditional threads in other programming languages when it comes to memory and CPU utilization.

### Example

```go
package main

import (
	"fmt"
	"time"
)

func processTransaction(transactionNumber int) {
	fmt.Println("Processing transaction #", transactionNumber)
	time.Sleep(2*time.Second) // Simulating a time consuming task
	fmt.Println("Processed transaction #", transactionNumber)
}

func main() {
	for i := 1; i <= 5; i++ {
		go processTransaction(i) // Initialize a go routine for each transaction
	}

	time.Sleep( 3 * timeSecond) // Wait for all transactions to finish
	fmt.Println("All transactions processed!")
}

```

## 4. Wait Groups

In some scenarios, you may need to block certain parts of code to allow GoRoutines to complete their execution according to your needs. We did this in the Goroutine example above using the `time.Sleep()` function however, this is ineeficient because we don't know exactly how long a process is going to take to execute. A common usage of WaitGroup is to block the main function because we know that the main function itself is also a GoRoutine.

### Methods of Waitgroups in Golang
- **Add**: The Waitgroup acts as a counter holding the number of functions or go routines to wait for. When the counter becomes 0 the Waitgroup releases the goroutines. 
- **Wait**: The wait method blocks the execution of the application until the Waitgroup counter becomes 0.
- **Done**: Decreases the Waitgroup counter by a value of 1

### Example

```go
package main

import (
	"fmt"
	"sync"
	"time"
)


func main() {
	var wg sync.WaitGroup

	for i := 1; i <= 5; i++ {
		wg.Add(1) // This increments the waitGroup counter by 1
		// We are executing processTransaction Anonymously instead
		go func(transactionNumber int) {
			fmt.Println("Processing transaction #", transactionNumber)
			time.Sleep(2*time.Second) // Simulating a time consuming task
			fmt.Println("Processed transaction #", transactionNumber)
		}(i)
	}

	wg.Wait() // Wait for all goroutines to finish
	fmt.Println("All transactions processed!")
}

```

## 5. Adding Concurrency to our CLI Application

With our knowledge on goroutines and wait groups, we can refactor our `main()` func to allow for concurrency as follows:

```go
func main() {
	stocks, err := Load("./opg.csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	stocks = slices.DeleteFunc(stocks , func(s Stock) bool {
		return math.Abs(s.Gap) < .1 
	})

	var selections []Selection

	// We will implement concurrency here, since this is where all the processing takes place

	var wg sync.WaitGroup

	for _, stock := range stocks {
		wg.Add(1)

		go func(s Stock) {
			defer wg.Done()

			position := Calculate(stock.Gap, stock.OpeningPrice)

			articles, err := FetchNews(stock.Ticker)
			if err != nil {
				log.Printf("error loading news about %s, %v", stock.Ticker, err)
				return
			} else {
				log.Printf("Found %d articles about %s", len(articles), stock.Ticker)
			}

			sel := Selection {
				Ticker:   stock.Ticker,
				Position: position,
				Articles: articles,
			}

			selections = append(selections, sel)
		}(stock) // Loop variable stock is passed to the anonymous function
		
	}

	wg.Wait() // Wait for news to be loaded before delivering results
	
	outputPath := "./opg.json"

	err = Deliver(outputPath, selections)
	if err != nil {
		log.Printf("Error writing output, %v", err)
		return
	}

	log.Printf("Finished writing output to %s\n", outputPath)

}
```

## 5. Channels

We can also apply channels to manage concurrency in our Go applications.

Channels are a go data type that allow goroutines to communicate and synchronize their execution. They can be thought of as conduits for passing data between go routines.

### Example 1

```go
package main

import (
	"fmt"
	"time"
)

func processTransaction(transactionNumber int, done chan<- bool) {
	fmt.Println("Processing transaction #", transactionNumber)
	time.Sleep(2*time.Second) // Simulating a time consuming task
	fmt.Println("Processed transaction #", transactionNumber)
	done <- true //Send a signal that the transaction has been processed
}

func main() {
	complete := make(chan bool) // Create a channel to communicate completion

	for i := 1; i <= 5; i++ {
		go processTransaction(i, complete) // Pass the channel to go routines
	}

	// Wait for all transactions to be processed
	for i := 1; i <= 5; i++ {
		<-complete // Wait for signal from each go routine
	}

	fmt.Println("All transactions processed!")
}

```

Channels can also be used to pass data as follows:

### Example 2

```go
package main

import (
	"fmt"
	"time"
)

func processTransaction(transactionNumber int, done chan<- int) {
	fmt.Printf("Processing transaction #%d\n", transactionNumber)
	time.Sleep(2*time.Second) // Simulating a time consuming task
	done <- transactionNumber //Send transaction number to indicate completion
}

func main() {
	totalTransactions := 5
	processed := make(chan int, totalTransactions) // Create a channel to communicate completion

	// Start a goroutine for each transaction
	for i := 1; i <= totalTransactions; i++ {
		go processTransaction(i, processed) // Pass the channel to go routines
	}

	// Use 'range' to recieve from the channel
	for transactionNumber := range processed {
		fmt.Printf("Received completion signal for transaction #%d\n", transactionNumber)
		if transactionNumber == totalTransactions {
			close(processed) // Close the channel when the last transaction is completed
		}
	}

	fmt.Println("All transactions processed!")
}

```


## 6. Implenting channels in our CLI Application

Re-implementing the `main()` function using channels as follows:

```go
func main() {
	stocks, err := Load("./opg.csv")
	if err != nil {
		fmt.Println(err)
		return
	}

	stocks = slices.DeleteFunc(stocks , func(s Stock) bool {
		return math.Abs(s.Gap) < .1 
	})

	
	selectionsChan := make(chan Selection, len(stocks))

	for _, stock := range stocks {
		go func(s Stock, selected chan<-Selection) {
		
			position := Calculate(s.Gap, s.OpeningPrice)
			articles, err := FetchNews(s.Ticker)

			if err != nil {
				log.Printf("error loading news about %s, %v", s.Ticker, err)
				selected <- Selection{}
				return
			} else {
				log.Printf("Found %d articles about %s", len(articles), s.Ticker)
			}

			// We provide each selected stock with its calculated position and related articles
			sel := Selection {
				Ticker:   s.Ticker,
				Position: position,
				Articles: articles,
			}

			selected <- sel

		}(stock, selectionsChan)
	}

	var selections []Selection

	for sel := range selectionsChan {
		selections = append(selections, sel)
		if len(selections) == len(stocks) {
			close(selectionsChan)
		}
	}

	outputPath := "./opg.json"

	// Output the results
	err = Deliver(outputPath, selections)
	if err != nil {
		log.Printf("Error writing output, %v", err)
		return
	}

	log.Printf("Finished writing output to %s\n", outputPath)
}
```