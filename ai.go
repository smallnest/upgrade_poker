package main

import "math/rand"

// AI functions for bidding and playing

// aiDiscard selects 8 cards to discard from the dealer's hand (including picked-up bottom cards)
func aiDiscard(player *Player, trumpSuit Suit, level Rank) []Card {
	hand := make([]Card, len(player.Hand))
	copy(hand, player.Hand)

	// Strategy: discard non-trump low cards, then non-point trump cards
	// Sort by "value" - we want to keep high trump, point cards, and tractors

	var discards []Card
	remaining := make([]Card, len(hand))
	copy(remaining, hand)

	// Phase 1: Discard non-trump, non-point, lowest cards
	nonTrumpNonPoint := filterCards(remaining, func(c Card) bool {
		return !IsTrump(c, trumpSuit, level) && c.Points() == 0
	})

	// Sort by rank ascending (lowest first)
	sortCardsByRank(nonTrumpNonPoint)

	for _, c := range nonTrumpNonPoint {
		if len(discards) >= 8 {
			break
		}
		discards = append(discards, c)
		remaining = removeCard(remaining, c)
	}

	// Phase 2: If still need more, discard non-trump point cards (5, 10, K) starting from lowest
	if len(discards) < 8 {
		nonTrumpPoint := filterCards(remaining, func(c Card) bool {
			return !IsTrump(c, trumpSuit, level) && c.Points() > 0
		})
		sortCardsByRank(nonTrumpPoint)

		for _, c := range nonTrumpPoint {
			if len(discards) >= 8 {
				break
			}
			discards = append(discards, c)
			remaining = removeCard(remaining, c)
		}
	}

	// Phase 3: If still need more, discard low trump cards
	if len(discards) < 8 {
		trumpCards := filterCards(remaining, func(c Card) bool {
			return IsTrump(c, trumpSuit, level)
		})

		// Sort by trump order ascending (weakest first)
		sortCardsByTrumpOrder(trumpCards, trumpSuit, level)

		for _, c := range trumpCards {
			if len(discards) >= 8 {
				break
			}
			discards = append(discards, c)
			remaining = removeCard(remaining, c)
		}
	}

	player.Hand = remaining
	return discards
}

// aiPlay selects cards for an AI player to play
func aiPlay(player *Player, trick *Trick, game *Game) []Card {
	level := game.DealerLevel()
	trumpSuit := game.TrumpSuit

	var cards []Card
	if trick.PlayerCount() == 0 {
		cards = aiLead(player, trumpSuit, level)
	} else {
		cards = aiFollow(player, trick, trumpSuit, level, game)
	}

	// Validate the play; if invalid, fall back to a safe play
	var leadCards []Card
	if trick.PlayerCount() > 0 {
		leadCards = trick.LeadCards()
	}
	if !ValidatePlay(cards, leadCards, player.Hand, trumpSuit, level) {
		cards = aiSafePlay(player, trick, trumpSuit, level)
	}

	return cards
}

// aiSafePlay generates a guaranteed valid play as a fallback
func aiSafePlay(player *Player, trick *Trick, trumpSuit Suit, level Rank) []Card {
	hand := player.Hand

	if len(hand) == 0 {
		return nil
	}

	if trick.PlayerCount() == 0 {
		// Leading: just play the first single card
		return []Card{hand[0]}
	}

	leadCards := trick.LeadCards()
	leadSuit := trick.LeadSuit()
	needCount := len(leadCards)

	// If we have fewer cards than needed, play all we have
	if len(hand) <= needCount {
		return hand
	}

	// Check if we have cards of the lead suit
	leadSuitCards := player.GetCardsOfSuit(leadSuit, trumpSuit, level)

	if len(leadSuitCards) > 0 {
		// Must follow suit - play the same number of cards as the lead
		if len(leadSuitCards) >= needCount {
			return leadSuitCards[:needCount]
		}
		// Not enough lead-suit cards, play what we have + fill from other cards
		result := make([]Card, 0, needCount)
		result = append(result, leadSuitCards...)
		for _, c := range hand {
			if EffectiveSuit(c, trumpSuit, level) != leadSuit && len(result) < needCount {
				result = append(result, c)
			}
		}
		return result
	}

	// No lead suit cards: just play same count from hand
	return hand[:needCount]
}

