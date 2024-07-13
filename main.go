package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"
)

type Stock struct {
	Ticker       string
	Gap          float64
	OpeningPrice float64
}

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

// Money in the trading account
var accountBalance = 10000.0

// Percentage of balance i can tolerate losing
var lossTolerance = .02

// Max amount i can tolerate losing
var maxLossPerTrade = accountBalance * lossTolerance

// Percentage of gap i want to take as profit
var profitPercent = .8

type Position struct {
	EntryPrice       float64
	Shares           int
	TakeProfitPrice  float64
	StopLossPrice    float64
	Profit           float64
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

type Selection struct {
	Ticker   string
	Position
	Articles []Article
}

const (
	url          = "https://seeking-alpha.p.rapidapi.com/news/v2/list-by-symbol?size=5&id="
	apiKeyHeader = "x-rapidapi-key"
	apiKey       = "your-api-key-here"
)

type attributes struct {
	PublishOn time.Time  `json:"publishOn"`
	Title     string     `json:"title"`
}

type seekingAlphaNews struct {
	Attributes  attributes  `json:"attributes"`
}

type seekingAlphaResponse struct {
	Data []seekingAlphaNews `json:"data"`
}

type Article struct {
	PublishOn time.Time  
	Headline     string    
}

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