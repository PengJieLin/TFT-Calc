package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"context"
	
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// --- CONFIGURATION ---
const TargetActiveTraits = 6
const UseHighCost = false
const PreferHighCost = false

var bestTeam []Champion

type Champion struct {
	Name   string   `json:"name"`
	Traits []string `json:"traits"`
	Cost   int      `json:"cost"`
}

type Trait struct {
	Name        string `json:"name"`
	MinRequired int    `json:"minRequired"`
}

func HandleRequest(ctx context.Context) (string, error){

	// Load AWS configuration (credentials, region, etc.)
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil { return "", err }

    // Create the S3 client
    s3Client := s3.NewFromConfig(cfg)
    
    // Get the bucket name from an environment variable (we'll set this in Terraform later)
    bucketName := os.Getenv("DATA_BUCKET")

    // Now call your updated loaders
    champs, err := loadChampions(ctx, s3Client, bucketName, "champions.csv")
    if err != nil { return "", err }

	traits, err := loadTraits(ctx, s3Client, bucketName, "traits.csv")
	if err != nil { return "", err }


	fmt.Printf("=== TFT Optimizer | Target: %d Active Traits ===\n", TargetActiveTraits)

	// 1. Define Initial Board
	initialTeam := []Champion{
		{Name: "Kayn", Traits: []string{}, Cost: 3},
	}

	startTeam := make([]Champion, len(initialTeam))
	copy(startTeam, initialTeam)

	// 2. Step 1: Search WITHOUT 4-cost additions
	if(!UseHighCost){
		fmt.Println("Step 1: Searching for low-cost solutions (Cost < 4)...")
		lowCostPool := filterPool(champs, initialTeam, 3)
		solve(startTeam, lowCostPool, traits, 0, len(initialTeam))
	}

	// 3. Step 2: Fallback to include 4-cost additions if no solution found
	if len(bestTeam) == 0 {
		fmt.Println("No solution found with low-cost units. Step 2: Including 4-cost units...")
		fullPool := filterPool(champs, initialTeam, 4)
		solve(startTeam, fullPool, traits, 0, len(initialTeam))
	}

	// 4. Final Output
	if len(bestTeam) == 0 {
		fmt.Println("\n❌ No solution found even with 4-cost units.")
	} else {
		displayFinalResults(initialTeam, bestTeam, traits)
	}

	return "Done!", nil
}

func main() {
	lambda.Start(HandleRequest)
}

func solve(currentTeam []Champion, pool []Champion, allTraits []Trait, startIndex int, initialSize int) {
	numAdded := len(currentTeam) - initialSize

	if len(bestTeam) > 0 && numAdded >= len(bestTeam) {
		return
	}

	if isSatisfied(currentTeam, allTraits, TargetActiveTraits) {
		newUnits := make([]Champion, numAdded)
		copy(newUnits, currentTeam[initialSize:])
		bestTeam = newUnits
		return
	}

	for i := startIndex; i < len(pool); i++ {
		currentTeam = append(currentTeam, pool[i])
		solve(currentTeam, pool, allTraits, i+1, initialSize)
		currentTeam = currentTeam[:len(currentTeam)-1]
	}
}

func isSatisfied(team []Champion, allTraits []Trait, threshold int) bool {
	counts := make(map[string]int)
	for _, champ := range team {
		for _, traitName := range champ.Traits {
			counts[traitName]++
		}
	}
	activeCount := 0
	for _, t := range allTraits {
		if counts[t.Name] >= t.MinRequired {
			activeCount++
		}
	}
	return activeCount >= threshold
}

func filterPool(champs []Champion, initialTeam []Champion, maxCost int) []Champion {
	var pool []Champion
	for _, c := range champs {
		isDuplicate := false
		for _, initC := range initialTeam {
			if c.Name == initC.Name {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate && c.Cost <= maxCost {
			pool = append(pool, c)
		}
	}
	sort.Slice(pool, func(i, j int) bool {
		if(PreferHighCost){
			if pool[i].Cost != pool[j].Cost {
			return pool[i].Cost > pool[j].Cost
			}
		}else{
			if pool[i].Cost != pool[j].Cost {
				return pool[i].Cost < pool[j].Cost
			}
		}
		return len(pool[i].Traits) > len(pool[j].Traits)
	})
	return pool
}

func displayFinalResults(initial []Champion, additions []Champion, allTraits []Trait) {
	fullBoard := append(initial, additions...)

	fmt.Println("\n==========================================")
	fmt.Println("             OPTIMAL FULL TEAM            ")
	fmt.Println("==========================================")
	
	totalCost := 0
	for _, c := range fullBoard {
		label := "[ADDED]"
		for _, initC := range initial {
			if c.Name == initC.Name {
				label = "[START]"
			}
		}
		fmt.Printf("%-7s %-14s | Cost: %d | %s\n", label, c.Name, c.Cost, strings.Join(c.Traits, ", "))
		totalCost += c.Cost
	}

	fmt.Printf("\nTotal Team Size: %d", len(fullBoard))
	fmt.Printf("\nTotal Team Cost: %d", totalCost)

	// Display active traits for the entire team
	counts := make(map[string]int)
	for _, champ := range fullBoard {
		for _, tName := range champ.Traits {
			counts[tName]++
		}
	}

	fmt.Println("\n\n--- Active Traits ---")
	for _, t := range allTraits {
		if counts[t.Name] >= t.MinRequired {
			fmt.Printf(" ✅ %-14s (%d/%d)\n", t.Name, counts[t.Name], t.MinRequired)
		}
	}
	fmt.Println("==========================================")
}

// --- LOADERS ---

func loadChampions(ctx context.Context, client *s3.Client, bucket string, key string) ([]Champion, error) {
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
        Bucket: &bucket,
        Key:    &key,
    })
	if err != nil {
        return nil, fmt.Errorf("unable to fetch %s from bucket %s: %v", key, bucket, err)
    }
    defer output.Body.Close()

	reader := csv.NewReader(output.Body)
	records, _ := reader.ReadAll()
	var champions []Champion
	for i, row := range records {
		if i == 0 || len(row) < 3 { continue }
		cost, _ := strconv.Atoi(row[2])
		champions = append(champions, Champion{
			Name: row[0], Traits: strings.Split(row[1], ";"), Cost: cost,
		})
	}
	return champions, nil
}

func loadTraits(ctx context.Context, client *s3.Client, bucket string, key string) ([]Trait, error) {
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:	&bucket,
		Key:	&key,
	})
	if err != nil {
        return nil, fmt.Errorf("unable to fetch %s from bucket %s: %v", key, bucket, err)
    }
    defer output.Body.Close()

	reader := csv.NewReader(output.Body)
	records, _ := reader.ReadAll()
	var traits []Trait
	for i, row := range records {
		if i == 0 || len(row) < 2 { continue }
		minReq, _ := strconv.Atoi(row[1])
		traits = append(traits, Trait{Name: row[0], MinRequired: minReq})
	}
	return traits, nil
}