// aiLead selects cards when AI is leading
func aiLead(player *Player, trumpSuit Suit, level Rank) []Card {
	hand := player.Hand

	// Strategy: lead with off-suit A or K to collect points
	// Or lead with tractors if we have them
	// Or lead with a single low card to probe

	// Try to lead with off-suit A (single)
	for _, c := range hand {
		if !IsTrump(c, trumpSuit, level) && c.Rank == RankA {
			return []Card{c}
		}
	}

	// Try to lead with off-suit K
	for _, c := range hand {
		if !IsTrump(c, trumpSuit, level) && c.Rank == RankK {
			return []Card{c}
		}
	}

	// Try to lead with a pair of off-suit cards
	offSuitPairs := findPairsInHand(hand, trumpSuit, level, false)
	if len(offSuitPairs) > 0 {
		// Lead the pair with the highest rank
		bestPair := offSuitPairs[0]
		for _, p := range offSuitPairs {
			if p[0].Rank > bestPair[0].Rank {
				bestPair = p
			}
		}
		return bestPair
	}

	// Lead with a single trump (if we have many)
	trumpCards := filterCards(hand, func(c Card) bool {
		return IsTrump(c, trumpSuit, level)
	})
	if len(trumpCards) > 4 {
		// Lead with lowest trump
		sortCardsByTrumpOrder(trumpCards, trumpSuit, level)
		return []Card{trumpCards[len(trumpCards)-1]}
	}

	// Lead with lowest non-trump card
	nonTrump := filterCards(hand, func(c Card) bool {
		return !IsTrump(c, trumpSuit, level)
	})
	if len(nonTrump) > 0 {
		sortCardsByRank(nonTrump)
		return []Card{nonTrump[0]}
	}

	// Lead with lowest trump (or any trump if that's all we have)
	if len(trumpCards) > 0 {
		sortCardsByTrumpOrder(trumpCards, trumpSuit, level)
		return []Card{trumpCards[len(trumpCards)-1]}
	}

	// Fallback: just play the first card
	if len(hand) > 0 {
		return []Card{hand[0]}
	}
	return nil
}

// aiFollow selects cards when AI is following
func aiFollow(player *Player, trick *Trick, trumpSuit Suit, level Rank, game *Game) []Card {
	leadCards := trick.LeadCards()
	leadSuit := trick.LeadSuit()

	// Check if we have cards of the lead suit
	hasLeadSuit := player.HasSuit(leadSuit, trumpSuit, level)
	leadSuitCards := player.GetCardsOfSuit(leadSuit, trumpSuit, level)

	if hasLeadSuit && len(leadSuitCards) > 0 {
		// Must follow suit
		return aiFollowSuit(player, trick, leadSuit, leadCards, trumpSuit, level)
	}

	// No lead suit cards - consider killing with trump or just discarding
	return aiKillOrDiscard(player, trick, leadCards, trumpSuit, level, game)
}

// aiFollowSuit handles following in the lead suit
func aiFollowSuit(player *Player, trick *Trick, leadSuit Suit, leadCards []Card, trumpSuit Suit, level Rank) []Card {
	leadGroups := AnalyzePlay(leadCards, trumpSuit, level)
	suitCards := player.GetCardsOfSuit(leadSuit, trumpSuit, level)
	needCount := len(leadCards)

	// If lead is a single card
	if needCount == 1 {
		sortCardsByRank(suitCards)
		// Try to play a card that beats the lead
		leadCard := leadCards[0]
		for _, c := range suitCards {
			if CompareCards(c, leadCard, trumpSuit, level, leadSuit) > 0 {
				return []Card{c}
			}
		}
		// Can't beat it, play smallest
		return []Card{suitCards[0]}
	}

	// If lead is a pair
	if len(leadGroups) == 1 && leadGroups[0].IsPair {
		pairs := findPairsInCards(suitCards, trumpSuit, level)
		if len(pairs) > 0 {
			return pairs[0]
		}
		// No pair available, play two smallest cards of the suit
		sortCardsByRank(suitCards)
		count := min(2, len(suitCards))
		return suitCards[:count]
	}

	// If lead is a tractor
	for _, g := range leadGroups {
		if g.IsTractor {
			tractors := findTractorsInCards(suitCards, trumpSuit, level)
			if len(tractors) > 0 {
				return tractors[0]
			}
			// No tractor, try pairs
			pairs := findPairsInCards(suitCards, trumpSuit, level)
			if len(pairs) > 0 {
				result := pairs[0]
				// Need more cards to match the count
				for len(result) < needCount && len(pairs) > 1 {
					pairs = pairs[1:]
					result = append(result, pairs[0]...)
				}
				if len(result) > needCount {
					result = result[:needCount]
				}
				return result
			}
			// Play smallest cards matching the count
			sortCardsByRank(suitCards)
			count := min(needCount, len(suitCards))
			return suitCards[:count]
		}
	}

	// Mixed lead (singles + pairs): try to match the structure
	result := aiMatchStructure(suitCards, leadGroups, trumpSuit, level)
	if len(result) > 0 {
		return result
	}

	// Fallback: just play smallest cards of the suit
	sortCardsByRank(suitCards)
	count := min(needCount, len(suitCards))
	return suitCards[:count]
}

