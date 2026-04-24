package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
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

func main() {
	fmt.Printf("=== TFT Optimizer | Target: %d Active Traits ===\n", TargetActiveTraits)

	champs, err := loadChampions("data/champions.csv")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	traits, err := loadTraits("data/traits.csv")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

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

func loadChampions(filename string) ([]Champion, error) {
	file, err := os.Open(filename)
	if err != nil { return nil, err }
	defer file.Close()
	reader := csv.NewReader(file)
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

func loadTraits(filename string) ([]Trait, error) {
	file, err := os.Open(filename)
	if err != nil { return nil, err }
	defer file.Close()
	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()
	var traits []Trait
	for i, row := range records {
		if i == 0 || len(row) < 2 { continue }
		minReq, _ := strconv.Atoi(row[1])
		traits = append(traits, Trait{Name: row[0], MinRequired: minReq})
	}
	return traits, nil
}