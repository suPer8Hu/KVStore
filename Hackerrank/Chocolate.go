package main

/**
Given a country name, find the chocolate from that country that provides the most energy per gram using a chocolate database.
Access the database with HTTP GET requests to:
https://jsonmock.hackerrank.com/api/chocolates?countryOfOrigin={country}
(replace {country}). Results are paginated and accessible by appending &page={num} to the URL (replace {num}).
The API response includes these fields:

page: current page
per_page: maximum results per page
total: total records
total_pages: total pages for the query
data: array of chocolate information

Each data object contains:

countryOfOrigin: country of origin
brand: brand name
type: chocolate type
nutritionalInformation: object with nutritional data
prices: array of different prices
weights: array of different weights
other details not relevant to this question

Example record excerpt:
json{
    "type": "Truffle",
    "brand": "Tony's Chocolonely",
    "ingredients": [...],
    "prices": [614, 283, 564, 365, 689],
    "weights": [125, 71, 116, 100, 126],
    "countryOfOrigin": "Norway",
    "nutritionalInformation": {
        "fats": 1.01,
        "kcal": 115,
        "carbs": 7.49,
        ...
    },
    ...
}
For a given country, return the brand and type of chocolate in the format "brand:type" that provides the most energy per gram (kcal × 0.01 × weight). Round the energy calculation down to the nearest integer (the floor). For chocolates with multiple weights, use the average weight rounded down. If multiple chocolates provide the same energy, return the one with the lowest average price, again rounded down.
Function Description
Complete the function energyBar in the editor with the following parameter(s):

string country: the country of origin for the chocolate

Returns

string: the brand and type of chocolate that provides the most energy per gram in the format "brand:type"


Sample Case 0
Input: France
Output: Patchi:Caramel Chocolate
Explanation: For the countryOfOrigin as France, the chocolate brand Patchi and type Caramel Chocolate is the best in providing energy, providing 384 kcal of energy at an average price of 382.
Sample Case 1
Input: Belgium
Output: Ritter Sport:Toffee Chocolate
Explanation: For the countryOfOrigin as Belgium, the chocolate brand Ritter Sport and type Toffee Chocolate is the best in providing energy, providing 402 kcal of energy at an average price of 687.
**/

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sync"
)

type Response struct {
	TotalPages int         `json:"total_pages"`
	Data       []Chocolate `json:"data"`
}

type Chocolate struct {
	Brand                  string    `json:"brand"`
	Type                   string    `json:"type"`
	Weights                []float64 `json:"weights"`
	Prices                 []float64 `json:"prices"`
	NutritionalInformation struct {
		Kcal float64 `json:"kcal"`
	} `json:"nutritionalInformation"`
}

func energyBar(country string) string {

	fetchPage := func(country string, page int) (*Response, error) {
		apiUrl := fmt.Sprintf(
			"https://jsonmock.hackerrank.com/api/chocolates?countryOfOrigin=%s&page=%d",
			url.QueryEscape(country), page,
		)
		resp, err := http.Get(apiUrl)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var result Response
		json.NewDecoder(resp.Body).Decode(&result)
		return &result, nil
	}

	first, _ := fetchPage(country, 1)
	totalPages := first.TotalPages

	ch := make(chan []Chocolate, totalPages)
	ch <- first.Data

	var wg sync.WaitGroup
	for page := 2; page <= totalPages; page++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			resp, _ := fetchPage(country, p)
			ch <- resp.Data
		}(page)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	bestBrand, bestType := "", ""
	bestEnergy := -1
	bestAvgPrice := math.MaxInt64

	for data := range ch {
		for _, choc := range data {

			avgWeight := func() int {
				sum := 0.0
				for _, w := range choc.Weights {
					sum += w
				}
				return int(sum / float64(len(choc.Weights)))
			}()

			avgPrice := func() int {
				sum := 0.0
				for _, p := range choc.Prices {
					sum += p
				}
				return int(sum / float64(len(choc.Prices)))
			}()

			energy := int(choc.NutritionalInformation.Kcal * 0.01 * float64(avgWeight))

			if energy > bestEnergy ||
				(energy == bestEnergy && avgPrice < bestAvgPrice) {
				bestEnergy = energy
				bestAvgPrice = avgPrice
				bestBrand = choc.Brand
				bestType = choc.Type
			}
		}
	}

	return bestBrand + ":" + bestType
}

func main() {
	fmt.Println(energyBar("France"))  // Patchi:Caramel Chocolate
	fmt.Println(energyBar("Belgium")) // Ritter Sport:Toffee Chocolate
}