// aiMatchStructure tries to match the lead structure with available cards
func aiMatchStructure(cards []Card, leadGroups []CardGroup, trumpSuit Suit, level Rank) []Card {
	var result []Card
	remaining := make([]Card, len(cards))
	copy(remaining, cards)
	sortCardsByRank(remaining)

	for _, lg := range leadGroups {
		if lg.IsTractor {
			// Try to find a tractor
			tractors := findTractorsInCards(remaining, trumpSuit, level)
			if len(tractors) > 0 {
				result = append(result, tractors[0]...)
				remaining = removeCards(remaining, tractors[0])
				continue
			}
			// Try pairs
			pairs := findPairsInCards(remaining, trumpSuit, level)
			needPairs := len(lg.Cards) / 2
			for i := 0; i < min(needPairs, len(pairs)); i++ {
				result = append(result, pairs[i]...)
				remaining = removeCards(remaining, pairs[i])
			}
		} else if lg.IsPair {
			pairs := findPairsInCards(remaining, trumpSuit, level)
			if len(pairs) > 0 {
				result = append(result, pairs[0]...)
				remaining = removeCards(remaining, pairs[0])
			} else {
				// No pairs, play singles
				count := min(2, len(remaining))
				result = append(result, remaining[:count]...)
				remaining = remaining[count:]
			}
		} else {
			// Single
			if len(remaining) > 0 {
				result = append(result, remaining[0])
				remaining = remaining[1:]
			}
		}
	}

	if len(result) == 0 && len(cards) > 0 {
		return cards[:min(len(leadGroups), len(cards))]
	}

	return result
}

// aiKillOrDiscard handles playing when we don't have the lead suit
func aiKillOrDiscard(player *Player, trick *Trick, leadCards []Card, trumpSuit Suit, level Rank, game *Game) []Card {
	trumpCards := filterCards(player.Hand, func(c Card) bool {
		return IsTrump(c, trumpSuit, level)
	})

	// Check if there are points in the trick
	trickPoints := trick.Points()
	currentWinner := trick.Winner()
	partnerWinning := PlayerTeam(currentWinner) == PlayerTeam(player.Position)

	// If partner is winning and there are few/no points, don't waste trump
	if partnerWinning && trickPoints < 10 {
		// Discard lowest non-trump cards
		nonTrump := filterCards(player.Hand, func(c Card) bool {
			return !IsTrump(c, trumpSuit, level)
		})
		needCount := len(leadCards)
		if len(nonTrump) >= needCount {
			sortCardsByRank(nonTrump)
			return nonTrump[:needCount]
		}
		// Not enough non-trump, fill with lowest trump
		sortCardsByRank(nonTrump)
		sortCardsByTrumpOrder(trumpCards, trumpSuit, level)
		result := make([]Card, len(nonTrump))
		copy(result, nonTrump)
		for i := len(trumpCards) - 1; i >= 0 && len(result) < needCount; i-- {
			result = append(result, trumpCards[i])
		}
		return result
	}

	// Consider killing with trump if there are points
	if len(trumpCards) > 0 && (trickPoints >= 10 || !partnerWinning) {
		// Try to kill - need to match structure
		leadGroups := AnalyzePlay(leadCards, trumpSuit, level)
		leadPairCount := 0
		leadTractorCount := 0
		for _, g := range leadGroups {
			if g.IsTractor {
				leadTractorCount++
				leadPairCount += len(g.Cards) / 2
			} else if g.IsPair {
				leadPairCount++
			}
		}

		// Check if we can form a valid kill
		killCards := aiFormKill(trumpCards, leadPairCount, leadTractorCount, trumpSuit, level)
		if killCards != nil {
			return killCards
		}
	}

	// Discard: play lowest cards
	hand := player.Hand
	sortCardsByRank(hand)
	count := min(len(leadCards), len(hand))
	return hand[:count]
}

