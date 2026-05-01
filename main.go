package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// --- GLOBALS (Cached across warm starts) ---
var (
	cachedChamps []Champion
	cachedTraits []Trait
	once         sync.Once
	initErr      error
)

// --- TYPES ---

type SolverRequest struct {
	UseHighCost        bool       `json:"use_high_cost"`
	PreferHighCost     bool       `json:"prefer_high_cost"`
	TargetActiveTraits int        `json:"target_active_traits"`
	InitialTeam        []Champion `json:"initial_team"`
}

type Champion struct {
	Name   string   `json:"name"`
	Traits []string `json:"traits"`
	Cost   int      `json:"cost"`
}

type Trait struct {
	Name        string `json:"name"`
	MinRequired int    `json:"minRequired"`
}

// --- CORE LOGIC ---

func cleanRequest(req SolverRequest) SolverRequest {
	if req.TargetActiveTraits == 0 {
		req.TargetActiveTraits = 6
	} else if req.TargetActiveTraits > 11 {
		req.TargetActiveTraits = 11
	}
	if len(req.InitialTeam) > 8 {
		req.InitialTeam = req.InitialTeam[:8]
	}
	return req
}

func HandleRequest(ctx context.Context, req SolverRequest) (interface{}, error) {
	bestTeam := []Champion{}
	req = cleanRequest(req)

	once.Do(func() {
        var err error 
        var cfg aws.Config

        cfg, err = config.LoadDefaultConfig(ctx)
        if err != nil {
            initErr = err
            return
        }
        
        s3Client := s3.NewFromConfig(cfg)
        bucketName := os.Getenv("DATA_BUCKET")

        cachedChamps, err = loadChampions(ctx, s3Client, bucketName, "champions.csv")
        if err != nil {
            initErr = err
            return
        }

        cachedTraits, err = loadTraits(ctx, s3Client, bucketName, "traits.csv")
        if err != nil {
            initErr = err
            return
        }
    })

    if initErr != nil {
        return nil, fmt.Errorf("initialization failed: %v", initErr)
    }

	fmt.Printf("=== TFT Optimizer | Target: %d Active Traits ===\n", req.TargetActiveTraits)

	startTeam := make([]Champion, len(req.InitialTeam))
	copy(startTeam, req.InitialTeam)

	// maxAdditions prevents the recursion from blowing up and timing out
	maxAdditions := 8 

	// 3. Step 1: Search WITHOUT 4-cost additions if requested
	if !req.UseHighCost {
		fmt.Println("Step 1: Searching for low-cost solutions (Cost < 4)...")
		lowCostPool := filterPool(cachedChamps, req.InitialTeam, 3, req.PreferHighCost)
		solve(startTeam, lowCostPool, cachedTraits, 0, len(req.InitialTeam), req.TargetActiveTraits, &bestTeam, maxAdditions)
	}

	// 4. Step 2: Fallback to include 4-cost units
	if len(bestTeam) == 0 {
		fmt.Println("Step 2: Searching with units up to 4-cost...")
		fullPool := filterPool(cachedChamps, req.InitialTeam, 4, req.PreferHighCost)
		solve(startTeam, fullPool, cachedTraits, 0, len(req.InitialTeam), req.TargetActiveTraits, &bestTeam, maxAdditions)
	}

	// 5. Final Output & Logging
	if len(bestTeam) == 0 {
		fmt.Println("\n❌ No solution found within addition limit.")
	} else {
		displayFinalResults(req.InitialTeam, bestTeam, cachedTraits)
	}

	return bestTeam, nil
}

func main() {
	lambda.Start(HandleRequest)
}

// solve now uses a pointer to bestTeam and a maxDepth limit
func solve(currentTeam []Champion, pool []Champion, allTraits []Trait, startIndex, initialSize int, targetActiveTraits int, bestTeam *[]Champion, maxDepth int) {
	numAdded := len(currentTeam) - initialSize

	// Pruning: If we found a solution, don't look for longer ones
	if len(*bestTeam) > 0 && numAdded >= len(*bestTeam) {
		return
	}

	// Safety: Prevent recursion explosion
	if numAdded > maxDepth {
		return
	}

	if isSatisfied(currentTeam, allTraits, targetActiveTraits) {
		newUnits := make([]Champion, numAdded)
		copy(newUnits, currentTeam[initialSize:])
		*bestTeam = newUnits
		return
	}

	for i := startIndex; i < len(pool); i++ {
		currentTeam = append(currentTeam, pool[i])
		solve(currentTeam, pool, allTraits, i+1, initialSize, targetActiveTraits, bestTeam, maxDepth)
		currentTeam = currentTeam[:len(currentTeam)-1] // Backtrack
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

func filterPool(champs []Champion, initialTeam []Champion, maxCost int, preferHighCost bool) []Champion {
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
		if preferHighCost {
			if pool[i].Cost != pool[j].Cost {
				return pool[i].Cost > pool[j].Cost
			}
		} else {
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

	// Active Traits Calculation
	counts := make(map[string]int)
	for _, champ := range fullBoard {
		for _, tName := range champ.Traits {
			counts[tName]++
		}
	}

	fmt.Println("\n--- Active Traits ---")
	for _, t := range allTraits {
		if counts[t.Name] >= t.MinRequired {
			fmt.Printf(" ✅ %-14s (%d/%d)\n", t.Name, counts[t.Name], t.MinRequired)
		}
	}
	fmt.Printf("\nTotal Cost: %d | Total Size: %d\n", totalCost, len(fullBoard))
	fmt.Println("==========================================")
}

// --- S3 LOADERS ---

func loadChampions(ctx context.Context, client *s3.Client, bucket string, key string) ([]Champion, error) {
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	reader := csv.NewReader(output.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var champions []Champion
	for i, row := range records {
		if i == 0 || len(row) < 3 {
			continue
		}
		cost, _ := strconv.Atoi(row[2])
		champions = append(champions, Champion{
			Name:   row[0],
			Traits: strings.Split(row[1], ";"),
			Cost:   cost,
		})
	}
	return champions, nil
}

func loadTraits(ctx context.Context, client *s3.Client, bucket string, key string) ([]Trait, error) {
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	reader := csv.NewReader(output.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var traits []Trait
	for i, row := range records {
		if i == 0 || len(row) < 2 {
			continue
		}
		minReq, _ := strconv.Atoi(row[1])
		traits = append(traits, Trait{Name: row[0], MinRequired: minReq})
	}
	return traits, nil
}