// aiFormKill tries to form a valid kill play from trump cards
func aiFormKill(trumpCards []Card, needPairs, needTractors int, trumpSuit Suit, level Rank) []Card {
	if len(trumpCards) == 0 {
		return nil
	}

	// Simple case: single card lead
	if needPairs == 0 && needTractors == 0 {
		sortCardsByTrumpOrder(trumpCards, trumpSuit, level)
		return []Card{trumpCards[len(trumpCards)-1]} // Play strongest trump
	}

	// Need pairs
	pairs := findPairsInCards(trumpCards, trumpSuit, level)

	if needTractors > 0 {
		// Need tractors
		tractors := findTractorsInCards(trumpCards, trumpSuit, level)
		if len(tractors) > 0 && needPairs <= len(pairs)+len(tractors)*2 {
			return tractors[0]
		}
		// Can't form tractor but can use pairs
		if len(pairs) >= needPairs {
			var result []Card
			for i := 0; i < needPairs; i++ {
				result = append(result, pairs[i]...)
			}
			return result
		}
		return nil
	}

	// Just need pairs
	if len(pairs) >= needPairs {
		var result []Card
		for i := 0; i < needPairs; i++ {
			result = append(result, pairs[i]...)
		}
		return result
	}

	// Not enough pairs, play singles with trump
	sortCardsByTrumpOrder(trumpCards, trumpSuit, level)
	return []Card{trumpCards[len(trumpCards)-1]}
}

// Helper functions

func filterCards(cards []Card, pred func(Card) bool) []Card {
	var result []Card
	for _, c := range cards {
		if pred(c) {
			result = append(result, c)
		}
	}
	return result
}

func removeCard(cards []Card, target Card) []Card {
	for i, c := range cards {
		if c == target {
			return append(cards[:i], cards[i+1:]...)
		}
	}
	return cards
}

func removeCards(cards []Card, targets []Card) []Card {
	toRemove := make(map[Card]bool)
	for _, t := range targets {
		toRemove[t] = true
	}
	var result []Card
	for _, c := range cards {
		if !toRemove[c] {
			result = append(result, c)
		} else {
			delete(toRemove, c)
		}
	}
	return result
}

func sortCardsByRank(cards []Card) {
	rand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] }) // shuffle first for randomness
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].Rank > cards[j].Rank { // ascending (smallest first)
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

func sortCardsByTrumpOrder(cards []Card, trumpSuit Suit, level Rank) {
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			if TrumpOrder(cards[i], trumpSuit, level) > TrumpOrder(cards[j], trumpSuit, level) {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

// findPairsInHand finds all pairs in the player's hand
func findPairsInHand(hand []Card, trumpSuit Suit, level Rank, trumpOnly bool) [][]Card {
	return findPairsInCards(hand, trumpSuit, level)
}

// findPairsInCards finds all pairs in a set of cards
func findPairsInCards(cards []Card, trumpSuit Suit, level Rank) [][]Card {
	// Group by effective suit and rank
	type key struct {
		suit Suit
		rank Rank
	}
	count := make(map[key]int)
	cardMap := make(map[key][]Card)

	for _, c := range cards {
		s := EffectiveSuit(c, trumpSuit, level)
		k := key{s, c.Rank}
		count[k]++
		cardMap[k] = append(cardMap[k], c)
	}

	var pairs [][]Card
	for k, c := range count {
		if c >= 2 {
			p := make([]Card, 2)
			copy(p, cardMap[k][:2])
			pairs = append(pairs, p)
		}
	}

	return pairs
}

// findTractorsInCards finds all tractors in a set of cards
func findTractorsInCards(cards []Card, trumpSuit Suit, level Rank) [][]Card {
	pairs := findPairsInCards(cards, trumpSuit, level)
	if len(pairs) < 2 {
		return nil
	}

	// Get pair ranks grouped by effective suit
	type suitRank struct {
		suit Suit
		rank Rank
	}
	pairRanksBySuit := make(map[Suit][]Rank)
	for _, p := range pairs {
		s := EffectiveSuit(p[0], trumpSuit, level)
		pairRanksBySuit[s] = append(pairRanksBySuit[s], p[0].Rank)
	}

	var tractors [][]Card
	for _, ranks := range pairRanksBySuit {
		sortRanks(ranks, trumpSuit, level)

		// Find consecutive ranks
		current := []Rank{ranks[0]}
		for i := 1; i < len(ranks); i++ {
			if areConsecutiveRanks(current[len(current)-1], ranks[i], trumpSuit, level) {
				current = append(current, ranks[i])
			} else {
				if len(current) >= 2 {
					tractors = append(tractors, buildTractor(current, cards, trumpSuit, level))
				}
				current = []Rank{ranks[i]}
			}
		}
		if len(current) >= 2 {
			tractors = append(tractors, buildTractor(current, cards, trumpSuit, level))
		}
	}

	return tractors
}

func buildTractor(ranks []Rank, cards []Card, trumpSuit Suit, level Rank) []Card {
	var result []Card
	for _, r := range ranks {
		count := 0
		for _, c := range cards {
			if c.Rank == r && EffectiveSuit(c, trumpSuit, level) == EffectiveSuit(cards[0], trumpSuit, level) && count < 2 {
				result = append(result, c)
				count++
			}
		}
	}
	return result
}
